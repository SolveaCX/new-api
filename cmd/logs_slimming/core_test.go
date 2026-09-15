package main

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

type secretString string

func (s secretString) String() string { return string(s) }

func validTestConfig() config {
	return config{
		command:              "backfill",
		schema:               "newapi_staging",
		source:               "logs",
		target:               "logs_compact_20260811",
		old:                  "logs_old_20260811",
		checkpoint:           "logs_slim_checkpoint_20260811",
		batch:                "20260811",
		expectedProject:      "vocai-gemini-prod",
		expectedInstance:     "newapi-mysql",
		expectedHostname:     "newapi-mysql-primary",
		expectedServerUUID:   "00000000-0000-0000-0000-000000000001",
		expectedDatabaseUser: "newapi_staging_app@%",
		triggerDefiner:       "newapi_staging_app@%",
		phase:                "seed",
		channelIDs:           []int64{57, 61},
		batchSize:            2000,
		batchDelay:           250 * time.Millisecond,
		statementTimeout:     2 * time.Second,
		ddlTimeout:           3 * time.Second,
		lockWaitSeconds:      1,
		autoIncrementReserve: 1_000_000,
		maxThreadsRunning:    32,
	}
}

func TestConfigRequiresStagingIdentity(t *testing.T) {
	cfg := validTestConfig()
	if err := cfg.validate(); err != nil {
		t.Fatal(err)
	}

	for name, mutate := range map[string]func(*config){
		"schema":   func(c *config) { c.schema = "newapi" },
		"project":  func(c *config) { c.expectedProject = "" },
		"instance": func(c *config) { c.expectedInstance = "" },
		"hostname": func(c *config) { c.expectedHostname = "" },
		"uuid":     func(c *config) { c.expectedServerUUID = "" },
	} {
		t.Run(name, func(t *testing.T) {
			bad := cfg
			mutate(&bad)
			if err := bad.validate(); err == nil {
				t.Fatal("unsafe configuration unexpectedly accepted")
			}
		})
	}
}

func TestStagingArtifactHardRejectsProduction(t *testing.T) {
	cfg := validTestConfig()
	cfg.schema = "newapi"
	if err := cfg.validate(); err == nil {
		t.Fatal("staging artifact accepted production")
	}
}

func TestProductionAuthorizationFlagsDoNotExist(t *testing.T) {
	args := []string{"preflight", "--allow-production", "--schema=newapi_staging"}
	if _, err := parseConfig(args); err == nil {
		t.Fatal("deprecated production authorization flag accepted")
	}
}

func TestRollbackCommandIsExplicitAndStillStagingOnly(t *testing.T) {
	cfg := validTestConfig()
	cfg.command = "rollback"
	if err := cfg.validate(); err != nil {
		t.Fatalf("rollback command rejected: %v", err)
	}
	cfg.schema = productionSchema
	if err := cfg.validate(); err == nil {
		t.Fatal("rollback command accepted production schema")
	}
}

func TestAuditedStagingSchemaOnlySwapsLastTwoColumns(t *testing.T) {
	staging := auditedStagingLogColumns()
	if len(staging) != len(auditedLogColumns) {
		t.Fatalf("staging schema has %d columns", len(staging))
	}
	for i := 0; i < 19; i++ {
		if staging[i] != auditedLogColumns[i] {
			t.Fatalf("staging column %d changed", i+1)
		}
	}
	if staging[19].name != "upstream_request_id" || staging[20].name != "other" {
		t.Fatalf("unexpected staging tail: %+v", staging[19:])
	}
}

func TestValidateChannelSnapshot(t *testing.T) {
	if err := validateChannelSnapshot([]int64{57, 61}, []int64{57, 61}); err != nil {
		t.Fatalf("matching snapshots rejected: %v", err)
	}
	if err := validateChannelSnapshot([]int64{57}, []int64{57, 61}); err == nil {
		t.Fatal("mismatched snapshots accepted")
	}
}

func TestCutoverObservationTablesAreExplicit(t *testing.T) {
	want := []string{"metadata_locks", "threads"}
	if !slices.Equal(cutoverObservationTables, want) {
		t.Fatalf("observation tables=%v want=%v", cutoverObservationTables, want)
	}
	for _, table := range want {
		if !strings.Contains(cutoverObservationProbeSQL(table), "performance_schema."+table) {
			t.Fatalf("missing observation table %s", table)
		}
	}
}

