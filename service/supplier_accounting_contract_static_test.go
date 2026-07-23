package service

import (
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	supplierWriterDynamic         = "dynamic"
	supplierWriterUnsupportedPath = "unsupported_path"
	supplierWriterRefund          = "refund"
)

type supplierLogWriterExpectation struct {
	Classification    string
	RequireConsumeIf  bool
	ControlFlowAnchor string
}

type supplierLogWriterCallSite struct {
	File              string
	Function          string
	Writer            string
	Ordinal           int
	Line              int
	ControlFlowAnchor string
	functionDecl      *ast.FuncDecl
	call              *ast.CallExpr
	modelAliases      map[string]struct{}
}

func (s supplierLogWriterCallSite) identityKey() string {
	return fmt.Sprintf("%s|%s|%s|%d", s.File, s.Function, s.Writer, s.Ordinal)
}

func (s supplierLogWriterCallSite) stableKey() string {
	return fmt.Sprintf("%s|flow=%s", s.identityKey(), s.ControlFlowAnchor)
}

func TestSupplierAccountingProductionConsumeWritersInjectExactlyOnce(t *testing.T) {
	expected := map[string]supplierLogWriterExpectation{
		"controller/channel-test.go|testChannelWithOptions|RecordConsumeLog|0": {
			Classification:    supplierWriterUnsupportedPath,
			ControlFlowAnchor: `body/if(!options.SkipLog).then`,
		},
		"controller/midjourney.go|UpdateMidjourneyTaskBulk|RecordTaskBillingLog|0": {
			Classification:    supplierWriterRefund,
			ControlFlowAnchor: `body/for().body/range(taskChannelM).body/range(responseItems).body/if(err != nil).else/if(won && shouldReturnQuota).then`,
		},
		"relay/mjproxy_handler.go|RelaySwapFace|RecordConsumeLog|0": {
			Classification:    supplierWriterUnsupportedPath,
			ControlFlowAnchor: `body/defer#0/func-lit#0/if(mjResp.StatusCode == 200 && mjResp.Response.Code == 1).then`,
		},
		"relay/mjproxy_handler.go|RelayMidjourneySubmit|RecordConsumeLog|0": {
			Classification:    supplierWriterUnsupportedPath,
			ControlFlowAnchor: `body/defer#0/func-lit#0/if(consumeQuota && midjResponseWithStatus.StatusCode == 200).then`,
		},
		"service/quota.go|PostWssConsumeQuota|RecordConsumeLog|0": {
			Classification:    supplierWriterDynamic,
			ControlFlowAnchor: `body`,
		},
		"service/quota.go|PostAudioConsumeQuota|RecordConsumeLog|0": {
			Classification:    supplierWriterDynamic,
			ControlFlowAnchor: `body`,
		},
		"service/task_billing.go|LogTaskConsumption|RecordConsumeLog|0": {
			Classification:    supplierWriterUnsupportedPath,
			ControlFlowAnchor: `body`,
		},
		"service/task_billing.go|RefundTaskQuota|RecordTaskBillingLog|0": {
			Classification:    supplierWriterRefund,
			ControlFlowAnchor: `body`,
		},
		"service/task_billing.go|RecalculateTaskQuota|RecordTaskBillingLog|0": {
			Classification:    supplierWriterUnsupportedPath,
			RequireConsumeIf:  true,
			ControlFlowAnchor: `body`,
		},
		"service/text_quota.go|PostTextConsumeQuota|RecordConsumeLog|0": {
			Classification:    supplierWriterDynamic,
			ControlFlowAnchor: `body`,
		},
		"service/violation_fee.go|ChargeViolationFeeIfNeeded|RecordConsumeLog|0": {
			Classification:    supplierWriterUnsupportedPath,
			ControlFlowAnchor: `body`,
		},
	}

	discovered := discoverSupplierLogWriterCallSites(t)
	require.Equal(t, sortedSupplierWriterKeys(expected), sortedSupplierCallSiteKeys(discovered),
		"production consume/refund log writers were added, removed, or moved across a control-flow boundary; characterize every writer explicitly")

	for _, site := range discovered {
		expectation, ok := expected[site.identityKey()]
		if !ok {
			continue
		}
		t.Run(strings.ReplaceAll(site.stableKey(), "/", "_"), func(t *testing.T) {
			params := supplierWriterParamsLiteral(t, site)
			if params == nil {
				return
			}
			if expectation.Classification == supplierWriterRefund {
				requireSupplierRefundWriterContract(t, site, params)
				return
			}
			requireSupplierConsumeWriterContract(t, site, params, expectation)
		})
	}
}

