package services

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
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

func UploadFile(c *gin.Context) {
    // userID := c.MustGet("userID").(string) // 获取用户ID

    file, err := c.FormFile("file")
    if err != nil {
        utils.Respond(c,http.StatusBadRequest,"error","File is required")
        return
    }

    // 打开文件
    fileContent, err := file.Open()
    if err != nil {
        utils.Respond(c,http.StatusInternalServerError,"error","Unable to open file")
        return
    }
    defer fileContent.Close()

    // 计算文件的SHA-256哈希
    hash, err := utils.GenerateFileHash(fileContent)
    if err != nil {
        utils.Respond(c,http.StatusInternalServerError,"error","Failed to calculate file hash")
        return
    }
    // 重置读指针复用fileContent
    if _, err := fileContent.Seek(0, io.SeekStart); err != nil {
        utils.Respond(c, http.StatusInternalServerError, "error", "Failed to reset file pointer")
        return
    }

    // 查询 MySQL 中是否存在该哈希记录
    var existingFile models.File
    if err := config.MySQLDB.Where("hash = ?", hash).First(&existingFile).Error; err == nil {
        log.Printf("File already exists: %v", existingFile)
        utils.Respond(c,http.StatusOK,"message","File exists.")
        return
    }

    nowtime := time.Now()
    // 使用 GridFS 存储文件内容
    bucket, err := gridfs.NewBucket(
        config.MongoClient.Database("yingwu-netdisk"),
        options.GridFSBucket().SetName("files"),
    )
    if err != nil {
        log.Printf("Failed to create GridFS bucket %v", err)
        utils.Respond(c,http.StatusOK,"message","File exists.")
        return
    }

    // 上传文件内容到 GridFS
    fileID, err := bucket.UploadFromStream(file.Filename, fileContent)
    strFileID:=fileID.Hex()
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to upload file to GridFS"})
        return
    }

    if !config.MySQLDB.HasTable(&models.File{}) {
        // 表不存在，创建表
        if err := config.MySQLDB.AutoMigrate(&models.File{}); err != nil {
            log.Fatalf("failed to migrate database: %v", err)
        }
    }

    // 在MySQL中保存文件元信息
    fileRecord := models.File{
        Filename:  file.Filename,
        Size:      file.Size,
        UploadedAt: nowtime,
        Hash: hash,
        FileID: strFileID,
    }
    config.MySQLDB.Create(&fileRecord)

    // 存储哈希到 Redis
    redisKey := "file_" + hash
    err = config.RedisClient.Set(context.TODO(), redisKey, strFileID, 24*time.Hour).Err()
    if err != nil {
        log.Printf("Failed to save hash to Redis: %v", err)
        return
    }
    log.Printf("redis key: %s, file_id:%s", redisKey,strFileID)
    utils.Respond(c,http.StatusOK,"message","File uploaded successfully")
}

func DownloadFile(c *gin.Context) {
    fileHash := c.Param("hash")

    // 从 Redis 获取上传者信息
    redisKey := "file_" + fileHash
    fileID, err := config.RedisClient.Get(context.TODO(), redisKey).Result()
    if err == redis.Nil {
        log.Printf("Error retrieving key from Redis: %v", err)
        utils.Respond(c, http.StatusNotFound, "error", "File has expired or does not exist")
        return
    } else if err!= nil {
        log.Printf("Failed to get file ID from Redis: %v", err)
        utils.Respond(c, http.StatusInternalServerError, "error", "Failed to retrieve file information")
        return
    } else {
        log.Printf("File ID retrieved from Redis for key %s: %s", redisKey, fileID)
    }

    // 将 fileID 转换为 MongoDB 的 ObjectID 类型
    objectID, err := primitive.ObjectIDFromHex(fileID)
    if err != nil {
        utils.Respond(c, http.StatusBadRequest, "error", "Invalid file ID format")
        return
    }

    // 从 MongoDB 的 GridFS 中获取文件内容
    bucket, err := gridfs.NewBucket(
        config.MongoClient.Database("yingwu-netdisk"),
        options.GridFSBucket().SetName("files"),
    )
    if err != nil {
        utils.Respond(c, http.StatusInternalServerError, "error", "Failed to create GridFS bucket")
        return
    }

    downloadStream, err := bucket.OpenDownloadStream(objectID)
    if err != nil {
        log.Printf("Error retrieving file from GridFS: %v", err)
        utils.Respond(c, http.StatusNotFound, "error", "File not found in GridFS")
        return
    }
    defer downloadStream.Close()

    // 设置响应头，指定文件类型和文件名
    c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileHash))
    c.Header("Content-Type", "application/octet-stream")

    // 将文件内容写入响应
    _, err = io.Copy(c.Writer, downloadStream)
    if err != nil {
        utils.Respond(c, http.StatusInternalServerError, "error", "Failed to send file content")
    }
}

func GetAllFiles(c *gin.Context) {
	var files []models.File

	// 查询所有文件记录
	if err := config.MySQLDB.Find(&files).Error; err != nil {
        utils.Respond(c,http.StatusInternalServerError,"error","Failed to retrieve files")
		return
	}
	c.JSON(http.StatusOK, files)
}
