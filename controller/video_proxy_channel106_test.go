package controller

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestVideoProxyChannel106UsesPersistedUpstreamURLInsteadOfFlatkeyResultURL(t *testing.T) {
	restoreDB := useVideoProxyDBForTest(t)
	defer restoreDB()
	restoreFetchSetting := allowPrivateVideoProxyURLsForTest(t)
	defer restoreFetchSetting()
	service.InitHttpClient()

	var upstreamHits atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamHits.Add(1)
		require.Equal(t, "/upstream.mp4", r.URL.Path)
		w.Header().Set("Content-Type", "video/mp4")
		_, _ = w.Write([]byte("channel-106-video"))
	}))
	defer upstream.Close()

	var flatkeyHits atomic.Int32
	flatkey := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flatkeyHits.Add(1)
		http.Error(w, "recursive Flatkey content request", http.StatusLoopDetected)
	}))
	defer flatkey.Close()

	require.NoError(t, model.DB.Create(&model.Channel{
		Id:     106,
		Type:   constant.ChannelTypeDoubaoVideo,
		Status: common.ChannelStatusEnabled,
		Name:   "channel-106",
		Group:  "plg",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Task{
		TaskID:    "task-channel-106-plg",
		ChannelId: 106,
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
		PrivateData: model.TaskPrivateData{
			ResultURL: flatkey.URL + "/v1/videos/task-channel-106-plg/content",
		},
		Data: []byte(`{"id":"upstream-task","status":"succeeded","content":{"video_url":"` + upstream.URL + `/upstream.mp4"}}`),
	}).Error)

	recorder, c := newChannel106VideoProxyContext("task-channel-106-plg")
	VideoProxy(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "channel-106-video", recorder.Body.String())
	require.EqualValues(t, 1, upstreamHits.Load())
	require.Zero(t, flatkeyHits.Load())
}

func TestVideoProxyNon106DoubaoChannelKeepsUsingResultURL(t *testing.T) {
	restoreDB := useVideoProxyDBForTest(t)
	defer restoreDB()
	restoreFetchSetting := allowPrivateVideoProxyURLsForTest(t)
	defer restoreFetchSetting()
	service.InitHttpClient()

	var resultHits atomic.Int32
	result := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resultHits.Add(1)
		_, _ = w.Write([]byte("existing-result-url"))
	}))
	defer result.Close()

	var dataURLHits atomic.Int32
	dataURL := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dataURLHits.Add(1)
		_, _ = w.Write([]byte("unexpected-data-url"))
	}))
	defer dataURL.Close()

	require.NoError(t, model.DB.Create(&model.Channel{
		Id:     107,
		Type:   constant.ChannelTypeDoubaoVideo,
		Status: common.ChannelStatusEnabled,
		Name:   "non-106-doubao",
		Group:  "default",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Task{
		TaskID:    "task-non-106-doubao",
		ChannelId: 107,
		Status:    model.TaskStatusSuccess,
		Progress:  "100%",
		PrivateData: model.TaskPrivateData{
			ResultURL: result.URL + "/result.mp4",
		},
		Data: []byte(`{"id":"upstream-task","status":"succeeded","content":{"video_url":"` + dataURL.URL + `/data.mp4"}}`),
	}).Error)

	recorder, c := newChannel106VideoProxyContext("task-non-106-doubao")
	VideoProxy(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "existing-result-url", recorder.Body.String())
	require.EqualValues(t, 1, resultHits.Load())
	require.Zero(t, dataURLHits.Load())
}

func allowPrivateVideoProxyURLsForTest(t *testing.T) func() {
	t.Helper()
	setting := system_setting.GetFetchSetting()
	original := *setting
	setting.EnableSSRFProtection = false
	return func() { *setting = original }
}

func newChannel106VideoProxyContext(taskID string) (*httptest.ResponseRecorder, *gin.Context) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Params = gin.Params{{Key: "task_id", Value: taskID}}
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/videos/"+taskID+"/content", nil)
	return recorder, c
}