func TestSupplierConsumeWriterASTDominanceMutations(t *testing.T) {
	tests := []struct {
		name             string
		body             string
		writer           string
		expectation      supplierLogWriterExpectation
		wantErrorContain string
	}{
		{
			name: "valid same block",
			body: `
				InjectUnsupportedSupplierAccountingEnvelopeV1(other)
				noop()
				model.RecordConsumeLog(nil, 1, model.RecordConsumeLogParams{Other: other})
			`,
			writer:      "RecordConsumeLog",
			expectation: supplierLogWriterExpectation{Classification: supplierWriterUnsupportedPath},
		},
		{
			name: "if false does not dominate",
			body: `
				if false {
					InjectUnsupportedSupplierAccountingEnvelopeV1(other)
				}
				model.RecordConsumeLog(nil, 1, model.RecordConsumeLogParams{Other: other})
			`,
			writer:           "RecordConsumeLog",
			expectation:      supplierLogWriterExpectation{Classification: supplierWriterUnsupportedPath},
			wantErrorContain: "same executable block",
		},
		{
			name: "sibling branch does not dominate",
			body: `
				if cond {
					InjectUnsupportedSupplierAccountingEnvelopeV1(other)
				} else {
					model.RecordConsumeLog(nil, 1, model.RecordConsumeLogParams{Other: other})
				}
			`,
			writer:           "RecordConsumeLog",
			expectation:      supplierLogWriterExpectation{Classification: supplierWriterUnsupportedPath},
			wantErrorContain: "same executable block",
		},
		{
			name: "injection after writer",
			body: `
				model.RecordConsumeLog(nil, 1, model.RecordConsumeLogParams{Other: other})
				InjectUnsupportedSupplierAccountingEnvelopeV1(other)
			`,
			writer:           "RecordConsumeLog",
			expectation:      supplierLogWriterExpectation{Classification: supplierWriterUnsupportedPath},
			wantErrorContain: "before the durable writer",
		},
		{
			name: "wrong Other map",
			body: `
				InjectUnsupportedSupplierAccountingEnvelopeV1(wrong)
				model.RecordConsumeLog(nil, 1, model.RecordConsumeLogParams{Other: other})
			`,
			writer:           "RecordConsumeLog",
			expectation:      supplierLogWriterExpectation{Classification: supplierWriterUnsupportedPath},
			wantErrorContain: "exactly one supplier envelope",
		},
		{
			name: "valid consume-only task guard",
			body: `
				if logType == model.LogTypeConsume {
					InjectUnsupportedSupplierAccountingEnvelopeV1(other)
				}
				model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{LogType: logType, Other: other})
			`,
			writer: "RecordTaskBillingLog",
			expectation: supplierLogWriterExpectation{
				Classification:   supplierWriterUnsupportedPath,
				RequireConsumeIf: true,
			},
		},
		{
			name: "consume guard must be adjacent",
			body: `
				if logType == model.LogTypeConsume {
					InjectUnsupportedSupplierAccountingEnvelopeV1(other)
				}
				noop()
				model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{LogType: logType, Other: other})
			`,
			writer: "RecordTaskBillingLog",
			expectation: supplierLogWriterExpectation{
				Classification:   supplierWriterUnsupportedPath,
				RequireConsumeIf: true,
			},
			wantErrorContain: "immediately precede",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			site := supplierSyntheticWriterSite(t, test.body, test.writer)
			params := supplierWriterParamsLiteral(t, site)
			require.NotNil(t, params)
			err := supplierConsumeWriterContractError(site, params, test.expectation)
			if test.wantErrorContain == "" {
				require.NoError(t, err)
				return
			}
			require.ErrorContains(t, err, test.wantErrorContain)
		})
	}
}

