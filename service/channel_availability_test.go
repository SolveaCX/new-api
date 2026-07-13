package service

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

func TestChannelConcurrencyLoadsDoNotReadCooldownForFiftyChannels(t *testing.T) {
	resetChannelConcurrencyForTest()
	restore, hook := useCountedRedisChannelConcurrencyForTest(t, 0)
	defer restore()

	channels := make([]*model.Channel, 0, 50)
	for i := 0; i < 50; i++ {
		channels = append(channels, &model.Channel{Id: 911000 + i, MaxConcurrency: 2})
	}
	_, err := GetChannelConcurrencyLoads(context.Background(), channels)
	require.NoError(t, err)
	require.Equal(t, 1, hook.CommandCount("time"))
	require.Equal(t, 50, hook.CommandCount("zremrangebyscore"))
	require.Equal(t, 50, hook.CommandCount("zcard"))
	require.Equal(t, 50, hook.CommandCount("get"))
	require.Zero(t, hook.CommandCount("exists"))
}

func TestChannelAvailabilityCoalescesConcurrentFiftyChannelReads(t *testing.T) {
	resetChannelConcurrencyForTest()
	restore, hook := useCountedRedisChannelConcurrencyForTest(t, 50*time.Millisecond)
	defer restore()

	channels := make([]*model.Channel, 0, 50)
	for i := 0; i < 50; i++ {
		channels = append(channels, &model.Channel{Id: 912000 + i, MaxConcurrency: 1})
	}

	const callers = 100
	start := make(chan struct{})
	errs := make(chan error, callers)
	var wg sync.WaitGroup
	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			_, err := GetChannelConcurrencyCooldowns(context.Background(), channels)
			errs <- err
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, 50, hook.CommandCount("exists"))
}

func TestChannelAvailabilityMarkPrimesPositiveCache(t *testing.T) {
	resetChannelConcurrencyForTest()
	restore, hook := useCountedRedisChannelConcurrencyForTest(t, 0)
	defer restore()

	const channelID = 913001
	require.NoError(t, MarkChannelConcurrencyCooldown(context.Background(), channelID, time.Second, "test cooldown"))
	hook.Reset()

	coolingDown, err := IsChannelConcurrencyCoolingDown(context.Background(), channelID)
	require.NoError(t, err)
	require.True(t, coolingDown)
	require.Zero(t, hook.CommandCount("exists"))
}

func TestChannelAvailabilityInFlightReadCannotOverwriteNewCooldown(t *testing.T) {
	resetChannelConcurrencyForTest()
	mr := miniredis.RunT(t)
	gate := &redisAvailabilityPipelineGate{
		reached: make(chan struct{}),
		release: make(chan struct{}),
	}
	prevRDB := common.RDB
	prevRedisEnabled := common.RedisEnabled
	common.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	common.RDB.AddHook(gate)
	common.RedisEnabled = true
	defer func() {
		_ = common.RDB.Close()
		common.RDB = prevRDB
		common.RedisEnabled = prevRedisEnabled
		mr.Close()
		resetChannelConcurrencyForTest()
	}()

	const channelID = 913004
	type availabilityResult struct {
		coolingDown bool
		err         error
	}
	result := make(chan availabilityResult, 1)
	go func() {
		coolingDown, err := IsChannelConcurrencyCoolingDown(context.Background(), channelID)
		result <- availabilityResult{coolingDown: coolingDown, err: err}
	}()

	<-gate.reached
	require.NoError(t, MarkChannelConcurrencyCooldown(context.Background(), channelID, time.Second, "new cooldown"))
	close(gate.release)

	read := <-result
	require.NoError(t, read.err)
	require.True(t, read.coolingDown)
	coolingDown, err := IsChannelConcurrencyCoolingDown(context.Background(), channelID)
	require.NoError(t, err)
	require.True(t, coolingDown)
}

func TestChannelAvailabilityRefreshesExpiredCacheFromRedis(t *testing.T) {
	resetChannelConcurrencyForTest()
	restore, hook := useCountedRedisChannelConcurrencyForTest(t, 0)
	defer restore()

	originalTTL := channelAvailabilityCacheTTL
	channelAvailabilityCacheTTL = 20 * time.Millisecond
	defer func() { channelAvailabilityCacheTTL = originalTTL }()

	const channelID = 913002
	key := channelConcurrencyCooldownRedisKey(channelID)
	require.NoError(t, common.RDB.Set(context.Background(), key, "remote cooldown", time.Second).Err())
	hook.Reset()

	coolingDown, err := IsChannelConcurrencyCoolingDown(context.Background(), channelID)
	require.NoError(t, err)
	require.True(t, coolingDown)
	require.Equal(t, 1, hook.CommandCount("exists"))

	require.NoError(t, common.RDB.Del(context.Background(), key).Err())
	coolingDown, err = IsChannelConcurrencyCoolingDown(context.Background(), channelID)
	require.NoError(t, err)
	require.True(t, coolingDown)
	require.Equal(t, 1, hook.CommandCount("exists"))

	time.Sleep(30 * time.Millisecond)
	coolingDown, err = IsChannelConcurrencyCoolingDown(context.Background(), channelID)
	require.NoError(t, err)
	require.False(t, coolingDown)
	require.Equal(t, 2, hook.CommandCount("exists"))
}

func TestChannelAvailabilitySkipsUnlimitedChannels(t *testing.T) {
	resetChannelConcurrencyForTest()
	restore, hook := useCountedRedisChannelConcurrencyForTest(t, 0)
	defer restore()

	coolingDown, err := GetChannelConcurrencyCooldowns(context.Background(), []*model.Channel{
		nil,
		{Id: 913003, MaxConcurrency: 0},
		{Id: 0, MaxConcurrency: 1},
	})
	require.NoError(t, err)
	require.Empty(t, coolingDown)
	require.Empty(t, hook.Commands())
}

func useCountedRedisChannelConcurrencyForTest(t *testing.T, pipelineDelay time.Duration) (func(), *redisCommandCounterHook) {
	t.Helper()
	mr := miniredis.RunT(t)
	hook := &redisCommandCounterHook{pipelineDelay: pipelineDelay}
	prevRDB := common.RDB
	prevRedisEnabled := common.RedisEnabled
	common.RDB = redis.NewClient(&redis.Options{Addr: mr.Addr()})
	common.RDB.AddHook(hook)
	common.RedisEnabled = true
	return func() {
		_ = common.RDB.Close()
		common.RDB = prevRDB
		common.RedisEnabled = prevRedisEnabled
		mr.Close()
		resetChannelConcurrencyForTest()
	}, hook
}

type redisAvailabilityPipelineGate struct {
	once    sync.Once
	reached chan struct{}
	release chan struct{}
}

func (g *redisAvailabilityPipelineGate) BeforeProcess(ctx context.Context, _ redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (g *redisAvailabilityPipelineGate) AfterProcess(context.Context, redis.Cmder) error {
	return nil
}

func (g *redisAvailabilityPipelineGate) BeforeProcessPipeline(ctx context.Context, _ []redis.Cmder) (context.Context, error) {
	return ctx, nil
}

func (g *redisAvailabilityPipelineGate) AfterProcessPipeline(_ context.Context, commands []redis.Cmder) error {
	for _, command := range commands {
		if command.Name() != "exists" {
			continue
		}
		g.once.Do(func() { close(g.reached) })
		<-g.release
		break
	}
	return nil
}
