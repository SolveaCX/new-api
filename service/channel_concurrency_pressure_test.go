package service

import (
	"context"
	"fmt"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
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

func TestChannelSelectionFreshReorderFindsCandidateOutsideStaleTopSeven(t *testing.T) {
	resetChannelConcurrencyForTest()
	restoreRedis, hook := useCountedRedisChannelConcurrencyForTest(t, 0)
	defer restoreRedis()
	restoreDB := useChannelSelectionDBForTest(t)
	defer restoreDB()
	restoreSetting := disableChannelConcurrencyWaitingForTest(t)
	defer restoreSetting()

	channels := make([]*model.Channel, 0, 8)
	for i := 0; i < 8; i++ {
		channel := createChannelSelectionFixture(t, 914000+i, 1, 1, "gpt-fresh-top-seven")
		channels = append(channels, channel)
		if i > 0 {
			require.NoError(t, common.RDB.Set(context.Background(), channelConcurrencyWaitingRedisKey(channel.Id), i, time.Minute).Err())
		}
	}
	model.InitChannelCache()
	_, err := GetChannelConcurrencyLoads(context.Background(), channels)
	require.NoError(t, err)
	for _, channel := range channels[:7] {
		fillChannelConcurrencySlotForTest(t, channel)
	}
	hook.Reset()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	selected, _, err := getRandomSatisfiedChannelWithConcurrency(c, "default", "gpt-fresh-top-seven", 0)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, channels[7].Id, selected.Id)
	require.Equal(t, 1, hook.CommandCount("time"))
	require.NoError(t, ReleaseChannelConcurrencyForContext(c))
}

func TestChannelSelectionNeverAttemptsMoreThanSevenCandidatesPerPass(t *testing.T) {
	plan := channelConcurrencySelectionPlan{attempts: make([]channelCandidateWithRetry, 50)}
	for i := range plan.attempts {
		plan.attempts[i] = channelCandidateWithRetry{
			channel: &model.Channel{Id: 915000 + i, MaxConcurrency: 1},
		}
	}
	var calls int
	alwaysFull := func(*gin.Context, *model.Channel) (bool, error) {
		calls++
		return false, nil
	}

	_, _, _, err := tryAcquireChannelSelectionPlan(nil, plan, alwaysFull)
	require.NoError(t, err)
	require.Equal(t, 7, calls)
	_, _, _, err = tryAcquireChannelSelectionPlan(nil, plan, alwaysFull)
	require.NoError(t, err)
	require.Equal(t, 14, calls)
}

func TestChannelSelectionOrdersPriorityBeforeLoadAndKeepsBucketsSeparate(t *testing.T) {
	highPriority := int64(100)
	lowPriority := int64(10)
	weight := uint(1)
	candidates := []channelCandidateWithRetry{
		{
			channel: &model.Channel{Id: 915100, Priority: &highPriority, Weight: &weight},
			load:    ChannelConcurrencyLoad{LoadRate: 0.2},
		},
		{
			channel: &model.Channel{Id: 915101, Priority: &highPriority, Weight: &weight},
			load:    ChannelConcurrencyLoad{LoadRate: 0.2},
		},
		{
			channel: &model.Channel{Id: 915102, Priority: &highPriority, Weight: &weight},
			load:    ChannelConcurrencyLoad{LoadRate: 0.8},
		},
		{
			channel: &model.Channel{Id: 915103, Priority: &lowPriority, Weight: &weight},
			load:    ChannelConcurrencyLoad{LoadRate: 0.0},
		},
	}

	ordered, err := orderChannelConcurrencyCandidates(candidates)
	require.NoError(t, err)
	require.Len(t, ordered, 4)
	require.ElementsMatch(t, []int{915100, 915101}, []int{ordered[0].channel.Id, ordered[1].channel.Id})
	require.Equal(t, 915102, ordered[2].channel.Id)
	require.Equal(t, 915103, ordered[3].channel.Id)
}

