package groksubscription

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"

	"github.com/gin-gonic/gin"
)

func newAdaptorTestContext(body string) (*gin.Context, *relaycommon.RelayInfo) {
	c, info, _ := newAdaptorTestContextWithRecorder(body)
	return c, info
}

func newAdaptorTestContextWithRecorder(body string) (*gin.Context, *relaycommon.RelayInfo, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/videos", strings.NewReader(body))
	c.Request.Header.Set("Content-Type", "application/json")
	return c, &relaycommon.RelayInfo{
		ChannelMeta: &relaycommon.ChannelMeta{
			ChannelId:      27,
			ChannelType:    constant.ChannelTypeGrokSubscription,
			ChannelBaseUrl: "https://ignored.example",
			ApiKey:         "must-not-leak",
		},
		TaskRelayInfo:   &relaycommon.TaskRelayInfo{PublicTaskID: "task_public"},
		OriginModelName: ModelGrokImagineVideo15,
	}, w
}

func TestAdaptorBuildsFixedActionURLsAndOmitsActionFromBody(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{"generate", `{"model":"grok-imagine-video-1.5","action":"generate","prompt":"x"}`, "https://api.x.ai/v1/videos/generations"},
		{"edit", `{"model":"grok-imagine-video","action":"edit","prompt":"x","video":{"url":"https://example.com/in.mp4"}}`, "https://api.x.ai/v1/videos/edits"},
		{"extend", `{"model":"grok-imagine-video","action":"extend","prompt":"x","video":{"url":"https://example.com/in.mp4"}}`, "https://api.x.ai/v1/videos/extensions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, info := newAdaptorTestContext(tt.body)
			a := &TaskAdaptor{}
			a.Init(info)
			if taskErr := a.ValidateRequestAndSetAction(c, info); taskErr != nil {
				t.Fatalf("ValidateRequestAndSetAction: %v", taskErr)
			}
			got, err := a.BuildRequestURL(info)
			if err != nil {
				t.Fatalf("BuildRequestURL: %v", err)
			}
			if got != tt.want {
				t.Fatalf("BuildRequestURL() = %q, want %q", got, tt.want)
			}
			body, err := a.BuildRequestBody(c, info)
			if err != nil {
				t.Fatalf("BuildRequestBody: %v", err)
			}
			data, _ := io.ReadAll(body)
			if strings.Contains(string(data), `"action"`) {
				t.Fatalf("upstream body leaked internal action selector: %s", data)
			}
		})
	}
}

func TestBuildRequestBodyUsesMappedUpstreamModel(t *testing.T) {
	c, info := newAdaptorTestContext(`{"model":"grok-imagine-video-1.5","prompt":"x"}`)
	info.UpstreamModelName = "mapped-grok-video"
	a := &TaskAdaptor{}
	if taskErr := a.ValidateRequestAndSetAction(c, info); taskErr != nil {
		t.Fatalf("ValidateRequestAndSetAction: %v", taskErr)
	}
	body, err := a.BuildRequestBody(c, info)
	if err != nil {
		t.Fatalf("BuildRequestBody: %v", err)
	}
	data, _ := io.ReadAll(body)
	if !strings.Contains(string(data), `"model":"mapped-grok-video"`) {
		t.Fatalf("body = %s, want mapped upstream model", data)
	}
}

func TestBuildRequestHeaderUsesPaidMediaBearerOnly(t *testing.T) {
	var called struct {
		channelID   int
		requirePaid bool
	}
	restore := setMediaCredentialForTest(func(ctx *gin.Context, info *relaycommon.RelayInfo) (string, error) {
		called.channelID = info.ChannelId
		called.requirePaid = true
		return "current-paid-token", nil
	})
	defer restore()

	c, info := newAdaptorTestContext(`{"model":"grok-imagine-video-1.5","prompt":"x"}`)
	req := httptest.NewRequest(http.MethodPost, "https://api.x.ai/v1/videos/generations", nil)
	req.Header.Set("X-Should-Be-Removed", "yes")
	info.ChannelId = 27
	info.ApiKey = "stale-channel-key"

	if err := (&TaskAdaptor{}).BuildRequestHeader(c, req, info); err != nil {
		t.Fatalf("BuildRequestHeader: %v", err)
	}
	if called.channelID != 27 || !called.requirePaid {
		t.Fatalf("preflight call = %+v, want channel 27 requirePaid", called)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer current-paid-token" {
		t.Fatalf("Authorization = %q", got)
	}
	if got := req.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q", got)
	}
	if got := req.Header.Get("Accept"); got != "application/json" {
		t.Fatalf("Accept = %q", got)
	}
	if req.Header.Get("X-Should-Be-Removed") != "" {
		t.Fatalf("unexpected custom header survived: %#v", req.Header)
	}
	if strings.Contains(req.Header.Get("Authorization"), info.ApiKey) {
		t.Fatal("BuildRequestHeader used info.ApiKey instead of current OAuth credential")
	}
}

func TestNormalizeAcceptedStatus(t *testing.T) {
	resp := &http.Response{StatusCode: http.StatusAccepted}
	normalizeAcceptedStatus(resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestDoResponseIsolatesPublicAndUpstreamIDs(t *testing.T) {
	c, info, recorder := newAdaptorTestContextWithRecorder(`{"model":"grok-imagine-video-1.5","prompt":"x"}`)
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(`{"request_id":"upstream-secret"}`)),
	}
	taskID, taskData, taskErr := (&TaskAdaptor{}).DoResponse(c, resp, info)
	if taskErr != nil {
		t.Fatalf("DoResponse error: %v", taskErr)
	}
	if taskID != "upstream-secret" {
		t.Fatalf("taskID = %q, want private upstream ID", taskID)
	}
	if string(taskData) != `{}` {
		t.Fatalf("taskData = %s, want sanitized empty object", taskData)
	}
	body := recorder.Body.String()
	if strings.Contains(body, "upstream-secret") {
		t.Fatalf("public response leaked upstream ID: %s", body)
	}
	if !strings.Contains(body, "task_public") {
		t.Fatalf("public response = %s, want public task id", body)
	}
}

