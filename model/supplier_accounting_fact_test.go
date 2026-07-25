package model

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func supplierAccountingFactTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, EnsureSupplierAccountingFactSchema(db))
	return db
}

func supplierAccountingCapturedEnvelope(t *testing.T) types.SupplierAccountingEnvelopeV1 {
	t.Helper()
	official, procurement := int64(1_000), int64(650)
	sales, gross, multiplier := int64(700), int64(50), int64(700_000)
	return types.SupplierAccountingEnvelopeV1{
		EnvelopeSchemaVersion: types.SupplierAccountingEnvelopeSchemaVersionV1,
		Disposition:           types.SupplierAccountingDispositionCaptured,
		Captured: &types.SupplierAccountingLogSnapshotV1{
			BindingVersionId: 11, SupplierId: 12, ContractId: 13, RateVersionId: 14,
			ProcurementMultiplierPpm: 650_000, SalesMultiplierPpm: &multiplier,
			OfficialListMicroUsd: &official, ProcurementCostMicroUsd: &procurement,
			SalesMicroUsd: &sales, GrossProfitMicroUsd: &gross,
			StatisticsScope: string(types.SupplierStatisticsScopeBusiness), ExclusionDecision: "included",
			FinanciallyCommittedAt: time.Now().Unix(),
			PricingProvenance: &types.SupplierPricingProvenanceV1{Ratio: &types.SupplierRatioPricingProvenanceV1{
				ModelRatioPpm: 1_000_000, GroupRatioPpm: multiplier, ModelRatioVersion: 1, GroupRatioVersion: 1,
			}},
		},
	}
}

func TestSupplierAccountingFactLifecycleIsCASAndIdempotent(t *testing.T) {
	db := supplierAccountingFactTestDB(t)
	ctx := context.Background()
	prepared, err := PrepareSupplierAccountingFact(ctx, db, SupplierAccountingFactPrepare{
		AttemptId: "018f843e-7e3a-7f61-a0a0-000000000001", ParentRequestId: "req-parent", RetryIndex: 2,
		SupplierId: 12, ContractId: 13, BindingVersionId: 11, RateVersionId: 14,
		ChannelId: 15, ModelName: "gpt-test", CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
	})
	require.NoError(t, err)
	require.Equal(t, SupplierAccountingFactStatusPending, prepared.Status)
	require.NotZero(t, prepared.PreparedAt)
	require.Regexp(t, `^\d{4}-\d{2}-\d{2}$`, prepared.PreparedDay)
	replayed, err := PrepareSupplierAccountingFact(ctx, db, SupplierAccountingFactPrepare{
		AttemptId: prepared.AttemptId, ParentRequestId: "req-parent", RetryIndex: 2,
		SupplierId: 12, ContractId: 13, BindingVersionId: 11, RateVersionId: 14,
		ChannelId: 15, ModelName: "gpt-test", CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
	})
	require.NoError(t, err)
	require.Equal(t, prepared.PreparedAt, replayed.PreparedAt, "replay must retain the original database timestamp")
	require.Equal(t, prepared.PreparedDay, replayed.PreparedDay)

	envelope := supplierAccountingCapturedEnvelope(t)
	require.NoError(t, FinalizeSupplierAccountingFactCaptured(ctx, db, prepared.AttemptId, envelope, time.Now().Unix()))
	require.NoError(t, FinalizeSupplierAccountingFactCaptured(ctx, db, prepared.AttemptId, envelope, time.Now().Unix()), "same payload finalize must be idempotent")
	require.ErrorIs(t, FinalizeSupplierAccountingFactVoid(ctx, db, prepared.AttemptId, time.Now().Unix()), ErrSupplierAccountingFactTerminalConflict)

	var stored SupplierAccountingFact
	require.NoError(t, db.Where("attempt_id = ?", prepared.AttemptId).First(&stored).Error)
	require.Equal(t, SupplierAccountingFactStatusCaptured, stored.Status)
	require.NotEmpty(t, stored.Payload)
	require.Len(t, stored.PayloadHash, 64)
}

