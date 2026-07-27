package service

import (
	"context"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
)

// ── margin floor ────────────────────────────────────────────────────────────
//
// The floor is the one rule that must never be defeated by a bad catalogue
// number, so it gets the most direct tests.

func TestMarginFloor_NeverSellsBelowTargetMargin(t *testing.T) {
	cases := []struct {
		name  string
		price float64
		cost  float64
		want  float64
	}{
		{"declared price already above the floor", 0.0015, 0.001, 0.0015},
		{"declared price below the floor is lifted", 0.0011, 0.001, 0.00125},
		{"declared price equal to cost is lifted", 0.001, 0.001, 0.00125},
		{"declared price below cost is lifted", 0.0005, 0.001, 0.00125},
		{"no known cost leaves the price alone", 0.002, 0, 0.002},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := marginFloored(tc.price, tc.cost)
			if math.Abs(got-tc.want) > 1e-9 {
				t.Fatalf("marginFloored(%v, %v) = %v, want %v", tc.price, tc.cost, got, tc.want)
			}
			if tc.cost > 0 {
				margin := (got - tc.cost) / got
				if margin < ToolMinMarginRate-1e-9 {
					t.Fatalf("realised margin %.4f is below the %.2f floor", margin, ToolMinMarginRate)
				}
			}
		})
	}
}

func TestPriceUSD_AppliesFloorAndPayOnMatch(t *testing.T) {
	// A per-call tool whose catalogue price would lose money.
	underpriced := &ToolSpec{Pricing: ToolPricing{
		Model: ToolPerCall, Amount: 0.0005, Cost: 0.001, PayOnMatch: true,
	}}
	if got := underpriced.PriceUSD(3); math.Abs(got-0.00125) > 1e-9 {
		t.Fatalf("per_call price = %v, want the 0.00125 floor", got)
	}
	if got := underpriced.PriceUSD(0); got != 0 {
		t.Fatalf("pay-on-match must not charge for an empty result, got %v", got)
	}

	// A per-result tool charges base + floored unit per item.
	perResult := &ToolSpec{Pricing: ToolPricing{
		Model: ToolPerResult, Amount: 0.001, Base: 0.002, Cost: 0.001,
	}}
	// unit floors to 0.00125; 0.002 + 0.00125*4 = 0.007
	if got := perResult.PriceUSD(4); math.Abs(got-0.007) > 1e-9 {
		t.Fatalf("per_result price = %v, want 0.007", got)
	}

	free := &ToolSpec{Pricing: ToolPricing{Model: ToolFree}}
	if got := free.PriceUSD(10); got != 0 {
		t.Fatalf("free tools charge nothing, got %v", got)
	}
}

func TestMaxPriceUSD_IsTheWorstCaseForOneCall(t *testing.T) {
	perResult := &ToolSpec{Pricing: ToolPricing{
		Model: ToolPerResult, Amount: 0.001, Base: 0.002, Cost: 0.001,
	}}
	// Pre-flight must reserve base + one floored unit, never zero.
	if got := perResult.MaxPriceUSD(); math.Abs(got-0.00325) > 1e-9 {
		t.Fatalf("MaxPriceUSD = %v, want 0.00325", got)
	}
	if perResult.MaxPriceUSD() < perResult.PriceUSD(1)-1e-9 {
		t.Fatal("worst case must be at least the price of a single-result call")
	}
}

// ── quota conversion ────────────────────────────────────────────────────────

func TestDataToolQuota_MatchesTokenBillingConversion(t *testing.T) {
	// quota = USD * QuotaPerUnit * groupRatio — identical to the conversion
	// ComputeToolCallQuota performs for in-request tool calls.
	got := DataToolQuota(0.0015, 1)
	want := int(math.Ceil(0.0015 * common.QuotaPerUnit))
	if got != want {
		t.Fatalf("DataToolQuota = %d, want %d", got, want)
	}

	// A discounted group must be discounted on tools too, not only on models.
	half := DataToolQuota(0.0015, 0.5)
	if half >= got {
		t.Fatalf("group ratio 0.5 should reduce the charge: got %d vs %d", half, got)
	}

	if DataToolQuota(0, 1) != 0 {
		t.Fatal("a zero price must cost zero quota")
	}
	// An unset ratio must not silently zero the charge.
	if DataToolQuota(0.0015, 0) != got {
		t.Fatal("a zero group ratio should fall back to 1, not to free")
	}
	// Rounding is up so sub-quota prices are never free.
	if DataToolQuota(0.0000001, 1) != 1 {
		t.Fatal("a tiny but non-zero price must still cost at least 1 quota")
	}
}

