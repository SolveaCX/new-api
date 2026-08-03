package service

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/go-redis/redis/v8"
	"golang.org/x/sync/singleflight"
)

const (
	defaultChannelAvailabilityCacheTTL = time.Second
	channelAvailabilityFetchTimeout    = 3 * time.Second
)

type cachedChannelAvailability struct {
	coolingDown bool
	expiresAt   time.Time
}

var (
	channelAvailabilityCacheTTL = defaultChannelAvailabilityCacheTTL
	channelAvailabilityMu       sync.RWMutex
	channelAvailabilityCache    = make(map[int]cachedChannelAvailability)
	channelAvailabilityGroup    singleflight.Group
	channelAvailabilityMemoryMu sync.Mutex
	channelAvailabilityMemory   = make(map[int]time.Time)
)

func GetChannelConcurrencyCooldowns(ctx context.Context, channels []*model.Channel) (map[int]bool, error) {
	result := make(map[int]bool, len(channels))
	if !operation_setting.IsChannelConcurrencyCooldownEnabled() {
		return result, nil
	}

	ids := make([]int, 0, len(channels))
	seen := make(map[int]struct{}, len(channels))
	for _, channel := range channels {
		if channel == nil || channel.Id <= 0 || channel.GetMaxConcurrency() <= 0 {
			continue
		}
		if _, ok := seen[channel.Id]; ok {
			continue
		}
		seen[channel.Id] = struct{}{}
		ids = append(ids, channel.Id)
	}

	coolingDown, err := getChannelConcurrencyCooldownsByID(ctx, ids)
	for channelID, value := range coolingDown {
		result[channelID] = value
	}
	return result, err
}

func IsChannelConcurrencyCoolingDown(ctx context.Context, channelID int) (bool, error) {
	if !operation_setting.IsChannelConcurrencyCooldownEnabled() || channelID <= 0 {
		return false, nil
	}
	result, err := getChannelConcurrencyCooldownsByID(ctx, []int{channelID})
	return result[channelID], err
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

	now := time.Now()
	cooldownUntil := now.Add(duration)
	storeMemoryChannelCooldown(channelID, cooldownUntil)
	if common.RedisEnabled && common.RDB != nil {
		if err := common.RDB.Set(ctx, channelConcurrencyCooldownRedisKey(channelID), reason, duration).Err(); err != nil {
			common.SysError(fmt.Sprintf("mark channel concurrency cooldown in redis failed, fallback to memory: channel_id=%d, error=%s", channelID, err.Error()))
			storePositiveChannelAvailability(channelID, now, duration)
			return nil
		}
	}

	storePositiveChannelAvailability(channelID, now, duration)
	return nil
}

