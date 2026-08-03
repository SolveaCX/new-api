package service

import (
	"context"
	"errors"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/model"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestReleaseChannelConcurrencyRetriesThenSucceeds(t *testing.T) {
	resetChannelConcurrencyForTest()
	restore := useRedisChannelConcurrencyForTest(t)
	defer restore()

	channel := &model.Channel{Id: 910101, MaxConcurrency: 1}
	lease, ok, err := TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, lease)

	original := removeRedisChannelConcurrencySlot
	defer func() { removeRedisChannelConcurrencySlot = original }()
	var calls int
	removeRedisChannelConcurrencySlot = func(ctx context.Context, channelID int, token string) error {
		calls++
		if calls == 1 {
			return errors.New("transient zrem failure")
		}
		return original(ctx, channelID, token)
	}

	require.NoError(t, ReleaseChannelConcurrency(context.Background(), lease))
	require.Equal(t, 2, calls)
	require.True(t, lease.released.Load())
}

func TestReleaseChannelConcurrencyForContextRetainsLeaseOnFailure(t *testing.T) {
	resetChannelConcurrencyForTest()
	restore := useRedisChannelConcurrencyForTest(t)
	defer restore()

	channel := &model.Channel{Id: 910102, MaxConcurrency: 1}
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	ok, err := AcquireChannelConcurrencyForContext(c, channel)
	require.NoError(t, err)
	require.True(t, ok)
	lease := getChannelConcurrencyLeaseForContext(c)
	require.NotNil(t, lease)

	original := removeRedisChannelConcurrencySlot
	defer func() { removeRedisChannelConcurrencySlot = original }()
	removeRedisChannelConcurrencySlot = func(context.Context, int, string) error {
		return errors.New("persistent zrem failure")
	}

	require.Error(t, ReleaseChannelConcurrencyForContext(c))
	require.Same(t, lease, getChannelConcurrencyLeaseForContext(c))
	require.False(t, lease.released.Load())

	removeRedisChannelConcurrencySlot = original
	require.NoError(t, ReleaseChannelConcurrencyForContext(c))
	require.Nil(t, getChannelConcurrencyLeaseForContext(c))
}

func TestReleaseChannelConcurrencyConcurrentFailuresDoNotReportSuccess(t *testing.T) {
	resetChannelConcurrencyForTest()
	restore := useRedisChannelConcurrencyForTest(t)
	defer restore()

	channel := &model.Channel{Id: 910103, MaxConcurrency: 1}
	lease, ok, err := TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, lease)

	originalRemove := removeRedisChannelConcurrencySlot
	originalBackoffs := channelConcurrencyReleaseBackoffs
	defer func() {
		removeRedisChannelConcurrencySlot = originalRemove
		channelConcurrencyReleaseBackoffs = originalBackoffs
	}()
	channelConcurrencyReleaseBackoffs = nil
	var calls atomic.Int32
	removeRedisChannelConcurrencySlot = func(context.Context, int, string) error {
		calls.Add(1)
		return errors.New("persistent zrem failure")
	}

	start := make(chan struct{})
	errs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			errs <- ReleaseChannelConcurrency(context.Background(), lease)
		}()
	}
	close(start)
	wg.Wait()
	close(errs)

	for releaseErr := range errs {
		require.Error(t, releaseErr)
	}
	require.Equal(t, int32(2), calls.Load())
	require.False(t, lease.released.Load())

	removeRedisChannelConcurrencySlot = originalRemove
	require.NoError(t, ReleaseChannelConcurrency(context.Background(), lease))
}

func TestReleaseChannelConcurrencyStopsRenewalOnceAfterTerminalFailure(t *testing.T) {
	originalRemove := removeRedisChannelConcurrencySlot
	originalBackoffs := channelConcurrencyReleaseBackoffs
	defer func() {
		removeRedisChannelConcurrencySlot = originalRemove
		channelConcurrencyReleaseBackoffs = originalBackoffs
	}()
	channelConcurrencyReleaseBackoffs = nil
	removeRedisChannelConcurrencySlot = func(context.Context, int, string) error {
		return errors.New("persistent zrem failure")
	}

	var stops atomic.Int32
	lease := &ChannelConcurrencyLease{
		ChannelID: 910104,
		token:     "release-stop-test",
		useRedis:  true,
		renewCancel: func() {
			stops.Add(1)
		},
	}

	require.Error(t, ReleaseChannelConcurrency(context.Background(), lease))
	require.Error(t, ReleaseChannelConcurrency(context.Background(), lease))
	require.Equal(t, int32(1), stops.Load())
	require.False(t, lease.released.Load())
}

func TestReleaseChannelConcurrencyUsesDetachedContext(t *testing.T) {
	resetChannelConcurrencyForTest()
	restore := useRedisChannelConcurrencyForTest(t)
	defer restore()

	channel := &model.Channel{Id: 910105, MaxConcurrency: 1}
	lease, ok, err := TryAcquireChannelConcurrency(context.Background(), channel)
	require.NoError(t, err)
	require.True(t, ok)
	require.NotNil(t, lease)

	original := removeRedisChannelConcurrencySlot
	defer func() { removeRedisChannelConcurrencySlot = original }()
	var removerContextErr error
	removeRedisChannelConcurrencySlot = func(ctx context.Context, channelID int, token string) error {
		removerContextErr = ctx.Err()
		return original(ctx, channelID, token)
	}

	canceledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	require.NoError(t, ReleaseChannelConcurrency(canceledCtx, lease))
	require.NoError(t, removerContextErr)
	require.True(t, lease.released.Load())
}