func TestPrepareSupplierAccountingFactFastUsesOneStatementAndReloadsOnlyOnCollision(t *testing.T) {
	db := supplierAccountingFactTestDB(t)
	rawCount, queryCount := 0, 0
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register("test:count_fact_raw", func(tx *gorm.DB) {
		if strings.Contains(tx.Statement.SQL.String(), "supplier_accounting_facts") {
			rawCount++
		}
	}))
	require.NoError(t, db.Callback().Query().Before("gorm:query").Register("test:count_fact_query", func(tx *gorm.DB) {
		if tx.Statement.Table == "supplier_accounting_facts" {
			queryCount++
		}
	}))
	input := SupplierAccountingFactPrepare{
		AttemptId: "018f843e-7e3a-7f61-a0a0-000000000097", ParentRequestId: "req-insert-first", ChannelId: 1,
		SupplierId: 12, ContractId: 13, BindingVersionId: 11, RateVersionId: 14, ModelName: "gpt-test",
		CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
	}
	firstAttemptID, err := PrepareSupplierAccountingFactFast(context.Background(), db, input)
	require.NoError(t, err)
	require.Equal(t, input.AttemptId, firstAttemptID)
	require.Equal(t, 1, rawCount, "successful hot-path prepare must execute exactly one database statement")
	require.Zero(t, queryCount, "successful hot-path prepare must not reload the fact")

	replayedAttemptID, err := PrepareSupplierAccountingFactFast(context.Background(), db, input)
	require.NoError(t, err)
	require.Equal(t, firstAttemptID, replayedAttemptID)
	require.Equal(t, 2, rawCount)
	require.Equal(t, 1, queryCount, "unique collision must reload the frozen fact exactly once")
}

func TestPrepareSupplierAccountingFactRejectsIncompleteBoundIdentityWithoutWrite(t *testing.T) {
	db := supplierAccountingFactTestDB(t)
	rawCount := 0
	require.NoError(t, db.Callback().Raw().Before("gorm:raw").Register("test:count_invalid_fact_raw", func(tx *gorm.DB) {
		if strings.Contains(tx.Statement.SQL.String(), "supplier_accounting_facts") {
			rawCount++
		}
	}))
	valid := SupplierAccountingFactPrepare{
		AttemptId: "018f843e-7e3a-7f61-a0a0-000000000096", ParentRequestId: "req-valid", RetryIndex: 0,
		SupplierId: 1, ContractId: 2, BindingVersionId: 3, RateVersionId: 4, ChannelId: 5, ModelName: "gpt-test",
		CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
	}
	tests := map[string]func(*SupplierAccountingFactPrepare){
		"supplier": func(input *SupplierAccountingFactPrepare) { input.SupplierId = 0 },
		"contract": func(input *SupplierAccountingFactPrepare) { input.ContractId = 0 },
		"binding":  func(input *SupplierAccountingFactPrepare) { input.BindingVersionId = 0 },
		"rate":     func(input *SupplierAccountingFactPrepare) { input.RateVersionId = 0 },
		"channel":  func(input *SupplierAccountingFactPrepare) { input.ChannelId = 0 },
		"model":    func(input *SupplierAccountingFactPrepare) { input.ModelName = " " },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			input := valid
			mutate(&input)
			_, err := PrepareSupplierAccountingFact(context.Background(), db, input)
			require.ErrorIs(t, err, ErrSupplierAccountingFactResolutionInvalid)
		})
	}
	require.Zero(t, rawCount)
}