func TestCutoverObservationAccessReportsAllDeniedTables(t *testing.T) {
	var probed []string
	err := probeCutoverObservationAccess(context.Background(), func(_ context.Context, table string) error {
		probed = append(probed, table)
		return &mysql.MySQLError{Number: 1142, Message: "SELECT command denied"}
	})
	if !slices.Equal(probed, cutoverObservationTables) {
		t.Fatalf("probed=%v want=%v", probed, cutoverObservationTables)
	}
	if err == nil {
		t.Fatal("missing SELECT privileges unexpectedly accepted")
	}
	for _, table := range cutoverObservationTables {
		if !strings.Contains(err.Error(), "performance_schema."+table) {
			t.Fatalf("error does not include %s: %v", table, err)
		}
	}
}

func TestCutoverObservationAccessPreservesProbeFailures(t *testing.T) {
	err := probeCutoverObservationAccess(context.Background(), func(_ context.Context, table string) error {
		if table == "metadata_locks" {
			return &mysql.MySQLError{Number: 1142, Message: "SELECT command denied"}
		}
		return errors.New("connection reset")
	})
	if err == nil {
		t.Fatal("failed probes unexpectedly accepted")
	}
	for _, want := range []string{
		"SELECT denied on performance_schema.metadata_locks",
		"probe execution failed: performance_schema.threads: connection reset",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error missing %q: %v", want, err)
		}
	}
}

func TestAuditedColumnsAllowOnlyKnownLogCollations(t *testing.T) {
	want := columnSpec{name: "content", columnType: "longtext", nullable: "YES", charset: "utf8mb4", collation: "utf8mb4_unicode_ci"}
	staging := want
	staging.collation = "utf8mb4_0900_ai_ci"
	if !matchesAuditedColumn(staging, want) {
		t.Fatal("known staging collation rejected")
	}
	unexpected := want
	unexpected.collation = "utf8mb4_bin"
	if matchesAuditedColumn(unexpected, want) {
		t.Fatal("unexpected collation accepted")
	}
}

func TestIdentifiersAreStrict(t *testing.T) {
	for _, value := range []string{"logs", "logs_compact_20260811"} {
		if _, err := quoteIdentifier(value); err != nil {
			t.Fatalf("safe identifier %q rejected: %v", value, err)
		}
	}
	for _, value := range []string{"newapi.logs", "logs` DROP TABLE x", "UPPER", ""} {
		if _, err := quoteIdentifier(value); err == nil {
			t.Fatalf("unsafe identifier %q accepted", value)
		}
	}
}

func TestCopySQLIsBoundedExplicitAndNullSafe(t *testing.T) {
	cfg := validTestConfig()
	query, err := buildCopySQL(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		"FORCE INDEX (PRIMARY)", "id > ?", "id <= ?", "id <= ?",
		"COALESCE(user_id, -1) = 1", "COALESCE(token_id, 0) > 0",
		"COALESCE(channel_id, -1) IN (57,61)", "ON DUPLICATE KEY UPDATE id = `newapi_staging`.`logs_compact_20260811`.id",
	} {
		if !strings.Contains(query, required) {
			t.Errorf("copy SQL missing %q\n%s", required, query)
		}
	}
	if strings.Contains(query, "SELECT *") {
		t.Fatal("copy SQL must not use SELECT *")
	}
	if strings.Contains(query, "UPDATE id = id") {
		t.Fatal("copy SQL no-op update must qualify the target id")
	}
	for _, column := range logColumns {
		if !strings.Contains(query, column) {
			t.Errorf("copy SQL missing column %s", column)
		}
	}
}

func TestRetainedAndFilteredPredicatesAreExactComplements(t *testing.T) {
	filtered := filteredPredicate("l", []int64{57, 61})
	retained := retainedPredicate("l", []int64{57, 61})
	if retained != "NOT ("+filtered+")" {
		t.Fatalf("predicates are not exact complements: filtered=%s retained=%s", filtered, retained)
	}
	for _, want := range []string{"l.user_id", "l.token_id", "l.channel_id", "IN (57,61)"} {
		if !strings.Contains(filtered, want) {
			t.Fatalf("filtered predicate missing %q: %s", want, filtered)
		}
	}
}

