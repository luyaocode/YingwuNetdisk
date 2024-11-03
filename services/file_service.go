package services

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"

	"yingwu/config"
	"yingwu/models"
	"yingwu/utils"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func handleUploadFile(c *gin.Context, file *multipart.FileHeader) error {
	// 打开文件
	fileContent, err := file.Open()
	if err != nil {
		return err
	}
	defer fileContent.Close()

	// 计算文件的哈希
	hashType := "md5"
	hash, err := utils.GenerateFileHash(hashType, fileContent)
	if err != nil {
		return err
	}
	// 重置读指针复用fileContent
	if _, err := fileContent.Seek(0, io.SeekStart); err != nil {
		return err
	}

	// 查询 MySQL 中是否存在该哈希记录
	var existingFile models.File
	if err := config.MySQLDB.Where("hash = ?", hash).First(&existingFile).Error; err == nil {
		log.Printf("File already exists: %v", existingFile)
		return err
	}

	nowtime := time.Now()
	fileName := file.Filename
	// 使用 GridFS 存储文件内容
	bucket, err := gridfs.NewBucket(
		config.MongoClient.Database("yingwu-netdisk"),
		options.GridFSBucket().SetName("files"),
	)
	if err != nil {
		log.Printf("Failed to create GridFS bucket %v", err)
		return err
	}

	// 上传文件内容到 GridFS
	fileID, err := bucket.UploadFromStream(fileName, fileContent)
	strFileID := fileID.Hex()
	if err != nil {
		return err
	}

	if !config.MySQLDB.HasTable(&models.File{}) {
		// 表不存在，创建表
		if err := config.MySQLDB.AutoMigrate(&models.File{}); err != nil {
			log.Fatalf("failed to migrate database: %v", err)
		}
	}

	// 在MySQL中保存文件元信息
	fileRecord := models.File{
		Filename:   file.Filename,
		Size:       file.Size,
		UploadedAt: nowtime,
		Hash:       hash,
		FileID:     strFileID,
	}
	config.MySQLDB.Create(&fileRecord)

	redisKeyShort := "file_" + hash[:6]
	// 将文件 ID 和文件名存储到 Redis 的哈希中
	err = config.RedisClient.HMSet(context.TODO(), redisKeyShort, map[string]interface{}{
		"file_id":   strFileID,
		"file_name": fileName,
	}).Err()

	if err != nil {
		log.Printf("Failed to save hash to Redis: %v", err)
		return err
	}

	log.Printf("redis key: %s, file_id: %s, file_name: %s", redisKeyShort, strFileID, fileName)
	return nil
}

func UploadFile(c *gin.Context) {
	// userID := c.MustGet("userID").(string) // 获取用户ID
	form, err := c.MultipartForm()
	if err != nil {
		utils.Respond(c, http.StatusBadRequest, "error", fmt.Sprintf("Failed to get multipart form: %v", err))
		return
	}

	// 获取名为 "files" 的文件数组
	files := form.File["files"]
	if len(files) == 0 {
		utils.Respond(c, http.StatusBadRequest, "error", "No files uploaded")
		return
	}

	var errorDetails []map[string]string // 存储错误信息
	failureCount := 0                    // 记录上传失败的个数

	for _, file := range files {
		// 处理每个文件
		err := handleUploadFile(c, file) // 假设 handleFile 是处理文件的函数
		if err != nil {
			errorDetail := map[string]string{
				"id":     file.Filename, // 或者使用其他唯一标识符
				"reason": err.Error(),   // 错误原因
			}
			errorDetails = append(errorDetails, errorDetail)
			failureCount++ // 增加失败计数
		}
	}

	// 返回结果
	if failureCount == 0 {
		utils.Respond(c, http.StatusOK, "message", "Successful.")
	} else {
		utils.RespondWithFailures(c, http.StatusOK, failureCount, errorDetails)
	}
}

func getFileID(c *gin.Context) (string, string, error) {
	fileHash := c.Param("hash")
	var fileID string
	var fileName string
	if len(fileHash) == 6 {
		// 从 Redis 获取上传者信息
		redisKey := "file_" + fileHash
		fileInfo, err := config.RedisClient.HGetAll(context.TODO(), redisKey).Result()
		if err == redis.Nil {
			log.Printf("Error retrieving key from Redis: %v", err)
			utils.Respond(c, http.StatusNotFound, "error", "File has expired or does not exist")
			return "", "", err
		} else if err != nil {
			log.Printf("Failed to get file ID from Redis: %v", err)
			utils.Respond(c, http.StatusInternalServerError, "error", "Failed to retrieve file information")
			return "", "", err
		}
		fileID = fileInfo["file_id"]
		fileName = fileInfo["file_name"]
		log.Printf("File ID retrieved from Redis for key %s: %s, File Name: %s", redisKey, fileID, fileName)
	} else if len(fileHash) >= 32 {
		var file models.File
		if err := config.MySQLDB.Where("hash = ?", fileHash).First(&file).Error; err != nil {
			log.Printf("Failed to retrieve file from MySQL: %v", err)
			utils.Respond(c, http.StatusNotFound, "error", "Resource does not exist")
			return "", "", err
		}
		fileID = file.FileID     // 获取文件 ID
		fileName = file.Filename // 获取文件名
	} else {
		log.Printf("Error: Hash length is invalid.")
		return "", "", errors.New("invalid hash length")
	}

	return fileID, fileName, nil
}

func handleDownloadFile(c *gin.Context, fileID string, fileName string) error {
	// 将 fileID 转换为 MongoDB 的 ObjectID 类型
	objectID, err := primitive.ObjectIDFromHex(fileID)
	if err != nil {
		return err
	}

	// 从 MongoDB 的 GridFS 中获取文件内容
	bucket, err := gridfs.NewBucket(
		config.MongoClient.Database("yingwu-netdisk"),
		options.GridFSBucket().SetName("files"),
	)
	if err != nil {
		return err
	}

	downloadStream, err := bucket.OpenDownloadStream(objectID)
	if err != nil {
		log.Printf("Error retrieving file from GridFS: %v", err)
		return err
	}
	defer downloadStream.Close()

	// 设置响应头，指定文件类型和文件名
	encodedFileName := url.QueryEscape(fileName) // URL 编码文件名
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", encodedFileName))
	c.Header("Content-Type", "application/octet-stream")

	// 将文件内容写入响应
	_, err = io.Copy(c.Writer, downloadStream)
	if err != nil {
		return err
	}
	return nil
}

func DownloadFile(c *gin.Context) {
	fileID, fileName, err := getFileID(c)
	if err != nil {
		utils.Respond(c, http.StatusInternalServerError, "error", "Failed to retrieve file")
		return
	}
	err = handleDownloadFile(c, fileID, fileName)
	if err != nil {
		utils.Respond(c, http.StatusInternalServerError, "error", "Failed to download file")
	}
}

func GetAllFiles(c *gin.Context) {
	var files []models.File

	// 查询所有文件记录
	if err := config.MySQLDB.Order("id DESC").Find(&files).Error; err != nil {
		utils.Respond(c, http.StatusInternalServerError, "error", "Failed to retrieve files")
		return
	}
	c.JSON(http.StatusOK, files)
}