func TestSupplierAccountingFactPrepareSQLAcrossDialects(t *testing.T) {
	sqliteDB, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	connection, err := sqliteDB.DB()
	require.NoError(t, err)
	tests := map[string]struct {
		dialector gorm.Dialector
		fragments []string
	}{
		"sqlite": {
			dialector: sqlite.Open("file:" + t.Name() + "-sqlite?mode=memory&cache=shared"),
			fragments: []string{"CAST(strftime('%s','now') AS INTEGER)", "strftime('%Y-%m-%d','now','+8 hours')"},
		},
		"mysql57": {
			dialector: mysql.New(mysql.Config{Conn: connection, SkipInitializeWithVersion: true}),
			fragments: []string{"UNIX_TIMESTAMP()", "DATE_FORMAT(DATE_ADD(UTC_TIMESTAMP(), INTERVAL 8 HOUR), '%Y-%m-%d')"},
		},
		"postgres96": {
			dialector: postgres.New(postgres.Config{Conn: connection, WithoutReturning: true}),
			fragments: []string{"EXTRACT(EPOCH FROM NOW())::bigint", "TO_CHAR(NOW() AT TIME ZONE 'Asia/Shanghai', 'YYYY-MM-DD')"},
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			db, openErr := gorm.Open(test.dialector, &gorm.Config{DryRun: true, DisableAutomaticPing: true})
			require.NoError(t, openErr)
			generated := supplierAccountingFactPrepareSQL(db)
			statement := db.Exec(generated, "attempt", "request", 0, 1, 2, 3, 4, 5, "model", "scope", 1).Statement.SQL.String()
			require.Contains(t, statement, "INSERT INTO supplier_accounting_facts")
			require.Contains(t, statement, "SELECT")
			require.Contains(t, statement, "WHERE")
			for _, fragment := range test.fragments {
				require.Contains(t, statement, fragment)
			}
		})
	}
}

func TestPrepareSupplierAccountingFactReturnsDatabaseFailure(t *testing.T) {
	db := supplierAccountingFactTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	_, err = PrepareSupplierAccountingFact(context.Background(), db, SupplierAccountingFactPrepare{
		AttemptId: "018f843e-7e3a-7f61-a0a0-000000000099", ParentRequestId: "req-db-error", ChannelId: 1,
		SupplierId: 12, ContractId: 13, BindingVersionId: 11, RateVersionId: 14, ModelName: "gpt-test",
		CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
	})
	require.Error(t, err)
}

func TestSupplierAccountingFactManualResolutionRequiresAuditEvidence(t *testing.T) {
	db := supplierAccountingFactTestDB(t)
	fact, err := PrepareSupplierAccountingFact(context.Background(), db, SupplierAccountingFactPrepare{
		AttemptId: "018f843e-7e3a-7f61-a0a0-000000000002", ParentRequestId: "req-parent", ChannelId: 1,
		SupplierId: 12, ContractId: 13, BindingVersionId: 11, RateVersionId: 14, ModelName: "gpt-test",
		CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
	})
	require.NoError(t, err)
	require.Error(t, ResolveSupplierAccountingFact(context.Background(), db, SupplierAccountingFactResolution{
		AttemptId: fact.AttemptId, Status: SupplierAccountingFactStatusVoid,
	}))
	require.NoError(t, ResolveSupplierAccountingFact(context.Background(), db, SupplierAccountingFactResolution{
		AttemptId: fact.AttemptId, Status: SupplierAccountingFactStatusVoid,
		Actor: "root:1", Reason: "verified upstream rejection", Evidence: "ticket-123", TerminalAt: time.Now().Unix(),
	}))
	require.ErrorIs(t, ResolveSupplierAccountingFact(context.Background(), db, SupplierAccountingFactResolution{
		AttemptId: fact.AttemptId, Status: SupplierAccountingFactStatusVoid,
		Actor: "root:2", Reason: "different audit", Evidence: "ticket-999", TerminalAt: time.Now().Unix(),
	}), ErrSupplierAccountingFactTerminalConflict)
}

func TestSupplierAccountingFactCapturedIdentityMustMatchPreparedAttempt(t *testing.T) {
	db := supplierAccountingFactTestDB(t)
	fact, err := PrepareSupplierAccountingFact(context.Background(), db, SupplierAccountingFactPrepare{
		AttemptId: "018f843e-7e3a-7f61-a0a0-000000000098", ParentRequestId: "req-identity", ChannelId: 1,
		SupplierId: 12, ContractId: 13, BindingVersionId: 11, RateVersionId: 14, ModelName: "gpt-test",
		CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
	})
	require.NoError(t, err)
	envelope := supplierAccountingCapturedEnvelope(t)
	envelope.Captured.ContractId++
	require.ErrorIs(t, FinalizeSupplierAccountingFactCaptured(context.Background(), db, fact.AttemptId, envelope, time.Now().Unix()), ErrSupplierAccountingFactTerminalConflict)
	var stored SupplierAccountingFact
	require.NoError(t, db.First(&stored, fact.Id).Error)
	require.Equal(t, SupplierAccountingFactStatusPending, stored.Status)
}