func TestFullRowEqualityCoversAllColumnsWithBinaryText(t *testing.T) {
	equal := rowEqualitySQL("s", "d")
	if got := strings.Count(equal, "<=>"); got != len(logColumns) {
		t.Fatalf("row equality compares %d fields, want %d: %s", got, len(logColumns), equal)
	}
	for _, column := range textColumns {
		if !strings.Contains(equal, "BINARY s."+column+" <=> BINARY d."+column) {
			t.Errorf("text column %s is not compared with binary semantics", column)
		}
	}
}

func TestTriggerSQLUsesStrictInsertAndFrozenPredicate(t *testing.T) {
	cfg := validTestConfig()
	query, err := buildForwardTriggerSQL(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToUpper(query), "IGNORE") || strings.Contains(strings.ToUpper(query), "ON DUPLICATE") {
		t.Fatal("forward trigger must surface collisions")
	}
	if !strings.Contains(query, "IF NOT (") || !strings.Contains(query, "IN (57,61)") {
		t.Fatalf("forward trigger does not use frozen retained predicate: %s", query)
	}
	for _, prefix := range []string{"NEW.id", "NEW.user_id", "NEW.upstream_request_id"} {
		if !strings.Contains(query, prefix) {
			t.Errorf("forward trigger missing %s", prefix)
		}
	}
}

func TestUpdateGuardAllowsOnlyCompleteNoopDuplicateUpdate(t *testing.T) {
	cfg := validTestConfig()
	query, err := buildNamedGuardTriggerSQL(cfg, "future_guard_update", "update", cfg.target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, "BEGIN IF NOT (") || !strings.Contains(query, "THEN SIGNAL SQLSTATE '45000'") {
		t.Fatalf("update guard is not conditional: %s", query)
	}
	if got := strings.Count(query, "<=>"); got != len(logColumns) {
		t.Fatalf("update guard compares %d fields, want %d: %s", got, len(logColumns), query)
	}
	for _, column := range logColumns {
		if !strings.Contains(query, "OLD."+column+" <=>") && !strings.Contains(query, "BINARY OLD."+column+" <=>") {
			t.Fatalf("update guard omits OLD/NEW equality for %s", column)
		}
	}
	copySQL, err := buildCopySQL(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(copySQL, "ON DUPLICATE KEY UPDATE id = `newapi_staging`.`logs_compact_20260811`.id") {
		t.Fatalf("backfill duplicate path is not a qualified no-op: %s", copySQL)
	}
	mirrorSQL, err := buildMirrorCopySQL(cfg, cfg.source, cfg.old)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(mirrorSQL, "ON DUPLICATE KEY UPDATE id=`newapi_staging`.`logs_old_20260811`.id") {
		t.Fatalf("rollback mirror duplicate path is not a qualified no-op: %s", mirrorSQL)
	}
	deleteGuard, err := buildNamedGuardTriggerSQL(cfg, "future_guard_delete", "delete", cfg.target)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(deleteGuard, "IF NOT") || !strings.Contains(deleteGuard, "FOR EACH ROW SIGNAL") {
		t.Fatalf("delete guard must remain unconditional: %s", deleteGuard)
	}
}

func TestCheckpointCASIsGenerationGuarded(t *testing.T) {
	query, err := checkpointCASSQL(validTestConfig())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, "generation = generation + 1") || !strings.Contains(query, "WHERE id = 1 AND generation = ?") {
		t.Fatalf("checkpoint update is not CAS guarded: %s", query)
	}
}

func TestTopologyClassification(t *testing.T) {
	tests := []struct {
		name   string
		state  objectTopology
		status topologyStatus
	}{
		{"pre", objectTopology{source: true, target: true}, topologyPreCutover},
		{"post", objectTopology{source: true, old: true}, topologyPostCutover},
		{"unknown-all", objectTopology{source: true, target: true, old: true}, topologyUnknown},
		{"unknown-none", objectTopology{}, topologyUnknown},
		{"unknown-missing-live", objectTopology{target: true}, topologyUnknown},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyTopology(tt.state); got != tt.status {
				t.Fatalf("got %s want %s", got, tt.status)
			}
		})
	}
}