// ── result counting ─────────────────────────────────────────────────────────
//
// countToolResults decides whether a pay-on-match tool bills at all, so the
// empty-envelope shapes upstreams actually return are pinned down here.

func TestCountToolResults_EmptyEnvelopesAreZero(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"null", `null`, 0},
		{"empty object", `{}`, 0},
		{"empty array", `[]`, 0},
		{"plain array", `[1,2,3]`, 3},
		{"zero-count envelope", `{"count":0,"results":[]}`, 0},
		{"results array", `{"results":[{"a":1},{"b":2}]}`, 2},
		{"data array", `{"data":[1,2,3,4]}`, 4},
		{"count only", `{"count":0}`, 0},
		{"found false", `{"found":false}`, 0},
		{"status not_found", `{"status":"not_found"}`, 0},
		{"error envelope", `{"error":"nope"}`, 0},
		{"single object", `{"title":"one object"}`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var v any
			if err := common.UnmarshalJsonStr(tc.body, &v); err != nil {
				t.Fatalf("bad fixture: %v", err)
			}
			if got := countToolResults(v); got != tc.want {
				t.Fatalf("countToolResults(%s) = %d, want %d", tc.body, got, tc.want)
			}
		})
	}
}

// ── input validation ────────────────────────────────────────────────────────

func TestValidateToolInput_CoercesAndEnforces(t *testing.T) {
	spec := &ToolSpec{Input: ToolInputSchema{
		Type: "object",
		Properties: map[string]ToolField{
			"asin":  {Type: "string"},
			"count": {Type: "number", Default: float64(5)},
			"sort":  {Type: "string", Enum: []string{"recent", "helpful"}},
			"tags":  {Type: "array"},
			"flag":  {Type: "boolean"},
		},
		Required: []string{"asin"},
	}}

	if _, err := ValidateToolInput(spec, map[string]any{}); err == nil {
		t.Fatal("a missing required field must be rejected before we pay for an upstream call")
	}

	out, err := ValidateToolInput(spec, map[string]any{
		"asin": "B08N5WRWNW", "count": "12", "tags": "a, b ,c", "flag": "true",
	})
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	if out["count"] != float64(12) {
		t.Fatalf("numeric strings should coerce, got %#v", out["count"])
	}
	if arr, ok := out["tags"].([]any); !ok || len(arr) != 3 {
		t.Fatalf("comma strings should split into arrays, got %#v", out["tags"])
	}
	if out["flag"] != true {
		t.Fatalf("boolean strings should coerce, got %#v", out["flag"])
	}

	if _, err := ValidateToolInput(spec, map[string]any{"asin": "x", "sort": "bogus"}); err == nil {
		t.Fatal("a value outside the declared enum must be rejected")
	}

	filled, _ := ValidateToolInput(spec, map[string]any{"asin": "x"})
	if filled["count"] != float64(5) {
		t.Fatalf("default not applied, got %#v", filled["count"])
	}
}

// ── result path ─────────────────────────────────────────────────────────────

func TestPickToolPath_FallsBackWhenPathIsAbsent(t *testing.T) {
	body := map[string]any{"code": float64(200), "data": map[string]any{"x": float64(1)}}

	if got := pickToolPath(body, "data"); got.(map[string]any)["x"] != float64(1) {
		t.Fatal("resultPath should unwrap when it resolves")
	}
	// An upstream that wraps most endpoints in "data" still answers some at the
	// top level; failing those would be a mapping opinion, not a real error.
	if got := pickToolPath(body, "missing"); got.(map[string]any)["code"] != float64(200) {
		t.Fatal("an unresolvable resultPath must fall back to the whole body")
	}
	if got := pickToolPath(body, ""); got.(map[string]any)["code"] != float64(200) {
		t.Fatal("an empty resultPath must pass the body through")
	}
}

// ── templating ──────────────────────────────────────────────────────────────

func TestTemplateToolAny_PreservesScalarTypes(t *testing.T) {
	input := map[string]any{"n": float64(7), "s": "hello"}

	body := map[string]any{"num": "{{n}}", "str": "{{s}}", "mixed": "v={{s}}", "lit": "plain"}
	out := templateToolAny(body, input).(map[string]any)

	// A value that is exactly one placeholder keeps its type, so a number is
	// still a number when it reaches the upstream.
	if out["num"] != float64(7) {
		t.Fatalf("solo placeholder should keep its type, got %#v", out["num"])
	}
	if out["mixed"] != "v=hello" {
		t.Fatalf("interpolation failed, got %#v", out["mixed"])
	}
	if out["lit"] != "plain" {
		t.Fatalf("literals should pass through, got %#v", out["lit"])
	}

	// Unset placeholders must drop out rather than send empty strings upstream.
	partial := templateToolAny(map[string]any{"a": "{{missing}}"}, input).(map[string]any)
	if _, present := partial["a"]; present {
		t.Fatal("an unresolved placeholder should be omitted, not sent empty")
	}
}

