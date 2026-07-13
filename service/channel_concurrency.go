package service

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/singleflight"
)

var channelConcurrencyAcquireScriptSrc = `
local key = KEYS[1]
local ttl = tonumber(ARGV[1])
local max = tonumber(ARGV[2])
local token = ARGV[3]

local redis_time = redis.call('TIME')
local now = tonumber(redis_time[1]) * 1000 + math.floor(tonumber(redis_time[2]) / 1000)
redis.call('ZREMRANGEBYSCORE', key, '-inf', now)

if redis.call('ZSCORE', key, token) then
	redis.call('ZADD', key, now + ttl, token)
	redis.call('PEXPIRE', key, ttl)
	return 1
end

local count = redis.call('ZCARD', key)
if count >= max then
	redis.call('PEXPIRE', key, ttl)
	return 0
end

redis.call('ZADD', key, now + ttl, token)
redis.call('PEXPIRE', key, ttl)
return 1
`

var channelConcurrencyAcquireScript = redis.NewScript(channelConcurrencyAcquireScriptSrc)

var channelConcurrencyRenewScriptSrc = `
local key = KEYS[1]
local ttl = tonumber(ARGV[1])
local token = ARGV[2]

if not redis.call('ZSCORE', key, token) then
	return 0
end

local redis_time = redis.call('TIME')
local now = tonumber(redis_time[1]) * 1000 + math.floor(tonumber(redis_time[2]) / 1000)
redis.call('ZADD', key, now + ttl, token)
redis.call('PEXPIRE', key, ttl)
return 1
`

var channelConcurrencyRenewScript = redis.NewScript(channelConcurrencyRenewScriptSrc)

var channelConcurrencyWaitAcquireScript = redis.NewScript(`
local key = KEYS[1]
local ttl = tonumber(ARGV[1])
local max_waiting = tonumber(ARGV[2])
local current = tonumber(redis.call('GET', key) or '0')

if current >= max_waiting then
	return {0, current}
end

current = redis.call('INCR', key)
redis.call('PEXPIRE', key, ttl)
return {1, current}
`)

var channelConcurrencyWaitReleaseScript = redis.NewScript(`
local key = KEYS[1]
local current = tonumber(redis.call('GET', key) or '0')

if current <= 1 then
	redis.call('DEL', key)
	return 0
end

return redis.call('DECR', key)
`)

type ChannelConcurrencyLease struct {
	ChannelID int

	token         string
	useRedis      bool
	renewCancel   context.CancelFunc
	renewStopOnce sync.Once
	releaseMu     sync.Mutex
	released      atomic.Bool
}

type ChannelConcurrencyLoad struct {
	ChannelID      int
	MaxConcurrency int
	Active         int
	Waiting        int
	CoolingDown    bool
	LoadRate       float64
}

type channelConcurrencyWaitingLease struct {
	channelID int
	useRedis  bool
	released  atomic.Bool
}

type cachedChannelConcurrencyLoadBatch struct {
	loads     map[int]ChannelConcurrencyLoad
	expiresAt time.Time
}

const (
	defaultChannelConcurrencyLoadBatchCacheTTL = 200 * time.Millisecond
	maxChannelConcurrencyLoadBatchCacheEntries = 256
	channelConcurrencyInitialWaitBackoff       = 100 * time.Millisecond
	channelConcurrencyMaxWaitBackoff           = 2 * time.Second
	channelConcurrencyWaitBackoffMultiplier    = 1.5
	channelConcurrencyLoadFetchTimeout         = 3 * time.Second
)

var (
	channelConcurrencyMemoryMu        sync.Mutex
	channelConcurrencyMemorySlots     = make(map[int]map[string]time.Time)
	channelConcurrencyMemoryWaits     = make(map[int]int)
	channelConcurrencyMemoryCooldowns = make(map[int]time.Time)
	channelConcurrencyRequestPrefix   = common.GetUUID()
	channelConcurrencyLoadCacheTTL    = defaultChannelConcurrencyLoadBatchCacheTTL
	channelConcurrencyLoadCacheMu     sync.RWMutex
	channelConcurrencyLoadCache       = make(map[string]cachedChannelConcurrencyLoadBatch)
	channelConcurrencyFreshLoadCache  = make(map[string]cachedChannelConcurrencyLoadBatch)
	channelConcurrencyLoadGroup       singleflight.Group
	channelConcurrencyRenewInterval   = func(ttl time.Duration) time.Duration {
		interval := ttl / 3
		if interval < time.Second {
			return time.Second
		}
		return interval
	}
	removeRedisChannelConcurrencySlot = func(ctx context.Context, channelID int, token string) error {
		return common.RDB.ZRem(ctx, channelConcurrencyRedisKey(channelID), token).Err()
	}
	channelConcurrencyReleaseBackoffs = []time.Duration{25 * time.Millisecond, 75 * time.Millisecond}
)

func TryAcquireChannelConcurrency(ctx context.Context, channel *model.Channel) (*ChannelConcurrencyLease, bool, error) {
	return tryAcquireChannelConcurrencyWithToken(ctx, channel, newChannelConcurrencyToken())
}