func TestVerifyTopologyFollowsCheckpointPhase(t *testing.T) {
	if got := classifyForVerify(checkpoint{phase: "fresh"}); got != topologyPreCutover {
		t.Fatalf("fresh checkpoint classified as %s", got)
	}
	if got := classifyForVerify(checkpoint{phase: "rollback-ready"}); got != topologyPostCutover {
		t.Fatalf("rollback-ready checkpoint classified as %s", got)
	}
}

func TestPostCutoverOwnedTableIsLiveSource(t *testing.T) {
	if got := ownedTableForTopology(validTestConfig(), topologyPostCutover); got != "logs" {
		t.Fatalf("POST ownership checked on %q", got)
	}
	if got := ownedTableForTopology(validTestConfig(), topologyPreCutover); got != "logs_compact_20260811" {
		t.Fatalf("PRE ownership checked on %q", got)
	}
}

func TestPostCutoverRecoveryNeverAllowsTriggerCycle(t *testing.T) {
	for _, state := range []triggerTopology{
		{forward: true, reverse: true},
		{forward: true, reverse: true, updateGuard: true},
	} {
		if _, err := recoveryPlan(topologyPostCutover, state); err == nil {
			t.Fatal("recovery accepted simultaneous forward and reverse triggers")
		}
	}

	plan, err := recoveryPlan(topologyPostCutover, triggerTopology{forward: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []recoveryStep{stepRecordRollbackBase, stepDropForward, stepCreateReverse, stepReconcileRollbackGap}
	if len(plan) != len(want) {
		t.Fatalf("plan=%v want=%v", plan, want)
	}
	for i := range want {
		if plan[i] != want[i] {
			t.Fatalf("plan[%d]=%s want=%s", i, plan[i], want[i])
		}
	}
}

func TestCleanupRefusesUnknownOrPostCutoverTopology(t *testing.T) {
	for _, status := range []topologyStatus{topologyUnknown, topologyPostCutover} {
		if _, err := cleanupPlan(status, triggerTopology{}, true); err == nil {
			t.Fatalf("cleanup accepted topology %s", status)
		}
	}
	plan, err := cleanupPlan(topologyPreCutover, triggerTopology{}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan) == 0 || plan[len(plan)-1] != cleanupDropCheckpoint {
		t.Fatalf("unexpected cleanup plan %v", plan)
	}
	if _, err := cleanupPlan(topologyPreCutover, triggerTopology{reverse: true}, true); err == nil {
		t.Fatal("cleanup accepted a reverse trigger")
	}
}

func TestDDLKillerRequiresExactConnectionAndStatement(t *testing.T) {
	statement := "RENAME TABLE `logs` TO `logs_old`, `logs_compact` TO `logs`"
	if !sameDDL(statement, "  RENAME   TABLE `logs` TO `logs_old`, `logs_compact` TO `logs`  ") {
		t.Fatal("whitespace-only difference should match")
	}
	if sameDDL(statement, "ALTER TABLE `logs` ADD COLUMN bad INT") {
		t.Fatal("different DDL unexpectedly matched")
	}
}

func TestCutoverAndRollbackRenameStatementsAreSymmetric(t *testing.T) {
	cfg := validTestConfig()
	cutoverSQL := renameStatement(cfg)
	rollbackSQL := rollbackStatement(cfg)
	for _, want := range []string{"`newapi_staging`.`logs` TO `newapi_staging`.`logs_old_20260811`", "`newapi_staging`.`logs_compact_20260811` TO `newapi_staging`.`logs`"} {
		if !strings.Contains(cutoverSQL, want) {
			t.Fatalf("cutover SQL missing %q: %s", want, cutoverSQL)
		}
	}
	for _, want := range []string{"`newapi_staging`.`logs` TO `newapi_staging`.`logs_compact_20260811`", "`newapi_staging`.`logs_old_20260811` TO `newapi_staging`.`logs`"} {
		if !strings.Contains(rollbackSQL, want) {
			t.Fatalf("rollback SQL missing %q: %s", want, rollbackSQL)
		}
	}
	tagged := taggedDDL(cfg, rollbackOperation, "abc123", rollbackSQL)
	if !strings.Contains(tagged, "operation="+rollbackOperation+" nonce=abc123") || !strings.HasSuffix(tagged, rollbackSQL) {
		t.Fatalf("rollback DDL tag is not exact: %s", tagged)
	}
}

func TestEvidenceRedactsSecrets(t *testing.T) {
	sink := &memoryEvidence{}
	quotedSecret := `s"e\cret`
	e := newEvidence(sink, []string{"user:secret@tcp(host)/db", "secret", quotedSecret})
	e.emit("failed", map[string]any{
		"error":  errors.New("dial user:secret@tcp(host)/db failed"),
		"nested": map[string]any{"slice": []any{"secret", []byte("secret")}},
		"struct": struct {
			Token string `json:"token"`
		}{Token: "secret"},
		"stringer": secretString("secret"),
		"escaped": struct {
			Token string `json:"token"`
		}{Token: quotedSecret},
	})
	output := sink.String()
	if strings.Contains(output, "secret") || strings.Contains(output, "user:") || strings.Contains(output, quotedSecret) || strings.Contains(output, `s\"e\\cret`) {
		t.Fatalf("evidence leaked secret: %s", output)
	}
	if !strings.Contains(output, "<redacted>") {
		t.Fatalf("evidence did not mark redaction: %s", output)
	}
}

func TestReserveRejectsOverflowAndUsesMinimum(t *testing.T) {
	if got, err := targetAutoIncrement(100, 10); err != nil || got != 110 {
		t.Fatalf("got=%d err=%v", got, err)
	}
	if _, err := targetAutoIncrement(^uint64(0)-5, 10); err == nil {
		t.Fatal("overflow accepted")
	}
}

func TestStableDDLPostconditionUsesObservedStateAfterResolvedExecution(t *testing.T) {
	if got := stableDDLPostcondition(true, []ddlState{ddlPost, ddlPost}); got != ddlPost {
		t.Fatalf("stable POST classified as %s", got)
	}
	for _, states := range [][]ddlState{{ddlPost}, {ddlPost, ddlPre}, {ddlUnknown, ddlUnknown}} {
		if got := stableDDLPostcondition(true, states); got != ddlUnknown {
			t.Fatalf("unstable states %v classified as %s", states, got)
		}
	}
	if got := stableDDLPostcondition(false, []ddlState{ddlPost, ddlPost}); got != ddlUnknown {
		t.Fatalf("unresolved execution classified as %s", got)
	}
}

func TestDDLObservationContextNeverExtendsWatchdogDeadline(t *testing.T) {
	deadline := time.Now().Add(100 * time.Millisecond)
	ctx, cancel := boundedBackground(deadline, 250*time.Millisecond)
	defer cancel()
	got, ok := ctx.Deadline()
	if !ok || got.After(deadline) {
		t.Fatalf("observation deadline %v exceeds watchdog %v", got, deadline)
	}
}

func TestCutoverUsesDoubleReserveBeforeFinalBarrier(t *testing.T) {
	got, err := plannedTargetAutoIncrement(1_000, 1_000_000)
	if err != nil || got != 2_001_000 {
		t.Fatalf("got=%d err=%v", got, err)
	}
	if _, err := plannedTargetAutoIncrement(1, ^uint64(0)); err == nil {
		t.Fatal("double reserve overflow accepted")
	}
}

func TestCutoverCheckpointRequiresFreshCompletedNoIntent(t *testing.T) {
	ready := checkpoint{phase: "fresh", last: 200, final: sql.NullInt64{Int64: 200, Valid: true}}
	if err := cutoverCheckpointReady(ready); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*checkpoint){
		"phase":      func(s *checkpoint) { s.phase = "gap" },
		"incomplete": func(s *checkpoint) { s.last = 199 },
		"no-final":   func(s *checkpoint) { s.final.Valid = false },
		"operation":  func(s *checkpoint) { s.ddlOperation = sql.NullString{String: cutoverOperation, Valid: true} },
		"nonce":      func(s *checkpoint) { s.ddlNonce = sql.NullString{String: "nonce", Valid: true} },
	} {
		t.Run(name, func(t *testing.T) {
			state := ready
			mutate(&state)
			if err := cutoverCheckpointReady(state); err == nil {
				t.Fatal("unsafe checkpoint accepted")
			}
		})
	}
}

func TestMDLDrainAllowsOnlyBarrierAndExactRename(t *testing.T) {
	if !mdlOwnerAllowed(10, "GRANTED", 10, 20) {
		t.Fatal("barrier shared MDL rejected")
	}
	if !mdlOwnerAllowed(20, "GRANTED", 10, 20) || !mdlOwnerAllowed(20, "PENDING", 10, 20) {
		t.Fatal("RENAME granted/pending MDL rejected")
	}
	for _, row := range []struct {
		owner  int64
		status string
	}{{30, "GRANTED"}, {30, "PENDING"}, {10, "PENDING"}} {
		if mdlOwnerAllowed(row.owner, row.status, 10, 20) {
			t.Fatalf("foreign/invalid MDL accepted: %+v", row)
		}
	}
}

func TestRenameStatementIsAtomicAndBarrierDoesNotUseLockTables(t *testing.T) {
	statement := renameStatement(validTestConfig())
	for _, required := range []string{"RENAME TABLE", "`newapi_staging`.`logs` TO `newapi_staging`.`logs_old_20260811`", "`newapi_staging`.`logs_compact_20260811` TO `newapi_staging`.`logs`"} {
		if !strings.Contains(statement, required) {
			t.Fatalf("rename statement missing %q: %s", required, statement)
		}
	}
	if strings.Contains(strings.ToUpper(statement), "LOCK TABLES") {
		t.Fatal("cutover must not use LOCK TABLES")
	}
}

func TestPostTriggerPlanIsIdempotentAndNeverCycles(t *testing.T) {
	plan, err := postTriggerPlan(triggerTopology{forward: true})
	if err != nil || len(plan) != 2 || plan[0] != postTriggerDropForward || plan[1] != postTriggerCreateReverse {
		t.Fatalf("forward transition plan=%v err=%v", plan, err)
	}
	plan, err = postTriggerPlan(triggerTopology{})
	if err != nil || len(plan) != 1 || plan[0] != postTriggerCreateReverse {
		t.Fatalf("missing reverse repair plan=%v err=%v", plan, err)
	}
	plan, err = postTriggerPlan(triggerTopology{reverse: true})
	if err != nil || len(plan) != 0 {
		t.Fatalf("already stable reverse plan=%v err=%v", plan, err)
	}
	if _, err := postTriggerPlan(triggerTopology{forward: true, reverse: true}); err == nil {
		t.Fatal("forward/reverse cycle accepted")
	}
}

func TestFutureGuardsMoveAtomicallyWithCompactTable(t *testing.T) {
	cfg := validTestConfig()
	query, err := buildNamedGuardTriggerSQL(cfg, "future_guard_update", "update", cfg.target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, "logs_slim_future_guard_update_") || !strings.Contains(query, "BEFORE UPDATE ON `newapi_staging`.`logs_compact_20260811`") {
		t.Fatalf("future guard is not preinstalled on compact target: %s", query)
	}
	pre := triggerTopology{
		updateGuard: true, deleteGuard: true, updateGuardTable: cfg.source, deleteGuardTable: cfg.source,
		futureUpdateGuard: true, futureDeleteGuard: true, futureUpdateGuardTable: cfg.target, futureDeleteGuardTable: cfg.target,
	}
	if !preGuardsReady(pre, cfg) {
		t.Fatal("complete pre-cutover guard topology rejected")
	}
	post := pre
	post.updateGuardTable, post.deleteGuardTable = cfg.old, cfg.old
	post.futureUpdateGuardTable, post.futureDeleteGuardTable = cfg.source, cfg.source
	if !postGuardsReady(post, cfg) {
		t.Fatal("future guards were not recognized on post-cutover live table")
	}
}

func TestFilteredCleanupGuardPlanAllowsOnlyRollbackReconcile(t *testing.T) {
	cfg := validTestConfig()
	complete := triggerTopology{
		updateGuard: true, deleteGuard: true, updateGuardTable: cfg.source, deleteGuardTable: cfg.source,
		futureUpdateGuard: true, futureDeleteGuard: true, futureUpdateGuardTable: cfg.target, futureDeleteGuardTable: cfg.target,
	}

	drop, err := filteredCleanupGuardPlan("rollback-reconcile", complete, cfg)
	if err != nil || !drop {
		t.Fatalf("complete rollback cleanup topology rejected: drop=%t err=%v", drop, err)
	}

	missingDelete := complete
	missingDelete.futureDeleteGuard = false
	missingDelete.futureDeleteGuardTable = ""
	drop, err = filteredCleanupGuardPlan("rollback-reconcile", missingDelete, cfg)
	if err != nil || drop {
		t.Fatalf("recoverable missing future DELETE guard rejected: drop=%t err=%v", drop, err)
	}

	for _, phase := range []string{"fresh", "rollback-intent"} {
		if _, err := filteredCleanupGuardPlan(phase, missingDelete, cfg); err == nil {
			t.Fatalf("phase %s accepted a missing future DELETE guard", phase)
		}
	}
}

func TestFilteredCleanupGuardPlanRejectsUnsafeGuardTopology(t *testing.T) {
	cfg := validTestConfig()
	complete := triggerTopology{
		updateGuard: true, deleteGuard: true, updateGuardTable: cfg.source, deleteGuardTable: cfg.source,
		futureUpdateGuard: true, futureDeleteGuard: true, futureUpdateGuardTable: cfg.target, futureDeleteGuardTable: cfg.target,
	}

	tests := map[string]func(*triggerTopology){
		"future delete on source": func(topology *triggerTopology) { topology.futureDeleteGuardTable = cfg.source },
		"future delete elsewhere": func(topology *triggerTopology) { topology.futureDeleteGuardTable = "other_logs" },
		"missing future delete with stale table": func(topology *triggerTopology) {
			topology.futureDeleteGuard = false
			topology.futureDeleteGuardTable = cfg.target
		},
		"missing source update": func(topology *triggerTopology) { topology.updateGuard = false },
		"missing source delete": func(topology *triggerTopology) { topology.deleteGuard = false },
		"missing future update": func(topology *triggerTopology) { topology.futureUpdateGuard = false },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			topology := complete
			mutate(&topology)
			if _, err := filteredCleanupGuardPlan("rollback-reconcile", topology, cfg); err == nil {
				t.Fatalf("unsafe topology accepted: %+v", topology)
			}
		})
	}
}

