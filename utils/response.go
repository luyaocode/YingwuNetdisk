package utils

import (
	"github.com/gin-gonic/gin"
)

func Respond(c *gin.Context, status int, key string, message string) {
    c.JSON(status, gin.H{key: message})
}

func RespondWithFailures(c *gin.Context, status int, failureCount int, errorDetails []map[string]string) {
    response := gin.H{
        "failure_count": failureCount,
        "errors":        errorDetails,
    }
    c.JSON(status, response)
}


