package controller

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// 单优先级档位的渠道组里，首发失败后重试会把档位下标推到越界，
// getChannel 返回"可用渠道不存在"。此时若本请求已带着真实的上游错误
// (relayInfo.LastError，例如 429)，必须把它原样还给客户端，而不是
// 包装成一个新的 500 —— 否则 LiteLLM 等下游会把限流当成服务器故障。
func TestGetChannelExhaustedTiersPreservesUpstreamError(t *testing.T) {
	prevDB := model.DB
	prevMemoryCacheEnabled := common.MemoryCacheEnabled
	prevRDB := common.RDB
	prevRedisEnabled := common.RedisEnabled
	t.Cleanup(func() {
		model.DB = prevDB
		common.MemoryCacheEnabled = prevMemoryCacheEnabled
		common.RDB = prevRDB
		common.RedisEnabled = prevRedisEnabled
	})

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.Channel{}, &model.Ability{}))
	model.DB = db
	common.MemoryCacheEnabled = false
	common.RDB = nil
	common.RedisEnabled = false

	priority := int64(8)
	channel := &model.Channel{
		Id:     909911,
		Type:   constant.ChannelTypeOpenAI,
		Key:    "sk-test",
		Status: common.ChannelStatusEnabled,
		Name:   "single-tier",
		Group:  "default",
		Models: "gpt-test",
	}
	require.NoError(t, model.DB.Create(channel).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group:     "default",
		Model:     "gpt-test",
		ChannelId: channel.Id,
		Enabled:   true,
		Priority:  &priority,
	}).Error)

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())

	upstreamErr := types.NewErrorWithStatusCode(
		errors.New("codex upstream status 429: Rate limit exceeded"),
		types.ErrorCodeBadResponseStatusCode,
		http.StatusTooManyRequests,
	)
	relayInfo := &relaycommon.RelayInfo{
		TokenGroup:      "default",
		OriginModelName: "gpt-test",
		LastError:       upstreamErr,
		// 非 nil ChannelMeta = 重试路径（首发已消费过 context 里的渠道）
		ChannelMeta: &relaycommon.ChannelMeta{ChannelId: channel.Id},
	}

	// retry=1 越过唯一档位(仅 priority=8 一档) → 选不到渠道
	retry := 1
	selected, channelErr := getChannel(c, relayInfo, &service.RetryParam{
		Ctx:        c,
		TokenGroup: "default",
		ModelName:  "gpt-test",
		Retry:      &retry,
	})

	require.Nil(t, selected)
	require.NotNil(t, channelErr)
	require.Equal(t, http.StatusTooManyRequests, channelErr.StatusCode,
		"exhausted-tier retry must surface the real upstream status, not a synthetic 500")
}
