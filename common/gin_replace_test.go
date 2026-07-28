package common

import (
	"bytes"
	"errors"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestReplaceRequestBodyUpdatesEveryReaderAndClosesOldStorage(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewBufferString(`{"model":"auto"}`))

	oldStorage, err := CreateBodyStorage([]byte(`{"model":"auto"}`))
	require.NoError(t, err)
	c.Set(KeyBodyStorage, oldStorage)
	c.Set(KeyRequestBody, []byte(`{"model":"stale"}`))

	replacement := []byte(`{"model":"gpt-real","zero":0,"flag":false}`)
	require.NoError(t, ReplaceRequestBody(c, replacement))
	require.Equal(t, int64(len(replacement)), c.Request.ContentLength)

	requestBytes, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	require.Equal(t, replacement, requestBytes)

	storage, err := GetBodyStorage(c)
	require.NoError(t, err)
	storageBytes, err := storage.Bytes()
	require.NoError(t, err)
	require.Equal(t, replacement, storageBytes)

	_, err = oldStorage.Bytes()
	require.ErrorIs(t, err, ErrStorageClosed)
	cached, exists := c.Get(KeyRequestBody)
	require.True(t, exists)
	require.Nil(t, cached)

	CleanupBodyStorage(c)
}

func TestReplaceRequestBodyFailureKeepsOldState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	original := []byte(`{"model":"auto"}`)
	c.Request = httptest.NewRequest("POST", "/v1/chat/completions", bytes.NewReader(original))
	oldStorage, err := CreateBodyStorage(original)
	require.NoError(t, err)
	c.Set(KeyBodyStorage, oldStorage)
	c.Set(KeyRequestBody, original)

	sentinel := errors.New("replacement storage failed")
	previousFactory := createReplacementBodyStorage
	createReplacementBodyStorage = func([]byte) (BodyStorage, error) { return nil, sentinel }
	t.Cleanup(func() { createReplacementBodyStorage = previousFactory })

	require.ErrorIs(t, ReplaceRequestBody(c, []byte(`{"model":"real"}`)), sentinel)
	stored, err := oldStorage.Bytes()
	require.NoError(t, err)
	require.Equal(t, original, stored)
	require.Equal(t, int64(len(original)), c.Request.ContentLength)
	requestBytes, err := io.ReadAll(c.Request.Body)
	require.NoError(t, err)
	require.Equal(t, original, requestBytes)

	CleanupBodyStorage(c)
}

func TestReplaceRequestBodyClosesOldDiskStorage(t *testing.T) {
	previousConfig := GetDiskCacheConfig()
	SetDiskCacheConfig(DiskCacheConfig{
		Enabled:     true,
		ThresholdMB: 0,
		MaxSizeMB:   1,
		Path:        t.TempDir(),
	})
	t.Cleanup(func() { SetDiskCacheConfig(previousConfig) })

	oldStorage, err := CreateBodyStorage([]byte(`{"model":"auto"}`))
	require.NoError(t, err)
	require.True(t, oldStorage.IsDisk())

	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("POST", "/v1/responses", bytes.NewBufferString(`{"model":"auto"}`))
	c.Set(KeyBodyStorage, oldStorage)

	require.NoError(t, ReplaceRequestBody(c, []byte(`{"model":"gpt-real"}`)))
	_, err = oldStorage.Bytes()
	require.ErrorIs(t, err, ErrStorageClosed)

	CleanupBodyStorage(c)
}
