package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/stretchr/testify/require"
)

func TestDoubaoVideoProxyProtectedContentUsesStoredKeyAndSupportsRange(t *testing.T) {
	restoreDB := useVideoProxyDBForTest(t)
	defer restoreDB()
	restoreFetchSetting := allowPrivateVideoProxyURLsForTest(t)
	defer restoreFetchSetting()
	service.InitHttpClient()

	var gotAuthorization, gotRange, gotIfRange string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuthorization = r.Header.Get("Authorization")
		gotRange = r.Header.Get("Range")
		gotIfRange = r.Header.Get("If-Range")
		w.Header().Set("Content-Type", "video/mp4")
		w.Header().Set("Content-Range", "bytes 0-3/8")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = w.Write([]byte("part"))
	}))
	defer upstream.Close()

	seedDoubaoVideoProxyTask(t, 272, "task-protected", "upstream-protected", upstream.URL,
		upstream.URL+"/v1/videos/upstream-protected/content", "channel-key", "stored-key")
	recorder, c := newChannel106VideoProxyContext("task-protected")
	c.Request.Header.Set("Range", "bytes=0-3")
	c.Request.Header.Set("If-Range", `"etag-1"`)
	VideoProxy(c)

	require.Equal(t, http.StatusPartialContent, recorder.Code)
	require.Equal(t, "part", recorder.Body.String())
	require.Equal(t, "bytes 0-3/8", recorder.Header().Get("Content-Range"))
	require.Equal(t, "Bearer stored-key", gotAuthorization)
	require.Equal(t, "bytes=0-3", gotRange)
	require.Equal(t, `"etag-1"`, gotIfRange)
}

func TestDoubaoVideoProxyOnlyAuthorizesExactProtectedContentURL(t *testing.T) {
	restoreDB := useVideoProxyDBForTest(t)
	defer restoreDB()
	restoreFetchSetting := allowPrivateVideoProxyURLsForTest(t)
	defer restoreFetchSetting()
	service.InitHttpClient()

	var mu sync.Mutex
	authorizations := map[string]string{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		authorizations[r.URL.RequestURI()] = r.Header.Get("Authorization")
		mu.Unlock()
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		authorizations["foreign"] = r.Header.Get("Authorization")
		mu.Unlock()
		_, _ = w.Write([]byte("ok"))
	}))
	defer foreign.Close()

	tests := []struct {
		name      string
		resultURL string
	}{
		{name: "foreign host", resultURL: foreign.URL + "/v1/videos/upstream-boundary/content"},
		{name: "lookalike prefix", resultURL: upstream.URL + "/v1/videos/upstream-boundary/content/preview"},
		{name: "query", resultURL: upstream.URL + "/v1/videos/upstream-boundary/content?download=1"},
		{name: "userinfo", resultURL: strings.Replace(upstream.URL, "://", "://user@", 1) + "/v1/videos/upstream-boundary/content"},
		{name: "fragment", resultURL: upstream.URL + "/v1/videos/upstream-boundary/content#preview"},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			taskID := "task-boundary-" + strings.ReplaceAll(tt.name, " ", "-")
			seedDoubaoVideoProxyTask(t, 300+i, taskID, "upstream-boundary", upstream.URL, tt.resultURL, "channel-secret", "")
			recorder, c := newChannel106VideoProxyContext(taskID)
			VideoProxy(c)
			require.Equal(t, http.StatusOK, recorder.Code)
		})
	}

	mu.Lock()
	defer mu.Unlock()
	for requestURI, authorization := range authorizations {
		require.NotEqual(t, "Bearer channel-secret", authorization, "credential leaked for %s", requestURI)
	}
}

func TestDoubaoVideoProxyProtectedRedirectDoesNotLeakKeyAcrossOrigin(t *testing.T) {
	restoreDB := useVideoProxyDBForTest(t)
	defer restoreDB()
	restoreFetchSetting := allowPrivateVideoProxyURLsForTest(t)
	defer restoreFetchSetting()
	service.InitHttpClient()

	var redirectHits atomic.Int32
	var redirectedAuthorization string
	foreign := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectHits.Add(1)
		redirectedAuthorization = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("foreign"))
	}))
	defer foreign.Close()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, foreign.URL+"/video.mp4", http.StatusFound)
	}))
	defer upstream.Close()

	seedDoubaoVideoProxyTask(t, 350, "task-redirect", "upstream-redirect", upstream.URL,
		upstream.URL+"/v1/videos/upstream-redirect/content", "redirect-secret", "")
	recorder, c := newChannel106VideoProxyContext("task-redirect")
	VideoProxy(c)

	require.Equal(t, http.StatusBadGateway, recorder.Code)
	require.Zero(t, redirectHits.Load())
	require.Empty(t, redirectedAuthorization)
}

func TestNonDoubaoVideoProxyDoesNotGainAuthorization(t *testing.T) {
	restoreDB := useVideoProxyDBForTest(t)
	defer restoreDB()
	restoreFetchSetting := allowPrivateVideoProxyURLsForTest(t)
	defer restoreFetchSetting()
	service.InitHttpClient()

	var authorization string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization = r.Header.Get("Authorization")
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()
	baseURL := upstream.URL
	require.NoError(t, model.DB.Create(&model.Channel{
		Id: 400, Type: constant.ChannelTypeUnknown, Key: "must-not-leak", BaseURL: &baseURL,
		Status: common.ChannelStatusEnabled, Name: "non-doubao", Group: "default",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Task{
		TaskID: "task-non-doubao-auth", ChannelId: 400, Status: model.TaskStatusSuccess, Progress: "100%",
		PrivateData: model.TaskPrivateData{UpstreamTaskID: "upstream-non-doubao", ResultURL: upstream.URL + "/v1/videos/upstream-non-doubao/content"},
	}).Error)

	recorder, c := newChannel106VideoProxyContext("task-non-doubao-auth")
	VideoProxy(c)
	require.Equal(t, http.StatusOK, recorder.Code)
	require.Empty(t, authorization)
}

func seedDoubaoVideoProxyTask(t *testing.T, channelID int, taskID, upstreamTaskID, baseURL, resultURL, channelKey, privateKey string) {
	t.Helper()
	require.NoError(t, model.DB.Create(&model.Channel{
		Id: channelID, Type: constant.ChannelTypeDoubaoVideo, Key: channelKey, BaseURL: &baseURL,
		Status: common.ChannelStatusEnabled, Name: taskID, Group: "seedanceofficial",
	}).Error)
	require.NoError(t, model.DB.Create(&model.Task{
		TaskID: taskID, ChannelId: channelID, Status: model.TaskStatusSuccess, Progress: "100%",
		PrivateData: model.TaskPrivateData{Key: privateKey, UpstreamTaskID: upstreamTaskID, ResultURL: resultURL},
	}).Error)
}