func TestFilteredCleanupGuardRestoreUsesPersistedTriggerIdentity(t *testing.T) {
	cfg := validTestConfig()
	const persistedSQLMode = "STRICT_TRANS_TABLES,NO_ENGINE_SUBSTITUTION"
	spec, err := expectedTriggerSpec(cfg, "future_guard_delete", cfg.target, persistedSQLMode)
	if err != nil {
		t.Fatal(err)
	}
	name, err := triggerName("future_guard_delete", cfg.batch)
	if err != nil {
		t.Fatal(err)
	}
	if spec.name != name || spec.table != cfg.target || spec.sqlMode != persistedSQLMode || spec.definer != cfg.triggerDefiner {
		t.Fatalf("restore spec does not preserve trigger identity: %+v", spec)
	}
	query, err := buildNamedGuardTriggerSQL(cfg, "future_guard_delete", "delete", cfg.target)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(query, "TRIGGER `"+name+"`") || !strings.Contains(query, "BEFORE DELETE ON `"+cfg.schema+"`.`"+cfg.target+"`") {
		t.Fatalf("restore SQL targets the wrong trigger/table: %s", query)
	}
}

func TestFilteredCleanupGuardRestoreReturnsToStrictPreGuards(t *testing.T) {
	cfg := validTestConfig()
	topology := triggerTopology{
		updateGuard: true, deleteGuard: true, updateGuardTable: cfg.source, deleteGuardTable: cfg.source,
		futureUpdateGuard: true, futureDeleteGuard: true, futureUpdateGuardTable: cfg.target, futureDeleteGuardTable: cfg.target,
	}
	if !preGuardsReady(topology, cfg) {
		t.Fatal("restored cleanup guard topology is not strict pre-cutover topology")
	}
}

