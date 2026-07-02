package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/service"
	"github.com/gin-gonic/gin"
)

func GetRankings(c *gin.Context) {
	result, err := service.GetRankingsSnapshot(c.DefaultQuery("period", "week"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

func GetWebsiteRankings(c *gin.Context) {
	result, err := service.GetWebsiteRankingsSnapshot(c.DefaultQuery("period", "week"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	c.Header("Cache-Control", "public, max-age=300, stale-while-revalidate=60")
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}