func TestSupplierWriterStableKeyDetectsControlFlowMoves(t *testing.T) {
	directBody := `
		InjectUnsupportedSupplierAccountingEnvelopeV1(other)
		model.RecordConsumeLog(nil, 1, model.RecordConsumeLogParams{Other: other})
	`
	direct := supplierSyntheticWriterSite(t, directBody, "RecordConsumeLog")
	directParams := supplierWriterParamsLiteral(t, direct)
	require.NoError(t, supplierConsumeWriterContractError(direct, directParams, supplierLogWriterExpectation{Classification: supplierWriterUnsupportedPath}))

	movedBodies := map[string]string{
		"if branch": `
			if cond {
				noop()
			} else {
				InjectUnsupportedSupplierAccountingEnvelopeV1(other)
				model.RecordConsumeLog(nil, 1, model.RecordConsumeLogParams{Other: other})
			}
		`,
		"defer closure": `
			defer func() {
				InjectUnsupportedSupplierAccountingEnvelopeV1(other)
				model.RecordConsumeLog(nil, 1, model.RecordConsumeLogParams{Other: other})
			}()
		`,
		"function literal": `
			func() {
				InjectUnsupportedSupplierAccountingEnvelopeV1(other)
				model.RecordConsumeLog(nil, 1, model.RecordConsumeLogParams{Other: other})
			}()
		`,
		"switch case": `
			switch {
			case cond:
				{
					InjectUnsupportedSupplierAccountingEnvelopeV1(other)
					model.RecordConsumeLog(nil, 1, model.RecordConsumeLogParams{Other: other})
				}
			}
		`,
		"loop body": `
			for cond {
				InjectUnsupportedSupplierAccountingEnvelopeV1(other)
				model.RecordConsumeLog(nil, 1, model.RecordConsumeLogParams{Other: other})
			}
		`,
	}

	for name, body := range movedBodies {
		t.Run(name, func(t *testing.T) {
			moved := supplierSyntheticWriterSite(t, body, "RecordConsumeLog")
			params := supplierWriterParamsLiteral(t, moved)
			require.NoError(t, supplierConsumeWriterContractError(moved, params, supplierLogWriterExpectation{Classification: supplierWriterUnsupportedPath}),
				"the injection still dominates after this move; the stable allow-list must catch the structural relocation")
			require.Equal(t, direct.identityKey(), moved.identityKey())
			require.NotEqual(t, direct.ControlFlowAnchor, moved.ControlFlowAnchor,
				"control-flow ancestry must be part of the stable writer identity")
			require.NotEqual(t, direct.stableKey(), moved.stableKey())
		})
	}
}

func TestSupplierRefundWriterRejectsEnvelopeInjection(t *testing.T) {
	valid := supplierSyntheticWriterSite(t, `
		model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
			LogType: model.LogTypeRefund,
			Other: other,
		})
	`, "RecordTaskBillingLog")
	require.NoError(t, supplierRefundWriterContractError(valid, supplierWriterParamsLiteral(t, valid)))

	invalid := supplierSyntheticWriterSite(t, `
		InjectUnsupportedSupplierAccountingEnvelopeV1(other)
		model.RecordTaskBillingLog(model.RecordTaskBillingLogParams{
			LogType: model.LogTypeRefund,
			Other: other,
		})
	`, "RecordTaskBillingLog")
	require.ErrorContains(t, supplierRefundWriterContractError(invalid, supplierWriterParamsLiteral(t, invalid)), "refund Other map")
}