func TestNextPostVerifyEndUsesSmallestValidBoundedEndpoint(t *testing.T) {
	tests := []struct {
		name        string
		old, source sql.NullInt64
		want        sql.NullInt64
	}{
		{"both-old-first", sql.NullInt64{Int64: 10, Valid: true}, sql.NullInt64{Int64: 20, Valid: true}, sql.NullInt64{Int64: 10, Valid: true}},
		{"both-source-first", sql.NullInt64{Int64: 30, Valid: true}, sql.NullInt64{Int64: 20, Valid: true}, sql.NullInt64{Int64: 20, Valid: true}},
		{"only-old", sql.NullInt64{Int64: 10, Valid: true}, sql.NullInt64{}, sql.NullInt64{Int64: 10, Valid: true}},
		{"only-source", sql.NullInt64{}, sql.NullInt64{Int64: 20, Valid: true}, sql.NullInt64{Int64: 20, Valid: true}},
		{"neither", sql.NullInt64{}, sql.NullInt64{}, sql.NullInt64{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := nextPostVerifyEnd(tt.old, tt.source); got != tt.want {
				t.Fatalf("got=%v want=%v", got, tt.want)
			}
		})
	}
}

func TestFullPostHighWaterIncludesEitherTable(t *testing.T) {
	if got := maxInt64(100, 2_000_100); got != 2_000_100 {
		t.Fatalf("source high-water omitted: %d", got)
	}
	if got := maxInt64(3_000_000, 2_000_100); got != 3_000_000 {
		t.Fatalf("old high-water omitted: %d", got)
	}
}

