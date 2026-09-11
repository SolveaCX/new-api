package service

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting/operation_setting"
	"github.com/stretchr/testify/require"
)

type plgCatalogWatchTestServer struct {
	server   *httptest.Server
	requests int32
	bodies   chan string
}

func newPLGCatalogWatchTestServer(t *testing.T) *plgCatalogWatchTestServer {
	t.Helper()
	ts := &plgCatalogWatchTestServer{bodies: make(chan string, 8)}
	ts.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		atomic.AddInt32(&ts.requests, 1)
		ts.bodies <- string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"errcode":0,"errmsg":"ok"}`))
	}))
	t.Cleanup(ts.server.Close)
	return ts
}

func (ts *plgCatalogWatchTestServer) count() int32 {
	return atomic.LoadInt32(&ts.requests)
}

func setupPLGCatalogWatchTest(t *testing.T) *plgCatalogWatchTestServer {
	t.Helper()
	allowDingTalkTestServer(t)
	db, _ := setupServiceModelAccessDB(t)
	require.NoError(t, db.AutoMigrate(&model.PLGModelCatalogSnapshot{}))

	originalSetting := *operation_setting.GetPLGCatalogNotifySetting()
	originalHTTPClient := httpClient
	t.Cleanup(func() {
		*operation_setting.GetPLGCatalogNotifySetting() = originalSetting
		httpClient = originalHTTPClient
	})

	ts := newPLGCatalogWatchTestServer(t)
	httpClient = ts.server.Client()
	setting := operation_setting.GetPLGCatalogNotifySetting()
	setting.DingTalkAlertEnabled = true
	setting.DingTalkAlertWebhookURL = ts.server.URL
	setting.DingTalkAlertSecret = ""
	return ts
}

func seedPLGCatalog(t *testing.T, channelID int, modelNames ...string) {
	t.Helper()
	seedModelAccessScope(t, model.DB, channelID, "plg", constant.ChannelTypeOpenAI, modelNames...)
	ratios := make(map[string]float64, len(modelNames))
	for _, name := range modelNames {
		ratios[name] = 1
	}
	setModelAccessBilling(t, ratios, nil, nil)
}

func TestRunPLGModelCatalogWatchOnceWritesBaselineWithoutNotifying(t *testing.T) {
	ts := setupPLGCatalogWatchTest(t)
	seedPLGCatalog(t, 301, "plg-a", "plg-b")

	result, err := RunPLGModelCatalogWatchOnce(time.Unix(1_000, 0))
	require.NoError(t, err)
	require.Equal(t, PLGCatalogWatchBaselineCreated, result.Outcome)
	require.Zero(t, ts.count())

	snapshot, err := model.GetPLGModelCatalogSnapshot("plg")
	require.NoError(t, err)
	require.NotNil(t, snapshot)
	require.Equal(t, []string{"plg-a", "plg-b"}, snapshot.ModelNameList())
	require.Equal(t, int64(1_000), snapshot.UpdatedAt)
}

func TestRunPLGModelCatalogWatchOnceNotifiesOnChangeThenStaysQuiet(t *testing.T) {
	ts := setupPLGCatalogWatchTest(t)
	seedPLGCatalog(t, 311, "plg-keep", "plg-gone")
	_, err := RunPLGModelCatalogWatchOnce(time.Unix(1_000, 0))
	require.NoError(t, err)

	// Second cycle with the same catalog: nothing happens.
	result, err := RunPLGModelCatalogWatchOnce(time.Unix(1_060, 0))
	require.NoError(t, err)
	require.Equal(t, PLGCatalogWatchUnchanged, result.Outcome)
	require.Zero(t, ts.count())

	// Catalog changes: one model removed by disabling its channel, one added.
	require.NoError(t, model.DB.Model(&model.Channel{}).Where("id = ?", 311).Update("models", "plg-keep").Error)
	require.NoError(t, model.DB.Where("channel_id = ? AND model = ?", 311, "plg-gone").Delete(&model.Ability{}).Error)
	seedPLGCatalog(t, 312, "plg-keep", "plg-new")

	result, err = RunPLGModelCatalogWatchOnce(time.Unix(1_120, 0))
	require.NoError(t, err)
	require.Equal(t, PLGCatalogWatchNotified, result.Outcome)
	require.Equal(t, []string{"plg-new"}, result.Added)
	require.Equal(t, []string{"plg-gone"}, result.Removed)

	select {
	case body := <-ts.bodies:
		require.Contains(t, body, "PLG 可用模型变更")
		require.Contains(t, body, "plg-new")
		require.Contains(t, body, "plg-gone")
		require.Contains(t, body, "当前共 2 个模型")
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for DingTalk request")
	}

	// The snapshot advanced, so the next cycle does not resend.
	result, err = RunPLGModelCatalogWatchOnce(time.Unix(1_180, 0))
	require.NoError(t, err)
	require.Equal(t, PLGCatalogWatchUnchanged, result.Outcome)
	require.Equal(t, int32(1), ts.count())
}

func TestRunPLGModelCatalogWatchOnceSkipsSendWhenDisabledButKeepsSnapshotCurrent(t *testing.T) {
	ts := setupPLGCatalogWatchTest(t)
	seedPLGCatalog(t, 321, "plg-a")
	_, err := RunPLGModelCatalogWatchOnce(time.Unix(1_000, 0))
	require.NoError(t, err)

	operation_setting.GetPLGCatalogNotifySetting().DingTalkAlertEnabled = false
	seedPLGCatalog(t, 322, "plg-a", "plg-b")

	result, err := RunPLGModelCatalogWatchOnce(time.Unix(1_060, 0))
	require.NoError(t, err)
	require.Equal(t, PLGCatalogWatchChangedNotificationDisabled, result.Outcome)
	require.Zero(t, ts.count())

	snapshot, err := model.GetPLGModelCatalogSnapshot("plg")
	require.NoError(t, err)
	require.Equal(t, []string{"plg-a", "plg-b"}, snapshot.ModelNameList())
}

func TestRunPLGModelCatalogWatchOnceLosesRaceWhenSnapshotAlreadyAdvanced(t *testing.T) {
	ts := setupPLGCatalogWatchTest(t)
	seedPLGCatalog(t, 331, "plg-a")
	_, err := RunPLGModelCatalogWatchOnce(time.Unix(1_000, 0))
	require.NoError(t, err)

	// Simulate another node having already claimed the same change.
	seedPLGCatalog(t, 332, "plg-a", "plg-b")
	next, err := ResolvePLGCatalogModelNames()
	require.NoError(t, err)
	claimed, err := model.ClaimPLGModelCatalogSnapshotChange("plg", FingerprintModelNames([]string{"plg-a"}), FingerprintModelNames(next), next, 1_030)
	require.NoError(t, err)
	require.True(t, claimed)

	result, err := RunPLGModelCatalogWatchOnce(time.Unix(1_060, 0))
	require.NoError(t, err)
	require.Equal(t, PLGCatalogWatchUnchanged, result.Outcome)
	require.Zero(t, ts.count())
}

func TestBuildPLGModelCatalogChangeContentCapsLongLists(t *testing.T) {
	added := make([]string, 0, 25)
	for i := 0; i < 25; i++ {
		added = append(added, "added-"+string(rune('a'+i%26))+string(rune('a'+i/26)))
	}
	content := BuildPLGModelCatalogChangeContent(PLGCatalogChange{
		Added:      added,
		Removed:    []string{"gone-1"},
		TotalCount: 90,
		Now:        time.Date(2026, 9, 10, 4, 0, 0, 0, time.UTC),
	})
	require.Contains(t, content, "PLG 可用模型变更")
	require.Contains(t, content, "2026-09-10 12:00:00")
	require.Contains(t, content, "新增 25 个")
	require.Contains(t, content, "其余 5 个已省略")
	require.Contains(t, content, "移除 1 个：gone-1")
	require.Contains(t, content, "当前共 90 个模型")
	require.NotContains(t, content, "sk-")
}

func TestFingerprintModelNamesIsOrderInsensitiveAndStable(t *testing.T) {
	require.Equal(t, FingerprintModelNames([]string{"b", "a"}), FingerprintModelNames([]string{"a", "b"}))
	require.NotEqual(t, FingerprintModelNames([]string{"a"}), FingerprintModelNames([]string{"a", "b"}))
	require.Len(t, FingerprintModelNames(nil), 64)
}
