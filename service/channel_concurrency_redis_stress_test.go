package service

import (
	"context"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/require"
)

const (
	channelConcurrencyStressChannelCount = 50
	channelConcurrencyStressMaxSlots     = 8
)

var channelConcurrencyStressLatencyLimits = [...]time.Duration{
	time.Millisecond,
	2 * time.Millisecond,
	5 * time.Millisecond,
	10 * time.Millisecond,
	20 * time.Millisecond,
	50 * time.Millisecond,
	100 * time.Millisecond,
	250 * time.Millisecond,
	500 * time.Millisecond,
	time.Second,
}

type channelConcurrencyStressLatency struct {
	buckets    [len(channelConcurrencyStressLatencyLimits) + 1]atomic.Int64
	count      atomic.Int64
	totalNanos atomic.Int64
	maxNanos   atomic.Int64
}

func (latency *channelConcurrencyStressLatency) observe(value time.Duration) {
	bucket := len(channelConcurrencyStressLatencyLimits)
	for index, limit := range channelConcurrencyStressLatencyLimits {
		if value <= limit {
			bucket = index
			break
		}
	}
	latency.buckets[bucket].Add(1)
	latency.count.Add(1)
	latency.totalNanos.Add(value.Nanoseconds())
	updateChannelConcurrencyStressMax(&latency.maxNanos, value.Nanoseconds())
}

func (latency *channelConcurrencyStressLatency) percentile(numerator int64, denominator int64) time.Duration {
	total := latency.count.Load()
	if total == 0 || numerator <= 0 || denominator <= 0 {
		return 0
	}
	target := (total*numerator + denominator - 1) / denominator
	var observed int64
	for index := range latency.buckets {
		observed += latency.buckets[index].Load()
		if observed < target {
			continue
		}
		if index < len(channelConcurrencyStressLatencyLimits) {
			return channelConcurrencyStressLatencyLimits[index]
		}
		return time.Duration(latency.maxNanos.Load())
	}
	return time.Duration(latency.maxNanos.Load())
}

func (latency *channelConcurrencyStressLatency) average() time.Duration {
	count := latency.count.Load()
	if count == 0 {
		return 0
	}
	return time.Duration(latency.totalNanos.Load() / count)
}

func TestChannelConcurrencyRealRedisFiftyChannelPressure(t *testing.T) {
	addr := os.Getenv("CHANNEL_CONCURRENCY_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("CHANNEL_CONCURRENCY_TEST_REDIS_ADDR is not set")
	}
	if os.Getenv("CHANNEL_CONCURRENCY_STRESS_ACK") != "test-redis-only" {
		t.Fatal("set CHANNEL_CONCURRENCY_STRESS_ACK=test-redis-only after confirming this is disposable test Redis")
	}

	duration := 30 * time.Second
	if raw := os.Getenv("CHANNEL_CONCURRENCY_STRESS_DURATION"); raw != "" {
		parsed, err := time.ParseDuration(raw)
		require.NoError(t, err)
		require.Positive(t, parsed)
		duration = parsed
	}
	workers := channelConcurrencyStressPositiveInt(t, "CHANNEL_CONCURRENCY_STRESS_WORKERS", 100)
	poolSize := channelConcurrencyStressPositiveInt(t, "CHANNEL_CONCURRENCY_STRESS_POOL_SIZE", 10)

	client := redis.NewClient(&redis.Options{Addr: addr, PoolSize: poolSize})
	require.NoError(t, client.Ping(context.Background()).Err())
	previousRDB, previousEnabled := common.RDB, common.RedisEnabled
	common.RDB, common.RedisEnabled = client, true
	resetChannelConcurrencyForTest()
	defer func() {
		common.RDB, common.RedisEnabled = previousRDB, previousEnabled
		_ = client.Close()
		resetChannelConcurrencyForTest()
	}()

	baseID := int(time.Now().UnixNano()%1_000_000) + 8_000_000
	channels := make([]*model.Channel, channelConcurrencyStressChannelCount)
	for index := range channels {
		channels[index] = &model.Channel{Id: baseID + index, MaxConcurrency: channelConcurrencyStressMaxSlots}
	}
	require.NoError(t, cleanupChannelConcurrencyStressKeys(context.Background(), client, channels))
	defer func() {
		if err := cleanupChannelConcurrencyStressKeys(context.Background(), client, channels); err != nil {
			t.Errorf("cleanup channel concurrency stress keys: %v", err)
		}
	}()

	poolStatsBefore := client.PoolStats()
	goroutinesBefore := runtime.NumGoroutine()
	ctx, cancel := context.WithTimeout(context.Background(), duration)
	defer cancel()

	var operations atomic.Int64
	var failures atomic.Int64
	var rejected atomic.Int64
	var overAllocated atomic.Int64
	var selectionLatency channelConcurrencyStressLatency
	active := make([]atomic.Int64, len(channels))
	peakActive := make([]atomic.Int64, len(channels))
	activeMu := make([]sync.Mutex, len(channels))

	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(seed int64) {
			defer wg.Done()
			rng := rand.New(rand.NewSource(seed))
			for ctx.Err() == nil {
				selectionStarted := time.Now()
				if _, err := GetChannelConcurrencyLoads(ctx, channels); err != nil {
					if ctx.Err() != nil {
						return
					}
					failures.Add(1)
					continue
				}

				channelIndex := rng.Intn(len(channels))
				if rng.Intn(4) == 0 {
					channelIndex = 0
				}
				lease, ok, err := TryAcquireChannelConcurrency(ctx, channels[channelIndex])
				selectionLatency.observe(time.Since(selectionStarted))
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					failures.Add(1)
					continue
				}
				if !ok || lease == nil {
					rejected.Add(1)
					operations.Add(1)
					continue
				}

				activeMu[channelIndex].Lock()
				current := active[channelIndex].Add(1)
				updateChannelConcurrencyStressMax(&peakActive[channelIndex], current)
				if current > channelConcurrencyStressMaxSlots {
					overAllocated.Add(1)
				}
				activeMu[channelIndex].Unlock()
				time.Sleep(time.Duration(rng.Intn(3)+1) * time.Millisecond)
				activeMu[channelIndex].Lock()
				if err := ReleaseChannelConcurrency(context.Background(), lease); err != nil {
					failures.Add(1)
				} else {
					active[channelIndex].Add(-1)
				}
				activeMu[channelIndex].Unlock()
				operations.Add(1)
			}
		}(int64(worker + 1))
	}
	wg.Wait()

	poolStatsAfter := client.PoolStats()
	poolTimeouts := poolStatsAfter.Timeouts - poolStatsBefore.Timeouts
	selectionP99 := selectionLatency.percentile(99, 100)
	var highestActive int64
	for index := range peakActive {
		if value := peakActive[index].Load(); value > highestActive {
			highestActive = value
		}
	}

	time.Sleep(100 * time.Millisecond)
	goroutinesAfter := runtime.NumGoroutine()
	t.Logf(
		"operations=%d duration=%s ops_per_second=%.2f rejected=%d failures=%d pool_timeouts=%d selection_avg=%s selection_p99<=%s selection_max=%s peak_active=%d goroutines_before=%d goroutines_after=%d",
		operations.Load(),
		duration,
		float64(operations.Load())/duration.Seconds(),
		rejected.Load(),
		failures.Load(),
		poolTimeouts,
		selectionLatency.average(),
		selectionP99,
		time.Duration(selectionLatency.maxNanos.Load()),
		highestActive,
		goroutinesBefore,
		goroutinesAfter,
	)

	require.Positive(t, operations.Load())
	require.Zero(t, failures.Load())
	require.Zero(t, overAllocated.Load())
	require.Zero(t, poolTimeouts)
	require.LessOrEqual(t, selectionP99, 10*time.Millisecond)
	require.LessOrEqual(t, goroutinesAfter, goroutinesBefore+10)
	assertChannelConcurrencyStressStateClean(t, client, channels)
}

