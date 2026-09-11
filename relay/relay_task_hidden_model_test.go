package relay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

const remixHiddenGateModel = "seedance-gate-remix"

func withRelayHiddenPricingModels(t *testing.T, hiddenModels string) {
	t.Helper()
	visibility := operation_setting.GetPricingVisibilitySetting()
	original := visibility.HiddenModels
	visibility.HiddenModels = hiddenModels
	t.Cleanup(func() {
		visibility.HiddenModels = original
	})
}

// seedRemixOriginTask stores an accepted video task on an enabled channel so
// ResolveOriginTask can derive the model and lock the channel from it.
func seedRemixOriginTask(t *testing.T, userID int, taskID string, channelID int) {
	t.Helper()
	setupRelayTaskTestDB(t)
	require.NoError(t, model.DB.Create(&model.Channel{
		Id: channelID, Type: constant.ChannelTypeOpenAI, Key: "sk-remix", Name: "remix-origin",
		Status: common.ChannelStatusEnabled, Models: remixHiddenGateModel, Group: "plg,default",
	}).Error)
	seedRelayTask(t, &model.Task{
		TaskID:     taskID,
		Platform:   constant.TaskPlatform("openai"),
		UserId:     userID,
		ChannelId:  channelID,
		Action:     constant.TaskActionGenerate,
		Status:     model.TaskStatusSuccess,
		Properties: model.Properties{OriginModelName: remixHiddenGateModel},
		Data:       []byte(`{"model":"` + remixHiddenGateModel + `","seconds":"4","size":"1280x720"}`),
	})
}

func resolveRemixForIdentity(t *testing.T, identityGroup string, userID int, taskID string, channelID int) (*relaycommon.RelayInfo, *dto.TaskError) {
	t.Helper()
	require.NoError(t, i18n.Init())
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos/"+taskID+"/remix", nil)
	c.Params = gin.Params{{Key: "video_id", Value: taskID}}
	info := &relaycommon.RelayInfo{
		UserId:    userID,
		UserGroup: identityGroup,
		// Same channel as the origin task: ResolveOriginTask skips key rotation.
		ChannelMeta:   &relaycommon.ChannelMeta{ChannelId: channelID},
		TaskRelayInfo: &relaycommon.TaskRelayInfo{},
	}
	return info, ResolveOriginTask(c, info)
}

func TestResolveOriginTaskBlocksPLGRemixOfHiddenModel(t *testing.T) {
	withRelayHiddenPricingModels(t, "seedance-gate-*")
	seedRemixOriginTask(t, 123, "task_remix_plg_hidden", 5001)

	info, taskErr := resolveRemixForIdentity(t, "plg", 123, "task_remix_plg_hidden", 5001)

	require.NotNil(t, taskErr, "PLG remix of a hidden model must be rejected")
	require.Equal(t, http.StatusNotFound, taskErr.StatusCode)
	require.Equal(t, "model_not_found", taskErr.Code)
	require.True(t, taskErr.LocalError, "rejection is request-local; the channel must not be penalised or retried")
	require.Contains(t, taskErr.Message, remixHiddenGateModel)
	require.NotContains(t, strings.ToLower(taskErr.Message), "hidden")
	require.Nil(t, info.LockedChannel, "a rejected remix must not lock the origin channel")
}

func TestResolveOriginTaskAllowsEnterpriseRemixOfHiddenModel(t *testing.T) {
	withRelayHiddenPricingModels(t, "seedance-gate-*")
	seedRemixOriginTask(t, 124, "task_remix_ent_hidden", 5002)

	info, taskErr := resolveRemixForIdentity(t, "default", 124, "task_remix_ent_hidden", 5002)

	require.Nil(t, taskErr)
	require.Equal(t, remixHiddenGateModel, info.OriginModelName)
	require.NotNil(t, info.LockedChannel)
}

func TestResolveOriginTaskAllowsPLGRemixOfVisibleModel(t *testing.T) {
	withRelayHiddenPricingModels(t, "some-other-model")
	seedRemixOriginTask(t, 125, "task_remix_plg_visible", 5003)

	info, taskErr := resolveRemixForIdentity(t, "plg", 125, "task_remix_plg_visible", 5003)

	require.Nil(t, taskErr)
	require.Equal(t, remixHiddenGateModel, info.OriginModelName)
	require.NotNil(t, info.LockedChannel)
}
