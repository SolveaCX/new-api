package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestChannelConcurrencyBatchFetchesUseAtMostTwoRedisConnections(t *testing.T) {
	resetChannelConcurrencyForTest()
	restore, hook := useCountedRedisChannelConcurrencyForTest(t, 75*time.Millisecond)
	defer restore()

	const fetches = 12
	start := make(chan struct{})
	errs := make(chan error, fetches)
	var wg sync.WaitGroup
	for i := 0; i < fetches; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			<-start
			channel := &model.Channel{Id: 914000 + index, MaxConcurrency: 1}
			if index%2 == 0 {
				_, err := GetChannelConcurrencyLoads(context.Background(), []*model.Channel{channel})
				errs <- err
				return
			}
			_, err := GetChannelConcurrencyCooldowns(context.Background(), []*model.Channel{channel})
			errs <- err
		}(i)
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, 2, hook.MaxActivePipelines())
	require.Equal(t, 6, hook.CommandCount("time"))
	require.Equal(t, 6, hook.CommandCount("exists"))
}

func TestFreshChannelConcurrencyLoadsUseIndependentFiveHundredMillisecondThrottle(t *testing.T) {
	resetChannelConcurrencyForTest()
	restore, hook := useCountedRedisChannelConcurrencyForTest(t, 0)
	defer restore()

	oldNormal := channelConcurrencyLoadCacheTTL
	oldFresh := channelConcurrencyFreshLoadCacheTTL
	channelConcurrencyLoadCacheTTL = 0
	channelConcurrencyFreshLoadCacheTTL = 50 * time.Millisecond
	defer func() {
		channelConcurrencyLoadCacheTTL = oldNormal
		channelConcurrencyFreshLoadCacheTTL = oldFresh
	}()

	channels := []*model.Channel{{Id: 914100, MaxConcurrency: 1}}
	_, err := getChannelConcurrencyLoadsFreshThrottled(context.Background(), channels)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	_, err = getChannelConcurrencyLoadsFreshThrottled(context.Background(), channels)
	require.NoError(t, err)
	require.Equal(t, 1, hook.CommandCount("time"))
}