func AcquireChannelConcurrencyWithWait(ctx context.Context, channel *model.Channel) (*ChannelConcurrencyLease, bool, error) {
	lease, ok, err := TryAcquireChannelConcurrency(ctx, channel)
	if err != nil || ok {
		return lease, ok, err
	}
	if channel == nil {
		return nil, false, fmt.Errorf("channel is nil")
	}
	maxConcurrency := channel.GetMaxConcurrency()
	if maxConcurrency <= 0 {
		return nil, true, nil
	}
	if !operation_setting.IsChannelConcurrencyWaitEnabled() {
		return nil, false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	waitingLease, admitted, _, err := acquireChannelConcurrencyWaiting(
		ctx,
		channel.Id,
		operation_setting.GetChannelConcurrencyMaxWaiting(maxConcurrency),
	)
	if err != nil {
		return nil, false, err
	}
	if !admitted {
		return nil, false, ErrChannelConcurrencyLimit
	}
	defer func() {
		releaseChannelConcurrencyWaitingLeaseWithLog(waitingLease, channel.Id)
	}()

	waitCtx, cancel := context.WithTimeout(ctx, operation_setting.GetChannelConcurrencyWaitTimeout())
	defer cancel()

	waitInterval := operation_setting.GetChannelConcurrencyWaitInterval()
	if waitInterval <= 0 {
		waitInterval = channelConcurrencyInitialWaitBackoff
	}
	if waitInterval < channelConcurrencyInitialWaitBackoff {
		waitInterval = channelConcurrencyInitialWaitBackoff
	}
	backoff := waitInterval
	timer := time.NewTimer(backoff)
	defer timer.Stop()

	for {
		select {
		case <-waitCtx.Done():
			return nil, false, ErrChannelConcurrencyLimit
		case <-timer.C:
			lease, ok, err = TryAcquireChannelConcurrency(waitCtx, channel)
			if err != nil || ok {
				return lease, ok, err
			}
			backoff = nextChannelConcurrencyWaitBackoff(backoff, waitInterval, rand.Float64())
			timer.Reset(backoff)
		}
	}
}

func tryAcquireChannelConcurrencyWithToken(ctx context.Context, channel *model.Channel, token string) (*ChannelConcurrencyLease, bool, error) {
	if channel == nil {
		return nil, false, fmt.Errorf("channel is nil")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	maxConcurrency := channel.GetMaxConcurrency()
	if maxConcurrency <= 0 {
		return nil, true, nil
	}

	coolingDown, err := isChannelConcurrencyCoolingDown(ctx, channel.Id)
	if err != nil {
		return nil, false, err
	}
	if coolingDown {
		return nil, false, nil
	}

	lease := &ChannelConcurrencyLease{
		ChannelID: channel.Id,
		token:     token,
		useRedis:  common.RedisEnabled && common.RDB != nil,
	}

	if lease.useRedis {
		ok, err := acquireRedisChannelConcurrency(ctx, channel.Id, maxConcurrency, token)
		if err != nil {
			return nil, false, fmt.Errorf("acquire channel concurrency in redis failed for channel %d: %w", channel.Id, err)
		} else if !ok {
			return nil, false, nil
		} else {
			startChannelConcurrencyLeaseRenewal(lease)
			return lease, true, nil
		}
	}

	if !acquireMemoryChannelConcurrency(channel.Id, maxConcurrency, token) {
		return nil, false, nil
	}
	startChannelConcurrencyLeaseRenewal(lease)
	return lease, true, nil
}

func GetChannelConcurrencyLoads(ctx context.Context, channels []*model.Channel) (map[int]ChannelConcurrencyLoad, error) {
	return getChannelConcurrencyLoads(ctx, channels, false, false)
}

func GetChannelConcurrencyLoadsFresh(ctx context.Context, channels []*model.Channel) (map[int]ChannelConcurrencyLoad, error) {
	return getChannelConcurrencyLoads(ctx, channels, true, false)
}

func getChannelConcurrencyLoadsFreshThrottled(ctx context.Context, channels []*model.Channel) (map[int]ChannelConcurrencyLoad, error) {
	return getChannelConcurrencyLoads(ctx, channels, true, true)
}

func getChannelConcurrencyLoads(ctx context.Context, channels []*model.Channel, fresh bool, throttledFresh bool) (map[int]ChannelConcurrencyLoad, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	loads := make(map[int]ChannelConcurrencyLoad, len(channels))
	if len(channels) == 0 {
		return loads, nil
	}
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		loads[channel.Id] = ChannelConcurrencyLoad{
			ChannelID:      channel.Id,
			MaxConcurrency: channel.GetMaxConcurrency(),
		}
	}

	if common.RedisEnabled && common.RDB != nil {
		boundedLoads := boundedChannelConcurrencyLoads(loads)
		var redisLoads map[int]ChannelConcurrencyLoad
		var err error
		if fresh && throttledFresh {
			redisLoads, err = getRedisChannelConcurrencyLoadsFreshThrottled(ctx, boundedLoads)
		} else if fresh {
			redisLoads, err = fetchRedisChannelConcurrencyLoads(ctx, boundedLoads)
		} else {
			redisLoads, err = getRedisChannelConcurrencyLoads(ctx, boundedLoads)
		}
		if err == nil {
			for channelID, load := range redisLoads {
				loads[channelID] = load
			}
			return loads, nil
		}
		common.SysError(fmt.Sprintf("get channel concurrency loads from redis failed, fallback to memory: %s", err.Error()))
	}

	return getMemoryChannelConcurrencyLoads(loads), nil
}

func MarkChannelConcurrencyCooldown(ctx context.Context, channelID int, duration time.Duration, reason string) error {
	if !operation_setting.IsChannelConcurrencyCooldownEnabled() {
		return nil
	}
	if channelID <= 0 {
		return fmt.Errorf("channel id is invalid")
	}
	if duration <= 0 {
		duration = operation_setting.GetChannelConcurrencyCooldown()
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if common.RedisEnabled && common.RDB != nil {
		if err := common.RDB.Set(ctx, channelConcurrencyCooldownRedisKey(channelID), reason, duration).Err(); err == nil {
			return nil
		} else {
			common.SysError(fmt.Sprintf("mark channel concurrency cooldown in redis failed, fallback to memory: channel_id=%d, error=%s", channelID, err.Error()))
		}
	}

	channelConcurrencyMemoryMu.Lock()
	defer channelConcurrencyMemoryMu.Unlock()
	channelConcurrencyMemoryCooldowns[channelID] = time.Now().Add(duration)
	return nil
}

func isChannelConcurrencyCoolingDown(ctx context.Context, channelID int) (bool, error) {
	if !operation_setting.IsChannelConcurrencyCooldownEnabled() || channelID <= 0 {
		return false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	if common.RedisEnabled && common.RDB != nil {
		coolingDown, err := common.RDB.Exists(ctx, channelConcurrencyCooldownRedisKey(channelID)).Result()
		if err == nil {
			return coolingDown > 0, nil
		}
		common.SysError(fmt.Sprintf("check channel concurrency cooldown in redis failed, fallback to memory: channel_id=%d, error=%s", channelID, err.Error()))
	}

	channelConcurrencyMemoryMu.Lock()
	defer channelConcurrencyMemoryMu.Unlock()
	cooldownUntil, ok := channelConcurrencyMemoryCooldowns[channelID]
	return ok && cooldownUntil.After(time.Now()), nil
}

func ReleaseChannelConcurrency(ctx context.Context, lease *ChannelConcurrencyLease) error {
	if lease == nil {
		return nil
	}

	lease.releaseMu.Lock()
	defer lease.releaseMu.Unlock()
	if lease.released.Load() {
		return nil
	}

	if lease.useRedis {
		if common.RDB == nil {
			lease.stopRenewal()
			return fmt.Errorf("release channel concurrency in redis failed for channel %d: redis client is nil", lease.ChannelID)
		}
		releaseCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := releaseRedisChannelConcurrencyWithRetry(releaseCtx, lease); err != nil {
			lease.stopRenewal()
			return fmt.Errorf("release channel concurrency in redis failed for channel %d: %w", lease.ChannelID, err)
		}
		lease.released.Store(true)
		lease.stopRenewal()
		return nil
	}

	releaseMemoryChannelConcurrency(lease.ChannelID, lease.token)
	lease.released.Store(true)
	lease.stopRenewal()
	return nil
}

func releaseRedisChannelConcurrencyWithRetry(ctx context.Context, lease *ChannelConcurrencyLease) error {
	var err error
	for attempt := 0; attempt <= len(channelConcurrencyReleaseBackoffs); attempt++ {
		err = removeRedisChannelConcurrencySlot(ctx, lease.ChannelID, lease.token)
		if err == nil {
			return nil
		}
		if attempt == len(channelConcurrencyReleaseBackoffs) {
			break
		}

		backoff := channelConcurrencyReleaseBackoffs[attempt]
		if backoff <= 0 {
			continue
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return err
}

func (lease *ChannelConcurrencyLease) stopRenewal() {
	if lease == nil {
		return
	}
	lease.renewStopOnce.Do(func() {
		if lease.renewCancel != nil {
			lease.renewCancel()
		}
	})
}

func EnsureChannelConcurrencyForContext(c *gin.Context, channel *model.Channel) (bool, error) {
	if c == nil {
		return false, fmt.Errorf("gin context is nil")
	}
	if channel == nil {
		return false, fmt.Errorf("channel is nil")
	}
	if lease := getChannelConcurrencyLeaseForContext(c); lease != nil {
		if lease.ChannelID == channel.Id {
			return true, nil
		}
		if err := ReleaseChannelConcurrencyForContext(c); err != nil {
			return false, err
		}
	}
	return AcquireChannelConcurrencyForContext(c, channel)
}

func EnsureChannelConcurrencyWithWaitForContext(c *gin.Context, channel *model.Channel) (bool, error) {
	if c == nil {
		return false, fmt.Errorf("gin context is nil")
	}
	if channel == nil {
		return false, fmt.Errorf("channel is nil")
	}
	if lease := getChannelConcurrencyLeaseForContext(c); lease != nil {
		if lease.ChannelID == channel.Id {
			return true, nil
		}
		if err := ReleaseChannelConcurrencyForContext(c); err != nil {
			return false, err
		}
	}
	return AcquireChannelConcurrencyWithWaitForContext(c, channel)
}

func AcquireChannelConcurrencyForContext(c *gin.Context, channel *model.Channel) (bool, error) {
	if c == nil {
		return false, fmt.Errorf("gin context is nil")
	}
	ctx := context.Background()
	if c.Request != nil {
		ctx = c.Request.Context()
	}
	lease, ok, err := TryAcquireChannelConcurrency(ctx, channel)
	if err != nil || !ok {
		return ok, err
	}
	if lease != nil && c != nil {
		common.SetContextKey(c, constant.ContextKeyChannelConcurrencyLease, lease)
	}
	return true, nil
}

func AcquireChannelConcurrencyWithWaitForContext(c *gin.Context, channel *model.Channel) (bool, error) {
	if c == nil {
		return false, fmt.Errorf("gin context is nil")
	}
	ctx := context.Background()
	if c.Request != nil {
		ctx = c.Request.Context()
	}
	lease, ok, err := AcquireChannelConcurrencyWithWait(ctx, channel)
	if err != nil || !ok {
		return ok, err
	}
	if lease != nil {
		common.SetContextKey(c, constant.ContextKeyChannelConcurrencyLease, lease)
	}
	return true, nil
}

func ReleaseChannelConcurrencyForContext(c *gin.Context) error {
	if c == nil {
		return nil
	}
	lease := getChannelConcurrencyLeaseForContext(c)
	if lease == nil {
		return nil
	}
	if err := ReleaseChannelConcurrency(context.Background(), lease); err != nil {
		return err
	}
	c.Set(string(constant.ContextKeyChannelConcurrencyLease), nil)
	return nil
}

func getChannelConcurrencyLeaseForContext(c *gin.Context) *ChannelConcurrencyLease {
	if c == nil {
		return nil
	}
	value, ok := common.GetContextKey(c, constant.ContextKeyChannelConcurrencyLease)
	if !ok || value == nil {
		return nil
	}
	lease, _ := value.(*ChannelConcurrencyLease)
	return lease
}

func acquireRedisChannelConcurrency(ctx context.Context, channelID int, maxConcurrency int, token string) (bool, error) {
	result, err := channelConcurrencyAcquireScript.Run(
		ctx,
		common.RDB,
		[]string{channelConcurrencyRedisKey(channelID)},
		operation_setting.GetChannelConcurrencySlotTTL().Milliseconds(),
		maxConcurrency,
		token,
	).Int()
	if err != nil {
		return false, fmt.Errorf("acquire channel concurrency in redis failed: %w", err)
	}
	return result == 1, nil
}

func startChannelConcurrencyLeaseRenewal(lease *ChannelConcurrencyLease) {
	if lease == nil {
		return
	}
	ttl := operation_setting.GetChannelConcurrencySlotTTL()
	interval := channelConcurrencyRenewInterval(ttl)
	if interval <= 0 || interval >= ttl {
		interval = ttl / 3
		if interval <= 0 {
			interval = time.Second
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	lease.renewCancel = cancel
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if lease.released.Load() {
					return
				}
				if lease.useRedis {
					if common.RDB == nil {
						continue
					}
					renewCtx, renewCancel := context.WithTimeout(context.Background(), 3*time.Second)
					if ok, err := renewRedisChannelConcurrency(renewCtx, lease.ChannelID, lease.token); err != nil {
						common.SysError(fmt.Sprintf("renew channel concurrency lease in redis failed: channel_id=%d, error=%s", lease.ChannelID, err.Error()))
					} else if !ok {
						renewCancel()
						return
					}
					renewCancel()
					continue
				}
				refreshMemoryChannelConcurrency(lease.ChannelID, lease.token)
			}
		}
	}()
}

func renewRedisChannelConcurrency(ctx context.Context, channelID int, token string) (bool, error) {
	result, err := channelConcurrencyRenewScript.Run(
		ctx,
		common.RDB,
		[]string{channelConcurrencyRedisKey(channelID)},
		operation_setting.GetChannelConcurrencySlotTTL().Milliseconds(),
		token,
	).Int()
	if err != nil {
		return false, fmt.Errorf("renew channel concurrency in redis failed: %w", err)
	}
	return result == 1, nil
}

func channelConcurrencyRedisKey(channelID int) string {
	return fmt.Sprintf("new-api:channel_concurrency:%d", channelID)
}

func channelConcurrencyWaitingRedisKey(channelID int) string {
	return fmt.Sprintf("new-api:channel_concurrency_wait:%d", channelID)
}

func channelConcurrencyCooldownRedisKey(channelID int) string {
	return fmt.Sprintf("new-api:channel_concurrency_cooldown:%d", channelID)
}

func newChannelConcurrencyToken() string {
	return channelConcurrencyRequestPrefix + ":" + common.GetUUID()
}

func getRedisChannelConcurrencyLoads(ctx context.Context, initial map[int]ChannelConcurrencyLoad) (map[int]ChannelConcurrencyLoad, error) {
	initial = boundedChannelConcurrencyLoads(initial)
	if len(initial) == 0 {
		return map[int]ChannelConcurrencyLoad{}, nil
	}
	if channelConcurrencyLoadCacheTTL <= 0 {
		return fetchRedisChannelConcurrencyLoads(ctx, initial)
	}

	key := channelConcurrencyLoadBatchCacheKey(initial)
	now := time.Now()
	if cached, ok := getCachedChannelConcurrencyLoads(key, now); ok {
		return cached, nil
	}

	value, err, _ := channelConcurrencyLoadGroup.Do(key, func() (any, error) {
		now := time.Now()
		if cached, ok := getCachedChannelConcurrencyLoads(key, now); ok {
			return cached, nil
		}
		fetchCtx, cancel := context.WithTimeout(context.Background(), channelConcurrencyLoadFetchTimeout)
		defer cancel()
		loads, fetchErr := fetchRedisChannelConcurrencyLoads(fetchCtx, initial)
		if fetchErr != nil {
			return nil, fetchErr
		}
		storeCachedChannelConcurrencyLoads(key, loads, now.Add(channelConcurrencyLoadCacheTTL))
		return cloneChannelConcurrencyLoadMap(loads), nil
	})
	if err != nil {
		return nil, err
	}
	loads, _ := value.(map[int]ChannelConcurrencyLoad)
	if loads == nil {
		return map[int]ChannelConcurrencyLoad{}, nil
	}
	return cloneChannelConcurrencyLoadMap(loads), nil
}

func getRedisChannelConcurrencyLoadsFreshThrottled(ctx context.Context, initial map[int]ChannelConcurrencyLoad) (map[int]ChannelConcurrencyLoad, error) {
	initial = boundedChannelConcurrencyLoads(initial)
	if len(initial) == 0 {
		return map[int]ChannelConcurrencyLoad{}, nil
	}
	if channelConcurrencyLoadCacheTTL <= 0 {
		return fetchRedisChannelConcurrencyLoads(ctx, initial)
	}

	key := channelConcurrencyLoadBatchCacheKey(initial)
	now := time.Now()
	if cached, ok := getCachedFreshChannelConcurrencyLoads(key, now); ok {
		return cached, nil
	}

	value, err, _ := channelConcurrencyLoadGroup.Do("fresh:"+key, func() (any, error) {
		now := time.Now()
		if cached, ok := getCachedFreshChannelConcurrencyLoads(key, now); ok {
			return cached, nil
		}
		fetchCtx, cancel := context.WithTimeout(context.Background(), channelConcurrencyLoadFetchTimeout)
		defer cancel()
		loads, fetchErr := fetchRedisChannelConcurrencyLoads(fetchCtx, initial)
		if fetchErr != nil {
			return nil, fetchErr
		}
		expiresAt := now.Add(channelConcurrencyLoadCacheTTL)
		storeCachedFreshChannelConcurrencyLoads(key, loads, expiresAt)
		storeCachedChannelConcurrencyLoads(key, loads, expiresAt)
		return cloneChannelConcurrencyLoadMap(loads), nil
	})
	if err != nil {
		return nil, err
	}
	loads, _ := value.(map[int]ChannelConcurrencyLoad)
	if loads == nil {
		return map[int]ChannelConcurrencyLoad{}, nil
	}
	return cloneChannelConcurrencyLoadMap(loads), nil
}

func fetchRedisChannelConcurrencyLoads(ctx context.Context, initial map[int]ChannelConcurrencyLoad) (map[int]ChannelConcurrencyLoad, error) {
	initial = boundedChannelConcurrencyLoads(initial)
	if len(initial) == 0 {
		return map[int]ChannelConcurrencyLoad{}, nil
	}

	now := time.Now()
	if redisNow, err := common.RDB.Time(ctx).Result(); err == nil {
		now = redisNow
	} else {
		return nil, err
	}

	type loadCommands struct {
		channelID   int
		activeCmd   *redis.IntCmd
		waitingCmd  *redis.StringCmd
		cooldownCmd *redis.IntCmd
	}

	pipe := common.RDB.Pipeline()
	commands := make([]loadCommands, 0, len(initial))
	for channelID := range initial {
		key := channelConcurrencyRedisKey(channelID)
		pipe.ZRemRangeByScore(ctx, key, "-inf", strconv.FormatInt(now.UnixMilli(), 10))
		commands = append(commands, loadCommands{
			channelID:   channelID,
			activeCmd:   pipe.ZCard(ctx, key),
			waitingCmd:  pipe.Get(ctx, channelConcurrencyWaitingRedisKey(channelID)),
			cooldownCmd: pipe.Exists(ctx, channelConcurrencyCooldownRedisKey(channelID)),
		})
	}
	if _, err := pipe.Exec(ctx); err != nil && err != redis.Nil {
		return nil, err
	}

	loads := make(map[int]ChannelConcurrencyLoad, len(initial))
	for channelID, load := range initial {
		loads[channelID] = load
	}
	for _, command := range commands {
		load := loads[command.channelID]
		if active, err := command.activeCmd.Result(); err == nil {
			load.Active = int(active)
		}
		if waitingValue, err := command.waitingCmd.Result(); err == nil {
			if waiting, parseErr := strconv.Atoi(waitingValue); parseErr == nil && waiting > 0 {
				load.Waiting = waiting
			}
		}
		if coolingDown, err := command.cooldownCmd.Result(); err == nil {
			load.CoolingDown = coolingDown > 0
		}
		load.LoadRate = calculateChannelConcurrencyLoadRate(load.Active, load.Waiting, load.MaxConcurrency)
		loads[command.channelID] = load
	}
	return loads, nil
}

func getCachedChannelConcurrencyLoads(key string, now time.Time) (map[int]ChannelConcurrencyLoad, bool) {
	channelConcurrencyLoadCacheMu.RLock()
	cached, ok := channelConcurrencyLoadCache[key]
	channelConcurrencyLoadCacheMu.RUnlock()
	if !ok {
		return nil, false
	}
	if !now.Before(cached.expiresAt) {
		channelConcurrencyLoadCacheMu.Lock()
		if current, exists := channelConcurrencyLoadCache[key]; exists && !now.Before(current.expiresAt) {
			delete(channelConcurrencyLoadCache, key)
		}
		channelConcurrencyLoadCacheMu.Unlock()
		return nil, false
	}
	return cloneChannelConcurrencyLoadMap(cached.loads), true
}

func getCachedFreshChannelConcurrencyLoads(key string, now time.Time) (map[int]ChannelConcurrencyLoad, bool) {
	channelConcurrencyLoadCacheMu.RLock()
	cached, ok := channelConcurrencyFreshLoadCache[key]
	channelConcurrencyLoadCacheMu.RUnlock()
	if !ok {
		return nil, false
	}
	if !now.Before(cached.expiresAt) {
		channelConcurrencyLoadCacheMu.Lock()
		if current, exists := channelConcurrencyFreshLoadCache[key]; exists && !now.Before(current.expiresAt) {
			delete(channelConcurrencyFreshLoadCache, key)
		}
		channelConcurrencyLoadCacheMu.Unlock()
		return nil, false
	}
	return cloneChannelConcurrencyLoadMap(cached.loads), true
}

func storeCachedChannelConcurrencyLoads(key string, loads map[int]ChannelConcurrencyLoad, expiresAt time.Time) {
	channelConcurrencyLoadCacheMu.Lock()
	if channelConcurrencyLoadCache == nil {
		channelConcurrencyLoadCache = make(map[string]cachedChannelConcurrencyLoadBatch)
	}
	if len(channelConcurrencyLoadCache) >= maxChannelConcurrencyLoadBatchCacheEntries {
		now := time.Now()
		for cacheKey, cached := range channelConcurrencyLoadCache {
			if !now.Before(cached.expiresAt) {
				delete(channelConcurrencyLoadCache, cacheKey)
			}
		}
		for len(channelConcurrencyLoadCache) >= maxChannelConcurrencyLoadBatchCacheEntries {
			for cacheKey := range channelConcurrencyLoadCache {
				delete(channelConcurrencyLoadCache, cacheKey)
				break
			}
		}
	}
	channelConcurrencyLoadCache[key] = cachedChannelConcurrencyLoadBatch{
		loads:     cloneChannelConcurrencyLoadMap(loads),
		expiresAt: expiresAt,
	}
	channelConcurrencyLoadCacheMu.Unlock()
}

func storeCachedFreshChannelConcurrencyLoads(key string, loads map[int]ChannelConcurrencyLoad, expiresAt time.Time) {
	channelConcurrencyLoadCacheMu.Lock()
	if channelConcurrencyFreshLoadCache == nil {
		channelConcurrencyFreshLoadCache = make(map[string]cachedChannelConcurrencyLoadBatch)
	}
	if len(channelConcurrencyFreshLoadCache) >= maxChannelConcurrencyLoadBatchCacheEntries {
		now := time.Now()
		for cacheKey, cached := range channelConcurrencyFreshLoadCache {
			if !now.Before(cached.expiresAt) {
				delete(channelConcurrencyFreshLoadCache, cacheKey)
			}
		}
		for len(channelConcurrencyFreshLoadCache) >= maxChannelConcurrencyLoadBatchCacheEntries {
			for cacheKey := range channelConcurrencyFreshLoadCache {
				delete(channelConcurrencyFreshLoadCache, cacheKey)
				break
			}
		}
	}
	channelConcurrencyFreshLoadCache[key] = cachedChannelConcurrencyLoadBatch{
		loads:     cloneChannelConcurrencyLoadMap(loads),
		expiresAt: expiresAt,
	}
	channelConcurrencyLoadCacheMu.Unlock()
}

func channelConcurrencyLoadBatchCacheKey(loads map[int]ChannelConcurrencyLoad) string {
	channelIDs := make([]int, 0, len(loads))
	for channelID := range loads {
		channelIDs = append(channelIDs, channelID)
	}
	sort.Ints(channelIDs)

	hash := sha256.New()
	var buf [16]byte
	for _, channelID := range channelIDs {
		load := loads[channelID]
		binary.LittleEndian.PutUint64(buf[:8], uint64(channelID))
		binary.LittleEndian.PutUint64(buf[8:], uint64(load.MaxConcurrency))
		_, _ = hash.Write(buf[:])
	}
	return strconv.Itoa(len(channelIDs)) + ":" + hex.EncodeToString(hash.Sum(nil))
}

func cloneChannelConcurrencyLoadMap(loads map[int]ChannelConcurrencyLoad) map[int]ChannelConcurrencyLoad {
	if len(loads) == 0 {
		return map[int]ChannelConcurrencyLoad{}
	}
	clone := make(map[int]ChannelConcurrencyLoad, len(loads))
	for channelID, load := range loads {
		clone[channelID] = load
	}
	return clone
}

func acquireMemoryChannelConcurrency(channelID int, maxConcurrency int, token string) bool {
	channelConcurrencyMemoryMu.Lock()
	defer channelConcurrencyMemoryMu.Unlock()

	now := time.Now()
	cleanupMemoryChannelConcurrencyLocked(now)
	slots := channelConcurrencyMemorySlots[channelID]
	if slots == nil {
		slots = make(map[string]time.Time)
		channelConcurrencyMemorySlots[channelID] = slots
	}

	if _, exists := slots[token]; exists {
		slots[token] = now.Add(operation_setting.GetChannelConcurrencySlotTTL())
		return true
	}

	if len(slots) >= maxConcurrency {
		return false
	}
	slots[token] = now.Add(operation_setting.GetChannelConcurrencySlotTTL())
	return true
}

func releaseMemoryChannelConcurrency(channelID int, token string) {
	channelConcurrencyMemoryMu.Lock()
	defer channelConcurrencyMemoryMu.Unlock()

	slots := channelConcurrencyMemorySlots[channelID]
	if slots == nil {
		return
	}
	delete(slots, token)
	if len(slots) == 0 {
		delete(channelConcurrencyMemorySlots, channelID)
	}
}

func refreshMemoryChannelConcurrency(channelID int, token string) {
	channelConcurrencyMemoryMu.Lock()
	defer channelConcurrencyMemoryMu.Unlock()

	slots := channelConcurrencyMemorySlots[channelID]
	if slots == nil {
		return
	}
	if _, ok := slots[token]; ok {
		slots[token] = time.Now().Add(operation_setting.GetChannelConcurrencySlotTTL())
	}
}

func acquireChannelConcurrencyWaiting(ctx context.Context, channelID int, maxWaiting int) (*channelConcurrencyWaitingLease, bool, int, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if maxWaiting < 1 {
		maxWaiting = 1
	}
	lease := &channelConcurrencyWaitingLease{
		channelID: channelID,
		useRedis:  common.RedisEnabled && common.RDB != nil,
	}
	if lease.useRedis {
		result, err := channelConcurrencyWaitAcquireScript.Run(
			ctx,
			common.RDB,
			[]string{channelConcurrencyWaitingRedisKey(channelID)},
			(operation_setting.GetChannelConcurrencyWaitTimeout() + time.Minute).Milliseconds(),
			maxWaiting,
		).Slice()
		if err != nil {
			return nil, false, 0, fmt.Errorf("admit channel concurrency waiter in redis for channel %d: %w", channelID, err)
		}
		if len(result) != 2 {
			return nil, false, 0, fmt.Errorf("admit channel concurrency waiter in redis for channel %d returned %d values", channelID, len(result))
		}
		admitted, ok := result[0].(int64)
		if !ok {
			return nil, false, 0, fmt.Errorf("admit channel concurrency waiter in redis for channel %d returned invalid admission result %T", channelID, result[0])
		}
		count, ok := result[1].(int64)
		if !ok {
			return nil, false, 0, fmt.Errorf("admit channel concurrency waiter in redis for channel %d returned invalid count %T", channelID, result[1])
		}
		if admitted != 1 {
			return nil, false, int(count), nil
		}
		return lease, true, int(count), nil
	}

	channelConcurrencyMemoryMu.Lock()
	defer channelConcurrencyMemoryMu.Unlock()
	current := channelConcurrencyMemoryWaits[channelID]
	if current >= maxWaiting {
		return nil, false, current, nil
	}
	current++
	channelConcurrencyMemoryWaits[channelID] = current
	return lease, true, current, nil
}

func releaseChannelConcurrencyWaitingLeaseWithLog(lease *channelConcurrencyWaitingLease, channelID int) {
	releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer releaseCancel()
	if err := lease.Release(releaseCtx); err != nil {
		common.SysError(fmt.Sprintf("release channel concurrency waiting lease failed: channel_id=%d, error=%s", channelID, err.Error()))
	}
}

func incrementChannelConcurrencyWaiting(ctx context.Context, channelID int, maxConcurrency int) (int, error) {
	_, admitted, waiting, err := acquireChannelConcurrencyWaiting(
		ctx,
		channelID,
		operation_setting.GetChannelConcurrencyMaxWaiting(maxConcurrency),
	)
	if err != nil {
		return 0, err
	}
	if !admitted {
		return waiting, ErrChannelConcurrencyLimit
	}
	return waiting, nil
}

func decrementChannelConcurrencyWaiting(ctx context.Context, channelID int) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if common.RedisEnabled && common.RDB != nil {
		if err := channelConcurrencyWaitReleaseScript.Run(
			ctx,
			common.RDB,
			[]string{channelConcurrencyWaitingRedisKey(channelID)},
		).Err(); err != nil {
			return fmt.Errorf("decrement channel concurrency waiting in redis failed for channel %d: %w", channelID, err)
		}
		return nil
	}

	channelConcurrencyMemoryMu.Lock()
	defer channelConcurrencyMemoryMu.Unlock()
	if channelConcurrencyMemoryWaits[channelID] <= 1 {
		delete(channelConcurrencyMemoryWaits, channelID)
		return nil
	}
	channelConcurrencyMemoryWaits[channelID]--
	return nil
}

func (lease *channelConcurrencyWaitingLease) Release(ctx context.Context) error {
	if lease == nil {
		return nil
	}
	if !lease.released.CompareAndSwap(false, true) {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if lease.useRedis {
		if common.RDB == nil {
			lease.released.Store(false)
			return fmt.Errorf("decrement channel concurrency waiting in redis failed for channel %d: redis client is unavailable", lease.channelID)
		}
		if err := channelConcurrencyWaitReleaseScript.Run(
			ctx,
			common.RDB,
			[]string{channelConcurrencyWaitingRedisKey(lease.channelID)},
		).Err(); err != nil {
			lease.released.Store(false)
			return fmt.Errorf("decrement channel concurrency waiting in redis failed for channel %d: %w", lease.channelID, err)
		}
		return nil
	}

	channelConcurrencyMemoryMu.Lock()
	defer channelConcurrencyMemoryMu.Unlock()
	if channelConcurrencyMemoryWaits[lease.channelID] <= 1 {
		delete(channelConcurrencyMemoryWaits, lease.channelID)
		return nil
	}
	channelConcurrencyMemoryWaits[lease.channelID]--
	return nil
}

func getMemoryChannelConcurrencyLoads(initial map[int]ChannelConcurrencyLoad) map[int]ChannelConcurrencyLoad {
	channelConcurrencyMemoryMu.Lock()
	defer channelConcurrencyMemoryMu.Unlock()

	now := time.Now()
	cleanupMemoryChannelConcurrencyLocked(now)

	loads := make(map[int]ChannelConcurrencyLoad, len(initial))
	for channelID, load := range initial {
		if load.MaxConcurrency <= 0 {
			loads[channelID] = load
			continue
		}
		load.Active = len(channelConcurrencyMemorySlots[channelID])
		load.Waiting = channelConcurrencyMemoryWaits[channelID]
		if cooldownUntil, ok := channelConcurrencyMemoryCooldowns[channelID]; ok {
			load.CoolingDown = cooldownUntil.After(now)
		}
		load.LoadRate = calculateChannelConcurrencyLoadRate(load.Active, load.Waiting, load.MaxConcurrency)
		loads[channelID] = load
	}
	return loads
}

func boundedChannelConcurrencyLoads(loads map[int]ChannelConcurrencyLoad) map[int]ChannelConcurrencyLoad {
	bounded := make(map[int]ChannelConcurrencyLoad, len(loads))
	for channelID, load := range loads {
		if load.MaxConcurrency > 0 {
			bounded[channelID] = load
		}
	}
	return bounded
}

func cleanupMemoryChannelConcurrencyLocked(now time.Time) {
	for channelID, slots := range channelConcurrencyMemorySlots {
		for token, expiresAt := range slots {
			if !expiresAt.After(now) {
				delete(slots, token)
			}
		}
		if len(slots) == 0 {
			delete(channelConcurrencyMemorySlots, channelID)
		}
	}
	for channelID, expiresAt := range channelConcurrencyMemoryCooldowns {
		if !expiresAt.After(now) {
			delete(channelConcurrencyMemoryCooldowns, channelID)
		}
	}
}

func calculateChannelConcurrencyLoadRate(active int, waiting int, maxConcurrency int) float64 {
	if maxConcurrency <= 0 {
		return 0
	}
	return float64(active+waiting) / float64(maxConcurrency)
}

func nextChannelConcurrencyWaitBackoff(current time.Duration, initial time.Duration, jitterSample float64) time.Duration {
	if initial <= 0 {
		initial = channelConcurrencyInitialWaitBackoff
	}
	if jitterSample < 0 {
		jitterSample = 0
	}
	if jitterSample > 1 {
		jitterSample = 1
	}
	next := time.Duration(float64(current) * channelConcurrencyWaitBackoffMultiplier)
	if next > channelConcurrencyMaxWaitBackoff {
		next = channelConcurrencyMaxWaitBackoff
	}
	jitter := 0.8 + jitterSample*0.4
	next = time.Duration(float64(next) * jitter)
	if next < initial {
		return initial
	}
	if next > channelConcurrencyMaxWaitBackoff {
		return channelConcurrencyMaxWaitBackoff
	}
	return next
}

func resetChannelConcurrencyForTest() {
	channelConcurrencyMemoryMu.Lock()
	defer channelConcurrencyMemoryMu.Unlock()
	channelConcurrencyMemorySlots = make(map[int]map[string]time.Time)
	channelConcurrencyMemoryWaits = make(map[int]int)
	channelConcurrencyMemoryCooldowns = make(map[int]time.Time)
	channelConcurrencyLoadCacheMu.Lock()
	channelConcurrencyLoadCache = make(map[string]cachedChannelConcurrencyLoadBatch)
	channelConcurrencyFreshLoadCache = make(map[string]cachedChannelConcurrencyLoadBatch)
	channelConcurrencyLoadCacheTTL = defaultChannelConcurrencyLoadBatchCacheTTL
	channelConcurrencyLoadGroup = singleflight.Group{}
	channelConcurrencyLoadCacheMu.Unlock()
}
