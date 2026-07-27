package controller

import (
	"errors"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func useRelaySupplierAccountingFactDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, model.EnsureSupplierAccountingFactSchema(db))
	original := model.LOG_DB
	model.LOG_DB = db
	t.Cleanup(func() { model.LOG_DB = original })
	return db
}

func relaySupplierAccountingContext() *gin.Context {
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", nil)
	return c
}

func relaySupplierAccountingInfo(t *testing.T) *relaycommon.RelayInfo {
	t.Helper()
	location, err := time.LoadLocation(service.SupplierDailyBatchTimezone)
	require.NoError(t, err)
	yesterday := time.Now().In(location).AddDate(0, 0, -1)
	cutover := time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, location)
	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", fmt.Sprintf("%d", cutover.Unix()))
	return &relaycommon.RelayInfo{
		RequestId:       "req-parent",
		OriginModelName: "gpt-test",
		SupplierCostSnapshot: types.SupplierCostSnapshot{
			BindingVersionId: 11,
			SupplierId:       12,
			ContractId:       13,
			RateVersionId:    14,
		},
	}
}

func relaySupplierAccountingEnvelope() types.SupplierAccountingEnvelopeV1 {
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

func relaySupplierAccountingFixedEnvelope() types.SupplierAccountingEnvelopeV1 {
	envelope := relaySupplierAccountingEnvelope()
	envelope.Captured.PricingProvenance = &types.SupplierPricingProvenanceV1{
		Fixed: &types.SupplierFixedPricingProvenanceV1{
			Source: "price_data", Key: "model_price", PriceVersion: 1,
			GroupMultiplierPpm: *envelope.Captured.SalesMultiplierPpm, GroupRatioVersion: 1,
		},
	}
	return envelope
}

func relaySupplierAccountingUpstreamError() *types.NewAPIError {
	return types.NewError(errors.New("upstream failed"), types.ErrorCodeDoRequestFailed)
}

func TestSynchronousRelaySupplierAccountingPreparePersistenceFailureDoesNotBlockHandler(t *testing.T) {
	db := useRelaySupplierAccountingFactDB(t)
	sqlDB, sqlErr := db.DB()
	require.NoError(t, sqlErr)
	require.NoError(t, sqlDB.Close())

	called := false
	err := runSynchronousRelayAttempt(relaySupplierAccountingContext(), relaySupplierAccountingInfo(t), 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
		called = true
		return nil
	})
	require.Nil(t, err)
	require.True(t, called)
}

func TestRealtimeSupplierAccountingPreparePersistenceFailureDoesNotBlockHandler(t *testing.T) {
	db := useRelaySupplierAccountingFactDB(t)
	sqlDB, sqlErr := db.DB()
	require.NoError(t, sqlErr)
	require.NoError(t, sqlDB.Close())

	called := false
	err := runSynchronousRelayAttempt(relaySupplierAccountingContext(), relaySupplierAccountingInfo(t), 15, types.RelayFormatOpenAIRealtime, func() *types.NewAPIError {
		called = true
		return nil
	})
	require.Nil(t, err)
	require.True(t, called)
}

func TestSynchronousRelaySupplierAccountingCapturePersistenceFailureDoesNotReplaceSuccess(t *testing.T) {
	db := useRelaySupplierAccountingFactDB(t)
	c := relaySupplierAccountingContext()
	info := relaySupplierAccountingInfo(t)

	err := runSynchronousRelayAttempt(c, info, 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
		sqlDB, sqlErr := db.DB()
		require.NoError(t, sqlErr)
		require.NoError(t, sqlDB.Close())
		finalizeErr := service.FinalizeSupplierAccountingAttempt(c, info, relaySupplierAccountingEnvelope())
		require.ErrorIs(t, finalizeErr, service.ErrSupplierAccountingFactPersistence)
		return nil
	})
	require.Nil(t, err)
}