func TestCheckpointPhaseTransitionsAreExplicitAndBoundaryGuarded(t *testing.T) {
	seed := checkpoint{phase: "seed", last: 100, seed: 100}
	last, final, needsForward, err := checkpointTransitionPlan(seed, "gap", 150)
	if err != nil || last != 100 || !final.Valid || final.Int64 != 150 || !needsForward {
		t.Fatalf("seed->gap plan got last=%d final=%v forward=%t err=%v", last, final, needsForward, err)
	}
	seed.last = 99
	if _, _, _, err := checkpointTransitionPlan(seed, "gap", 150); err == nil {
		t.Fatal("incomplete seed was allowed to transition")
	}
	gap := checkpoint{phase: "gap", last: 150, final: sql.NullInt64{Int64: 150, Valid: true}}
	last, _, _, err = checkpointTransitionPlan(gap, "fresh", 0)
	if err != nil || last != 0 {
		t.Fatalf("gap->fresh must reset to zero: last=%d err=%v", last, err)
	}
	if _, _, _, err := checkpointTransitionPlan(gap, "stable", 0); err == nil {
		t.Fatal("illegal phase transition accepted")
	}
}

func TestVerifyRejectsTransitionalRollbackPhases(t *testing.T) {
	for _, phase := range []string{"rollback-intent", "rollback-reconcile"} {
		if got := classifyForVerify(checkpoint{phase: phase}); got != topologyUnknown {
			t.Fatalf("transitional phase %s classified as %s", phase, got)
		}
	}
}

