package perfmetrics

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/config"
	"github.com/QuantumNous/new-api/setting/perf_metrics_setting"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestClassifyAvailabilityOutcome(t *testing.T) {
	tests := []struct {
		name    string
		success bool
		err     *types.NewAPIError
		want    AvailabilityOutcome
	}{
		{name: "success", success: true, want: AvailabilityEligibleSuccess},
		{name: "final failure without detail", want: AvailabilityEligibleFailure},
		{name: "upstream timeout", err: types.NewError(context.DeadlineExceeded, types.ErrorCodeDoRequestFailed), want: AvailabilityEligibleFailure},
		{name: "upstream server error", err: types.NewErrorWithStatusCode(errors.New("unavailable"), types.ErrorCodeBadResponseStatusCode, http.StatusServiceUnavailable), want: AvailabilityEligibleFailure},
		{name: "upstream bad response", err: types.NewError(errors.New("bad body"), types.ErrorCodeBadResponseBody), want: AvailabilityEligibleFailure},
		{name: "router exhausted", err: types.NewError(errors.New("no channel"), types.ErrorCodeGetChannelFailed), want: AvailabilityEligibleFailure},
		{name: "channel auth", err: types.NewErrorWithStatusCode(errors.New("bad upstream key"), types.ErrorCodeChannelInvalidKey, http.StatusUnauthorized), want: AvailabilityEligibleFailure},
		{name: "upstream rate limit", err: types.NewErrorWithStatusCode(errors.New("limited"), types.ErrorCodeBadResponseStatusCode, http.StatusTooManyRequests), want: AvailabilityEligibleFailure},
		{name: "client cancellation", err: types.NewError(context.Canceled, types.ErrorCodeDoRequestFailed), want: AvailabilityExcluded},
		{name: "invalid request", err: types.NewErrorWithStatusCode(errors.New("invalid"), types.ErrorCodeInvalidRequest, http.StatusBadRequest), want: AvailabilityExcluded},
		{name: "user quota", err: types.NewErrorWithStatusCode(errors.New("quota"), types.ErrorCodeInsufficientUserQuota, http.StatusForbidden), want: AvailabilityExcluded},
		{name: "policy rejection", err: types.NewErrorWithStatusCode(errors.New("blocked"), types.ErrorCodeAccessDenied, http.StatusForbidden), want: AvailabilityExcluded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, ClassifyAvailabilityOutcome(tt.success, tt.err))
		})
	}
}

func TestAtomicAvailabilityBucketKeepsSuccessPairedWithEligibleAcrossDrain(t *testing.T) {
	var bucket atomicAvailabilityBucket
	addReady := make(chan struct{})
	continueSuccessfulAdd := make(chan struct{})
	addDone := make(chan struct{})

	go func() {
		close(addReady)
		<-continueSuccessfulAdd
		bucket.add(AvailabilityEligibleSuccess)
		close(addDone)
	}()

	// This drain occupies the interleaving point that used to split eligible
	// from success. The packed add now linearizes wholly before or after it.
	<-addReady
	first := bucket.drain()
	close(continueSuccessfulAdd)
	<-addDone
	second := bucket.drain()

	require.LessOrEqual(t, first.success, first.eligible)
	require.LessOrEqual(t, second.success, second.eligible)
}

func TestAtomicAvailabilityBucketRejectsInvalidOrOverflowingRestore(t *testing.T) {
	var bucket atomicAvailabilityBucket
	require.False(t, bucket.addCounters(availabilityCounters{eligible: 1, success: 2}))
	require.True(t, bucket.addCounters(availabilityCounters{eligible: 1<<32 - 1, success: 1<<32 - 1}))
	require.False(t, bucket.addCounters(availabilityCounters{eligible: 1, success: 1}))
	require.Equal(t, availabilityCounters{eligible: 1<<32 - 1, success: 1<<32 - 1}, bucket.snapshot())
}

func TestAvailabilityCountersPackRoundTrip(t *testing.T) {
	counters := availabilityCounters{eligible: 42, success: 39}
	require.Equal(t, counters, unpackAvailabilityCounters(packAvailabilityCounters(counters)))
}

func TestRecordRelaySampleCountsAvailabilityOnce(t *testing.T) {
	resetPerfMetricsStateForTest(t)

	RecordRelaySample(&relaycommon.RelayInfo{
		OriginModelName: "gpt-5",
		UsingGroup:      "default",
		StartTime:       time.Now().Add(-time.Second),
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 42},
	}, false, 0, types.NewError(context.DeadlineExceeded, types.ErrorCodeDoRequestFailed))

	var snapshot counters
	hotBuckets.Range(func(_, value any) bool {
		snapshot = value.(*atomicBucket).snapshot()
		return false
	})
	require.EqualValues(t, 1, snapshot.availabilityEligibleCount)
	require.Zero(t, snapshot.availabilitySuccessCount)
}

