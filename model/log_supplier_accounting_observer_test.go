package model

import (
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type supplierAccountingWriteObservation struct {
	disposition types.SupplierAccountingDisposition
	outcome     SupplierAccountingConsumeLogWriteOutcome
}

func installSupplierAccountingObserverForTest(t *testing.T) *[]supplierAccountingWriteObservation {
	t.Helper()
	original := supplierAccountingConsumeLogWriteObserver.Load()
	supplierAccountingConsumeLogWriteObserver.Store(nil)
	observations := make([]supplierAccountingWriteObservation, 0, 1)
	require.True(t, InstallSupplierAccountingConsumeLogWriteObserver(func(disposition types.SupplierAccountingDisposition, outcome SupplierAccountingConsumeLogWriteOutcome) {
		observations = append(observations, supplierAccountingWriteObservation{disposition: disposition, outcome: outcome})
	}))
	require.False(t, InstallSupplierAccountingConsumeLogWriteObserver(func(types.SupplierAccountingDisposition, SupplierAccountingConsumeLogWriteOutcome) {}))
	t.Cleanup(func() { supplierAccountingConsumeLogWriteObserver.Store(original) })
	return &observations
}

func supplierAccountingObserverTestParams(envelope any) RecordConsumeLogParams {
	return RecordConsumeLogParams{Other: map[string]any{types.SupplierAccountingEnvelopeKeyV1: envelope}}
}

func supplierAccountingObserverTestEnvelope() types.SupplierAccountingEnvelopeV1 {
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

func supplierAccountingObserverTestContext() *gin.Context {
	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
	return ctx
}

func useSupplierAccountingObserverLogDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Log{}))
	original := LOG_DB
	LOG_DB = db
	t.Cleanup(func() { LOG_DB = original })
	return db
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

