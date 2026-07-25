package service

import (
	"context"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func useSupplierAccountingOperationsDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Log{}))
	require.NoError(t, model.EnsureSupplierAccountingFactSchema(db))
	previous := model.LOG_DB
	model.LOG_DB = db
	t.Cleanup(func() { model.LOG_DB = previous })
	return db
}

func supplierAccountingOperationsFact(t *testing.T, parentRequestID string) model.SupplierAccountingFact {
	t.Helper()
	fact, err := model.PrepareSupplierAccountingFact(context.Background(), model.LOG_DB, model.SupplierAccountingFactPrepare{
		ParentRequestId: parentRequestID, SupplierId: 12, ContractId: 13, BindingVersionId: 11, RateVersionId: 14,
		ChannelId: 15, ModelName: "gpt-test", CoverageScope: string(types.SupplierAccountingCoverageScopeBoundSupplierSynchronousRelayV1), CutoverAt: 1,
	})
	require.NoError(t, err)
	return fact
}

func supplierAccountingOperationsEnvelope() types.SupplierAccountingEnvelopeV1 {
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

func createSupplierAccountingSourceLog(t *testing.T, requestID, attemptID string, envelope any) model.Log {
	t.Helper()
	other, err := common.Marshal(map[string]any{
		types.SupplierAccountingEnvelopeKeyV1:  envelope,
		types.SupplierAccountingAttemptIDKeyV1: attemptID,
	})
	require.NoError(t, err)
	log := model.Log{Type: model.LogTypeConsume, RequestId: requestID, Other: string(other)}
	require.NoError(t, model.LOG_DB.Create(&log).Error)
	return log
}

func TestResolvePendingSupplierAccountingFactCaptureFromLogIsIdempotent(t *testing.T) {
	useSupplierAccountingOperationsDB(t)
	fact := supplierAccountingOperationsFact(t, "req-capture")
	log := createSupplierAccountingSourceLog(t, fact.ParentRequestId, fact.AttemptId, supplierAccountingOperationsEnvelope())
	input := SupplierAccountingFactResolveInput{
		AttemptId: fact.AttemptId, Action: SupplierAccountingFactResolveCaptureFromLog, SourceLogId: log.Id,
		Actor: "root:17", Reason: "verified", Evidence: "ticket-1",
	}

	resolved, err := ResolvePendingSupplierAccountingFact(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, model.SupplierAccountingFactStatusCaptured, resolved.Status)
	require.Contains(t, resolved.ResolutionEvidence, "source_log_id=")
	replayed, err := ResolvePendingSupplierAccountingFact(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, resolved.PayloadHash, replayed.PayloadHash)
}

func TestResolvePendingSupplierAccountingFactRequiresExactRetryAttempt(t *testing.T) {
	useSupplierAccountingOperationsDB(t)
	first := supplierAccountingOperationsFact(t, "req-shared-parent")
	second := supplierAccountingOperationsFact(t, "req-shared-parent")
	log := createSupplierAccountingSourceLog(t, first.ParentRequestId, first.AttemptId, supplierAccountingOperationsEnvelope())
	base := SupplierAccountingFactResolveInput{
		Action: SupplierAccountingFactResolveCaptureFromLog, SourceLogId: log.Id,
		Actor: "root:17", Reason: "verified retry", Evidence: "ticket-retry",
	}
	wrong := base
	wrong.AttemptId = second.AttemptId
	_, err := ResolvePendingSupplierAccountingFact(context.Background(), wrong)
	require.ErrorIs(t, err, ErrSupplierAccountingFactSourceLogMismatch)

	correct := base
	correct.AttemptId = first.AttemptId
	resolved, err := ResolvePendingSupplierAccountingFact(context.Background(), correct)
	require.NoError(t, err)
	require.Equal(t, model.SupplierAccountingFactStatusCaptured, resolved.Status)
}

func TestResolvePendingSupplierAccountingFactVoidConflictsWithDifferentAudit(t *testing.T) {
	useSupplierAccountingOperationsDB(t)
	fact := supplierAccountingOperationsFact(t, "req-void")
	input := SupplierAccountingFactResolveInput{
		AttemptId: fact.AttemptId, Action: SupplierAccountingFactResolveVoid,
		Actor: "root:17", Reason: "verified rejection", Evidence: "ticket-2",
	}
	_, err := ResolvePendingSupplierAccountingFact(context.Background(), input)
	require.NoError(t, err)
	_, err = ResolvePendingSupplierAccountingFact(context.Background(), input)
	require.NoError(t, err)
	input.Evidence = "different-ticket"
	_, err = ResolvePendingSupplierAccountingFact(context.Background(), input)
	require.ErrorIs(t, err, model.ErrSupplierAccountingFactTerminalConflict)
}

func TestResolvePendingSupplierAccountingFactRejectsMismatchedSourceLog(t *testing.T) {
	useSupplierAccountingOperationsDB(t)
	fact := supplierAccountingOperationsFact(t, "req-expected")
	log := createSupplierAccountingSourceLog(t, "req-other", fact.AttemptId, supplierAccountingOperationsEnvelope())
	_, err := ResolvePendingSupplierAccountingFact(context.Background(), SupplierAccountingFactResolveInput{
		AttemptId: fact.AttemptId, Action: SupplierAccountingFactResolveCaptureFromLog, SourceLogId: log.Id,
		Actor: "root:17", Reason: "verified", Evidence: "ticket-3",
	})
	require.ErrorIs(t, err, ErrSupplierAccountingFactSourceLogMismatch)
}

func TestResolvePendingSupplierAccountingFactRejectsMismatchedFrozenIdentity(t *testing.T) {
	useSupplierAccountingOperationsDB(t)
	fact := supplierAccountingOperationsFact(t, "req-identity-mismatch")
	envelope := supplierAccountingOperationsEnvelope()
	envelope.Captured.RateVersionId++
	log := createSupplierAccountingSourceLog(t, fact.ParentRequestId, fact.AttemptId, envelope)
	_, err := ResolvePendingSupplierAccountingFact(context.Background(), SupplierAccountingFactResolveInput{
		AttemptId: fact.AttemptId, Action: SupplierAccountingFactResolveCaptureFromLog, SourceLogId: log.Id,
		Actor: "root:17", Reason: "verified", Evidence: "ticket-identity",
	})
	require.ErrorIs(t, err, ErrSupplierAccountingFactSourceLogMismatch)
}

func TestResolvePendingSupplierAccountingFactRejectsMalformedEnvelope(t *testing.T) {
	useSupplierAccountingOperationsDB(t)
	fact := supplierAccountingOperationsFact(t, "req-malformed")
	log := model.Log{Type: model.LogTypeConsume, RequestId: fact.ParentRequestId, Other: `{"supplier_accounting_attempt_id":"` + fact.AttemptId + `","supplier_accounting_v1":{"v":1,"d":"captured","s":"broken"}}`}
	require.NoError(t, model.LOG_DB.Create(&log).Error)

	_, err := ResolvePendingSupplierAccountingFact(context.Background(), SupplierAccountingFactResolveInput{
		AttemptId: fact.AttemptId, Action: SupplierAccountingFactResolveCaptureFromLog, SourceLogId: log.Id,
		Actor: "root:17", Reason: "verified", Evidence: "ticket-4",
	})
	require.ErrorIs(t, err, ErrSupplierAccountingFactSourceLogInvalid)
}
