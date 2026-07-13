package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/stretchr/testify/require"
)

func TestChannelConcurrencyWaitAdmissionRejectsWithoutIncrement(t *testing.T) {
	resetChannelConcurrencyForTest()
	restore := useRedisChannelConcurrencyForTest(t)
	defer restore()

	ctx := context.Background()
	channelID := 910001
	first, admitted, count, err := acquireChannelConcurrencyWaiting(ctx, channelID, 1)
	require.NoError(t, err)
	require.True(t, admitted)
	require.Equal(t, 1, count)

	second, admitted, count, err := acquireChannelConcurrencyWaiting(ctx, channelID, 1)
	require.NoError(t, err)
	require.False(t, admitted)
	require.Nil(t, second)
	require.Equal(t, 1, count)

	key := channelConcurrencyWaitingRedisKey(channelID)
	stored, err := common.RDB.Get(ctx, key).Int()
	require.NoError(t, err)
	require.Equal(t, 1, stored)
	ttl, err := common.RDB.PTTL(ctx, key).Result()
	require.NoError(t, err)
	require.Greater(t, ttl, time.Duration(0))
	require.NoError(t, first.Release(ctx))
}

func TestChannelConcurrencyWaitAdmissionNeverExceedsLimit(t *testing.T) {
	resetChannelConcurrencyForTest()
	restore := useRedisChannelConcurrencyForTest(t)
	defer restore()

	const maxWaiting = 3
	const callers = 32
	type admissionResult struct {
		lease    *channelConcurrencyWaitingLease
		admitted bool
		count    int
		err      error
	}

	var wg sync.WaitGroup
	results := make(chan admissionResult, callers)
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			lease, admitted, count, err := acquireChannelConcurrencyWaiting(context.Background(), 910002, maxWaiting)
			results <- admissionResult{lease: lease, admitted: admitted, count: count, err: err}
		}()
	}
	wg.Wait()
	close(results)

	leases := make([]*channelConcurrencyWaitingLease, 0, maxWaiting)
	for result := range results {
		require.NoError(t, result.err)
		require.LessOrEqual(t, result.count, maxWaiting)
		if result.admitted {
			require.NotNil(t, result.lease)
			leases = append(leases, result.lease)
		} else {
			require.Nil(t, result.lease)
		}
	}
	require.Len(t, leases, maxWaiting)

	for _, lease := range leases {
		require.NoError(t, lease.Release(context.Background()))
	}
	require.Equal(t, int64(0), common.RDB.Exists(context.Background(), channelConcurrencyWaitingRedisKey(910002)).Val())
}

func TestChannelConcurrencyWaitReleaseMissingKeyNeverCreatesNegativeCount(t *testing.T) {
	resetChannelConcurrencyForTest()
	restore := useRedisChannelConcurrencyForTest(t)
	defer restore()

	lease := &channelConcurrencyWaitingLease{channelID: 910003, useRedis: true}
	require.NoError(t, lease.Release(context.Background()))
	require.NoError(t, lease.Release(context.Background()))
	require.Equal(t, int64(0), common.RDB.Exists(context.Background(), channelConcurrencyWaitingRedisKey(910003)).Val())
}
