// verifyToken.go

package middleware

import (
	"context"
	"log"
	"time"
	"yingwu/gen"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var sessionValidationEnabled = true

// VerifyTokenMiddleware 用于验证 token
func VerifyToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !sessionValidationEnabled {
			c.Set("userID", "test")
			c.Next()
			return
		}
		// 从请求的 cookie 中获取 token
		token, err := c.Cookie("auth_token")
		if err != nil {
			c.Set("userID", "guest")
			c.Next()
			return
		}

		// 创建一个 context，可以设置超时
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 使用 grpc.WithTransportCredentials(insecure.NewCredentials()) 代替 grpc.WithInsecure()
		conn, err := grpc.DialContext(ctx, "api.chaosgomoku.fun:50051", grpc.WithTransportCredentials(insecure.NewCredentials())) // 使用不安全的连接
		if err != nil {
			c.Set("userID", "guest")
			c.Next()
			return
		}
		defer conn.Close()

		// 创建客户端
		client := gen.NewAuthServiceClient(conn)

		// 调用 VerifyToken 方法
		resp, err := client.VerifyToken(context.Background(), &gen.VerifyTokenRequest{
			Token: token,
		})
		if err != nil || !resp.GetValid() {
			c.Set("userID", "guest")
			c.Next()
			return
		}

		// 如果 token 验证通过，将 userID 设置到上下文中
		c.Set("userID", resp.GetUserId())
		log.Printf("已授权：token=" + token)
		log.Printf("欢迎" + resp.GetUserId())
		c.Next()
	}
}

// 上传中间件，验证会话
func UploadMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

	}
	// sessionID := c.GetHeader("Authorization")
	// userID, err := sessionService.ValidateSession(sessionID)
	// if err != nil {
	// 	c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
	// 	c.Abort()
	// 	return
	// }
	// c.Set("userID", userID) // 将userID存入上下文
}

// 下载中间件，验证会话
func DownloadMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

	}
	// sessionID := c.GetHeader("Authorization")
	// userID, err := sessionService.ValidateSession(sessionID)
	// if err != nil {
	// 	c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
	// 	c.Abort()
	// 	return
	// }
	// c.Set("userID", userID) // 将userID存入上下文
}

func GetAllFilesMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

	}
	// sessionID := c.GetHeader("Authorization")
	// userID, err := sessionService.ValidateSession(sessionID)
	// if err != nil {
	// 	c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
	// 	c.Abort()
	// 	return
	// }
	// c.Set("userID", userID) // 将userID存入上下文
}