func TestSynchronousRelaySupplierAccountingVoidPersistenceFailurePreservesOriginalError(t *testing.T) {
	db := useRelaySupplierAccountingFactDB(t)
	originalErr := types.NewError(errors.New("invalid converted request"), types.ErrorCodeConvertRequestFailed)

	err := runSynchronousRelayAttempt(relaySupplierAccountingContext(), relaySupplierAccountingInfo(t), 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
		sqlDB, sqlErr := db.DB()
		require.NoError(t, sqlErr)
		require.NoError(t, sqlDB.Close())
		return originalErr
	})
	require.Same(t, originalErr, err)
}

func TestSynchronousRelaySupplierAccountingInvalidPrepareIdentityRemainsFailClosed(t *testing.T) {
	useRelaySupplierAccountingFactDB(t)
	info := relaySupplierAccountingInfo(t)
	info.RequestId = ""
	called := false

	err := runSynchronousRelayAttempt(relaySupplierAccountingContext(), info, 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
		called = true
		return nil
	})
	require.NotNil(t, err)
	require.ErrorIs(t, err, model.ErrSupplierAccountingFactResolutionInvalid)
	require.False(t, called)
}

func TestSynchronousRelaySupplierAccountingMissingFactRemainsFailClosed(t *testing.T) {
	db := useRelaySupplierAccountingFactDB(t)
	c := relaySupplierAccountingContext()
	info := relaySupplierAccountingInfo(t)

	err := runSynchronousRelayAttempt(c, info, 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
		require.NoError(t, db.Where("attempt_id = ?", c.GetString(types.SupplierAccountingAttemptIDKeyV1)).Delete(&model.SupplierAccountingFact{}).Error)
		finalizeErr := service.FinalizeSupplierAccountingAttempt(c, info, relaySupplierAccountingEnvelope())
		require.ErrorIs(t, finalizeErr, model.ErrSupplierAccountingFactNotFound)
		return nil
	})
	require.NotNil(t, err)
	require.ErrorIs(t, err, model.ErrSupplierAccountingFactNotFound)
}

func TestSynchronousRelaySupplierAccountingTerminalConflictRemainsFailClosed(t *testing.T) {
	db := useRelaySupplierAccountingFactDB(t)
	c := relaySupplierAccountingContext()
	info := relaySupplierAccountingInfo(t)

	err := runSynchronousRelayAttempt(c, info, 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
		require.NoError(t, db.Model(&model.SupplierAccountingFact{}).
			Where("attempt_id = ?", c.GetString(types.SupplierAccountingAttemptIDKeyV1)).
			Update("status", model.SupplierAccountingFactStatusVoid).Error)
		finalizeErr := service.FinalizeSupplierAccountingAttempt(c, info, relaySupplierAccountingEnvelope())
		require.ErrorIs(t, finalizeErr, model.ErrSupplierAccountingFactTerminalConflict)
		return nil
	})
	require.NotNil(t, err)
	require.ErrorIs(t, err, model.ErrSupplierAccountingFactTerminalConflict)
}

func TestSupplierAccountingMalformedClaimedBindingPreventsHandler(t *testing.T) {
	tests := map[string]func(*relaycommon.RelayInfo) int{
		"missing binding version": func(info *relaycommon.RelayInfo) int {
			info.SupplierCostSnapshot.BindingVersionId = 0
			return 15
		},
		"missing supplier": func(info *relaycommon.RelayInfo) int {
			info.SupplierCostSnapshot.SupplierId = 0
			return 15
		},
		"missing contract": func(info *relaycommon.RelayInfo) int {
			info.SupplierCostSnapshot.ContractId = 0
			return 15
		},
		"missing rate version": func(info *relaycommon.RelayInfo) int {
			info.SupplierCostSnapshot.RateVersionId = 0
			return 15
		},
		"missing channel": func(info *relaycommon.RelayInfo) int { return 0 },
		"missing model": func(info *relaycommon.RelayInfo) int {
			info.OriginModelName = ""
			return 15
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			useRelaySupplierAccountingFactDB(t)
			called := false
			info := relaySupplierAccountingInfo(t)
			channelID := mutate(info)
			err := runSynchronousRelayAttempt(relaySupplierAccountingContext(), info, channelID, types.RelayFormatOpenAI, func() *types.NewAPIError {
				called = true
				return nil
			})
			require.NotNil(t, err)
			require.ErrorIs(t, err, service.ErrSupplierAccountingAttemptBindingInvalid)
			require.False(t, called)
		})
	}
}

