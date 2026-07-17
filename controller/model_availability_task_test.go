package controller

import (
	"errors"
	"net/http"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/types"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestStartModelAvailabilityDetectionTaskSkipsWhenStatusCenterEnabled(t *testing.T) {
	originalMaster := common.IsMasterNode
	originalOnce := modelAvailabilityTaskOnce
	originalLaunch := modelAvailabilityTaskLaunch
	t.Cleanup(func() {
		common.IsMasterNode = originalMaster
		modelAvailabilityTaskOnce = originalOnce
		modelAvailabilityTaskLaunch = originalLaunch
	})

	common.IsMasterNode = true
	modelAvailabilityTaskOnce = &sync.Once{}
	var launches atomic.Int64
	modelAvailabilityTaskLaunch = func() { launches.Add(1) }

	t.Setenv("STATUS_CENTER_ENABLED", "true")
	require.False(t, StartModelAvailabilityDetectionTask())
	require.EqualValues(t, 0, launches.Load())

	t.Setenv("STATUS_CENTER_ENABLED", "false")
	require.True(t, StartModelAvailabilityDetectionTask())
	require.False(t, StartModelAvailabilityDetectionTask())
	require.EqualValues(t, 1, launches.Load())
}

func TestWriteStatusModelAvailabilityRejectsStaleSchedulerFence(t *testing.T) {
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(append(model.StatusCenterModels(), &model.ModelAvailabilityState{})...))
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })

	first, acquired, err := model.AcquireStatusJobLease("status-center-scheduler", "node-a", 100, 10)
	require.NoError(t, err)
	require.True(t, acquired)
	require.NoError(t, WriteStatusModelAvailability("status-center-scheduler", "node-a", first.FencingToken, 100, "gpt-test", service.StatusProbeOutcome{Success: true, DiagnosticType: "ok"}))

	_, acquired, err = model.AcquireStatusJobLease("status-center-scheduler", "node-b", 111, 10)
	require.NoError(t, err)
	require.True(t, acquired)
	require.Error(t, WriteStatusModelAvailability("status-center-scheduler", "node-a", first.FencingToken, 111, "gpt-test", service.StatusProbeOutcome{DiagnosticType: "temporary_upstream_failure"}))

	state, err := model.GetModelAvailabilityState("gpt-test")
	require.NoError(t, err)
	require.NotNil(t, state)
	require.Equal(t, model.ModelAvailabilityAvailable, state.Status)
}

func TestWriteStatusModelAvailabilityKeepsLegacyFailureSemantics(t *testing.T) {
	originalDB := model.DB
	db, err := gorm.Open(sqlite.Open("file:"+t.Name()+"?mode=memory&cache=shared"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(append(model.StatusCenterModels(), &model.ModelAvailabilityState{})...))
	model.DB = db
	t.Cleanup(func() { model.DB = originalDB })
	t.Setenv("MODEL_AVAILABILITY_OFFICIAL_UNSUPPORTED_THRESHOLD", "2")

	lease, acquired, err := model.AcquireStatusJobLease("status-center-scheduler", "node-a", 100, 60)
	require.NoError(t, err)
	require.True(t, acquired)

	require.NoError(t, WriteStatusModelAvailability("status-center-scheduler", "node-a", lease.FencingToken, 100, "temporary-model", service.StatusProbeOutcome{DiagnosticType: "temporary_upstream_failure"}))
	temporary, err := model.GetModelAvailabilityState("temporary-model")
	require.NoError(t, err)
	require.Equal(t, model.ModelAvailabilityTemporaryFailure, temporary.Status)
	require.Equal(t, "temporary_upstream_failure", temporary.ReasonType)
	require.Equal(t, 1, temporary.ConsecutiveFailures)

	require.NoError(t, WriteStatusModelAvailability("status-center-scheduler", "node-a", lease.FencingToken, 101, "unsupported-model", service.StatusProbeOutcome{DiagnosticType: "official_model_unsupported"}))
	candidate, err := model.GetModelAvailabilityState("unsupported-model")
	require.NoError(t, err)
	require.Equal(t, model.ModelAvailabilityUnknownFailure, candidate.Status)
	require.Equal(t, "official_model_unsupported_candidate", candidate.ReasonType)
	require.Equal(t, 1, candidate.ConsecutiveFailures)

	require.NoError(t, WriteStatusModelAvailability("status-center-scheduler", "node-a", lease.FencingToken, 102, "unsupported-model", service.StatusProbeOutcome{DiagnosticType: "official_model_unsupported"}))
	unsupported, err := model.GetModelAvailabilityState("unsupported-model")
	require.NoError(t, err)
	require.Equal(t, model.ModelAvailabilityOfficialUnsupported, unsupported.Status)
	require.Equal(t, "official_model_unsupported", unsupported.ReasonType)
	require.Equal(t, 2, unsupported.ConsecutiveFailures)
}

func TestClassifyModelProbeError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		apiErr     *types.NewAPIError
		wantClass  modelProbeClass
		wantReason string
	}{
		{
			name:       "official unsupported model",
			err:        errors.New("The 'gpt-5.3-codex' model is not supported when using Codex with a ChatGPT account."),
			wantClass:  modelProbeOfficialUnsupported,
			wantReason: "official_model_unsupported",
		},
		{
			name:       "temporary status",
			apiErr:     types.NewOpenAIError(errors.New("upstream overloaded"), types.ErrorCodeBadResponse, http.StatusServiceUnavailable),
			wantClass:  modelProbeTemporaryFailure,
			wantReason: "temporary_upstream_failure",
		},
		{
			name:       "account issue",
			err:        errors.New("invalid api key"),
			wantClass:  modelProbeUnknownFailure,
			wantReason: "channel_account_issue",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyModelProbeError(tt.err, tt.apiErr)
			require.Equal(t, tt.wantClass, got.Class)
			require.Equal(t, tt.wantReason, got.ReasonType)
		})
	}
}

