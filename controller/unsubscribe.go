package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/logger"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

// HandleEmailUnsubscribe 处理退订链接点击,无需登录。
// GET /api/email/unsubscribe?uid=123&token=xxx
func HandleEmailUnsubscribe(c *gin.Context) {
	uid, err := strconv.Atoi(c.Query("uid"))
	token := c.Query("token")
	if err != nil || uid <= 0 || token == "" {
		c.String(http.StatusBadRequest, "Invalid unsubscribe link.")
		return
	}
	if _, ok := service.VerifyUnsubscribeToken(uid, token); !ok {
		c.String(http.StatusBadRequest, "Invalid or expired unsubscribe link.")
		return
	}
	if err := model.SetUserEmailOptOut(uid); err != nil {
		logger.LogError(c.Request.Context(), "unsubscribe failed user="+strconv.Itoa(uid)+": "+err.Error())
		c.String(http.StatusInternalServerError, "Failed to process unsubscribe, please try again later.")
		return
	}
	c.String(http.StatusOK, "You have been unsubscribed from promotional emails.")
}
