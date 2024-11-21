// verifyToken.go

package middleware

import (
	"context"
	"log"
	"time"
	"yingwu/config"
	"yingwu/gen"

	"github.com/gin-gonic/gin"
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

		// 调用 VerifyToken 方法
		resp, err := config.GrpcClient.VerifyToken(ctx, &gen.VerifyTokenRequest{
			Token: token,
		})

		if err != nil || !resp.GetValid() {
			c.Set("userID", "guest")
			c.Next()
			return
		}

		log.Printf("已授权：token=" + token)
		// 如果 token 验证通过，将 userID 设置到上下文中
		c.Set("userID", resp.GetUserId())
		c.Next()
	}
}