func TestCleanupChannelConcurrencyStressKeysOnlyDeletesGeneratedChannels(t *testing.T) {
	server := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: server.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	ctx := context.Background()
	channels := []*model.Channel{{Id: 918000}, {Id: 918001}}
	for _, channel := range channels {
		require.NoError(t, client.Set(ctx, channelConcurrencyRedisKey(channel.Id), "slot", 0).Err())
		require.NoError(t, client.Set(ctx, channelConcurrencyWaitingRedisKey(channel.Id), "wait", 0).Err())
		require.NoError(t, client.Set(ctx, channelConcurrencyCooldownRedisKey(channel.Id), "cooldown", 0).Err())
	}
	unrelatedKeys := []string{
		channelConcurrencyRedisKey(918999),
		channelConcurrencyWaitingRedisKey(918999),
		channelConcurrencyCooldownRedisKey(918999),
	}
	for _, key := range unrelatedKeys {
		require.NoError(t, client.Set(ctx, key, "keep", 0).Err())
	}

	require.NoError(t, cleanupChannelConcurrencyStressKeys(ctx, client, channels))
	for _, channel := range channels {
		require.Zero(t, client.Exists(
			ctx,
			channelConcurrencyRedisKey(channel.Id),
			channelConcurrencyWaitingRedisKey(channel.Id),
			channelConcurrencyCooldownRedisKey(channel.Id),
		).Val())
	}
	require.Equal(t, int64(len(unrelatedKeys)), client.Exists(ctx, unrelatedKeys...).Val())
}

func cleanupChannelConcurrencyStressKeys(ctx context.Context, client *redis.Client, channels []*model.Channel) error {
	if ctx == nil {
		ctx = context.Background()
	}
	pipe := client.Pipeline()
	for _, channel := range channels {
		if channel == nil || channel.Id <= 0 {
			continue
		}
		pipe.Del(
			ctx,
			channelConcurrencyRedisKey(channel.Id),
			channelConcurrencyWaitingRedisKey(channel.Id),
			channelConcurrencyCooldownRedisKey(channel.Id),
		)
	}
	_, err := pipe.Exec(ctx)
	if err == redis.Nil {
		return nil
	}
	return err
}

func channelConcurrencyStressPositiveInt(t *testing.T, envName string, fallback int) int {
	t.Helper()
	raw := os.Getenv(envName)
	if raw == "" {
		return fallback
	}
	value, err := strconv.Atoi(raw)
	require.NoError(t, err)
	require.Positive(t, value)
	return value
}

func updateChannelConcurrencyStressMax(target *atomic.Int64, value int64) {
	for {
		current := target.Load()
		if value <= current || target.CompareAndSwap(current, value) {
			return
		}
	}
}

func assertChannelConcurrencyStressStateClean(t *testing.T, client *redis.Client, channels []*model.Channel) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	pipe := client.Pipeline()
	slotCounts := make([]*redis.IntCmd, len(channels))
	waitCounts := make([]*redis.StringCmd, len(channels))
	for index, channel := range channels {
		slotCounts[index] = pipe.ZCard(ctx, channelConcurrencyRedisKey(channel.Id))
		waitCounts[index] = pipe.Get(ctx, channelConcurrencyWaitingRedisKey(channel.Id))
	}
	_, err := pipe.Exec(ctx)
	require.ErrorIs(t, err, redis.Nil)
	for index := range channels {
		require.Zero(t, slotCounts[index].Val())
		require.ErrorIs(t, waitCounts[index].Err(), redis.Nil)
	}
}