func discoverSupplierLogWriterCallSites(t *testing.T) []supplierLogWriterCallSite {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	repositoryRoot := filepath.Dir(filepath.Dir(currentFile))
	fset := token.NewFileSet()
	var sites []supplierLogWriterCallSite

	err := filepath.WalkDir(repositoryRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			switch entry.Name() {
			case ".git", ".omx", "node_modules", "vendor":
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		parsed, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return parseErr
		}
		relativePath, relativeErr := filepath.Rel(repositoryRoot, path)
		if relativeErr != nil {
			return relativeErr
		}
		sites = append(sites, supplierLogWriterCallSitesInFile(fset, parsed, filepath.ToSlash(relativePath))...)
		return nil
	})
	require.NoError(t, err)
	sort.Slice(sites, func(i, j int) bool { return sites[i].stableKey() < sites[j].stableKey() })
	return sites
}

func supplierLogWriterCallSitesInFile(fset *token.FileSet, parsed *ast.File, relativePath string) []supplierLogWriterCallSite {
	modelAliases := supplierModelImportAliases(parsed)
	if len(modelAliases) == 0 {
		return nil
	}
	var sites []supplierLogWriterCallSite
	for _, declaration := range parsed.Decls {
		function, isFunction := declaration.(*ast.FuncDecl)
		if !isFunction || function.Body == nil {
			continue
		}
		ordinals := make(map[string]int)
		ast.Inspect(function.Body, func(node ast.Node) bool {
			call, isCall := node.(*ast.CallExpr)
			if !isCall {
				return true
			}
			writer := supplierModelLogWriterName(call, modelAliases)
			if writer == "" {
				return true
			}
			ordinal := ordinals[writer]
			ordinals[writer]++
			sites = append(sites, supplierLogWriterCallSite{
				File:              relativePath,
				Function:          function.Name.Name,
				Writer:            writer,
				Ordinal:           ordinal,
				Line:              fset.Position(call.Pos()).Line,
				ControlFlowAnchor: supplierControlFlowAnchor(function, call),
				functionDecl:      function,
				call:              call,
				modelAliases:      modelAliases,
			})
			return true
		})
	}
	return sites
}

func supplierSyntheticWriterSite(t *testing.T, body string, writer string) supplierLogWriterCallSite {
	t.Helper()
	source := fmt.Sprintf(`package service

		import model "github.com/QuantumNous/new-api/model"

		func fixture(cond bool, logType int, other, wrong map[string]interface{}) {
			%s
		}
	`, body)
	fset := token.NewFileSet()
	parsed, err := parser.ParseFile(fset, "fixture.go", source, parser.SkipObjectResolution)
	require.NoError(t, err)
	sites := supplierLogWriterCallSitesInFile(fset, parsed, "fixture.go")
	var matching []supplierLogWriterCallSite
	for _, site := range sites {
		if site.Writer == writer {
			matching = append(matching, site)
		}
	}
	require.Len(t, matching, 1)
	return matching[0]
}

func supplierModelImportAliases(file *ast.File) map[string]struct{} {
	aliases := make(map[string]struct{})
	for _, imported := range file.Imports {
		importPath, err := strconv.Unquote(imported.Path.Value)
		if err != nil || importPath != "github.com/QuantumNous/new-api/model" {
			continue
		}
		alias := "model"
		if imported.Name != nil {
			alias = imported.Name.Name
		}
		if alias != "." && alias != "_" {
			aliases[alias] = struct{}{}
		}
	}
	return aliases
}

func supplierModelLogWriterName(call *ast.CallExpr, modelAliases map[string]struct{}) string {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || (selector.Sel.Name != "RecordConsumeLog" && selector.Sel.Name != "RecordTaskBillingLog") {
		return ""
	}
	identifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return ""
	}
	if _, isModel := modelAliases[identifier.Name]; !isModel {
		return ""
	}
	return selector.Sel.Name
}