func TestChannelSelectionPlanAttemptsUnlimitedAndOpenButWaitsOnHighestPriorityLimited(t *testing.T) {
	resetChannelConcurrencyForTest()
	restoreRedis, hook := useCountedRedisChannelConcurrencyForTest(t, 0)
	defer restoreRedis()

	highPriority := int64(100)
	middlePriority := int64(90)
	lowPriority := int64(80)
	weight := uint(1)
	full := &model.Channel{Id: 915200, MaxConcurrency: 1, Priority: &highPriority, Weight: &weight}
	unlimited := &model.Channel{Id: 915201, MaxConcurrency: 0, Priority: &middlePriority, Weight: &weight}
	open := &model.Channel{Id: 915202, MaxConcurrency: 2, Priority: &lowPriority, Weight: &weight}
	fillChannelConcurrencySlotForTest(t, full)
	hook.Reset()

	plan, err := buildChannelConcurrencySelectionPlan(context.Background(), []channelCandidateWithRetry{
		{channel: full, retry: 2},
		{channel: unlimited, retry: 3},
		{channel: open, retry: 4},
	}, false)
	require.NoError(t, err)
	require.Len(t, plan.attempts, 2)
	require.Equal(t, unlimited.Id, plan.attempts[0].channel.Id)
	require.Equal(t, open.Id, plan.attempts[1].channel.Id)
	require.NotNil(t, plan.waitCandidate)
	require.Equal(t, full.Id, plan.waitCandidate.channel.Id)
	require.Equal(t, 2, plan.waitCandidate.retry)
	require.Equal(t, 1, hook.CommandCount("time"))
	require.Equal(t, 2, hook.CommandCount("zremrangebyscore"))
	require.Equal(t, 2, hook.CommandCount("zcard"))
	require.Equal(t, 2, hook.CommandCount("get"))
}

func TestChannelSelectionStillFallsBackToAvailableLowerPriority(t *testing.T) {
	resetChannelConcurrencyForTest()
	restoreRedis, _ := useCountedRedisChannelConcurrencyForTest(t, 0)
	defer restoreRedis()
	restoreDB := useChannelSelectionDBForTest(t)
	defer restoreDB()
	restoreSetting := disableChannelConcurrencyWaitingForTest(t)
	defer restoreSetting()

	for i := 0; i < 8; i++ {
		high := createChannelSelectionFixture(t, 916000+i, 1, 100, "gpt-priority-fallback")
		fillChannelConcurrencySlotForTest(t, high)
	}
	low := createChannelSelectionFixture(t, 916100, 1, 10, "gpt-priority-fallback")
	model.InitChannelCache()

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	selected, selectedRetry, err := getRandomSatisfiedChannelWithConcurrency(c, "default", "gpt-priority-fallback", 0)
	require.NoError(t, err)
	require.NotNil(t, selected)
	require.Equal(t, low.Id, selected.Id)
	require.Equal(t, 1, selectedRetry)
	require.NoError(t, ReleaseChannelConcurrencyForContext(c))
}

func createChannelSelectionFixture(t *testing.T, id int, maxConcurrency int, priorityValue int64, modelName string) *model.Channel {
	t.Helper()
	priority := priorityValue
	weight := uint(1)
	channel := &model.Channel{
		Id:             id,
		Type:           1,
		Key:            fmt.Sprintf("sk-%d", id),
		Status:         common.ChannelStatusEnabled,
		Name:           fmt.Sprintf("channel-%d", id),
		Group:          "default",
		Models:         modelName,
		Priority:       &priority,
		Weight:         &weight,
		MaxConcurrency: maxConcurrency,
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, channel.AddAbilities(nil))
	return channel
}

func fillChannelConcurrencySlotForTest(t *testing.T, channel *model.Channel) {
	t.Helper()
	redisNow, err := common.RDB.Time(context.Background()).Result()
	require.NoError(t, err)
	score := float64(redisNow.Add(operation_setting.GetChannelConcurrencySlotTTL()).UnixMilli())
	require.NoError(t, common.RDB.ZAdd(
		context.Background(),
		channelConcurrencyRedisKey(channel.Id),
		&redis.Z{Score: score, Member: fmt.Sprintf("held-%d", channel.Id)},
	).Err())
}

func disableChannelConcurrencyWaitingForTest(t *testing.T) func() {
	t.Helper()
	setting := operation_setting.GetChannelConcurrencySetting()
	original := setting
	setting.WaitEnabled = false
	operation_setting.SetChannelConcurrencySettingForTest(setting)
	return func() { operation_setting.SetChannelConcurrencySettingForTest(original) }
}
