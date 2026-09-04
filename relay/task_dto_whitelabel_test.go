package relay

import (
	"encoding/json"
	"strconv"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/system_setting"
)

// kuaiziPlatform is the stringified channel type used as task.Platform for
// kuaizi video tasks (see relay.GetTaskPlatform).
var kuaiziPlatform = constant.TaskPlatform(strconv.Itoa(constant.ChannelTypeKuaiziLizhen))

func newKuaiziSuccessTask() *model.Task {
	return &model.Task{
		ID:       42,
		TaskID:   "task_abc",
		Platform: kuaiziPlatform,
		Status:   model.TaskStatusSuccess,
		Properties: model.Properties{
			UpstreamModelName: "kuaizi-lizhen-fast",
			OriginModelName:   "video-fast",
		},
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://api.example.com/v1/videos/task_abc/content",
		},
		FailReason: "",
		Data:       json.RawMessage(`{"code":0,"data":{"video_url":"https://x.volces.com/a.mp4","tos_key":"ai-open/x"}}`),
	}
}

func TestTaskModel2Dto_WhitelabelStripsEnvelopeAndUpstreamModel(t *testing.T) {
	task := newKuaiziSuccessTask()
	d := TaskModel2Dto(task)

	if d.Data != nil {
		t.Errorf("expected Data to be nil for whitelabel task, got %s", string(d.Data))
	}
	props, ok := d.Properties.(model.Properties)
	if !ok {
		t.Fatalf("expected Properties to be model.Properties, got %T", d.Properties)
	}
	if props.UpstreamModelName != "" {
		t.Errorf("expected UpstreamModelName stripped, got %q", props.UpstreamModelName)
	}
	if props.OriginModelName != "video-fast" {
		t.Errorf("OriginModelName must be preserved (customer-supplied), got %q", props.OriginModelName)
	}
	if d.ResultURL != "https://api.example.com/v1/videos/task_abc/content" {
		t.Errorf("ResultURL should be the proxy URL, got %q", d.ResultURL)
	}
}

func TestTaskModel2Dto_WhitelabelFailureScrubsFailReasonAndSkipsResultURLFallback(t *testing.T) {
	task := newKuaiziSuccessTask()
	task.Status = model.TaskStatusFailure
	task.PrivateData.ResultURL = ""
	task.FailReason = "kuaizi upstream code=500 message=internal"

	d := TaskModel2Dto(task)

	if d.ResultURL != "" {
		t.Errorf("ResultURL must not fall back to FailReason for whitelabel, got %q", d.ResultURL)
	}
	if d.FailReason == task.FailReason {
		t.Errorf("FailReason must be scrubbed, still contains branded text: %q", d.FailReason)
	}
	if d.FailReason != "task failed at upstream provider" {
		t.Errorf("unexpected scrubbed FailReason: %q", d.FailReason)
	}
}

func TestTaskModel2Dto_WhitelabelFailureKeepsCleanFailReason(t *testing.T) {
	task := newKuaiziSuccessTask()
	task.Status = model.TaskStatusFailure
	task.PrivateData.ResultURL = ""
	task.FailReason = "prompt rejected by safety filter"

	d := TaskModel2Dto(task)
	if d.FailReason != "prompt rejected by safety filter" {
		t.Errorf("clean FailReason must pass through, got %q", d.FailReason)
	}
}

func TestTaskModel2Dto_NonWhitelabelPlatformLeavesDataIntact(t *testing.T) {
	task := newKuaiziSuccessTask()
	task.Platform = constant.TaskPlatform("1") // pretend OpenAI

	d := TaskModel2Dto(task)
	if d.Data == nil {
		t.Error("non-whitelabel task should keep Data intact")
	}
	props := d.Properties.(model.Properties)
	if props.UpstreamModelName != "kuaizi-lizhen-fast" {
		t.Errorf("non-whitelabel must preserve UpstreamModelName, got %q", props.UpstreamModelName)
	}
}

