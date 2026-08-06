package service

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/model"
	"github.com/stretchr/testify/require"
)

func TestVideoResultStorageConfigDefaultsAndOverrides(t *testing.T) {
	cfg := CurrentVideoResultStorageConfig()
	require.Equal(t, "", cfg.Bucket)
	require.Equal(t, 15*time.Minute, cfg.SignedURLTTL)
	require.Equal(t, 24*time.Hour, cfg.Retention)
	require.Equal(t, 30*time.Minute, cfg.FetchTimeout)
	require.Equal(t, int64(500<<20), cfg.MaxBytes)
	require.Equal(t, "", cfg.ServiceAccountEmail)

	t.Setenv("VIDEO_RESULT_STORAGE_BUCKET", "video-results")
	t.Setenv("VIDEO_RESULT_SIGNED_URL_TTL_SECONDS", "120")
	t.Setenv("VIDEO_RESULT_RETENTION_SECONDS", "7200")
	t.Setenv("VIDEO_RESULT_FETCH_TIMEOUT_SECONDS", "60")
	t.Setenv("VIDEO_RESULT_MAX_BYTES", "1024")
	t.Setenv("VIDEO_RESULT_SERVICE_ACCOUNT_EMAIL", "video-signer@example.iam.gserviceaccount.com")

	cfg = CurrentVideoResultStorageConfig()
	require.Equal(t, "video-results", cfg.Bucket)
	require.Equal(t, 2*time.Minute, cfg.SignedURLTTL)
	require.Equal(t, 2*time.Hour, cfg.Retention)
	require.Equal(t, time.Minute, cfg.FetchTimeout)
	require.Equal(t, int64(1024), cfg.MaxBytes)
	require.Equal(t, "video-signer@example.iam.gserviceaccount.com", cfg.ServiceAccountEmail)
}

func TestVideoResultStorageConfigBounds(t *testing.T) {
	t.Setenv("VIDEO_RESULT_STORAGE_BUCKET", "video-results")
	t.Setenv("VIDEO_RESULT_SIGNED_URL_TTL_SECONDS", "0")
	t.Setenv("VIDEO_RESULT_RETENTION_SECONDS", "-1")
	t.Setenv("VIDEO_RESULT_FETCH_TIMEOUT_SECONDS", "0")
	t.Setenv("VIDEO_RESULT_MAX_BYTES", "0")

	cfg := CurrentVideoResultStorageConfig()
	require.Equal(t, 15*time.Minute, cfg.SignedURLTTL)
	require.Equal(t, 24*time.Hour, cfg.Retention)
	require.Equal(t, 30*time.Minute, cfg.FetchTimeout)
	require.Equal(t, int64(500<<20), cfg.MaxBytes)

	t.Setenv("VIDEO_RESULT_SIGNED_URL_TTL_SECONDS", "7200")
	t.Setenv("VIDEO_RESULT_FETCH_TIMEOUT_SECONDS", "3600")
	t.Setenv("VIDEO_RESULT_MAX_BYTES", "1073741824")

	cfg = CurrentVideoResultStorageConfig()
	require.Equal(t, time.Hour, cfg.SignedURLTTL)
	require.Equal(t, 30*time.Minute, cfg.FetchTimeout)
	require.Equal(t, int64(500<<20), cfg.MaxBytes)
}

func TestVideoResultObjectKey(t *testing.T) {
	now := time.Date(2026, 8, 6, 23, 59, 0, 0, time.FixedZone("CST", 8*3600))
	key, err := buildVideoResultObjectKey("task_Abc-123_ok", now)
	require.NoError(t, err)
	require.Equal(t, "video-results/20260806/task_Abc-123_ok.mp4", key)

	for _, taskID := range []string{"", "abc", "task_", "task_../x", "task_a/b", "../task_a", "task_ space"} {
		_, err := buildVideoResultObjectKey(taskID, now)
		require.ErrorIs(t, err, ErrVideoResultInvalidTaskID)
	}
}

func TestVideoResultBoundedReader(t *testing.T) {
	reader := newVideoResultBoundedReader(strings.NewReader("12345"), 5)
	body, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Equal(t, "12345", string(body))

	reader = newVideoResultBoundedReader(strings.NewReader("123456"), 5)
	_, err = io.ReadAll(reader)
	require.ErrorIs(t, err, ErrVideoResultTooLarge)
}

