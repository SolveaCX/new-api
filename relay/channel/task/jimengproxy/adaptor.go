package jimengproxy

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/dto"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/channel"
	"github.com/QuantumNous/new-api/relay/channel/task/taskcommon"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

// ============================
// Request / Response structures
// ============================

// submitPayload 发往即梦反代 POST /v1/videos/submit 的请求体。
type submitPayload struct {
	Model      string   `json:"model"`
	Prompt     string   `json:"prompt"`
	Ratio      string   `json:"ratio,omitempty"`
	Resolution string   `json:"resolution,omitempty"`
	Duration   int      `json:"duration,omitempty"`
	FilePaths  []string `json:"file_paths,omitempty"`
}

// submitResponse 是 /v1/videos/submit 的响应。
// submit_id 即即梦 history_record_id —— 无服务端任务态,状态来源于即梦侧,多节点安全。
type submitResponse struct {
	SubmitID string `json:"submit_id"`
	Model    string `json:"model"`
	Status   string `json:"status"`
}

// queryResponse 是 /v1/videos/query 的响应。
type queryResponse struct {
	SubmitID string `json:"submit_id"`
	Status   string `json:"status"` // processing | completed | failed
	FailCode int    `json:"fail_code"`
	Data     []struct {
		URL string `json:"url"`
	} `json:"data"`
}

// ============================
// Adaptor implementation
// ============================

type TaskAdaptor struct {
	taskcommon.BaseBilling
	ChannelType int
	apiKey      string
	baseURL     string
}

func (a *TaskAdaptor) Init(info *relaycommon.RelayInfo) {
	a.ChannelType = info.ChannelType
	a.baseURL = info.ChannelBaseUrl
	a.apiKey = info.ApiKey
}

func (a *TaskAdaptor) ValidateRequestAndSetAction(c *gin.Context, info *relaycommon.RelayInfo) *dto.TaskError {
	return relaycommon.ValidateBasicTaskRequest(c, info, constant.TaskActionGenerate)
}

// BuildRequestURL 提交任务: POST {baseURL}/v1/videos/submit
func (a *TaskAdaptor) BuildRequestURL(_ *relaycommon.RelayInfo) (string, error) {
	return fmt.Sprintf("%s/v1/videos/submit", a.baseURL), nil
}

func (a *TaskAdaptor) BuildRequestHeader(_ *gin.Context, req *http.Request, _ *relaycommon.RelayInfo) error {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+a.apiKey)
	return nil
}

func (a *TaskAdaptor) BuildRequestBody(c *gin.Context, info *relaycommon.RelayInfo) (io.Reader, error) {
	req, err := relaycommon.GetTaskRequest(c)
	if err != nil {
		return nil, err
	}

	body := a.convertToSubmitPayload(&req)
	if info.IsModelMapped {
		body.Model = info.UpstreamModelName
	} else {
		info.UpstreamModelName = body.Model
	}
	data, err := common.Marshal(body)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}

func (a *TaskAdaptor) convertToSubmitPayload(req *relaycommon.TaskSubmitReq) *submitPayload {
	p := &submitPayload{
		Model:      req.Model,
		Prompt:     req.Prompt,
		Resolution: req.Resolution,
		Ratio:      req.Ratio,
	}
	if req.Duration > 0 {
		p.Duration = req.Duration
	}
	// 图生视频:反代以 file_paths 接收输入图(URL,最多 2 张)。
	if req.HasImage() {
		p.FilePaths = req.Images
	}
	return p
}

func (a *TaskAdaptor) DoRequest(c *gin.Context, info *relaycommon.RelayInfo, requestBody io.Reader) (*http.Response, error) {
	return channel.DoTaskApiRequest(a, c, info, requestBody)
}