func supplierWriterParamsLiteral(t *testing.T, site supplierLogWriterCallSite) *ast.CompositeLit {
	t.Helper()
	argumentIndex := 0
	if site.Writer == "RecordConsumeLog" {
		argumentIndex = 2
	}
	if len(site.call.Args) <= argumentIndex {
		t.Errorf("%s:%d %s is missing its params argument", site.File, site.Line, site.Writer)
		return nil
	}
	literal, ok := site.call.Args[argumentIndex].(*ast.CompositeLit)
	if !ok {
		t.Errorf("%s:%d %s params must remain an inline composite literal for static contract inspection", site.File, site.Line, site.Writer)
		return nil
	}
	return literal
}

func requireSupplierConsumeWriterContract(t *testing.T, site supplierLogWriterCallSite, params *ast.CompositeLit, expectation supplierLogWriterExpectation) {
	t.Helper()
	require.NoError(t, supplierConsumeWriterContractError(site, params, expectation), "%s:%d", site.File, site.Line)
}

func supplierConsumeWriterContractError(site supplierLogWriterCallSite, params *ast.CompositeLit, expectation supplierLogWriterExpectation) error {
	other := supplierCompositeField(params, "Other")
	otherIdentifier, ok := other.(*ast.Ident)
	if !ok {
		return fmt.Errorf("consume writer Other must be a named map shared with its supplier envelope injection")
	}

	injections := supplierEnvelopeInjectionsForMap(site.functionDecl, otherIdentifier.Name)
	if len(injections) != 1 {
		return fmt.Errorf("consume writer must inject exactly one supplier envelope into the same Other map; found %d", len(injections))
	}
	expectedInjection := "InjectSupplierAccountingEnvelopeV1"
	if expectation.Classification == supplierWriterUnsupportedPath {
		expectedInjection = "InjectUnsupportedSupplierAccountingEnvelopeV1"
	}
	if actual := supplierCallName(injections[0]); actual != expectedInjection {
		return fmt.Errorf("supplier envelope disposition mismatch: got %s, want %s", actual, expectedInjection)
	}

	if site.Writer == "RecordTaskBillingLog" {
		logType := supplierCompositeField(params, "LogType")
		if !supplierIsIdentifier(logType, "logType") {
			return fmt.Errorf("conditional task writer must persist the branch-selected logType")
		}
	}
	if expectation.RequireConsumeIf {
		return supplierRequireConsumeGuardDominance(site, injections[0])
	}
	return supplierRequireSameBlockDominance(site, injections[0])
}

func supplierRequireSameBlockDominance(site supplierLogWriterCallSite, injection *ast.CallExpr) error {
	injectionLocation, ok := supplierDirectCallStatementLocation(site.functionDecl, injection)
	if !ok {
		return fmt.Errorf("supplier envelope injection must be a direct call statement in an executable block")
	}
	writerLocation, ok := supplierDirectCallStatementLocation(site.functionDecl, site.call)
	if !ok {
		return fmt.Errorf("durable writer must be a direct call statement in an executable block")
	}
	if injectionLocation.Block != writerLocation.Block {
		return fmt.Errorf("supplier envelope injection and durable writer must be in the same executable block")
	}
	if injectionLocation.Index >= writerLocation.Index {
		return fmt.Errorf("supplier envelope injection must execute before the durable writer")
	}
	return nil
}

