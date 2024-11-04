package routes

import (
	"net/http"
	"yingwu/config"
	"yingwu/middleware"
	"yingwu/services"

	"github.com/gin-gonic/gin"
)

var sessionService *services.SessionService
var sessionValidationEnabled = false

func SetupRoutes(r *gin.Engine, env string) {
	sessionService = services.NewSessionService(config.RedisClient)

	r.Use(middleware.CORSMiddleware())

	r.POST("/files/upload", uploadMiddleware, services.UploadFile)
	r.GET("/files/download/:hash", downloadMiddleware, services.DownloadFile)
	r.GET("/allfiles", services.GetAllFiles)

	if env == "dev" {
		r.GET("/test", services.Test)
	}
}

// 上传中间件，验证会话
func uploadMiddleware(c *gin.Context) {
	if !sessionValidationEnabled {
		c.Set("userID", "test")
		c.Next()
		return
	}
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
	if !sessionValidationEnabled {
		c.Set("userID", "test")
		c.Next()
		return
	}
	sessionID := c.GetHeader("Authorization")
	userID, err := sessionService.ValidateSession(sessionID)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		c.Abort()
		return
	}
	c.Set("userID", userID) // 将userID存入上下文
}