func TestDoResponseSanitizesMalformedSubmitErrors(t *testing.T) {
	c, info := newAdaptorTestContext(`{"model":"grok-imagine-video-1.5","prompt":"x"}`)
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(`{"error":{"message":"xAI api.x.ai token url https://api.x.ai/tmp"}}`)),
	}
	_, _, taskErr := (&TaskAdaptor{}).DoResponse(c, resp, info)
	if taskErr == nil {
		t.Fatal("expected task error")
	}
	msg := taskErr.Message
	for _, forbidden := range []string{"xAI", "x.ai", "api.x.ai", "https://", "token"} {
		if strings.Contains(strings.ToLower(msg), strings.ToLower(forbidden)) {
			t.Fatalf("error leaked %q in %q", forbidden, msg)
		}
	}
}

func TestParseTaskResultMapsDocumentedStatesAndRejectsUnknown(t *testing.T) {
	a := &TaskAdaptor{}
	tests := []struct {
		name       string
		body       string
		wantStatus model.TaskStatus
		wantURL    string
		wantUsage  bool
	}{
		{"pending", `{"status":"pending"}`, model.TaskStatusQueued, "", false},
		{"done", `{"status":"done","video":{"url":"https://vidgen.x.ai/tmp.mp4","duration":5,"resolution":"720p"},"usage":{"completion_tokens":7,"total_tokens":9,"cost_in_usd_ticks":123}}`, model.TaskStatusSuccess, "https://vidgen.x.ai/tmp.mp4", true},
		{"failed", `{"status":"failed","error":{"message":"Grok failed"}}`, model.TaskStatusFailure, "", false},
		{"expired", `{"status":"expired","message":"x.ai url expired"}`, model.TaskStatusFailure, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := a.ParseTaskResult([]byte(tt.body))
			if err != nil {
				t.Fatalf("ParseTaskResult: %v", err)
			}
			if info.Status != string(tt.wantStatus) {
				t.Fatalf("status = %q, want %q", info.Status, tt.wantStatus)
			}
			if info.Url != tt.wantURL {
				t.Fatalf("url = %q, want %q", info.Url, tt.wantURL)
			}
			if tt.wantUsage && (info.CompletionTokens != 7 || info.TotalTokens != 9) {
				t.Fatalf("usage = %d/%d, want 7/9", info.CompletionTokens, info.TotalTokens)
			}
			if tt.wantStatus == model.TaskStatusFailure && taskcommon.ContainsBrandKeyword(info.Reason) {
				t.Fatalf("failure reason was not scrubbed: %q", info.Reason)
			}
		})
	}
	if _, err := a.ParseTaskResult([]byte(`{"status":"mystery"}`)); err == nil {
		t.Fatal("unknown status must return an error so polling preserves prior state")
	}
	if _, err := a.ParseTaskResult([]byte(`{"status":""}`)); err == nil {
		t.Fatal("blank status must return an error")
	}
}

func TestParseTaskResultRequiresVideoURLOnDone(t *testing.T) {
	if _, err := (&TaskAdaptor{}).ParseTaskResult([]byte(`{"status":"done","video":{}}`)); err == nil {
		t.Fatal("done without video URL must fail")
	}
}

func TestConvertToOpenAIVideoUsesOnlyPublicTaskFields(t *testing.T) {
	task := &model.Task{
		TaskID:     "task_public",
		Status:     model.TaskStatusSuccess,
		Progress:   "100%",
		CreatedAt:  10,
		UpdatedAt:  20,
		FailReason: "https://vidgen.x.ai/tmp.mp4",
		Properties: model.Properties{OriginModelName: ModelGrokImagineVideo15},
		PrivateData: model.TaskPrivateData{
			UpstreamTaskID:   "upstream-secret",
			ResultURL:        "https://flatkey.example/v1/videos/task_public/content",
			CompletionTokens: 7,
			TotalTokens:      9,
		},
		Data: []byte(`{"request_id":"upstream-secret","video":{"url":"https://vidgen.x.ai/tmp.mp4"}}`),
	}
	data, err := (&TaskAdaptor{}).ConvertToOpenAIVideo(task)
	if err != nil {
		t.Fatalf("ConvertToOpenAIVideo: %v", err)
	}
	out := string(data)
	for _, forbidden := range []string{"upstream-secret", "vidgen.x.ai", "tmp.mp4"} {
		if strings.Contains(out, forbidden) {
			t.Fatalf("OpenAI video leaked %q: %s", forbidden, out)
		}
	}
	if !strings.Contains(out, "task_public") || !strings.Contains(out, "flatkey.example/v1/videos/task_public/content") {
		t.Fatalf("OpenAI video did not use public fields/proxy URL: %s", out)
	}
}

func TestValidateRequestAndSetActionUsesLocalBadRequest(t *testing.T) {
	c, info := newAdaptorTestContext(`{"model":"grok-imagine-video","action":"extend","prompt":"x"}`)
	taskErr := (&TaskAdaptor{}).ValidateRequestAndSetAction(c, info)
	if taskErr == nil {
		t.Fatal("expected validation error")
	}
	if taskErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", taskErr.StatusCode)
	}
}

var _ = common.Marshal
var _ = dto.VideoStatusQueued