func (a *TaskAdaptor) DoResponse(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo) (taskID string, taskData []byte, taskErr *dto.TaskError) {
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		taskErr = service.TaskErrorWrapper(err, "read_response_body_failed", http.StatusInternalServerError)
		return
	}
	_ = resp.Body.Close()

	var sResp submitResponse
	if err := common.Unmarshal(responseBody, &sResp); err != nil {
		taskErr = service.TaskErrorWrapper(errors.Wrapf(err, "body: %s", responseBody), "unmarshal_response_body_failed", http.StatusInternalServerError)
		return
	}

	// 提交成功必须拿到 submit_id;若上游即时拒绝(failed),视为提交失败,
	// 避免白白进入轮询并占用预扣额。
	if sResp.SubmitID == "" || sResp.Status == "failed" {
		taskErr = service.TaskErrorWrapper(fmt.Errorf("submit task failed, body=%s", responseBody), "invalid_response", http.StatusBadGateway)
		return
	}

	// 用公开 task_xxxx ID 返回给客户端,不暴露上游 submit_id(history_record_id)。
	ov := dto.NewOpenAIVideo()
	ov.ID = info.PublicTaskID
	ov.TaskID = info.PublicTaskID
	ov.CreatedAt = time.Now().Unix()
	ov.Model = info.OriginModelName

	c.JSON(http.StatusOK, ov)
	return sResp.SubmitID, responseBody, nil
}

// FetchTask 轮询: POST {baseURL}/v1/videos/query  body={"submit_id": <id>}
func (a *TaskAdaptor) FetchTask(baseUrl, key string, body map[string]any, proxy string) (*http.Response, error) {
	taskID, ok := body["task_id"].(string)
	if !ok || taskID == "" {
		return nil, fmt.Errorf("invalid task_id")
	}

	payload, err := common.Marshal(map[string]string{"submit_id": taskID})
	if err != nil {
		return nil, err
	}

	uri := fmt.Sprintf("%s/v1/videos/query", baseUrl)
	req, err := http.NewRequest(http.MethodPost, uri, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+key)

	client, err := service.GetHttpClientWithProxy(proxy)
	if err != nil {
		return nil, fmt.Errorf("new proxy http client failed: %w", err)
	}
	return client.Do(req)
}

func (a *TaskAdaptor) ParseTaskResult(respBody []byte) (*relaycommon.TaskInfo, error) {
	var qr queryResponse
	if err := common.Unmarshal(respBody, &qr); err != nil {
		return nil, errors.Wrap(err, "unmarshal task result failed")
	}

	info := &relaycommon.TaskInfo{Code: 0}
	switch qr.Status {
	case "completed":
		info.Status = model.TaskStatusSuccess
		info.Progress = "100%"
		info.Url = resultURL(qr)
	case "failed":
		info.Status = model.TaskStatusFailure
		info.Progress = "100%"
		info.Reason = failReason(qr)
	case "processing":
		info.Status = model.TaskStatusInProgress
		info.Progress = "50%"
	default:
		// 未知状态:保持进行中,等待下一轮轮询。
		info.Status = model.TaskStatusInProgress
		info.Progress = "30%"
	}
	return info, nil
}

// resultURL 返回查询响应里的视频地址:取 data[0].url,无则 ""。
func resultURL(qr queryResponse) string {
	if len(qr.Data) > 0 {
		return qr.Data[0].URL
	}
	return ""
}

func failReason(qr queryResponse) string {
	if qr.FailCode != 0 {
		return fmt.Sprintf("jimeng video generation failed, fail_code=%d", qr.FailCode)
	}
	return "jimeng video generation failed"
}

func (a *TaskAdaptor) GetModelList() []string {
	return ModelList
}

func (a *TaskAdaptor) GetChannelName() string {
	return ChannelName
}

// ConvertToOpenAIVideo 构造返回给客户端的 OpenAI Video API 对象。
// 非白标渠道:成功时直接返回即梦(dreamina)视频地址,失败时回错误信息。
func (a *TaskAdaptor) ConvertToOpenAIVideo(originTask *model.Task) ([]byte, error) {
	ov := dto.NewOpenAIVideo()
	ov.ID = originTask.TaskID
	ov.TaskID = originTask.TaskID
	ov.Status = originTask.Status.ToVideoStatus()
	ov.SetProgressStr(originTask.Progress)
	ov.CreatedAt = originTask.CreatedAt
	ov.CompletedAt = originTask.UpdatedAt
	ov.Model = originTask.Properties.OriginModelName

	if originTask.Status == model.TaskStatusSuccess {
		ov.SetMetadata("url", originTask.GetResultURL())
	}
	if originTask.Status == model.TaskStatusFailure {
		ov.Error = &dto.OpenAIVideoError{
			Message: originTask.FailReason,
		}
	}

	return common.Marshal(ov)
}
