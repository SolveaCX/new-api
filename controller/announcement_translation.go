package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type announcementTranslationRequest struct {
	Content string `json:"content" binding:"required"`
	Extra   string `json:"extra"`
}

func TranslateAnnouncement(c *gin.Context) {
	var request announcementTranslationRequest
	if err := common.DecodeJson(c.Request.Body, &request); err != nil || strings.TrimSpace(request.Content) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "公告内容不能为空"})
		return
	}
	translations, err := service.TranslateAnnouncement(c.Request.Context(), request.Content, request.Extra)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, translations)
}
