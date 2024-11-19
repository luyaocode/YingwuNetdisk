package middleware

import (
	"log"
	"net/http"
	"yingwu/utils"

	"github.com/gin-gonic/gin"
)

// 上传中间件，验证会话
func UploadMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("userID")
		strUserID, ok := userID.(string)
		if ok {
			log.Printf("用户[" + strUserID + "]开始上传")
		}
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
		userID, _ := c.Get("userID")
		strUserID, ok := userID.(string)
		if ok {
			log.Printf("用户[" + strUserID + "]开始下载")
		}
	}
}

func PreviewMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("userID")
		strUserID, ok := userID.(string)
		if ok {
			log.Printf("用户[" + strUserID + "]开始预览")
		}
	}
}

func GetAllFilesMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("userID")
		strUserID, ok := userID.(string)
		if ok {
			log.Printf("用户[" + strUserID + "]开始查询所有文件")
		}
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

func GetDownFilesMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("userID")
		strUserID, ok := userID.(string)
		if !ok || strUserID == "" || userID == "guest" || userID == "test" {
			utils.Respond(c, http.StatusInternalServerError, "error", "Failed to retrieve downloaded files.")
			return
		}
		log.Printf("用户[" + strUserID + "]开始查询下载文件记录")
	}
}

func GetUpFilesMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("userID")
		strUserID, ok := userID.(string)
		if !ok || strUserID == "" || userID == "guest" || userID == "test" {
			utils.Respond(c, http.StatusInternalServerError, "error", "Failed to retrieve uploaded files.")
			return
		}
		log.Printf("用户[" + strUserID + "]开始查询上传文件记录")
	}
}

// 获取文件下载量排名
func GetDownFilesRankMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID, _ := c.Get("userID")
		strUserID, ok := userID.(string)
		if ok {
			log.Printf("用户[" + strUserID + "]开始查询文件下载量排名")
		}
	}
}