func supplierRequireConsumeGuardDominance(site supplierLogWriterCallSite, injection *ast.CallExpr) error {
	injectionLocation, ok := supplierDirectCallStatementLocation(site.functionDecl, injection)
	if !ok {
		return fmt.Errorf("task consume injection must be a direct call statement in the consume guard body")
	}
	writerLocation, ok := supplierDirectCallStatementLocation(site.functionDecl, site.call)
	if !ok {
		return fmt.Errorf("conditional task writer must be a direct call statement in an executable block")
	}

	var guard *ast.IfStmt
	for _, node := range supplierNodeAncestry(site.functionDecl.Body, injection) {
		if candidate, isIf := node.(*ast.IfStmt); isIf && candidate.Body == injectionLocation.Block {
			guard = candidate
		}
	}
	if guard == nil {
		return fmt.Errorf("task consume injection must be directly inside an if logType == model.LogTypeConsume body")
	}
	if guard.Init != nil || guard.Else != nil || !supplierIsConsumeLogTypeCondition(guard.Cond, site.modelAliases) {
		return fmt.Errorf("task consume injection guard must be exactly if logType == model.LogTypeConsume")
	}
	guardIndex := supplierStatementIndex(writerLocation.Block, guard)
	if guardIndex < 0 {
		return fmt.Errorf("task consume guard and durable writer must have the same parent block")
	}
	if guardIndex != writerLocation.Index-1 {
		return fmt.Errorf("task consume guard must immediately precede the durable writer")
	}
	return nil
}

func requireSupplierRefundWriterContract(t *testing.T, site supplierLogWriterCallSite, params *ast.CompositeLit) {
	t.Helper()
	require.NoError(t, supplierRefundWriterContractError(site, params), "%s:%d", site.File, site.Line)
}

func supplierRefundWriterContractError(site supplierLogWriterCallSite, params *ast.CompositeLit) error {
	if site.Writer != "RecordTaskBillingLog" {
		return fmt.Errorf("only task billing writers may be characterized as refunds")
	}
	logType := supplierCompositeField(params, "LogType")
	if !supplierIsModelSelector(logType, site.modelAliases, "LogTypeRefund") {
		return fmt.Errorf("refund characterization requires an explicit model.LogTypeRefund")
	}
	other := supplierCompositeField(params, "Other")
	if otherIdentifier, ok := other.(*ast.Ident); ok {
		if injections := supplierEnvelopeInjectionsForMap(site.functionDecl, otherIdentifier.Name); len(injections) != 0 {
			return fmt.Errorf("refund Other map must not receive a supplier consume envelope; found %d injection(s)", len(injections))
		}
	}
	return nil
}

func supplierCompositeField(literal *ast.CompositeLit, field string) ast.Expr {
	if literal == nil {
		return nil
	}
	for _, element := range literal.Elts {
		keyValue, ok := element.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		key, ok := keyValue.Key.(*ast.Ident)
		if ok && key.Name == field {
			return keyValue.Value
		}
	}
	return nil
}

type supplierCallStatementLocation struct {
	Block     *ast.BlockStmt
	Statement *ast.ExprStmt
	Index     int
}

func supplierDirectCallStatementLocation(function *ast.FuncDecl, target *ast.CallExpr) (supplierCallStatementLocation, bool) {
	ancestry := supplierNodeAncestry(function.Body, target)
	for nodeIndex := len(ancestry) - 1; nodeIndex >= 0; nodeIndex-- {
		expressionStatement, ok := ancestry[nodeIndex].(*ast.ExprStmt)
		if !ok || expressionStatement.X != target {
			continue
		}
		for parentIndex := nodeIndex - 1; parentIndex >= 0; parentIndex-- {
			block, isBlock := ancestry[parentIndex].(*ast.BlockStmt)
			if !isBlock {
				continue
			}
			statementIndex := supplierStatementIndex(block, expressionStatement)
			if statementIndex >= 0 {
				return supplierCallStatementLocation{Block: block, Statement: expressionStatement, Index: statementIndex}, true
			}
		}
		return supplierCallStatementLocation{}, false
	}
	return supplierCallStatementLocation{}, false
}