func TestTaskModel2DtoAdmin_AlwaysReturnsFullPayload(t *testing.T) {
	task := newKuaiziSuccessTask()
	d := TaskModel2DtoAdmin(task)

	if d.Data == nil {
		t.Error("admin DTO must keep raw upstream Data for debugging")
	}
	props := d.Properties.(model.Properties)
	if props.UpstreamModelName != "kuaizi-lizhen-fast" {
		t.Errorf("admin DTO must keep UpstreamModelName, got %q", props.UpstreamModelName)
	}
}

func TestTaskModel2Dto_Channel106PlgAppliesResultPolicyAndWhitelabeling(t *testing.T) {
	originalServerAddress := system_setting.ServerAddress
	t.Cleanup(func() { system_setting.ServerAddress = originalServerAddress })
	system_setting.ServerAddress = "https://router.flatkey.ai"

	task := &model.Task{
		TaskID:    "task_106_plg",
		Platform:  constant.TaskPlatform("54"),
		ChannelId: 106,
		Group:     "plg",
		Status:    model.TaskStatusSuccess,
		Properties: model.Properties{
			UpstreamModelName: "doubao-seedance-private",
			OriginModelName:   "seedance-2.0",
		},
		PrivateData: model.TaskPrivateData{ResultURL: "https://cdn.volces.com/private.mp4"},
		Data:        json.RawMessage(`{"content":{"video_url":"https://cdn.volces.com/private.mp4"}}`),
	}

	d := TaskModel2Dto(task)
	if d.Data != nil {
		t.Fatalf("Data must be hidden, got %s", d.Data)
	}
	props := d.Properties.(model.Properties)
	if props.UpstreamModelName != "" || props.OriginModelName != "seedance-2.0" {
		t.Fatalf("properties = %+v", props)
	}
	if d.ResultURL != "https://router.flatkey.ai/v1/videos/task_106_plg/content" {
		t.Fatalf("ResultURL = %q", d.ResultURL)
	}
}

func TestTaskModel2Dto_Channel106PlgFailureScrubsWithoutResultURL(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_failed",
		Platform:   constant.TaskPlatform("54"),
		ChannelId:  106,
		Group:      "plg",
		Status:     model.TaskStatusFailure,
		FailReason: "Doubao Ark endpoint failed",
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://cdn.volces.com/failed-private.mp4",
		},
		Properties: model.Properties{UpstreamModelName: "doubao-private"},
		Data:       json.RawMessage(`{"error":{"message":"Volcengine endpoint failed"}}`),
	}

	d := TaskModel2Dto(task)
	if d.ResultURL != "" {
		t.Fatalf("failed task ResultURL = %q, want empty", d.ResultURL)
	}
	if d.FailReason != "task failed at upstream provider" {
		t.Fatalf("FailReason = %q", d.FailReason)
	}
	if d.Data != nil {
		t.Fatalf("Data must be hidden, got %s", d.Data)
	}
}

func TestTaskModel2Dto_Channel106NonPlgKeepsUpstreamFields(t *testing.T) {
	task := &model.Task{
		TaskID:    "task_106_default",
		Platform:  constant.TaskPlatform("54"),
		ChannelId: 106,
		Group:     "default",
		Status:    model.TaskStatusSuccess,
		Properties: model.Properties{
			UpstreamModelName: "doubao-seedance-private",
			OriginModelName:   "seedance-2.0",
		},
		PrivateData: model.TaskPrivateData{ResultURL: "https://cdn.volces.com/upstream.mp4"},
		Data:        json.RawMessage(`{"content":{"video_url":"https://cdn.volces.com/upstream.mp4"}}`),
	}

	d := TaskModel2Dto(task)
	if d.Data == nil {
		t.Fatal("non-plg task must keep upstream Data")
	}
	if d.Properties.(model.Properties).UpstreamModelName != "doubao-seedance-private" {
		t.Fatalf("properties = %+v", d.Properties)
	}
	if d.ResultURL != "https://cdn.volces.com/upstream.mp4" {
		t.Fatalf("ResultURL = %q", d.ResultURL)
	}
}

