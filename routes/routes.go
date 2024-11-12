package routes

import (
	"net/http"

	"yingwu/config"
	"yingwu/middleware"
	"yingwu/services"

	"github.com/gin-gonic/gin"
)

var sessionService *services.SessionService

func SetupRoutes(r *gin.Engine, env string) {
	sessionService = services.NewSessionService(config.RedisClient)

	r.POST("/files/upload", middleware.VerifyToken(), uploadMiddleware, services.UploadFile)
	r.GET("/files/download/:hash", middleware.VerifyToken(), downloadMiddleware, services.DownloadFile)
	r.GET("/allfiles", middleware.VerifyToken(), getAllFilesMiddleware, services.GetAllFiles)

	if env == "dev" {
		r.GET("/test", services.Test)
		r.GET("/test_delay", services.TestDelay)
	}
}

// 上传中间件，验证会话
func uploadMiddleware(c *gin.Context) {
	sessionID := c.GetHeader("Authorization")
	userID, err := sessionService.ValidateSession(sessionID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		c.Abort()
		return
	}
	c.Set("userID", userID) // 将userID存入上下文
}

// 下载中间件，验证会话
func downloadMiddleware(c *gin.Context) {
	sessionID := c.GetHeader("Authorization")
	userID, err := sessionService.ValidateSession(sessionID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		c.Abort()
		return
	}
	c.Set("userID", userID) // 将userID存入上下文
}

func getAllFilesMiddleware(c *gin.Context) {
	sessionID := c.GetHeader("Authorization")
	userID, err := sessionService.ValidateSession(sessionID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		c.Abort()
		return
	}
	c.Set("userID", userID) // 将userID存入上下文
}
