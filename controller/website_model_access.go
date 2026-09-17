package controller

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

// GetWebsiteModelAccess uses the console catalog resolver with a public PLG
// identity. Never accept a caller-supplied group or expose a signed-in user's
// private scopes through this anonymous endpoint.
func GetWebsiteModelAccess(c *gin.Context) {
	access, err := service.ResolveUserModelAccess(&model.UserBase{Group: websitePublicGroup})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	service.FilterHiddenModelsFromUserAccess(access)
	c.Header("Cache-Control", "no-store")
	common.ApiSuccess(c, access)
}
