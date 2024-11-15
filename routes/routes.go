package routes

import (
	"yingwu/config"
	"yingwu/middleware"
	"yingwu/services"

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
	r.GET("/files", middleware.VerifyToken(),
		middleware.GetAllFilesMiddleware(),
		services.GetAllFiles)

	if env == "dev" {
		r.GET("/test", services.Test)
		r.GET("/test_delay", services.TestDelay)
	}
}
