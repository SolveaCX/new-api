package router

import (
	"github.com/QuantumNous/new-api/controller"
	"github.com/QuantumNous/new-api/middleware"
	"github.com/gin-gonic/gin"
)

func SetBytePlusAssetRouter(router *gin.Engine) {
	assetRouter := router.Group("/v1")
	assetRouter.Use(middleware.RouteTag("asset"))
	assetRouter.Use(middleware.GlobalAPIRateLimit())
	assetRouter.Use(middleware.TokenAuth())
	assetRouter.Use(middleware.ModelRequestRateLimit())
	{
		assetWriteRouter := assetRouter.Group("/")
		assetWriteRouter.Use(middleware.UploadRateLimit())
		{
			assetWriteRouter.POST("/assets", controller.CreateAsset)
			assetWriteRouter.POST("/assets/upload", controller.UploadAsset)
			assetWriteRouter.POST("/assets/uploads", controller.CreateAssetUploadSession)
			assetWriteRouter.POST("/assets/uploads/:upload_id/complete", controller.CompleteAssetUpload)
		}
		assetRouter.GET("/assets/:asset_id", controller.GetAsset)
	}
}