func TestRecordConsumeLogSupplierAccountingObserverSuccessAfterCreate(t *testing.T) {
	observations := installSupplierAccountingObserverForTest(t)
	db := useSupplierAccountingObserverLogDB(t)
	originalEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = true
	t.Cleanup(func() { common.LogConsumeEnabled = originalEnabled })

	RecordConsumeLog(supplierAccountingObserverTestContext(), 0, supplierAccountingObserverTestParams(supplierAccountingObserverTestEnvelope()))

	var count int64
	require.NoError(t, db.Model(&Log{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	require.Equal(t, []supplierAccountingWriteObservation{{types.SupplierAccountingDispositionCaptured, SupplierAccountingConsumeLogWriteSuccess}}, *observations)
}

func TestRecordConsumeLogSupplierAccountingObserverRunsAfterSuccessfulCreate(t *testing.T) {
	db := useSupplierAccountingObserverLogDB(t)
	originalObserver := supplierAccountingConsumeLogWriteObserver.Load()
	supplierAccountingConsumeLogWriteObserver.Store(nil)
	observedPersistedCount := int64(-1)
	require.True(t, InstallSupplierAccountingConsumeLogWriteObserver(func(types.SupplierAccountingDisposition, SupplierAccountingConsumeLogWriteOutcome) {
		require.NoError(t, db.Model(&Log{}).Count(&observedPersistedCount).Error)
	}))
	t.Cleanup(func() { supplierAccountingConsumeLogWriteObserver.Store(originalObserver) })
	originalEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = true
	t.Cleanup(func() { common.LogConsumeEnabled = originalEnabled })

	RecordConsumeLog(supplierAccountingObserverTestContext(), 0, supplierAccountingObserverTestParams(supplierAccountingObserverTestEnvelope()))

	require.EqualValues(t, 1, observedPersistedCount)
}

func TestRecordConsumeLogSupplierAccountingObserverFailureAfterCreate(t *testing.T) {
	observations := installSupplierAccountingObserverForTest(t)
	db := useSupplierAccountingObserverLogDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	originalEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = true
	t.Cleanup(func() { common.LogConsumeEnabled = originalEnabled })

	RecordConsumeLog(supplierAccountingObserverTestContext(), 0, supplierAccountingObserverTestParams(supplierAccountingObserverTestEnvelope()))

	require.Equal(t, []supplierAccountingWriteObservation{{types.SupplierAccountingDispositionCaptured, SupplierAccountingConsumeLogWriteFailure}}, *observations)
}

func TestRecordConsumeLogSupplierAccountingObserverFailureWhenOtherSerializationFails(t *testing.T) {
	observations := installSupplierAccountingObserverForTest(t)
	db := useSupplierAccountingObserverLogDB(t)
	originalEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = true
	t.Cleanup(func() { common.LogConsumeEnabled = originalEnabled })
	params := supplierAccountingObserverTestParams(supplierAccountingObserverTestEnvelope())
	params.Other["unserializable"] = make(chan struct{})

	RecordConsumeLog(supplierAccountingObserverTestContext(), 0, params)

	var logs []Log
	require.NoError(t, db.Find(&logs).Error)
	require.Len(t, logs, 1, "ordinary consume log persistence must survive Other serialization failure")
	requireSupplierAccountingEnvelopePersisted(t, logs[0].Other)
	require.Equal(t, []supplierAccountingWriteObservation{{types.SupplierAccountingDispositionCaptured, SupplierAccountingConsumeLogWriteFailure}}, *observations)
}

func TestRecordConsumeLogSupplierAccountingObserverOmitsInvalidEnvelopeAndPersistsOrdinaryLog(t *testing.T) {
	observations := installSupplierAccountingObserverForTest(t)
	db := useSupplierAccountingObserverLogDB(t)
	originalEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = true
	t.Cleanup(func() { common.LogConsumeEnabled = originalEnabled })
	envelope := supplierAccountingObserverTestEnvelope()
	envelope.Captured = nil

	RecordConsumeLog(supplierAccountingObserverTestContext(), 0, supplierAccountingObserverTestParams(envelope))

	var count int64
	require.NoError(t, db.Model(&Log{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	var persisted Log
	require.NoError(t, db.First(&persisted).Error)
	require.NotContains(t, persisted.Other, types.SupplierAccountingEnvelopeKeyV1)
	require.Empty(t, *observations)
}

func TestRecordConsumeLogSupplierAccountingObserverDisabled(t *testing.T) {
	observations := installSupplierAccountingObserverForTest(t)
	originalEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = false
	t.Cleanup(func() { common.LogConsumeEnabled = originalEnabled })

	RecordConsumeLog(supplierAccountingObserverTestContext(), 0, supplierAccountingObserverTestParams(supplierAccountingObserverTestEnvelope()))

	require.Equal(t, []supplierAccountingWriteObservation{{types.SupplierAccountingDispositionCaptured, SupplierAccountingConsumeLogWriteDisabled}}, *observations)
}

func TestRecordConsumeLogSupplierAccountingObserverIgnoresMissingInvalidAndNonV1(t *testing.T) {
	observations := installSupplierAccountingObserverForTest(t)
	originalEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = false
	t.Cleanup(func() { common.LogConsumeEnabled = originalEnabled })

	RecordConsumeLog(supplierAccountingObserverTestContext(), 0, RecordConsumeLogParams{})
	RecordConsumeLog(supplierAccountingObserverTestContext(), 0, supplierAccountingObserverTestParams(map[string]any{"d": "captured"}))
	nonV1 := supplierAccountingObserverTestEnvelope()
	nonV1.EnvelopeSchemaVersion++
	RecordConsumeLog(supplierAccountingObserverTestContext(), 0, supplierAccountingObserverTestParams(nonV1))
	invalidDisposition := supplierAccountingObserverTestEnvelope()
	invalidDisposition.Disposition = "request-controlled"
	RecordConsumeLog(supplierAccountingObserverTestContext(), 0, supplierAccountingObserverTestParams(invalidDisposition))

	require.Empty(t, *observations)
}

func TestRecordTaskBillingLogSupplierAccountingObserverOnlyConsume(t *testing.T) {
	observations := installSupplierAccountingObserverForTest(t)
	useSupplierAccountingObserverLogDB(t)
	originalEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = true
	t.Cleanup(func() { common.LogConsumeEnabled = originalEnabled })
	other := map[string]any{types.SupplierAccountingEnvelopeKeyV1: supplierAccountingObserverTestEnvelope()}

	RecordTaskBillingLog(RecordTaskBillingLogParams{LogType: LogTypeRefund, Other: other})
	require.Empty(t, *observations, "refund rows are outside supplier consume-log write metrics")
	RecordTaskBillingLog(RecordTaskBillingLogParams{LogType: LogTypeConsume, Other: other})
	require.Equal(t, []supplierAccountingWriteObservation{{types.SupplierAccountingDispositionCaptured, SupplierAccountingConsumeLogWriteSuccess}}, *observations)
}

func TestRecordTaskBillingLogSupplierAccountingObserverLeavesRefundSerializationBehaviorUnchanged(t *testing.T) {
	observations := installSupplierAccountingObserverForTest(t)
	db := useSupplierAccountingObserverLogDB(t)
	other := map[string]any{
		types.SupplierAccountingEnvelopeKeyV1: supplierAccountingObserverTestEnvelope(),
		"unserializable":                      make(chan struct{}),
	}

	RecordTaskBillingLog(RecordTaskBillingLogParams{LogType: LogTypeRefund, Other: other})

	var logs []Log
	require.NoError(t, db.Find(&logs).Error)
	require.Len(t, logs, 1)
	require.Empty(t, logs[0].Other, "refund rows retain the legacy serialization fallback")
	require.Empty(t, *observations, "refund rows remain outside supplier consume-log write metrics")
}

func TestRecordTaskBillingLogSupplierAccountingObserverFailureAfterCreate(t *testing.T) {
	observations := installSupplierAccountingObserverForTest(t)
	db := useSupplierAccountingObserverLogDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	originalEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = true
	t.Cleanup(func() { common.LogConsumeEnabled = originalEnabled })

	RecordTaskBillingLog(RecordTaskBillingLogParams{
		LogType: LogTypeConsume,
		Other:   map[string]any{types.SupplierAccountingEnvelopeKeyV1: supplierAccountingObserverTestEnvelope()},
	})

	require.Equal(t, []supplierAccountingWriteObservation{{types.SupplierAccountingDispositionCaptured, SupplierAccountingConsumeLogWriteFailure}}, *observations)
}

func TestRecordTaskBillingLogSupplierAccountingObserverDisabled(t *testing.T) {
	observations := installSupplierAccountingObserverForTest(t)
	originalEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = false
	t.Cleanup(func() { common.LogConsumeEnabled = originalEnabled })

	RecordTaskBillingLog(RecordTaskBillingLogParams{
		LogType: LogTypeConsume,
		Other:   map[string]any{types.SupplierAccountingEnvelopeKeyV1: supplierAccountingObserverTestEnvelope()},
	})

	require.Equal(t, []supplierAccountingWriteObservation{{types.SupplierAccountingDispositionCaptured, SupplierAccountingConsumeLogWriteDisabled}}, *observations)
}

func TestRecordTaskBillingLogSupplierAccountingObserverFailureWhenOtherSerializationFails(t *testing.T) {
	observations := installSupplierAccountingObserverForTest(t)
	db := useSupplierAccountingObserverLogDB(t)
	originalEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = true
	t.Cleanup(func() { common.LogConsumeEnabled = originalEnabled })
	other := map[string]any{
		types.SupplierAccountingEnvelopeKeyV1: supplierAccountingObserverTestEnvelope(),
		"unserializable":                      make(chan struct{}),
	}

	RecordTaskBillingLog(RecordTaskBillingLogParams{LogType: LogTypeConsume, Other: other})

	var logs []Log
	require.NoError(t, db.Find(&logs).Error)
	require.Len(t, logs, 1, "ordinary task billing log persistence must survive Other serialization failure")
	requireSupplierAccountingEnvelopePersisted(t, logs[0].Other)
	require.Equal(t, []supplierAccountingWriteObservation{{types.SupplierAccountingDispositionCaptured, SupplierAccountingConsumeLogWriteFailure}}, *observations)
}

func TestRecordTaskBillingLogSupplierAccountingObserverOmitsInvalidEnvelopeAndPersistsOrdinaryLog(t *testing.T) {
	observations := installSupplierAccountingObserverForTest(t)
	db := useSupplierAccountingObserverLogDB(t)
	originalEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = true
	t.Cleanup(func() { common.LogConsumeEnabled = originalEnabled })
	envelope := supplierAccountingObserverTestEnvelope()
	envelope.Captured = nil

	RecordTaskBillingLog(RecordTaskBillingLogParams{
		LogType: LogTypeConsume,
		Other:   map[string]any{types.SupplierAccountingEnvelopeKeyV1: envelope},
	})

	var count int64
	require.NoError(t, db.Model(&Log{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	var persisted Log
	require.NoError(t, db.First(&persisted).Error)
	require.NotContains(t, persisted.Other, types.SupplierAccountingEnvelopeKeyV1)
	require.Empty(t, *observations)
}

func TestRecordConsumeLogSupplierAccountingObserverConcurrentDisabled(t *testing.T) {
	originalObserver := supplierAccountingConsumeLogWriteObserver.Load()
	supplierAccountingConsumeLogWriteObserver.Store(nil)
	var observed atomic.Int64
	require.True(t, InstallSupplierAccountingConsumeLogWriteObserver(func(types.SupplierAccountingDisposition, SupplierAccountingConsumeLogWriteOutcome) {
		observed.Add(1)
	}))
	t.Cleanup(func() { supplierAccountingConsumeLogWriteObserver.Store(originalObserver) })
	originalEnabled := common.LogConsumeEnabled
	common.LogConsumeEnabled = false
	t.Cleanup(func() { common.LogConsumeEnabled = originalEnabled })

	const callers = 32
	var wait sync.WaitGroup
	for index := 0; index < callers; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			RecordConsumeLog(nil, 0, supplierAccountingObserverTestParams(supplierAccountingObserverTestEnvelope()))
		}()
	}
	wait.Wait()
	require.EqualValues(t, callers, observed.Load())
}