func TestSupplierAccountingActivatedCacheUnavailablePreventsHandler(t *testing.T) {
	db := useRelaySupplierAccountingFactDB(t)
	info := relaySupplierAccountingInfo(t)
	info.SupplierCostSnapshot = types.SupplierCostSnapshot{CacheUnavailable: true}
	called := false

	relayErr := runSynchronousRelayAttempt(relaySupplierAccountingContext(), info, 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
		called = true
		return nil
	})
	require.NotNil(t, relayErr)
	require.ErrorIs(t, relayErr, service.ErrSupplierAccountingAttemptBindingInvalid)
	require.False(t, called)

	var count int64
	require.NoError(t, db.Model(&model.SupplierAccountingFact{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestSupplierAccountingMalformedBindingBeforeCutoverKeepsLegacyTraffic(t *testing.T) {
	db := useRelaySupplierAccountingFactDB(t)
	info := relaySupplierAccountingInfo(t)
	info.SupplierCostSnapshot.BindingVersionId = 0
	location, err := time.LoadLocation(service.SupplierDailyBatchTimezone)
	require.NoError(t, err)
	future := time.Date(2100, 1, 1, 0, 0, 0, 0, location)
	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", fmt.Sprintf("%d", future.Unix()))
	called := false

	relayErr := runSynchronousRelayAttempt(relaySupplierAccountingContext(), info, 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
		called = true
		return nil
	})
	require.Nil(t, relayErr)
	require.True(t, called)
	var count int64
	require.NoError(t, db.Model(&model.SupplierAccountingFact{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestSupplierAccountingBeforeCutoverKeepsLegacyTrafficWithoutIntent(t *testing.T) {
	db := useRelaySupplierAccountingFactDB(t)
	info := relaySupplierAccountingInfo(t)
	location, err := time.LoadLocation(service.SupplierDailyBatchTimezone)
	require.NoError(t, err)
	future := time.Date(2100, 1, 1, 0, 0, 0, 0, location)
	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", fmt.Sprintf("%d", future.Unix()))
	info.SupplierCostSnapshot = types.SupplierCostSnapshot{CacheUnavailable: true}
	called := false

	relayErr := runSynchronousRelayAttempt(relaySupplierAccountingContext(), info, 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
		called = true
		return nil
	})
	require.Nil(t, relayErr)
	require.True(t, called)
	var count int64
	require.NoError(t, db.Model(&model.SupplierAccountingFact{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestSupplierAccountingBoundAttemptUsesLogDatabaseCutoverClock(t *testing.T) {
	db := useRelaySupplierAccountingFactDB(t)
	info := relaySupplierAccountingInfo(t)
	location, err := time.LoadLocation(service.SupplierDailyBatchTimezone)
	require.NoError(t, err)
	future := time.Date(2100, 1, 1, 0, 0, 0, 0, location)
	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", fmt.Sprintf("%d", future.Unix()))
	called := false

	relayErr := runSynchronousRelayAttempt(relaySupplierAccountingContext(), info, 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
		called = true
		return nil
	})
	require.Nil(t, relayErr)
	require.True(t, called)
	var count int64
	require.NoError(t, db.Model(&model.SupplierAccountingFact{}).Count(&count).Error)
	require.Zero(t, count)
}

func TestSupplierAccountingNonMidnightCutoverFailsBeforeHandler(t *testing.T) {
	info := relaySupplierAccountingInfo(t)
	location, err := time.LoadLocation(service.SupplierDailyBatchTimezone)
	require.NoError(t, err)
	nonMidnight := time.Date(2026, 7, 25, 12, 0, 0, 0, location)
	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", fmt.Sprintf("%d", nonMidnight.Unix()))
	called := false

	relayErr := runSynchronousRelayAttempt(relaySupplierAccountingContext(), info, 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
		called = true
		return nil
	})
	require.NotNil(t, relayErr)
	require.False(t, called)
}

func TestSupplierAccountingUnsetCutoverDisablesAuthoritativePrepare(t *testing.T) {
	original := model.LOG_DB
	model.LOG_DB = nil
	t.Cleanup(func() { model.LOG_DB = original })
	info := relaySupplierAccountingInfo(t)
	t.Setenv("SUPPLIER_ACCOUNTING_CUTOVER_AT", "")
	called := false

	relayErr := runSynchronousRelayAttempt(relaySupplierAccountingContext(), info, 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
		called = true
		return nil
	})
	require.Nil(t, relayErr)
	require.True(t, called)
}

func TestSynchronousRelaySupplierAccountingRetriesUseDistinctIntents(t *testing.T) {
	db := useRelaySupplierAccountingFactDB(t)
	c := relaySupplierAccountingContext()
	info := relaySupplierAccountingInfo(t)

	for retry := 0; retry < 2; retry++ {
		info.RetryIndex = retry
		require.NotNil(t, runSynchronousRelayAttempt(c, info, 15, types.RelayFormatOpenAI, relaySupplierAccountingUpstreamError))
	}

	var facts []model.SupplierAccountingFact
	require.NoError(t, db.Order("id ASC").Find(&facts).Error)
	require.Len(t, facts, 2)
	require.NotEqual(t, facts[0].AttemptId, facts[1].AttemptId)
	require.Equal(t, []int{0, 1}, []int{facts[0].RetryIndex, facts[1].RetryIndex})
	require.Equal(t, model.SupplierAccountingFactStatusPending, facts[0].Status)
	require.Equal(t, model.SupplierAccountingFactStatusPending, facts[1].Status)
}

func TestSynchronousRelaySupplierAccountingFinalSuccessCaptured(t *testing.T) {
	db := useRelaySupplierAccountingFactDB(t)
	c := relaySupplierAccountingContext()
	info := relaySupplierAccountingInfo(t)

	err := runSynchronousRelayAttempt(c, info, 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
		require.NoError(t, service.FinalizeSupplierAccountingAttempt(c, info, relaySupplierAccountingEnvelope()))
		return nil
	})
	require.Nil(t, err)

	var fact model.SupplierAccountingFact
	require.NoError(t, db.First(&fact).Error)
	require.Equal(t, model.SupplierAccountingFactStatusCaptured, fact.Status)
	require.NotEmpty(t, fact.Payload)
	require.Equal(t, fact.AttemptId, c.GetString(types.SupplierAccountingAttemptIDKeyV1))
}

func TestRealtimeSupplierAccountingFinalSuccessCaptured(t *testing.T) {
	db := useRelaySupplierAccountingFactDB(t)
	c := relaySupplierAccountingContext()
	info := relaySupplierAccountingInfo(t)

	err := runSynchronousRelayAttempt(c, info, 15, types.RelayFormatOpenAIRealtime, func() *types.NewAPIError {
		info.IsStream = true
		info.StreamStatus = relaycommon.NewStreamStatus()
		info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonDone, nil)
		require.NoError(t, service.FinalizeSupplierAccountingAttempt(c, info, relaySupplierAccountingEnvelope()))
		return nil
	})
	require.Nil(t, err)

	var fact model.SupplierAccountingFact
	require.NoError(t, db.First(&fact).Error)
	require.Equal(t, model.SupplierAccountingFactStatusCaptured, fact.Status)
}

func TestSynchronousRelaySupplierAccountingCapturesFixedAndDynamicPricingSnapshots(t *testing.T) {
	for _, testCase := range []struct {
		name     string
		envelope types.SupplierAccountingEnvelopeV1
	}{
		{name: "dynamic_ratio", envelope: relaySupplierAccountingEnvelope()},
		{name: "fixed_price", envelope: relaySupplierAccountingFixedEnvelope()},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			db := useRelaySupplierAccountingFactDB(t)
			c := relaySupplierAccountingContext()
			info := relaySupplierAccountingInfo(t)
			err := runSynchronousRelayAttempt(c, info, 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
				require.NoError(t, service.FinalizeSupplierAccountingAttempt(c, info, testCase.envelope))
				return nil
			})
			require.Nil(t, err)

			var fact model.SupplierAccountingFact
			require.NoError(t, db.First(&fact).Error)
			require.Equal(t, model.SupplierAccountingFactStatusCaptured, fact.Status)
			require.NotEmpty(t, fact.PayloadHash)
		})
	}
}

func TestSynchronousRelaySupplierAccountingPreDispatchFailuresVoid(t *testing.T) {
	for _, errorCode := range []types.ErrorCode{
		types.ErrorCodeReadRequestBodyFailed,
		types.ErrorCodeChannelModelMappedError,
		types.ErrorCodeConvertRequestFailed,
		types.ErrorCodeChannelParamOverrideInvalid,
	} {
		t.Run(string(errorCode), func(t *testing.T) {
			db := useRelaySupplierAccountingFactDB(t)
			err := runSynchronousRelayAttempt(relaySupplierAccountingContext(), relaySupplierAccountingInfo(t), 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
				return types.NewError(errors.New("pre-dispatch failure"), errorCode)
			})
			require.NotNil(t, err)

			var fact model.SupplierAccountingFact
			require.NoError(t, db.First(&fact).Error)
			require.Equal(t, model.SupplierAccountingFactStatusVoid, fact.Status)
		})
	}
}

