package routes

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"yingwu/config"
	"yingwu/gen"
	"yingwu/middleware"
	"yingwu/services"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var sessionService *services.SessionService
var sessionValidationEnabled = false

func verifyToken(token string) (*gen.VerifyTokenResponse, error) {
	// 创建一个 context，可以设置超时
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 使用 grpc.WithTransportCredentials(insecure.NewCredentials()) 代替 grpc.WithInsecure()
	conn, err := grpc.DialContext(ctx, "localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials())) // 使用不安全的连接
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}
	defer conn.Close()

	// 创建客户端
	client := gen.NewAuthServiceClient(conn)

	// 调用 VerifyToken 方法
	resp, err := client.VerifyToken(context.Background(), &gen.VerifyTokenRequest{
		Token: token, // 传入 JWT Token
	})
	if err != nil {
		return nil, fmt.Errorf("could not verify token: %v", err)
	}

	return resp, nil
}

func SetupRoutes(r *gin.Engine, env string) {
	sessionService = services.NewSessionService(config.RedisClient)

	r.Use(middleware.CORSMiddleware())

	r.POST("/files/upload", uploadMiddleware, services.UploadFile)
	r.GET("/files/download/:hash", downloadMiddleware, services.DownloadFile)
	r.GET("/allfiles", getAllFilesMiddleware, services.GetAllFiles)

	if env == "dev" {
		r.GET("/test", services.Test)
		r.GET("/test_delay", services.TestDelay)
	}
}

// 上传中间件，验证会话
func uploadMiddleware(c *gin.Context) {
	if !sessionValidationEnabled {
		c.Set("userID", "test")
		c.Next()
		return
	}

	// 从请求的 cookie 中获取 token
	token, err := c.Cookie("auth_token")
	if err != nil {
		// 如果没有找到 token 或发生错误，返回 401 错误
		c.JSON(401, gin.H{
			"error": "Token not found in cookies",
		})
		c.Abort()
		return
	}

	// 调用 VerifyToken 验证 token
	resp, err := verifyToken(token)
	if err != nil {
		// 如果 token 无效，返回 401 错误
		c.JSON(401, gin.H{
			"error": "Invalid token",
		})
		c.Abort()
		return
	}

	// 如果 token 验证通过，将 userID 设置到上下文中
	if !resp.GetValid() {
		// 如果 token 无效，返回 401 错误
		c.JSON(401, gin.H{
			"error": "Invalid token",
		})
		c.Abort()
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

func getAllFilesMiddleware(c *gin.Context) {
	// if !sessionValidationEnabled {
	// 	c.Set("userID", "test")
	// 	c.Next()
	// 	return
	// }

	// 从请求的 cookie 中获取 token
	token, err := c.Cookie("auth_token")
	if err != nil {
		// 如果没有找到 token 或发生错误，返回 401 错误
		c.JSON(401, gin.H{
			"error": "Token not found in cookies",
		})
		c.Abort()
		return
	}

	// 调用 VerifyToken 验证 token
	resp, err := verifyToken(token)
	if err != nil {
		// 如果 token 无效，返回 401 错误
		c.JSON(401, gin.H{
			"error": "Invalid token",
		})
		c.Abort()
		return
	}

	// 如果 token 验证通过，将 userID 设置到上下文中
	if !resp.GetValid() {
		// 如果 token 无效，返回 401 错误
		c.JSON(401, gin.H{
			"error": "Invalid token",
		})
		c.Abort()
		return
	}

}
