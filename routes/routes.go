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

	r.GET("/check_cookie",
		middleware.VerifyToken(),
		middleware.CheckCookieMiddleware())
	r.POST("/files/upload",
		middleware.VerifyToken(),
		middleware.UploadMiddleware(),
		services.UploadFile)
	r.GET("/files/download/:hash",
		middleware.VerifyToken(),
		middleware.DownloadMiddleware(),
		services.DownloadFile)
	r.POST("/files/delete", middleware.VerifyToken(),
		middleware.DeleteMiddleware(),
		services.DeleteFile)
	r.POST("/files/lock/:status", middleware.VerifyToken(),
		middleware.LockMiddleware(),
		services.LockFile)
	r.POST("/files/file_info/:hash", middleware.VerifyToken(),
		middleware.SetFileInfoMiddleware(),
		services.SetFileInfo)
	r.POST("/files/tags", middleware.VerifyToken(),
		middleware.SetFileTagsMiddleware(),
		services.SetFileTags)
	r.GET("/files/tags", middleware.VerifyToken(),
		middleware.GetAllFileTagsMiddleware(),
		services.GetAllFileTags)
	r.GET("/files/preview/:hash",
		middleware.VerifyToken(),
		middleware.PreviewMiddleware(),
		services.PreviewFile)
	r.GET("/files/note_info/:hash",
		middleware.VerifyToken(),
		middleware.GetNoteInfoMiddleware(),
		services.GetNoteInfo)
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