func TestTaskModel2DtoAdmin_Channel106PlgKeepsRawPayload(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_failed",
		Platform:   constant.TaskPlatform("54"),
		ChannelId:  106,
		Group:      "plg",
		Status:     model.TaskStatusFailure,
		FailReason: "Doubao Ark endpoint failed",
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://cdn.volces.com/failed-private.mp4",
		},
		Properties: model.Properties{UpstreamModelName: "doubao-private"},
		Data:       json.RawMessage(`{"error":{"message":"Volcengine endpoint failed"}}`),
	}

	d := TaskModel2DtoAdmin(task)
	if d.Data == nil || d.FailReason != task.FailReason {
		t.Fatalf("admin DTO lost raw fields: %+v", d)
	}
	if d.Properties.(model.Properties).UpstreamModelName != "doubao-private" {
		t.Fatalf("admin properties = %+v", d.Properties)
	}
}

func TestGenerationTaskRespBody_Channel106PlgUsesPublicURL(t *testing.T) {
	originalServerAddress := system_setting.ServerAddress
	t.Cleanup(func() { system_setting.ServerAddress = originalServerAddress })
	system_setting.ServerAddress = "https://router.flatkey.ai"

	task := &model.Task{
		TaskID:      "task_generation_106",
		ChannelId:   106,
		Group:       "plg",
		Status:      model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{ResultURL: "https://cdn.volces.com/private.mp4"},
	}
	out, err := generationTaskRespBody(task)
	if err != nil {
		t.Fatalf("generationTaskRespBody error: %v", err)
	}
	var got struct {
		Content []struct {
			VideoURL struct {
				URL string `json:"url"`
			} `json:"video_url"`
		} `json:"content"`
	}
	if err := common.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(got.Content) != 1 || got.Content[0].VideoURL.URL != "https://router.flatkey.ai/v1/videos/task_generation_106/content" {
		t.Fatalf("content = %+v", got.Content)
	}
}

func TestGenerationTaskRespBody_Channel106PlgFailureHasNoContentAndScrubs(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_generation_failed",
		ChannelId:  106,
		Group:      "plg",
		Status:     model.TaskStatusFailure,
		FailReason: "Doubao Ark endpoint failed",
		PrivateData: model.TaskPrivateData{
			ResultURL: "https://cdn.volces.com/failed-private.mp4",
		},
	}
	out, err := generationTaskRespBody(task)
	if err != nil {
		t.Fatalf("generationTaskRespBody error: %v", err)
	}
	var got struct {
		Content []generationTaskContent `json:"content"`
		Error   *dto.OpenAIVideoError   `json:"error"`
	}
	if err := common.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(got.Content) != 0 {
		t.Fatalf("failed task content = %+v, want empty", got.Content)
	}
	if got.Error == nil || got.Error.Message != "task failed at upstream provider" {
		t.Fatalf("error = %+v", got.Error)
	}
}

func TestGenerationTaskRespBody_Channel106NonPlgKeepsUpstreamURL(t *testing.T) {
	task := &model.Task{
		TaskID:      "task_generation_default",
		ChannelId:   106,
		Group:       "default",
		Status:      model.TaskStatusSuccess,
		PrivateData: model.TaskPrivateData{ResultURL: "https://cdn.volces.com/upstream.mp4"},
	}
	out, err := generationTaskRespBody(task)
	if err != nil {
		t.Fatalf("generationTaskRespBody error: %v", err)
	}
	var got struct {
		Content []struct {
			VideoURL struct {
				URL string `json:"url"`
			} `json:"video_url"`
		} `json:"content"`
	}
	if err := common.Unmarshal(out, &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(got.Content) != 1 || got.Content[0].VideoURL.URL != "https://cdn.volces.com/upstream.mp4" {
		t.Fatalf("content = %+v", got.Content)
	}
}
