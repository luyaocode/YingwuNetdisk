package utils

import (
	"github.com/gin-gonic/gin"
)

// Respond 用于统一响应接口，返回状态码、响应键和值
func Respond(c *gin.Context, statusCode int, key string, data interface{}) {
	c.JSON(statusCode, gin.H{
		key: data,
	})
}