func TestSynchronousRelaySupplierAccountingPostDispatchFailuresStayPending(t *testing.T) {
	for _, errorCode := range []types.ErrorCode{
		types.ErrorCodeDoRequestFailed,
		types.ErrorCodeBadResponseStatusCode,
		types.ErrorCodeBadResponseBody,
		types.ErrorCodeJsonMarshalFailed,
	} {
		t.Run(string(errorCode), func(t *testing.T) {
			db := useRelaySupplierAccountingFactDB(t)
			err := runSynchronousRelayAttempt(relaySupplierAccountingContext(), relaySupplierAccountingInfo(t), 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
				return types.NewError(errors.New("post-dispatch outcome unknown"), errorCode)
			})
			require.NotNil(t, err)

			var fact model.SupplierAccountingFact
			require.NoError(t, db.First(&fact).Error)
			require.Equal(t, model.SupplierAccountingFactStatusPending, fact.Status)
		})
	}
}

func TestSynchronousRelaySupplierAccountingSuccessfulZeroUsageVoids(t *testing.T) {
	db := useRelaySupplierAccountingFactDB(t)
	c := relaySupplierAccountingContext()
	info := relaySupplierAccountingInfo(t)

	err := runSynchronousRelayAttempt(c, info, 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
		require.NoError(t, service.FinalizeSupplierAccountingAttempt(c, info, types.SupplierAccountingEnvelopeV1{
			EnvelopeSchemaVersion: types.SupplierAccountingEnvelopeSchemaVersionV1,
			Disposition:           types.SupplierAccountingDispositionZeroUsage,
		}))
		return nil
	})
	require.Nil(t, err)

	var fact model.SupplierAccountingFact
	require.NoError(t, db.First(&fact).Error)
	require.Equal(t, model.SupplierAccountingFactStatusVoid, fact.Status)
}

