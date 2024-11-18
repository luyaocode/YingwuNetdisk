package test

import (
	"time"

	"github.com/gin-gonic/gin"
)

func Test(c *gin.Context) {
	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Server is running",
	})
}

func TestDelay(c *gin.Context) {
	// 模拟延迟
	time.Sleep(100 * time.Millisecond)
	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "Server is running",
	})
}
