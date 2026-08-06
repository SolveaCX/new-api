package service

import (
	"testing"
	"time"

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