func TestLockAndOwnershipBindMigrationDomain(t *testing.T) {
	a := validTestConfig()
	b := a
	b.batch = "other"
	b.target = "logs_compact_other"
	b.old = "logs_old_other"
	b.checkpoint = "logs_slim_checkpoint_other"
	if advisoryLockName(a) != advisoryLockName(b) {
		t.Fatal("different batches for the same source must contend on one advisory lock")
	}
	if ownershipMarker(a) == ownershipMarker(b) {
		t.Fatal("ownership marker must bind concrete batch objects")
	}
	b = a
	b.source = "other_logs"
	if advisoryLockName(a) == advisoryLockName(b) {
		t.Fatal("advisory lock must bind the concrete source")
	}
}

func TestAdvisoryLockNameFitsMySQLLimit(t *testing.T) {
	cfg := validTestConfig()
	name := advisoryLockName(cfg)
	if len(name) > 64 {
		t.Fatalf("lock name has %d characters: %s", len(name), name)
	}
	otherBatch := cfg
	otherBatch.batch = "another_batch"
	if advisoryLockName(otherBatch) != name {
		t.Fatal("same source migration domain produced a batch-specific lock")
	}
	otherSource := cfg
	otherSource.source = "logs_other"
	if advisoryLockName(otherSource) == name {
		t.Fatal("different source tables share an advisory lock")
	}
}