func getChannelConcurrencyCooldownsByID(ctx context.Context, ids []int) (map[int]bool, error) {
	result := make(map[int]bool, len(ids))
	if len(ids) == 0 {
		return result, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	now := time.Now()
	missing := missingChannelAvailabilityIDs(ids, now)
	for channelID, coolingDown := range cachedChannelAvailabilityValues(ids, now) {
		result[channelID] = coolingDown
	}
	if len(missing) == 0 {
		return result, nil
	}

	if !common.RedisEnabled || common.RDB == nil {
		mergeMemoryChannelCooldowns(result, missing, now)
		return result, nil
	}

	sort.Ints(missing)
	key := channelAvailabilityBatchKey(missing)
	resultCh := channelAvailabilityGroup.DoChan(key, func() (any, error) {
		fetchCtx, cancel := context.WithTimeout(context.Background(), channelAvailabilityFetchTimeout)
		defer cancel()

		fetchNow := time.Now()
		fetchIDs := missingChannelAvailabilityIDs(missing, fetchNow)
		if len(fetchIDs) == 0 {
			return cachedChannelAvailabilityValues(missing, fetchNow), nil
		}

		fetched := make(map[int]bool, len(fetchIDs))
		if err := withChannelConcurrencyBatchFetchSlot(fetchCtx, func() error {
			for start := 0; start < len(fetchIDs); start += channelConcurrencyRedisReadBatchSize {
				end := start + channelConcurrencyRedisReadBatchSize
				if end > len(fetchIDs) {
					end = len(fetchIDs)
				}

				pipe := common.RDB.Pipeline()
				commands := make(map[int]*redis.IntCmd, end-start)
				for _, channelID := range fetchIDs[start:end] {
					commands[channelID] = pipe.Exists(fetchCtx, channelConcurrencyCooldownRedisKey(channelID))
				}
				if _, execErr := pipe.Exec(fetchCtx); execErr != nil && execErr != redis.Nil {
					return execErr
				}

				cacheNow := time.Now()
				expiresAt := cacheNow.Add(channelAvailabilityCacheTTL)
				channelAvailabilityMu.Lock()
				for channelID, command := range commands {
					coolingDown := command.Val() > 0
					entryExpiresAt := expiresAt
					if cached, ok := channelAvailabilityCache[channelID]; ok &&
						cached.coolingDown && cacheNow.Before(cached.expiresAt) && !coolingDown {
						coolingDown = true
						if cached.expiresAt.Before(entryExpiresAt) {
							entryExpiresAt = cached.expiresAt
						}
					}
					fetched[channelID] = coolingDown
					channelAvailabilityCache[channelID] = cachedChannelAvailability{
						coolingDown: coolingDown,
						expiresAt:   entryExpiresAt,
					}
				}
				channelAvailabilityMu.Unlock()
			}
			return nil
		}); err != nil {
			common.SysError(fmt.Sprintf("get channel cooldowns from redis failed: %s", err.Error()))
			return nil, err
		}
		return fetched, nil
	})

	fetched, err := awaitChannelAvailabilityResult(ctx, resultCh)
	if err != nil {
		return result, err
	}
	for channelID, coolingDown := range fetched {
		result[channelID] = coolingDown
	}
	for channelID, coolingDown := range cachedChannelAvailabilityValues(missing, time.Now()) {
		result[channelID] = coolingDown
	}
	remaining := make([]int, 0, len(missing))
	for _, channelID := range missing {
		if _, ok := result[channelID]; !ok {
			remaining = append(remaining, channelID)
		}
	}
	mergeMemoryChannelCooldowns(result, remaining, time.Now())
	return result, nil
}

func awaitChannelAvailabilityResult(ctx context.Context, resultCh <-chan singleflight.Result) (map[int]bool, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case result := <-resultCh:
		if result.Err != nil {
			return nil, result.Err
		}
		values, _ := result.Val.(map[int]bool)
		if values == nil {
			return map[int]bool{}, nil
		}
		return values, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func channelAvailabilityBatchKey(ids []int) string {
	var builder strings.Builder
	for index, channelID := range ids {
		if index > 0 {
			builder.WriteByte(',')
		}
		builder.WriteString(strconv.Itoa(channelID))
	}
	return builder.String()
}

func missingChannelAvailabilityIDs(ids []int, now time.Time) []int {
	missing := make([]int, 0, len(ids))
	channelAvailabilityMu.RLock()
	for _, channelID := range ids {
		cached, ok := channelAvailabilityCache[channelID]
		if !ok || !now.Before(cached.expiresAt) {
			missing = append(missing, channelID)
		}
	}
	channelAvailabilityMu.RUnlock()
	return missing
}

func cachedChannelAvailabilityValues(ids []int, now time.Time) map[int]bool {
	values := make(map[int]bool, len(ids))
	channelAvailabilityMu.RLock()
	for _, channelID := range ids {
		cached, ok := channelAvailabilityCache[channelID]
		if ok && now.Before(cached.expiresAt) {
			values[channelID] = cached.coolingDown
		}
	}
	channelAvailabilityMu.RUnlock()
	return values
}

func mergeMemoryChannelCooldowns(result map[int]bool, ids []int, now time.Time) {
	channelAvailabilityMemoryMu.Lock()
	defer channelAvailabilityMemoryMu.Unlock()
	for _, channelID := range ids {
		cooldownUntil, ok := channelAvailabilityMemory[channelID]
		if ok && cooldownUntil.After(now) {
			result[channelID] = true
			continue
		}
		if ok {
			delete(channelAvailabilityMemory, channelID)
		}
		result[channelID] = false
	}
}

func storeMemoryChannelCooldown(channelID int, cooldownUntil time.Time) {
	channelAvailabilityMemoryMu.Lock()
	channelAvailabilityMemory[channelID] = cooldownUntil
	channelAvailabilityMemoryMu.Unlock()
}

func storePositiveChannelAvailability(channelID int, now time.Time, duration time.Duration) {
	expiresAt := now.Add(duration)
	if channelAvailabilityCacheTTL > 0 {
		cacheExpiresAt := now.Add(channelAvailabilityCacheTTL)
		if cacheExpiresAt.Before(expiresAt) {
			expiresAt = cacheExpiresAt
		}
	}
	channelAvailabilityMu.Lock()
	channelAvailabilityCache[channelID] = cachedChannelAvailability{
		coolingDown: true,
		expiresAt:   expiresAt,
	}
	channelAvailabilityMu.Unlock()
}

func channelConcurrencyCooldownRedisKey(channelID int) string {
	return fmt.Sprintf("new-api:channel_concurrency_cooldown:%d", channelID)
}

func resetChannelAvailabilityForTest() {
	channelAvailabilityMu.Lock()
	channelAvailabilityCacheTTL = defaultChannelAvailabilityCacheTTL
	channelAvailabilityCache = make(map[int]cachedChannelAvailability)
	channelAvailabilityGroup = singleflight.Group{}
	channelAvailabilityMu.Unlock()

	channelAvailabilityMemoryMu.Lock()
	channelAvailabilityMemory = make(map[int]time.Time)
	channelAvailabilityMemoryMu.Unlock()
}
