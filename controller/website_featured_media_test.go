package controller

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	backendI18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestUploadWebsiteFeaturedMediaControllerReturnsPublicPath(t *testing.T) {
	require.NoError(t, backendI18n.Init())
	gin.SetMode(gin.TestMode)
	originalUpload := uploadWebsiteFeaturedMedia
	t.Cleanup(func() { uploadWebsiteFeaturedMedia = originalUpload })
	uploadWebsiteFeaturedMedia = func(_ context.Context, body io.Reader) (*service.WebsiteFeaturedMediaUploadResult, error) {
		payload, err := io.ReadAll(body)
		require.NoError(t, err)
		require.Equal(t, "payload", string(payload))
		return &service.WebsiteFeaturedMediaUploadResult{
			URL:         "/media/website-featured/hash.png",
			SHA256:      "hash",
			ContentType: "image/png",
			Size:        7,
		}, nil
	}
	ctx, recorder := newTempMediaMultipartContext(t, "file", "banner.png", "image/png", []byte("payload"))

	UploadWebsiteFeaturedMedia(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	var response struct {
		Success bool                                     `json:"success"`
		Data    service.WebsiteFeaturedMediaUploadResult `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.True(t, response.Success)
	require.Equal(t, "/media/website-featured/hash.png", response.Data.URL)
}

func TestUploadWebsiteFeaturedMediaControllerReturnsBadRequestForUnsupportedImage(t *testing.T) {
	require.NoError(t, backendI18n.Init())
	gin.SetMode(gin.TestMode)
	originalUpload := uploadWebsiteFeaturedMedia
	t.Cleanup(func() { uploadWebsiteFeaturedMedia = originalUpload })
	uploadWebsiteFeaturedMedia = func(context.Context, io.Reader) (*service.WebsiteFeaturedMediaUploadResult, error) {
		return nil, service.ErrWebsiteFeaturedMediaUnsupported
	}
	ctx, recorder := newTempMediaMultipartContext(t, "file", "banner.svg", "image/svg+xml", []byte("payload"))
	ctx.Request.Header.Set("Accept-Language", "zh-CN")

	UploadWebsiteFeaturedMedia(ctx)

	require.Equal(t, http.StatusBadRequest, recorder.Code)
	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &response))
	require.False(t, response.Success)
	require.Equal(t, "不支持的图片类型", response.Message)
}

func TestGetWebsiteFeaturedMediaStreamsWithImmutableHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	originalOpen := openWebsiteFeaturedMedia
	t.Cleanup(func() { openWebsiteFeaturedMedia = originalOpen })
	openWebsiteFeaturedMedia = func(context.Context, string) (*service.WebsiteFeaturedMedia, error) {
		return &service.WebsiteFeaturedMedia{
			Body:        io.NopCloser(strings.NewReader("image-bytes")),
			ContentType: "image/png",
			Size:        11,
			ETag:        `"hash"`,
		}, nil
	}
	recorder := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(recorder)
	ctx.Params = gin.Params{{Key: "media_id", Value: strings.Repeat("a", 64) + ".png"}}
	ctx.Request = httptest.NewRequest(http.MethodGet, "/media/website-featured/test.png", nil)

	GetWebsiteFeaturedMedia(ctx)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Equal(t, "image-bytes", recorder.Body.String())
	require.Equal(t, "image/png", recorder.Header().Get("Content-Type"))
	require.Equal(t, `"hash"`, recorder.Header().Get("ETag"))
	require.Equal(t, "public, max-age=31536000, immutable", recorder.Header().Get("Cache-Control"))
	require.Equal(t, "nosniff", recorder.Header().Get("X-Content-Type-Options"))
}
