package model

import (
	"bytes"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func supplierAccountingLogTestParams(envelope any) RecordConsumeLogParams {
	return RecordConsumeLogParams{Other: map[string]any{types.SupplierAccountingEnvelopeKeyV1: envelope}}
}

func supplierAccountingLogTestEnvelope() types.SupplierAccountingEnvelopeV1 {
	officialListMicroUSD := int64(10)
	procurementMultiplierPPM := int64(400_000)
	procurementCostMicroUSD := int64(4)
	salesMultiplierPPM := int64(1)
	salesMicroUSD := int64(10)
	grossProfitMicroUSD := int64(6)
	return types.SupplierAccountingEnvelopeV1{
		EnvelopeSchemaVersion: types.SupplierAccountingEnvelopeSchemaVersionV1,
		Disposition:           types.SupplierAccountingDispositionCaptured,
		Captured: &types.SupplierAccountingLogSnapshotV1{
			BindingVersionId:         1,
			SupplierId:               1,
			ContractId:               1,
			RateVersionId:            1,
			ProcurementMultiplierPpm: procurementMultiplierPPM,
			SalesMultiplierPpm:       &salesMultiplierPPM,
			OfficialListMicroUsd:     &officialListMicroUSD,
			SalesMicroUsd:            &salesMicroUSD,
			ProcurementCostMicroUsd:  &procurementCostMicroUSD,
			GrossProfitMicroUsd:      &grossProfitMicroUSD,
			StatisticsScope:          string(types.SupplierStatisticsScopeBusiness),
			ExclusionDecision:        "included",
			FinanciallyCommittedAt:   1,
			PricingProvenance: &types.SupplierPricingProvenanceV1{
				Fixed: &types.SupplierFixedPricingProvenanceV1{
					Source:             "price_data",
					Key:                "model_price",
					PriceVersion:       1,
					GroupMultiplierPpm: salesMultiplierPPM,
					GroupRatioVersion:  1,
				},
			},
		},
	}
}

func supplierAccountingLogTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	return ctx
}

func useSupplierAccountingLogDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	original := LOG_DB
	LOG_DB = db
	t.Cleanup(func() { LOG_DB = original })
	return db
}

func setConsumeLogEnabledForTest(t *testing.T, enabled bool) {
	t.Helper()
	original := common.LogConsumeEnabled
	common.LogConsumeEnabled = enabled
	t.Cleanup(func() { common.LogConsumeEnabled = original })
}

func decodePersistedLogOther(t *testing.T, other string) map[string]any {
	t.Helper()
	var decoded map[string]any
	require.NoError(t, common.Unmarshal([]byte(other), &decoded))
	return decoded
}

func requireSupplierAccountingEnvelopePersisted(t *testing.T, other string) {
	t.Helper()
	require.NotEmpty(t, other)
	var decoded map[string]types.SupplierAccountingEnvelopeV1
	require.NoError(t, common.Unmarshal([]byte(other), &decoded))
	require.Len(t, decoded, 1, "serialization fallback must retain only the supplier envelope")
	envelope, ok := decoded[types.SupplierAccountingEnvelopeKeyV1]
	require.True(t, ok)
	require.Equal(t, types.SupplierAccountingDispositionCaptured, envelope.Disposition)
}

func TestRecordConsumeLogPreservesValidSupplierAccountingEnvelope(t *testing.T) {
	db := useSupplierAccountingLogDB(t)
	setConsumeLogEnabledForTest(t, true)
	params := supplierAccountingLogTestParams(supplierAccountingLogTestEnvelope())
	params.Other["ordinary"] = "visible"

	RecordConsumeLog(supplierAccountingLogTestContext(), 0, params)

	var persisted Log
	require.NoError(t, db.First(&persisted).Error)
	decoded := decodePersistedLogOther(t, persisted.Other)
	require.Equal(t, "visible", decoded["ordinary"])
	require.Contains(t, decoded, types.SupplierAccountingEnvelopeKeyV1)
}