// ── HTTP execution ──────────────────────────────────────────────────────────

func TestRunTool_HTTPAdapterHappyPath(t *testing.T) {
	var gotPath, gotAuth, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotAuth, gotQuery = r.URL.Path, r.Header.Get("Authorization"), r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"results":[{"id":1},{"id":2}]}}`))
	}))
	defer srv.Close()

	t.Setenv("TEST_TOOL_KEY", "secret-token")

	spec := &ToolSpec{
		Id: "test.echo", Name: "Echo", Provider: "test:echo", Mode: ToolModeNative,
		Input: ToolInputSchema{Type: "object",
			Properties: map[string]ToolField{"kw": {Type: "string"}, "id": {Type: "string"}},
			Required:   []string{"kw"}},
		Pricing: ToolPricing{Model: ToolPerResult, Amount: 0.001, Cost: 0.001, PayOnMatch: true},
		Adapter: ToolAdapter{
			Kind: "http", Method: "GET",
			URL:        srv.URL + "/api/{{id}}",
			Auth:       &ToolAuth{Type: "bearer", EnvKey: "TEST_TOOL_KEY"},
			Query:      map[string]string{"q": "{{kw}}"},
			ResultPath: "data",
		},
	}

	res := RunTool(context.Background(), spec, map[string]any{"kw": "早C晚A", "id": "42"})
	if !res.OK {
		t.Fatalf("expected success, got %q", res.Error)
	}
	if gotPath != "/api/42" {
		t.Fatalf("path placeholder not substituted: %s", gotPath)
	}
	if gotAuth != "Bearer secret-token" {
		t.Fatalf("credential not attached: %q", gotAuth)
	}
	if !strings.Contains(gotQuery, "q=") {
		t.Fatalf("query placeholder not substituted: %s", gotQuery)
	}
	if res.ResultCount != 2 {
		t.Fatalf("result_count = %d, want 2", res.ResultCount)
	}
	// base 0 + floored unit 0.00125 * 2
	if got := spec.PriceUSD(res.ResultCount); math.Abs(got-0.0025) > 1e-9 {
		t.Fatalf("price = %v, want 0.0025", got)
	}
}

func TestRunTool_UpstreamFailureIsReportedNotThrown(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"Not Found"}`))
	}))
	defer srv.Close()

	spec := &ToolSpec{
		Id: "test.fail", Provider: "test:fail", Mode: ToolModeNative,
		Input:   ToolInputSchema{Type: "object", Properties: map[string]ToolField{}},
		Pricing: ToolPricing{Model: ToolPerCall, Amount: 0.01},
		Adapter: ToolAdapter{Kind: "http", Method: "GET", URL: srv.URL, Auth: &ToolAuth{Type: "none"}},
	}

	res := RunTool(context.Background(), spec, nil)
	if res.OK {
		t.Fatal("a 404 upstream must be reported as a failed call")
	}
	if res.ResultCount != 0 {
		t.Fatalf("a failed call produced %d results", res.ResultCount)
	}
	if !strings.Contains(res.Error, "404") {
		t.Fatalf("the error should carry the upstream status: %s", res.Error)
	}
}

func TestRunTool_MissingCredentialIsNamed(t *testing.T) {
	spec := &ToolSpec{
		Id: "test.nocred", Provider: "test:nocred", Mode: ToolModeNative,
		Input:   ToolInputSchema{Type: "object", Properties: map[string]ToolField{}},
		Pricing: ToolPricing{Model: ToolPerCall, Amount: 0.01},
		Adapter: ToolAdapter{Kind: "http", Method: "GET", URL: "https://example.invalid",
			Auth: &ToolAuth{Type: "bearer", EnvKey: "DEFINITELY_UNSET_TOOL_KEY_12345"}},
	}

	res := RunTool(context.Background(), spec, nil)
	if res.OK {
		t.Fatal("a tool with no credential must not report success")
	}
	if res.MissingEnv != "DEFINITELY_UNSET_TOOL_KEY_12345" {
		t.Fatalf("the missing env var should be named so an operator can fix it, got %q", res.MissingEnv)
	}
}