func supplierStatementIndex(block *ast.BlockStmt, target ast.Stmt) int {
	if block == nil {
		return -1
	}
	for index, statement := range block.List {
		if statement == target {
			return index
		}
	}
	return -1
}

func supplierEnvelopeInjectionsForMap(function *ast.FuncDecl, mapName string) []*ast.CallExpr {
	var injections []*ast.CallExpr
	ast.Inspect(function.Body, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := supplierCallName(call)
		if name != "InjectSupplierAccountingEnvelopeV1" && name != "InjectUnsupportedSupplierAccountingEnvelopeV1" {
			return true
		}
		if len(call.Args) > 0 && supplierIsIdentifier(call.Args[0], mapName) {
			injections = append(injections, call)
		}
		return true
	})
	return injections
}

func supplierCallName(call *ast.CallExpr) string {
	switch function := call.Fun.(type) {
	case *ast.Ident:
		return function.Name
	case *ast.SelectorExpr:
		return function.Sel.Name
	default:
		return ""
	}
}

func supplierIsConsumeLogTypeCondition(expression ast.Expr, modelAliases map[string]struct{}) bool {
	condition, ok := expression.(*ast.BinaryExpr)
	if !ok || condition.Op != token.EQL {
		return false
	}
	return (supplierIsIdentifier(condition.X, "logType") && supplierIsModelSelector(condition.Y, modelAliases, "LogTypeConsume")) ||
		(supplierIsIdentifier(condition.Y, "logType") && supplierIsModelSelector(condition.X, modelAliases, "LogTypeConsume"))
}

func supplierIsIdentifier(expression ast.Expr, name string) bool {
	identifier, ok := expression.(*ast.Ident)
	return ok && identifier.Name == name
}

func supplierIsModelSelector(expression ast.Expr, modelAliases map[string]struct{}, selectorName string) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != selectorName {
		return false
	}
	identifier, ok := selector.X.(*ast.Ident)
	if !ok {
		return false
	}
	_, isModel := modelAliases[identifier.Name]
	return isModel
}

func supplierControlFlowAnchor(function *ast.FuncDecl, target ast.Node) string {
	ancestry := supplierNodeAncestry(function.Body, target)
	if len(ancestry) == 0 {
		return "missing"
	}
	segments := []string{"body"}
	for index, node := range ancestry {
		switch typed := node.(type) {
		case *ast.IfStmt:
			branch := "header"
			if supplierNodeContains(typed.Body, target) {
				branch = "then"
			} else if typed.Else != nil && supplierNodeContains(typed.Else, target) {
				branch = "else"
			}
			segments = append(segments, fmt.Sprintf("if(%s).%s", supplierNodeText(typed.Cond), branch))
		case *ast.ForStmt:
			branch := "header"
			if supplierNodeContains(typed.Body, target) {
				branch = "body"
			}
			segments = append(segments, fmt.Sprintf("for(%s).%s", supplierNodeText(typed.Cond), branch))
		case *ast.RangeStmt:
			branch := "header"
			if supplierNodeContains(typed.Body, target) {
				branch = "body"
			}
			segments = append(segments, fmt.Sprintf("range(%s).%s", supplierNodeText(typed.X), branch))
		case *ast.SwitchStmt:
			branch := "header"
			if supplierNodeContains(typed.Body, target) {
				branch = "body"
			}
			segments = append(segments, fmt.Sprintf("switch(%s).%s", supplierNodeText(typed.Tag), branch))
		case *ast.TypeSwitchStmt:
			branch := "header"
			if supplierNodeContains(typed.Body, target) {
				branch = "body"
			}
			segments = append(segments, fmt.Sprintf("type-switch(%s).%s", supplierNodeText(typed.Assign), branch))
		case *ast.SelectStmt:
			segments = append(segments, "select.body")
		case *ast.CaseClause:
			segments = append(segments, "case("+supplierExpressionListText(typed.List, "default")+")")
		case *ast.CommClause:
			segments = append(segments, "comm("+supplierNodeText(typed.Comm)+")")
		case *ast.DeferStmt:
			segments = append(segments, fmt.Sprintf("defer#%d", supplierNodeTypeOrdinal(function.Body, typed)))
		case *ast.GoStmt:
			segments = append(segments, fmt.Sprintf("go#%d", supplierNodeTypeOrdinal(function.Body, typed)))
		case *ast.FuncLit:
			segments = append(segments, fmt.Sprintf("func-lit#%d", supplierNodeTypeOrdinal(function.Body, typed)))
		case *ast.LabeledStmt:
			segments = append(segments, "label("+typed.Label.Name+")")
		case *ast.BlockStmt:
			if typed == function.Body || supplierIsStructuredBodyBlock(ancestry, index, typed) {
				continue
			}
			segments = append(segments, fmt.Sprintf("block#%d", supplierNodeTypeOrdinal(function.Body, typed)))
		}
	}
	return strings.Join(segments, "/")
}