func TestRecordConsumeLogPersistsAttemptIDAndScrubsItFromNonRootViews(t *testing.T) {
	db := useSupplierAccountingLogDB(t)
	setConsumeLogEnabledForTest(t, true)
	params := supplierAccountingLogTestParams(supplierAccountingLogTestEnvelope())
	ctx := supplierAccountingLogTestContext()
	ctx.Set(types.SupplierAccountingAttemptIDKeyV1, "018f843e-7e3a-7f61-a0a0-000000000777")

	RecordConsumeLog(ctx, 0, params)
	var persisted Log
	require.NoError(t, db.First(&persisted).Error)
	decoded := decodePersistedLogOther(t, persisted.Other)
	require.Equal(t, "018f843e-7e3a-7f61-a0a0-000000000777", decoded[types.SupplierAccountingAttemptIDKeyV1])

	adminLog := persisted
	RedactSupplierAccountingFromLogs([]*Log{&adminLog})
	require.NotContains(t, adminLog.Other, types.SupplierAccountingAttemptIDKeyV1)
	require.NotContains(t, adminLog.Other, types.SupplierAccountingEnvelopeKeyV1)
	userLog := persisted
	formatUserLogs([]*Log{&userLog}, 0)
	require.NotContains(t, userLog.Other, types.SupplierAccountingAttemptIDKeyV1)
	require.NotContains(t, userLog.Other, types.SupplierAccountingEnvelopeKeyV1)
}

func TestRecordConsumeLogPreservesEnvelopeWhenUnrelatedOtherCannotSerialize(t *testing.T) {
	db := useSupplierAccountingLogDB(t)
	setConsumeLogEnabledForTest(t, true)
	params := supplierAccountingLogTestParams(supplierAccountingLogTestEnvelope())
	params.Other["unserializable"] = make(chan struct{})

	RecordConsumeLog(supplierAccountingLogTestContext(), 0, params)

	var persisted Log
	require.NoError(t, db.First(&persisted).Error)
	requireSupplierAccountingEnvelopePersisted(t, persisted.Other)
}

func TestRecordConsumeLogOmitsInvalidSupplierAccountingEnvelope(t *testing.T) {
	db := useSupplierAccountingLogDB(t)
	setConsumeLogEnabledForTest(t, true)
	envelope := supplierAccountingLogTestEnvelope()
	envelope.Captured = nil
	params := supplierAccountingLogTestParams(envelope)
	params.Other["ordinary"] = "visible"

	RecordConsumeLog(supplierAccountingLogTestContext(), 0, params)

	var persisted Log
	require.NoError(t, db.First(&persisted).Error)
	decoded := decodePersistedLogOther(t, persisted.Other)
	require.Equal(t, "visible", decoded["ordinary"])
	require.NotContains(t, decoded, types.SupplierAccountingEnvelopeKeyV1)
}

func TestRecordConsumeLogKeepsOrdinaryDisabledAndDiagnosticSemantics(t *testing.T) {
	t.Run("ordinary other persists", func(t *testing.T) {
		db := useSupplierAccountingLogDB(t)
		setConsumeLogEnabledForTest(t, true)

		RecordConsumeLog(supplierAccountingLogTestContext(), 0, RecordConsumeLogParams{Other: map[string]any{"ordinary": "visible"}})

		var persisted Log
		require.NoError(t, db.First(&persisted).Error)
		require.Equal(t, "visible", decodePersistedLogOther(t, persisted.Other)["ordinary"])
	})

	t.Run("disabled consume log writes nothing", func(t *testing.T) {
		db := useSupplierAccountingLogDB(t)
		setConsumeLogEnabledForTest(t, false)

		RecordConsumeLog(supplierAccountingLogTestContext(), 0, supplierAccountingLogTestParams(supplierAccountingLogTestEnvelope()))

		var count int64
		require.NoError(t, db.Model(&Log{}).Count(&count).Error)
		require.Zero(t, count)
	})

	t.Run("diagnostic output redacts supplier envelope", func(t *testing.T) {
		db := useSupplierAccountingLogDB(t)
		setConsumeLogEnabledForTest(t, true)
		var output bytes.Buffer
		common.LogWriterMu.Lock()
		originalWriter := gin.DefaultWriter
		gin.DefaultWriter = &output
		common.LogWriterMu.Unlock()
		t.Cleanup(func() {
			common.LogWriterMu.Lock()
			gin.DefaultWriter = originalWriter
			common.LogWriterMu.Unlock()
		})
		params := supplierAccountingLogTestParams(supplierAccountingLogTestEnvelope())
		params.Other["ordinary"] = "visible"

		RecordConsumeLog(supplierAccountingLogTestContext(), 0, params)

		require.NotContains(t, output.String(), types.SupplierAccountingEnvelopeKeyV1)
		require.Contains(t, output.String(), `"ordinary":"visible"`)
		var persisted Log
		require.NoError(t, db.First(&persisted).Error)
		require.Contains(t, persisted.Other, types.SupplierAccountingEnvelopeKeyV1)
	})
}