func TestRunTool_UnwiredAdapterFailsLoudly(t *testing.T) {
	spec := &ToolSpec{
		Id: "test.monid", Provider: "monid:x", Mode: ToolModeFederated,
		Input:   ToolInputSchema{Type: "object", Properties: map[string]ToolField{}},
		Pricing: ToolPricing{Model: ToolPerCall, Amount: 0.01},
		Adapter: ToolAdapter{Kind: "monid", Provider: "apify", Endpoint: "/x"},
	}
	res := RunTool(context.Background(), spec, nil)
	if res.OK {
		t.Fatal("an adapter with no executor must not report success")
	}
	if !strings.Contains(res.Error, "not wired up") {
		t.Fatalf("the error should say the adapter is unimplemented, got %q", res.Error)
	}
}

// ── registry ────────────────────────────────────────────────────────────────

func toolSpecFixtureDir(t *testing.T) string {
	t.Helper()
	dir := filepath.Join("..", "data", "tools")
	if _, err := os.Stat(filepath.Join(dir, "specs")); err != nil {
		t.Skipf("tool catalogue not present: %v", err)
	}
	return dir
}

func TestLoadToolRegistry_ShippedCatalogueIsCallable(t *testing.T) {
	reg, err := loadToolRegistry(toolSpecFixtureDir(t))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if reg.Len() < 500 {
		t.Fatalf("expected the shipped catalogue to hold hundreds of tools, got %d", reg.Len())
	}

	known := map[string]bool{"http": true, "mcp": true, "monid": true, "deepline": true, "waterfall": true}
	for _, s := range reg.All() {
		if s.Id == "" || s.Provider == "" || s.Name == "" {
			t.Fatalf("spec missing identity fields: %q", s.Id)
		}
		if !known[s.Adapter.Kind] {
			t.Fatalf("%s: unknown adapter kind %q", s.Id, s.Adapter.Kind)
		}
		// Every priced tool must clear the margin floor once priced.
		if s.Pricing.Model != ToolFree && s.Pricing.Cost > 0 {
			price := s.PriceUSD(1)
			if margin := (price - s.Pricing.Cost) / price; margin < ToolMinMarginRate-1e-9 {
				t.Fatalf("%s sells at %v against cost %v — margin %.4f below floor",
					s.Id, price, s.Pricing.Cost, margin)
			}
		}
		for _, req := range s.Input.Required {
			if _, ok := s.Input.Properties[req]; !ok {
				t.Fatalf("%s: required field %q is not declared in properties", s.Id, req)
			}
		}
	}
}

func TestDiscover_RanksAndRespectsLimits(t *testing.T) {
	reg, err := loadToolRegistry(toolSpecFixtureDir(t))
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	hits := reg.Discover("xiaohongshu note detail", 5, 0.2)
	if len(hits) == 0 {
		t.Fatal("expected discover to find the Xiaohongshu tools")
	}
	if !strings.Contains(hits[0].Id, "xiaohongshu") {
		t.Fatalf("expected a xiaohongshu tool first, got %s", hits[0].Id)
	}
	for i := 1; i < len(hits); i++ {
		if hits[i].Score > hits[i-1].Score {
			t.Fatal("hits are not sorted by descending score")
		}
	}
	if got := reg.Discover("", 5, 0.2); got != nil {
		t.Fatal("an empty query must return nothing rather than the whole catalogue")
	}
	if got := reg.Discover("douyin video", 3, 0.2); len(got) > 3 {
		t.Fatalf("limit not honoured: got %d", len(got))
	}
}

func TestRegistryFacets_PowerTheMarketplaceFilters(t *testing.T) {
	reg, err := loadToolRegistry(toolSpecFixtureDir(t))
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	platforms := reg.Platforms()
	if len(platforms) < 5 {
		t.Fatalf("expected several platform facets, got %d", len(platforms))
	}
	// Facets are sorted by size so the busiest chips render first.
	for i := 1; i < len(platforms); i++ {
		if platforms[i].Tools > platforms[i-1].Tools {
			t.Fatal("platform facets are not sorted by tool count")
		}
	}

	total := 0
	for _, p := range platforms {
		total += p.Tools
	}
	if total != reg.Len() {
		t.Fatalf("platform facets cover %d tools but the registry holds %d", total, reg.Len())
	}

	if len(reg.Categories()) == 0 {
		t.Fatal("expected category facets")
	}
	if len(reg.Providers()) == 0 {
		t.Fatal("expected provider facets")
	}
}

func TestPlatform_DerivesFromProvider(t *testing.T) {
	if got := (&ToolSpec{Provider: "tikhub:douyin"}).Platform(); got != "douyin" {
		t.Fatalf("Platform() = %q, want douyin", got)
	}
	if got := (&ToolSpec{Provider: "github"}).Platform(); got != "github" {
		t.Fatalf("a provider with no colon should be its own platform, got %q", got)
	}
}