func supplierNodeAncestry(root ast.Node, target ast.Node) []ast.Node {
	var stack []ast.Node
	var ancestry []ast.Node
	ast.Inspect(root, func(node ast.Node) bool {
		if node == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, node)
		if node == target && ancestry == nil {
			ancestry = append([]ast.Node(nil), stack...)
		}
		return true
	})
	return ancestry
}

func supplierNodeContains(container ast.Node, target ast.Node) bool {
	return container != nil && target != nil && target.Pos() >= container.Pos() && target.End() <= container.End()
}

func supplierIsStructuredBodyBlock(ancestry []ast.Node, index int, block *ast.BlockStmt) bool {
	if index <= 0 {
		return false
	}
	switch parent := ancestry[index-1].(type) {
	case *ast.IfStmt:
		return parent.Body == block
	case *ast.ForStmt:
		return parent.Body == block
	case *ast.RangeStmt:
		return parent.Body == block
	case *ast.SwitchStmt:
		return parent.Body == block
	case *ast.TypeSwitchStmt:
		return parent.Body == block
	case *ast.SelectStmt:
		return parent.Body == block
	case *ast.FuncLit:
		return parent.Body == block
	default:
		return false
	}
}

func supplierNodeTypeOrdinal(root ast.Node, target ast.Node) int {
	targetType := fmt.Sprintf("%T", target)
	ordinal := 0
	found := false
	ast.Inspect(root, func(node ast.Node) bool {
		if found || node == nil {
			return !found
		}
		if fmt.Sprintf("%T", node) != targetType {
			return true
		}
		if node == target {
			found = true
			return false
		}
		ordinal++
		return true
	})
	return ordinal
}

func supplierNodeText(node ast.Node) string {
	if node == nil {
		return ""
	}
	var builder strings.Builder
	if err := format.Node(&builder, token.NewFileSet(), node); err != nil {
		return fmt.Sprintf("%T", node)
	}
	return strings.Join(strings.Fields(builder.String()), " ")
}

func supplierExpressionListText(expressions []ast.Expr, empty string) string {
	if len(expressions) == 0 {
		return empty
	}
	parts := make([]string, 0, len(expressions))
	for _, expression := range expressions {
		parts = append(parts, supplierNodeText(expression))
	}
	return strings.Join(parts, ",")
}

func sortedSupplierWriterKeys(expectations map[string]supplierLogWriterExpectation) []string {
	keys := make([]string, 0, len(expectations))
	for identity, expectation := range expectations {
		keys = append(keys, fmt.Sprintf("%s|flow=%s", identity, expectation.ControlFlowAnchor))
	}
	sort.Strings(keys)
	return keys
}

func sortedSupplierCallSiteKeys(sites []supplierLogWriterCallSite) []string {
	keys := make([]string, 0, len(sites))
	for _, site := range sites {
		keys = append(keys, site.stableKey())
	}
	sort.Strings(keys)
	return keys
}