func TestArchiveVideoResult(t *testing.T) {
	t.Run("archives upstream video into private create-only object", func(t *testing.T) {
		start := time.Date(2026, 8, 6, 1, 2, 3, 0, time.UTC)
		store := newFakeVideoResultStore()
		restore := installVideoResultArchiveTestHooks(t, store, start)
		defer restore()
		t.Setenv("VIDEO_RESULT_STORAGE_BUCKET", "video-bucket")
		t.Setenv("VIDEO_RESULT_RETENTION_SECONDS", "3600")
		t.Setenv("VIDEO_RESULT_MAX_BYTES", "16")

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			w.Header().Set("Content-Type", "video/mp4; charset=binary")
			_, _ = w.Write([]byte("0123456789"))
		}))
		defer server.Close()

		result, err := ArchiveVideoResult(context.Background(), "task_archive-1", server.URL, "")
		require.NoError(t, err)
		require.Equal(t, &model.VideoResult{
			Bucket:      "video-bucket",
			Object:      "video-results/20260806/task_archive-1.mp4",
			Generation:  1,
			ContentType: "video/mp4",
			Size:        10,
			StoredAt:    start.Unix(),
			ExpiresAt:   start.Add(time.Hour).Unix(),
		}, result)

		created := store.created["video-bucket/video-results/20260806/task_archive-1.mp4"]
		require.Equal(t, "video/mp4", created.options.ContentType)
		require.Equal(t, "private, max-age=0, no-store", created.options.CacheControl)
		require.Equal(t, `attachment; filename="task_archive-1.mp4"`, created.options.ContentDisposition)
		require.True(t, store.validatedURLs[server.URL])
	})

	t.Run("allows exactly max bytes", func(t *testing.T) {
		start := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)
		store := newFakeVideoResultStore()
		restore := installVideoResultArchiveTestHooks(t, store, start)
		defer restore()
		t.Setenv("VIDEO_RESULT_STORAGE_BUCKET", "video-bucket")
		t.Setenv("VIDEO_RESULT_MAX_BYTES", "5")

		server := newVideoResultTestServer(t, http.StatusOK, "video/mp4", "12345")
		defer server.Close()

		result, err := ArchiveVideoResult(context.Background(), "task_exact", server.URL, "")
		require.NoError(t, err)
		require.Equal(t, int64(5), result.Size)
		require.False(t, store.closedWithError)
	})

	t.Run("aborts writer and does not finalize oversized object", func(t *testing.T) {
		start := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)
		store := newFakeVideoResultStore()
		restore := installVideoResultArchiveTestHooks(t, store, start)
		defer restore()
		t.Setenv("VIDEO_RESULT_STORAGE_BUCKET", "video-bucket")
		t.Setenv("VIDEO_RESULT_MAX_BYTES", "5")

		server := newVideoResultTestServer(t, http.StatusOK, "video/mp4", "123456")
		defer server.Close()

		_, err := ArchiveVideoResult(context.Background(), "task_too_large", server.URL, "")
		require.ErrorIs(t, err, ErrVideoResultTooLarge)
		require.True(t, store.closedWithError)
		require.Empty(t, store.created)
	})

	t.Run("rejects non video content type", func(t *testing.T) {
		start := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)
		store := newFakeVideoResultStore()
		restore := installVideoResultArchiveTestHooks(t, store, start)
		defer restore()
		t.Setenv("VIDEO_RESULT_STORAGE_BUCKET", "video-bucket")

		server := newVideoResultTestServer(t, http.StatusOK, "text/plain; charset=utf-8", "hello")
		defer server.Close()

		_, err := ArchiveVideoResult(context.Background(), "task_text", server.URL, "")
		require.ErrorIs(t, err, ErrVideoResultInvalidContent)
		require.Empty(t, store.created)
	})

	t.Run("rejects non 2xx upstream status", func(t *testing.T) {
		start := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)
		store := newFakeVideoResultStore()
		restore := installVideoResultArchiveTestHooks(t, store, start)
		defer restore()
		t.Setenv("VIDEO_RESULT_STORAGE_BUCKET", "video-bucket")

		server := newVideoResultTestServer(t, http.StatusBadGateway, "video/mp4", "bad")
		defer server.Close()

		_, err := ArchiveVideoResult(context.Background(), "task_bad_status", server.URL, "")
		require.ErrorIs(t, err, ErrVideoResultInvalidContent)
		require.Empty(t, store.created)
	})

	t.Run("rejects invalid task id before fetching", func(t *testing.T) {
		start := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)
		store := newFakeVideoResultStore()
		restore := installVideoResultArchiveTestHooks(t, store, start)
		defer restore()
		t.Setenv("VIDEO_RESULT_STORAGE_BUCKET", "video-bucket")

		_, err := ArchiveVideoResult(context.Background(), "task_bad/id", "https://example.com/video.mp4", "")
		require.ErrorIs(t, err, ErrVideoResultInvalidTaskID)
		require.Empty(t, store.validatedURLs)
	})

	t.Run("returns config error for empty bucket", func(t *testing.T) {
		start := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)
		store := newFakeVideoResultStore()
		restore := installVideoResultArchiveTestHooks(t, store, start)
		defer restore()

		_, err := ArchiveVideoResult(context.Background(), "task_no_bucket", "https://example.com/video.mp4", "")
		require.ErrorIs(t, err, ErrVideoResultConfig)
	})

	t.Run("reuses valid existing object after create conflict", func(t *testing.T) {
		start := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)
		created := start.Add(-time.Minute)
		store := newFakeVideoResultStore()
		restore := installVideoResultArchiveTestHooks(t, store, start)
		defer restore()
		t.Setenv("VIDEO_RESULT_STORAGE_BUCKET", "video-bucket")
		t.Setenv("VIDEO_RESULT_RETENTION_SECONDS", "7200")
		key := "video-bucket/video-results/20260806/task_conflict.mp4"
		store.createErr = ErrVideoResultAlreadyExists
		store.attrs[key] = VideoResultObjectAttrs{
			ContentType: "video/mp4",
			Size:        42,
			Generation:  7,
			Created:     created,
		}

		server := newVideoResultTestServer(t, http.StatusOK, "video/mp4", "123")
		defer server.Close()

		result, err := ArchiveVideoResult(context.Background(), "task_conflict", server.URL, "")
		require.NoError(t, err)
		require.Equal(t, int64(7), result.Generation)
		require.Equal(t, int64(42), result.Size)
		require.Equal(t, created.Unix(), result.StoredAt)
		require.Equal(t, created.Add(2*time.Hour).Unix(), result.ExpiresAt)
	})

	t.Run("rejects invalid existing object after create conflict", func(t *testing.T) {
		start := time.Date(2026, 8, 6, 0, 0, 0, 0, time.UTC)
		store := newFakeVideoResultStore()
		restore := installVideoResultArchiveTestHooks(t, store, start)
		defer restore()
		t.Setenv("VIDEO_RESULT_STORAGE_BUCKET", "video-bucket")
		key := "video-bucket/video-results/20260806/task_invalid_existing.mp4"
		store.createErr = ErrVideoResultAlreadyExists
		store.attrs[key] = VideoResultObjectAttrs{ContentType: "application/octet-stream", Size: 42, Generation: 7, Created: start}

		server := newVideoResultTestServer(t, http.StatusOK, "video/mp4", "123")
		defer server.Close()

		_, err := ArchiveVideoResult(context.Background(), "task_invalid_existing", server.URL, "")
		require.ErrorIs(t, err, ErrVideoResultUnavailable)
	})
}