func TestSynchronousRelaySupplierAccountingAmbiguousOutcomesStayPendingAndVisible(t *testing.T) {
	for _, disposition := range []types.SupplierAccountingDisposition{
		types.SupplierAccountingDispositionProducerError,
		types.SupplierAccountingDispositionNotFinanciallyCommitted,
	} {
		t.Run(string(disposition), func(t *testing.T) {
			db := useRelaySupplierAccountingFactDB(t)
			c := relaySupplierAccountingContext()
			info := relaySupplierAccountingInfo(t)

			err := runSynchronousRelayAttempt(c, info, 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
				terminalErr := service.FinalizeSupplierAccountingAttempt(c, info, types.SupplierAccountingEnvelopeV1{
					EnvelopeSchemaVersion: types.SupplierAccountingEnvelopeSchemaVersionV1,
					Disposition:           disposition,
				})
				require.ErrorIs(t, terminalErr, service.ErrSupplierAccountingAttemptTerminalAmbiguous)
				return nil
			})
			require.NotNil(t, err)
			require.Equal(t, types.ErrorCodeUpdateDataError, err.GetErrorCode())

			var fact model.SupplierAccountingFact
			require.NoError(t, db.First(&fact).Error)
			require.Equal(t, model.SupplierAccountingFactStatusPending, fact.Status)
		})
	}
}

