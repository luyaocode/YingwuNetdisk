// verifyToken.go

package middleware

import (
	"context"
	"fmt"
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
			// 如果没有找到 token 或发生错误，返回 401 错误
			c.JSON(401, gin.H{
				"error": "Token not found in cookies",
			})
			c.Abort()
			return
		}

		// 创建一个 context，可以设置超时
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		// 使用 grpc.WithTransportCredentials(insecure.NewCredentials()) 代替 grpc.WithInsecure()
		conn, err := grpc.DialContext(ctx, "api.chaosgomoku.fun:50051", grpc.WithTransportCredentials(insecure.NewCredentials())) // 使用不安全的连接
		if err != nil {
			c.JSON(500, gin.H{"error": fmt.Sprintf("failed to connect to gRPC server: %v", err)})
			c.Abort()
			return
		}
		defer conn.Close()

		// 创建客户端
		client := gen.NewAuthServiceClient(conn)

		// 调用 VerifyToken 方法
		resp, err := client.VerifyToken(context.Background(), &gen.VerifyTokenRequest{
			Token: token, // 传入 JWT Token
		})
		if err != nil || !resp.GetValid() {
			// 如果 token 无效，返回 401 错误
			c.JSON(401, gin.H{
				"error": "Invalid token",
			})
			c.Abort()
			return
		}

		// 如果 token 验证通过，将 userID 设置到上下文中
		c.Set("userID", resp.GetUserId())
		c.Next() // 继续处理请求
	}
}