func TestSupplierAccountingFactWatermarkRejectsPendingAndDetectsLateFacts(t *testing.T) {
	db := supplierAccountingFactTestDB(t)
	ctx := context.Background()
	first, err := PrepareSupplierAccountingFact(ctx, db, SupplierAccountingFactPrepare{
		AttemptId: "018f843e-7e3a-7f61-a0a0-000000000003", ParentRequestId: "req-1", ChannelId: 1,
		SupplierId: 12, ContractId: 13, BindingVersionId: 11, RateVersionId: 14, ModelName: "gpt-test",
		CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
	})
	require.NoError(t, err)
	watermark, err := FreezeSupplierAccountingFactDay(ctx, db, first.PreparedDay)
	require.ErrorIs(t, err, ErrSupplierAccountingFactsPending)
	require.Equal(t, first.Id, watermark.SourceMaxFactId)

	require.NoError(t, FinalizeSupplierAccountingFactCaptured(ctx, db, first.AttemptId, supplierAccountingCapturedEnvelope(t), time.Now().Unix()))
	watermark, err = FreezeSupplierAccountingFactDay(ctx, db, first.PreparedDay)
	require.NoError(t, err)
	require.Equal(t, first.Id, watermark.SourceMaxFactId)

	second, err := PrepareSupplierAccountingFact(ctx, db, SupplierAccountingFactPrepare{
		AttemptId: "018f843e-7e3a-7f61-a0a0-000000000004", ParentRequestId: "req-2", ChannelId: 1,
		SupplierId: 12, ContractId: 13, BindingVersionId: 11, RateVersionId: 14, ModelName: "gpt-test",
		CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
	})
	require.NoError(t, err)
	require.Equal(t, first.PreparedDay, second.PreparedDay)
	require.ErrorIs(t, VerifySupplierAccountingFactDayClosed(ctx, db, first.PreparedDay, watermark.SourceMaxFactId), ErrSupplierAccountingFactWatermarkChanged)
}

func TestSupplierAccountingFactDayStatusIndexColumnsAcrossDialects(t *testing.T) {
	sqliteDB, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	connection, err := sqliteDB.DB()
	require.NoError(t, err)
	dialectors := map[string]gorm.Dialector{
		"sqlite":     sqlite.Open("file:" + t.Name() + "-schema?mode=memory&cache=shared"),
		"mysql57":    mysql.New(mysql.Config{Conn: connection, SkipInitializeWithVersion: true}),
		"postgres96": postgres.New(postgres.Config{Conn: connection, WithoutReturning: true}),
	}
	for name, dialector := range dialectors {
		t.Run(name, func(t *testing.T) {
			db, openErr := gorm.Open(dialector, &gorm.Config{DryRun: true, DisableAutomaticPing: true})
			require.NoError(t, openErr)
			statement := &gorm.Statement{DB: db}
			require.NoError(t, statement.Parse(&SupplierAccountingFact{}))
			indexes := statement.Schema.ParseIndexes()
			var columns []string
			for _, index := range indexes {
				if index.Name != "idx_supplier_accounting_facts_day_status_id" {
					continue
				}
				for _, field := range index.Fields {
					columns = append(columns, field.DBName)
				}
			}
			require.Equal(t, []string{"prepared_day", "status", "id"}, columns)
		})
	}
}

