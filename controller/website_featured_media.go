package controller

import (
	"errors"
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	backendI18n "github.com/QuantumNous/new-api/i18n"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

const websiteFeaturedMediaMultipartEnvelopeMaxBytes = int64(1 << 20)

var (
	uploadWebsiteFeaturedMedia = service.UploadWebsiteFeaturedMedia
	openWebsiteFeaturedMedia   = service.OpenWebsiteFeaturedMedia
)

func UploadWebsiteFeaturedMedia(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(
		c.Writer,
		c.Request.Body,
		service.WebsiteFeaturedMediaMaxBytes+websiteFeaturedMediaMultipartEnvelopeMaxBytes,
	)
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		if common.IsRequestBodyTooLargeError(err) {
			writeWebsiteFeaturedMediaError(c, http.StatusRequestEntityTooLarge, backendI18n.MsgTempMediaImageTooLarge)
			return
		}
		writeWebsiteFeaturedMediaError(c, http.StatusBadRequest, backendI18n.MsgTempMediaFileRequired)
		return
	}
	defer file.Close()

	result, err := uploadWebsiteFeaturedMedia(c.Request.Context(), file)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrWebsiteFeaturedMediaFileRequired):
			writeWebsiteFeaturedMediaError(c, http.StatusBadRequest, backendI18n.MsgTempMediaFileRequired)
		case errors.Is(err, service.ErrWebsiteFeaturedMediaTooLarge):
			writeWebsiteFeaturedMediaError(c, http.StatusRequestEntityTooLarge, backendI18n.MsgTempMediaImageTooLarge)
		case errors.Is(err, service.ErrWebsiteFeaturedMediaUnsupported):
			writeWebsiteFeaturedMediaError(c, http.StatusBadRequest, backendI18n.MsgTempMediaUnsupportedImage)
		default:
			common.SysLog("failed to upload website featured media: " + err.Error())
			writeWebsiteFeaturedMediaError(c, http.StatusInternalServerError, backendI18n.MsgTempMediaUploadFailed)
		}
		return
	}
	common.ApiSuccess(c, result)
}

func GetWebsiteFeaturedMedia(c *gin.Context) {
	media, err := openWebsiteFeaturedMedia(c.Request.Context(), c.Param("media_id"))
	if err != nil {
		if errors.Is(err, service.ErrWebsiteFeaturedMediaInvalidID) || errors.Is(err, service.ErrWebsiteFeaturedMediaNotFound) {
			c.Status(http.StatusNotFound)
			return
		}
		common.SysLog("failed to read website featured media: " + err.Error())
		c.Status(http.StatusInternalServerError)
		return
	}
	defer media.Body.Close()

	headers := map[string]string{
		"Cache-Control":          "public, max-age=31536000, immutable",
		"ETag":                   media.ETag,
		"X-Content-Type-Options": "nosniff",
	}
	if strings.TrimSpace(c.GetHeader("If-None-Match")) == media.ETag {
		for key, value := range headers {
			c.Header(key, value)
		}
		c.Status(http.StatusNotModified)
		return
	}
	c.DataFromReader(http.StatusOK, media.Size, media.ContentType, media.Body, headers)
}

func writeWebsiteFeaturedMediaError(c *gin.Context, status int, key string) {
	c.JSON(status, gin.H{
		"success": false,
		"message": backendI18n.T(c, key),
	})
}
