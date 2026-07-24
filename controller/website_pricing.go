package controller

import (
	"net/http"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetWebsitePricingV2(c *gin.Context) {
	group := strings.TrimSpace(c.Query("group"))
	if group != websitePublicGroup {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "unsupported website pricing group"})
		return
	}

	payload, err := service.BuildWebsitePricingV2(
		model.GetPricing(),
		model.GetVendors(),
		model.GetSupportedEndpointMap(),
		service.GetUserAutoGroup(""),
		group,
		time.Now(),
	)
	if err != nil {
		common.ApiError(c, err)
		return
	}

	c.Header("Cache-Control", "public, max-age=30, stale-while-revalidate=60, stale-if-error=300")
	c.JSON(http.StatusOK, payload)
}