func TestRecordTaskBillingLogPreservesOnlyValidConsumeEnvelope(t *testing.T) {
	t.Run("valid envelope survives unrelated serialization failure", func(t *testing.T) {
		db := useSupplierAccountingLogDB(t)
		setConsumeLogEnabledForTest(t, true)
		other := map[string]any{
			types.SupplierAccountingEnvelopeKeyV1: supplierAccountingLogTestEnvelope(),
			"unserializable":                      make(chan struct{}),
		}

		RecordTaskBillingLog(RecordTaskBillingLogParams{LogType: LogTypeConsume, Other: other})

		var persisted Log
		require.NoError(t, db.First(&persisted).Error)
		requireSupplierAccountingEnvelopePersisted(t, persisted.Other)
	})

	t.Run("invalid envelope is omitted", func(t *testing.T) {
		db := useSupplierAccountingLogDB(t)
		setConsumeLogEnabledForTest(t, true)
		envelope := supplierAccountingLogTestEnvelope()
		envelope.Captured = nil

		RecordTaskBillingLog(RecordTaskBillingLogParams{
			LogType: LogTypeConsume,
			Other: map[string]any{
				types.SupplierAccountingEnvelopeKeyV1: envelope,
				"ordinary":                            "visible",
			},
		})

		var persisted Log
		require.NoError(t, db.First(&persisted).Error)
		decoded := decodePersistedLogOther(t, persisted.Other)
		require.Equal(t, "visible", decoded["ordinary"])
		require.NotContains(t, decoded, types.SupplierAccountingEnvelopeKeyV1)
	})
}

func TestRecordTaskBillingLogKeepsDisabledAndRefundSemantics(t *testing.T) {
	t.Run("disabled consume log writes nothing", func(t *testing.T) {
		db := useSupplierAccountingLogDB(t)
		setConsumeLogEnabledForTest(t, false)

		RecordTaskBillingLog(RecordTaskBillingLogParams{
			LogType: LogTypeConsume,
			Other:   map[string]any{types.SupplierAccountingEnvelopeKeyV1: supplierAccountingLogTestEnvelope()},
		})

		var count int64
		require.NoError(t, db.Model(&Log{}).Count(&count).Error)
		require.Zero(t, count)
	})

	t.Run("refund omits supplier envelope and preserves ordinary other", func(t *testing.T) {
		db := useSupplierAccountingLogDB(t)
		other := map[string]any{
			types.SupplierAccountingEnvelopeKeyV1: supplierAccountingLogTestEnvelope(),
			"ordinary":                            "visible",
		}

		RecordTaskBillingLog(RecordTaskBillingLogParams{LogType: LogTypeRefund, Other: other})

		var persisted Log
		require.NoError(t, db.First(&persisted).Error)
		decoded := decodePersistedLogOther(t, persisted.Other)
		require.Equal(t, "visible", decoded["ordinary"])
		require.NotContains(t, decoded, types.SupplierAccountingEnvelopeKeyV1)
	})

	t.Run("refund keeps legacy serialization failure fallback", func(t *testing.T) {
		db := useSupplierAccountingLogDB(t)
		other := map[string]any{
			types.SupplierAccountingEnvelopeKeyV1: supplierAccountingLogTestEnvelope(),
			"unserializable":                      make(chan struct{}),
		}

		RecordTaskBillingLog(RecordTaskBillingLogParams{LogType: LogTypeRefund, Other: other})

		var persisted Log
		require.NoError(t, db.First(&persisted).Error)
		require.Empty(t, persisted.Other)
	})
}
