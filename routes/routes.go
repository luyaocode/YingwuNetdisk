package routes

import (
	"yingwu/config"
	"yingwu/middleware"
	"yingwu/services"
	"yingwu/test"

	"github.com/gin-gonic/gin"
)

var sessionService *services.SessionService

func SetupRoutes(r *gin.Engine, env string) {
	sessionService = services.NewSessionService(config.RedisClient)

	// 全局中间件
	r.Use(middleware.CORSMiddleware())

	r.POST("/files/upload",
		middleware.VerifyToken(),
		middleware.UploadMiddleware(),
		services.UploadFile)
	r.GET("/files/download/:hash",
		middleware.VerifyToken(),
		middleware.DownloadMiddleware(),
		services.DownloadFile)
	r.GET("/files/preview/:hash",
		middleware.VerifyToken(),
		middleware.PreviewMiddleware(),
		services.PreviewFile)
	r.GET("/files", middleware.VerifyToken(),
		middleware.GetAllFilesMiddleware(),
		services.GetAllFiles)
	r.GET("/files/downloads", middleware.VerifyToken(),
		middleware.GetDownFilesMiddleware(),
		services.GetDownloads)
	r.GET("/files/uploads", middleware.VerifyToken(),
		middleware.GetUpFilesMiddleware(),
		services.GetUploads)
	r.GET("/files/downloadRank", middleware.VerifyToken(),
		middleware.GetDownFilesRankMiddleware(),
		services.GetDownFileRank)

	if env == "dev" {
		r.GET("/test", test.Test)
		r.GET("/test_delay", test.TestDelay)
	}
}