func TestRecordRelaySampleExcludesClientGoneFromAvailability(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	streamStatus := relaycommon.NewStreamStatus()
	streamStatus.SetEndReason(relaycommon.StreamEndReasonClientGone, nil)

	RecordRelaySample(&relaycommon.RelayInfo{
		OriginModelName: "gpt-5",
		UsingGroup:      "default",
		StartTime:       time.Now().Add(-time.Second),
		IsStream:        true,
		StreamStatus:    streamStatus,
		ChannelMeta:     &relaycommon.ChannelMeta{ChannelId: 42},
	}, true, 0, nil)

	var snapshot counters
	hotBuckets.Range(func(_, value any) bool {
		snapshot = value.(*atomicBucket).snapshot()
		return false
	})
	require.Zero(t, snapshot.availabilityEligibleCount)
	require.Zero(t, snapshot.availabilitySuccessCount)
}

func TestAvailabilitySignalUsesFixedFiveMinuteBucketsAndFlushesOnce(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.PerfMetricAvailability{}))
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })

	recordAvailabilityAt(Sample{Model: "gpt-5", Group: "default", Availability: AvailabilityEligibleFailure}, 301)
	recordAvailabilityAt(Sample{Model: "gpt-5", Group: "default", Availability: AvailabilityEligibleSuccess}, 599)

	var bucketStarts []int64
	availabilityHotBuckets.Range(func(key, _ any) bool {
		bucketStarts = append(bucketStarts, key.(availabilityBucketKey).bucketTs)
		return true
	})
	require.Equal(t, []int64{300}, bucketStarts)

	flushCompletedAvailabilityBuckets(600)
	flushCompletedAvailabilityBuckets(600)
	summaries, err := model.GetPerfMetricAvailabilitySummaryAll(300, 599, []string{"default"})
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	require.EqualValues(t, 2, summaries[0].AvailabilityEligibleCount)
	require.EqualValues(t, 1, summaries[0].AvailabilitySuccessCount)
}

func TestAvailabilityCollectionRunsWhenGenericPerfMetricsAreDisabled(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	setGenericPerfMetricsEnabledForTest(t, false)
	statusAvailabilityEnabled.Store(true)

	Record(Sample{Model: "gpt-5", Group: "default", Availability: AvailabilityEligibleSuccess})

	require.Equal(t, 1, syncMapEntryCount(&availabilityHotBuckets))
	require.Zero(t, syncMapEntryCount(&hotBuckets))
}

func TestAvailabilityCollectionDoesNoWorkWhenStatusCenterIsDisabled(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	setGenericPerfMetricsEnabledForTest(t, true)
	statusAvailabilityEnabled.Store(false)

	Record(Sample{Model: "gpt-5", Group: "default", Availability: AvailabilityEligibleSuccess})

	require.Zero(t, syncMapEntryCount(&availabilityHotBuckets))
	require.Equal(t, 1, syncMapEntryCount(&hotBuckets))
}

func TestAvailabilityCleanupUsesFixedSevenDayRetentionAndBoundedCadence(t *testing.T) {
	resetPerfMetricsStateForTest(t)
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.PerfMetricAvailability{}))
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })

	now := int64(10 * 24 * 60 * 60)
	cutoff := now - availabilityRetentionSeconds
	require.NoError(t, db.Create(&[]model.PerfMetricAvailability{
		{ModelName: "old", Group: "default", BucketTs: cutoff - 1, EligibleCount: 1},
		{ModelName: "kept", Group: "default", BucketTs: cutoff + availabilityCleanupIntervalSeconds, EligibleCount: 1},
	}).Error)

	flushCompletedAvailabilityBuckets(now)
	require.EqualValues(t, 1, countAvailabilityRows(t, db))

	require.NoError(t, db.Create(&model.PerfMetricAvailability{ModelName: "old-again", Group: "default", BucketTs: cutoff - 2, EligibleCount: 1}).Error)
	flushCompletedAvailabilityBuckets(now + availabilityCleanupIntervalSeconds - 1)
	require.EqualValues(t, 2, countAvailabilityRows(t, db), "cleanup must not run on every five-second flush")
	flushCompletedAvailabilityBuckets(now + availabilityCleanupIntervalSeconds)
	require.EqualValues(t, 1, countAvailabilityRows(t, db))
}

func setGenericPerfMetricsEnabledForTest(t *testing.T, enabled bool) {
	t.Helper()
	previous := perf_metrics_setting.GetSetting().Enabled
	t.Cleanup(func() {
		require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
			"perf_metrics_setting.enabled": strconv.FormatBool(previous),
		}))
	})
	require.NoError(t, config.GlobalConfig.LoadFromDB(map[string]string{
		"perf_metrics_setting.enabled": strconv.FormatBool(enabled),
	}))
}

func syncMapEntryCount(values *sync.Map) int {
	count := 0
	values.Range(func(_, _ any) bool {
		count++
		return true
	})
	return count
}

func countAvailabilityRows(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&model.PerfMetricAvailability{}).Count(&count).Error)
	return count
}