func TestListPendingSupplierAccountingFactsUsesDayAndIDKeyset(t *testing.T) {
	db := supplierAccountingFactTestDB(t)
	ctx := context.Background()
	inputs := []SupplierAccountingFactPrepare{
		{AttemptId: "018f843e-7e3a-7f61-a0a0-000000000011", ParentRequestId: "req-list-1", ChannelId: 1, SupplierId: 12, ContractId: 13, BindingVersionId: 11, RateVersionId: 14, ModelName: "gpt-test", CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1)},
		{AttemptId: "018f843e-7e3a-7f61-a0a0-000000000012", ParentRequestId: "req-list-2", ChannelId: 1, SupplierId: 12, ContractId: 13, BindingVersionId: 11, RateVersionId: 14, ModelName: "gpt-test", CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1)},
		{AttemptId: "018f843e-7e3a-7f61-a0a0-000000000013", ParentRequestId: "req-list-3", ChannelId: 1, SupplierId: 12, ContractId: 13, BindingVersionId: 11, RateVersionId: 14, ModelName: "gpt-test", CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1)},
	}
	facts := make([]SupplierAccountingFact, 0, len(inputs))
	for _, input := range inputs {
		fact, err := PrepareSupplierAccountingFact(ctx, db, input)
		require.NoError(t, err)
		facts = append(facts, fact)
	}
	require.NoError(t, FinalizeSupplierAccountingFactVoid(ctx, db, facts[1].AttemptId, time.Now().Unix()))

	page, err := ListPendingSupplierAccountingFacts(ctx, db, facts[0].PreparedDay, facts[0].Id, 10)
	require.NoError(t, err)
	require.Len(t, page, 1)
	require.Equal(t, facts[2].AttemptId, page[0].AttemptId)

	require.NoError(t, db.Model(&SupplierAccountingFact{}).Where("id = ?", facts[2].Id).Update("prepared_day", "2000-01-01").Error)
	page, err = ListPendingSupplierAccountingFacts(ctx, db, facts[0].PreparedDay, 0, 10)
	require.NoError(t, err)
	require.Len(t, page, 1)
	require.Equal(t, facts[0].AttemptId, page[0].AttemptId)

	_, err = ListPendingSupplierAccountingFacts(ctx, db, "2026-02-30", 0, 10)
	require.ErrorIs(t, err, ErrSupplierAccountingFactResolutionInvalid)
}

func TestGetSupplierAccountingFactByAttemptIDPreservesFrozenIdentity(t *testing.T) {
	db := supplierAccountingFactTestDB(t)
	expected, err := PrepareSupplierAccountingFact(context.Background(), db, SupplierAccountingFactPrepare{
		AttemptId: "018f843e-7e3a-7f61-a0a0-000000000014", ParentRequestId: "req-read-identity", RetryIndex: 3,
		ChannelId: 15, SupplierId: 12, ContractId: 13, BindingVersionId: 11, RateVersionId: 14, ModelName: "gpt-test",
		CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1),
	})
	require.NoError(t, err)

	actual, err := GetSupplierAccountingFactByAttemptID(context.Background(), db, "  "+expected.AttemptId+"  ")
	require.NoError(t, err)
	require.Equal(t, expected, actual)

	_, err = GetSupplierAccountingFactByAttemptID(context.Background(), db, "018f843e-7e3a-7f61-a0a0-000000000099")
	require.ErrorIs(t, err, ErrSupplierAccountingFactNotFound)
}

func TestPrepareSupplierAccountingFactUsesDatabaseClockForCutover(t *testing.T) {
	db := supplierAccountingFactTestDB(t)
	future := time.Now().Add(24 * time.Hour).Unix()
	_, err := PrepareSupplierAccountingFact(context.Background(), db, SupplierAccountingFactPrepare{
		AttemptId: "018f843e-7e3a-7f61-a0a0-000000000015", ParentRequestId: "req-before-db-cutover",
		ChannelId: 15, SupplierId: 12, ContractId: 13, BindingVersionId: 11, RateVersionId: 14, ModelName: "gpt-test",
		CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1), CutoverAt: future,
	})
	require.ErrorIs(t, err, ErrSupplierAccountingFactBeforeCutover)
	var count int64
	require.NoError(t, db.Model(&SupplierAccountingFact{}).Count(&count).Error)
	require.Zero(t, count)

	active, err := IsSupplierAccountingCutoverActive(context.Background(), db, future)
	require.NoError(t, err)
	require.False(t, active)
	active, err = IsSupplierAccountingCutoverActive(context.Background(), db, 1)
	require.NoError(t, err)
	require.True(t, active)
}