func TestGCSVideoResult(t *testing.T) {
	t.Run("sign url forwards request", func(t *testing.T) {
		var capturedBucket, capturedKey string
		var captured VideoResultSignedURLRequest
		restore := installGCSVideoResultSignURLHook(t, func(_ context.Context, bucket, objectKey string, request VideoResultSignedURLRequest) (string, error) {
			capturedBucket = bucket
			capturedKey = objectKey
			captured = request
			return "https://signed.example/video", nil
		})
		defer restore()

		store := gcsVideoResultObjectStore{}
		values := url.Values{"response-content-disposition": []string{"attachment"}}
		signed, err := store.SignURL(context.Background(), "bucket", "video-results/20260806/task_a.mp4", VideoResultSignedURLRequest{
			Method:              http.MethodGet,
			TTL:                 time.Minute,
			ServiceAccountEmail: "svc@example.iam.gserviceaccount.com",
			QueryParameters:     values,
		})
		require.NoError(t, err)
		require.Equal(t, "https://signed.example/video", signed)
		require.Equal(t, "bucket", capturedBucket)
		require.Equal(t, "video-results/20260806/task_a.mp4", capturedKey)
		require.Equal(t, http.MethodGet, captured.Method)
		require.Equal(t, time.Minute, captured.TTL)
		require.Equal(t, "svc@example.iam.gserviceaccount.com", captured.ServiceAccountEmail)
		require.Equal(t, values, captured.QueryParameters)
	})
}