func TestSynchronousRelaySupplierAccountingPartialResponseStaysPending(t *testing.T) {
	db := useRelaySupplierAccountingFactDB(t)
	c := relaySupplierAccountingContext()
	err := runSynchronousRelayAttempt(c, relaySupplierAccountingInfo(t), 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
		c.Writer.WriteHeader(200)
		_, _ = c.Writer.Write([]byte("partial"))
		return relaySupplierAccountingUpstreamError()
	})
	require.NotNil(t, err)

	var fact model.SupplierAccountingFact
	require.NoError(t, db.First(&fact).Error)
	require.Equal(t, model.SupplierAccountingFactStatusPending, fact.Status)
}

func TestRealtimeSupplierAccountingUpstreamHandshakeFailureStaysPending(t *testing.T) {
	db := useRelaySupplierAccountingFactDB(t)
	c := relaySupplierAccountingContext()
	c.Writer.WriteHeader(101)

	err := runSynchronousRelayAttempt(c, relaySupplierAccountingInfo(t), 15, types.RelayFormatOpenAIRealtime, relaySupplierAccountingUpstreamError)
	require.NotNil(t, err)

	var fact model.SupplierAccountingFact
	require.NoError(t, db.First(&fact).Error)
	require.Equal(t, model.SupplierAccountingFactStatusPending, fact.Status)
}

func TestRealtimeSupplierAccountingPartialSessionStaysPending(t *testing.T) {
	db := useRelaySupplierAccountingFactDB(t)
	c := relaySupplierAccountingContext()
	c.Writer.WriteHeader(101)
	info := relaySupplierAccountingInfo(t)

	err := runSynchronousRelayAttempt(c, info, 15, types.RelayFormatOpenAIRealtime, func() *types.NewAPIError {
		info.TargetWs = &websocket.Conn{}
		return relaySupplierAccountingUpstreamError()
	})
	require.NotNil(t, err)

	var fact model.SupplierAccountingFact
	require.NoError(t, db.First(&fact).Error)
	require.Equal(t, model.SupplierAccountingFactStatusPending, fact.Status)
}

func TestSynchronousRelaySupplierAccountingUnknownStreamOutcomeStaysPending(t *testing.T) {
	db := useRelaySupplierAccountingFactDB(t)
	c := relaySupplierAccountingContext()
	info := relaySupplierAccountingInfo(t)
	info.IsStream = true
	info.StreamStatus = relaycommon.NewStreamStatus()
	info.StreamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, nil)

	err := runSynchronousRelayAttempt(c, info, 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
		require.NoError(t, service.FinalizeSupplierAccountingAttempt(c, info, relaySupplierAccountingEnvelope()))
		return nil
	})
	require.Nil(t, err)

	var fact model.SupplierAccountingFact
	require.NoError(t, db.First(&fact).Error)
	require.Equal(t, model.SupplierAccountingFactStatusPending, fact.Status)
}

func TestSynchronousRelaySupplierAccountingUnboundChannelCreatesNoIntent(t *testing.T) {
	original := model.LOG_DB
	model.LOG_DB = nil
	t.Cleanup(func() { model.LOG_DB = original })

	called := false
	info := relaySupplierAccountingInfo(t)
	info.SupplierCostSnapshot = types.SupplierCostSnapshot{}
	err := runSynchronousRelayAttempt(relaySupplierAccountingContext(), info, 15, types.RelayFormatOpenAI, func() *types.NewAPIError {
		called = true
		return nil
	})
	require.Nil(t, err)
	require.True(t, called)
}
