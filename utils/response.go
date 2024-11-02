package utils

import (
	"github.com/gin-gonic/gin"
)

func Respond(c *gin.Context, status int, key string, message string) {
    c.JSON(status, gin.H{key: message})
}