type fakeVideoResultStore struct {
	created         map[string]fakeVideoResultObject
	attrs           map[string]VideoResultObjectAttrs
	createErr       error
	attrsErr        error
	closedWithError bool
	validatedURLs   map[string]bool
}

type fakeVideoResultObject struct {
	body    []byte
	options VideoResultCreateOptions
}

func newFakeVideoResultStore() *fakeVideoResultStore {
	return &fakeVideoResultStore{
		created:       map[string]fakeVideoResultObject{},
		attrs:         map[string]VideoResultObjectAttrs{},
		validatedURLs: map[string]bool{},
	}
}

func (f *fakeVideoResultStore) Create(_ context.Context, bucket, objectKey string, body io.Reader, options VideoResultCreateOptions) (VideoResultObjectAttrs, error) {
	data, err := io.ReadAll(body)
	if err != nil {
		f.closedWithError = true
		return VideoResultObjectAttrs{}, err
	}
	if f.createErr != nil {
		return VideoResultObjectAttrs{}, f.createErr
	}
	key := bucket + "/" + objectKey
	f.created[key] = fakeVideoResultObject{body: append([]byte(nil), data...), options: options}
	created := videoResultNow()
	attrs := VideoResultObjectAttrs{
		ContentType: options.ContentType,
		Size:        int64(len(data)),
		Generation:  1,
		Created:     created,
	}
	f.attrs[key] = attrs
	return attrs, nil
}

func (f *fakeVideoResultStore) Attrs(_ context.Context, bucket, objectKey string) (VideoResultObjectAttrs, error) {
	if f.attrsErr != nil {
		return VideoResultObjectAttrs{}, f.attrsErr
	}
	attrs, ok := f.attrs[bucket+"/"+objectKey]
	if !ok {
		return VideoResultObjectAttrs{}, ErrVideoResultUnavailable
	}
	return attrs, nil
}

func (f *fakeVideoResultStore) SignURL(_ context.Context, _ string, _ string, _ VideoResultSignedURLRequest) (string, error) {
	return "https://signed.example/video", nil
}

func installVideoResultArchiveTestHooks(t *testing.T, store *fakeVideoResultStore, now time.Time) func() {
	t.Helper()
	originalStore := videoResultObjectStore
	originalNow := videoResultNow
	originalValidate := videoResultValidateURL
	originalClient := httpClient
	videoResultObjectStore = store
	videoResultNow = func() time.Time { return now }
	videoResultValidateURL = func(rawURL string) error {
		store.validatedURLs[rawURL] = true
		return nil
	}
	httpClient = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).Client()
	httpClient.Transport = http.DefaultTransport
	t.Cleanup(func() {
		videoResultObjectStore = originalStore
		videoResultNow = originalNow
		videoResultValidateURL = originalValidate
		httpClient = originalClient
	})
	return func() {}
}

func installGCSVideoResultSignURLHook(t *testing.T, hook func(context.Context, string, string, VideoResultSignedURLRequest) (string, error)) func() {
	t.Helper()
	original := gcsVideoResultSignURL
	gcsVideoResultSignURL = hook
	t.Cleanup(func() { gcsVideoResultSignURL = original })
	return func() {}
}

func newVideoResultTestServer(t *testing.T, status int, contentType, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", contentType)
		w.WriteHeader(status)
		_, err := io.Copy(w, bytes.NewBufferString(body))
		require.NoError(t, err)
	}))
}

var _ VideoResultObjectStore = (*fakeVideoResultStore)(nil)
var _ = errors.Is
