package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func CORSMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        origin := c.Request.Header.Get("Origin")
        if origin != "" {
            // 动态设置 Access-Control-Allow-Origin
            c.Header("Access-Control-Allow-Origin", origin)
            c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
            c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization")
            // 跨域允许前端访问Content-Disposition（存放下载文件名）
            c.Header("Access-Control-Expose-Headers", "Content-Length,Content-Disposition")

            c.Header("Access-Control-Allow-Credentials", "true")
        }

        // 处理预检请求
        if c.Request.Method == http.MethodOptions {
            c.AbortWithStatus(http.StatusNoContent) // 返回 204 No Content
            return
        }

        c.Next() // 继续处理请求
    }
}