func TestValidatePongTestResponseBody(t *testing.T) {
	require.NoError(t, validatePongTestResponseBody([]byte(`{"choices":[{"message":{"content":"pong"}}]}`)))
	require.NoError(t, validatePongTestResponseBody([]byte(`{"output_text":"PONG!"}`)))
	require.NoError(t, validatePongTestResponseBody([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"pong\"},\"message\":{\"content\":\"pong\"}}]}\n\n")))
	// Tolerant contract: a non-pong but non-empty answer means the model is reachable.
	// Real upstream errors are filtered by validateTestResponseBody before this runs,
	// so a chat/reasoning model that does not echo "pong" must not be marked failed.
	require.NoError(t, validatePongTestResponseBody([]byte(`{"choices":[{"message":{"content":"hello"}}]}`)))
	require.NoError(t, validatePongTestResponseBody([]byte(`{"choices":[{"message":{"content":""}}],"usage":{"completion_tokens":64}}`)))
	// An empty body is still a genuine failure.
	require.Error(t, validatePongTestResponseBody([]byte(``)))
}

func TestModelAvailabilityProbeConfigUsesImageEndpointForImageModels(t *testing.T) {
	endpointType, options, testable := modelAvailabilityProbeConfig("gpt-image-2", constant.ChannelTypeOpenAI)

	require.True(t, testable)
	require.Equal(t, string(constant.EndpointTypeImageGeneration), endpointType)
	require.False(t, options.ExpectPong)
	require.Equal(t, "模型可用性检测", options.TokenName)
	require.Equal(t, "模型可用性检测", options.LogContent)
	require.True(t, options.SkipLog)
}

func TestModelAvailabilityProbeConfigKeepsPingPongForTextModels(t *testing.T) {
	endpointType, options, testable := modelAvailabilityProbeConfig("gpt-5.4", constant.ChannelTypeOpenAI)

	require.True(t, testable)
	require.Empty(t, endpointType)
	require.True(t, options.ExpectPong)
	require.Equal(t, modelAvailabilityProbePrompt, options.Prompt)
	require.Equal(t, uint(8), options.MaxTokens)
}

func TestModelAvailabilityProbeConfigMarksMediaModelsUntestable(t *testing.T) {
	// A TTS model on a generic chat channel cannot be probed synchronously.
	_, _, ttsTestable := modelAvailabilityProbeConfig("tts-1-hd", constant.ChannelTypeOpenAI)
	require.False(t, ttsTestable)

	// A video model likewise: no synchronous ping→pong is meaningful.
	_, _, videoTestable := modelAvailabilityProbeConfig("veo-3-video", constant.ChannelTypeOpenAI)
	require.False(t, videoTestable)

	// An async-task channel type is untestable regardless of the model name.
	_, _, taskTestable := modelAvailabilityProbeConfig("some-model", constant.ChannelTypeKling)
	require.False(t, taskTestable)

	_, _, bytePlusTestable := modelAvailabilityProbeConfig("some-model", constant.ChannelTypeBytePlus)
	require.False(t, bytePlusTestable)
}

func TestModelAvailabilityProbeConfigUsesEmbeddingEndpointForEmbeddingModels(t *testing.T) {
	endpointType, options, testable := modelAvailabilityProbeConfig("text-embedding-3-large", constant.ChannelTypeOpenAI)

	require.True(t, testable)
	require.Equal(t, string(constant.EndpointTypeEmbeddings), endpointType)
	require.False(t, options.ExpectPong)
}

func TestSummarizeModelProbeOutcomesKeepsUntestableProviderAvailable(t *testing.T) {
	outcome := summarizeModelProbeOutcomes([]modelProbeOutcome{
		{Class: modelProbeOfficialUnsupported, ReasonType: "official_model_unsupported"},
	}, true)

	require.Equal(t, modelProbeAvailable, outcome.Class)
}
