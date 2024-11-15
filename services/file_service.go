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

	"database/sql"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func saveFileToMongo(c *gin.Context, fileContent multipart.File,
	fileName string, nowTime time.Time) (primitive.ObjectID, error) {
	userID, _ := c.Get("userID")

	// 开始事务
	session, err := config.MongoClient.StartSession()
	if err != nil {
		log.Printf("Failed to start session: %v", err)
		return primitive.NilObjectID, err
	}
	defer session.EndSession(context.Background())

	// 运行事务
	err = session.StartTransaction()
	if err != nil {
		log.Printf("Failed to start transaction: %v", err)
		return primitive.NilObjectID, err
	}

	// 使用 GridFS 存储文件内容
	bucket, err := gridfs.NewBucket(
		config.MongoClient.Database("yingwu"),
	)
	if err != nil {
		log.Printf("Failed to create GridFS bucket %v", err)
		session.AbortTransaction(context.Background())
		return primitive.NilObjectID, err
	}

	// 上传文件内容到 GridFS
	fileID, err := bucket.UploadFromStream(fileName, fileContent)
	if err != nil {
		log.Printf("Failed to upload file to GridFS: %v", err)
		session.AbortTransaction(context.Background())
		return primitive.NilObjectID, err
	}

	// 将文件记录插入到 MongoDB 中，并设置过期时间
	if userID == nil || userID == "guest" {
		collection := config.MongoClient.Database("yingwu").Collection("fs.files")
		liveTime := config.FileLiveTime
		expireTime := nowTime.Add(liveTime)

		// 更新文件记录，设置过期时间
		_, err = collection.UpdateOne(
			context.Background(),
			bson.M{"_id": fileID},
			bson.M{
				"$set": bson.M{
					"expireAt": expireTime,
				},
			},
		)
		if err != nil {
			log.Printf("Failed to update file %v with expiration date %v: %v", fileID, expireTime, err)
			session.AbortTransaction(context.Background())
			return primitive.NilObjectID, err
		}

		// 确保有 TTL 索引
		_, err := collection.Indexes().CreateOne(
			context.Background(),
			mongo.IndexModel{
				Keys:    bson.D{{Key: "expireAt", Value: 1}},      // 按 expireAt 字段创建索引
				Options: options.Index().SetExpireAfterSeconds(0), // 设置 TTL
			},
		)
		if err != nil {
			log.Printf("Failed to create TTL index: %v", err)
			session.AbortTransaction(context.Background())
			return primitive.NilObjectID, err
		}
	}

	// 提交事务
	err = session.CommitTransaction(context.Background())
	if err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		return primitive.NilObjectID, err
	}

	return fileID, nil
}

func writeMySQL(c *gin.Context, file *multipart.FileHeader, strFileID string,
	nowTime time.Time, hash string) error {

	if !config.MySQLDB.HasTable(&models.File{}) {
		// 表不存在，创建表
		if err := config.MySQLDB.AutoMigrate(&models.File{}); err != nil {
			log.Fatalf("failed to migrate database: %v", err)
		}
	}

	// 在MySQL中保存文件元信息

	userID, _ := c.Get("userID")
	var expiredTime sql.NullTime
	// 根据 userID 判断是否设置过期时间
	if userID == nil || userID == "guest" {
		// 如果是 "guest" 或 userID 为 nil，则设置有效的过期时间
		expiredTime = sql.NullTime{
			Time:  nowTime.Add(config.FileLiveTime),
			Valid: true, // 有效时间
		}
	} else {
		// 如果是其他用户，则将 expiredTime 设置为 NULL
		expiredTime = sql.NullTime{
			Valid: false, // 无效（NULL）
		}
	}

	fileRecord := models.File{
		Filename:   file.Filename,
		Size:       file.Size,
		UploadedAt: nowTime,
		Hash:       hash,
		FileID:     strFileID,
		ExpiredAt:  expiredTime,
	}
	result := config.MySQLDB.Create(&fileRecord)
	if result.Error != nil {
		log.Printf("Error creating file record: %v", result.Error)
		return result.Error
	}

	// 插入成功
	log.Printf("MySQL: File record created successfully: %v", file.Filename)
	return nil
}

/**
* 返回文件标识，错误
 */
func writeRedis(strFileID string, fileName string, hash string) (string, error) {
	redisKeyShort := "file_" + hash[:6]
	// 将文件 ID 和文件名存储到 Redis 的哈希中
	err := config.RedisClient.HMSet(context.TODO(), redisKeyShort, map[string]interface{}{
		"file_id":   strFileID,
		"file_name": fileName,
	}).Err()

	if err != nil {
		log.Printf("Failed to save hash to Redis: %v", err)
		return "", err
	}
	// 设置过期时间
	err = config.RedisClient.Expire(context.TODO(), redisKeyShort, config.FileLiveTime).Err()
	if err != nil {
		log.Printf("Failed to set expiration time for Redis key: %v", err)
		return "", err
	}

	log.Printf("redis key: %s, file_id: %s, file_name: %s", redisKeyShort, strFileID, fileName)
	return redisKeyShort, nil
}

/**
* 返回文件名，文件标识，错误
 */
func handleUploadFile(c *gin.Context, file *multipart.FileHeader) (string, string, error) {
	// 打开文件
	fileContent, err := file.Open()
	if err != nil {
		return "", "", err
	}
	defer fileContent.Close()

	// 计算文件的哈希
	hashType := config.HashType
	hash, err := utils.GenerateFileHash(hashType, fileContent)
	if err != nil {
		return "", "", err
	}
	// 重置读指针复用fileContent
	if _, err := fileContent.Seek(0, io.SeekStart); err != nil {
		return "", "", err
	}

	// // 查询 MySQL 中是否存在该哈希记录
	// var existingFile models.File
	// if err := config.MySQLDB.Where("hash = ?", hash).First(&existingFile).Error; err == nil {
	// 	log.Printf("File already exists: %v", existingFile)
	// 	return "", "", errors.New("文件已存在")
	// }

	nowtime := time.Now()
	fileName := file.Filename
	fileID, err := saveFileToMongo(c, fileContent, fileName, nowtime)
	if err != nil {
		log.Printf("Failed to save file to Mongo: %v", err)
		return "", "", err
	}
	strFileID := fileID.Hex()

	err = writeMySQL(c, file, strFileID, nowtime, hash)
	if err != nil {
		log.Printf("Failed to save record to MySQL: %v", err)
		return "", "", err
	}
	var label string
	label, err = writeRedis(strFileID, fileName, hash)
	if err != nil {
		log.Printf("Failed to save record to Redis: %v", err)
		return "", "", err
	}
	return fileName, label, nil
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
	var successDetails []map[string]string
	failureCount := 0 // 记录上传失败的个数

	for _, file := range files {
		// 处理每个文件
		fileName, label, err := handleUploadFile(c, file) // 假设 handleFile 是处理文件的函数
		if err != nil {
			errorDetail := map[string]string{
				"id":     file.Filename, // 或者使用其他唯一标识符
				"reason": err.Error(),   // 错误原因
			}
			errorDetails = append(errorDetails, errorDetail)
			failureCount++ // 增加失败计数
		} else {
			// 如果文件上传成功，收集文件名和标签
			successDetail := map[string]string{
				"fileName": fileName,
				"label":    label,
			}
			successDetails = append(successDetails, successDetail)
		}
	}

	// 返回结果
	// 构造统一的返回结果
	response := map[string]interface{}{
		"successFiles": successDetails, // 上传成功的文件信息
		"failureFiles": errorDetails,   // 上传失败的文件信息
		"failureCount": failureCount,   // 失败的文件数量
		"message":      "上传处理完成",       // 通用的响应消息
	}

	// 返回统一的 JSON 格式
	utils.Respond(c, http.StatusOK, "result", response)
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
		if err := config.MySQLDB.Where("hash = ? AND (expired_at > ? OR expired_at IS NULL)", fileHash, time.Now()).
			First(&file).Error; err != nil {
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
		config.MongoClient.Database("yingwu"),
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

	userID, _ := c.Get("userID")
	if userID == nil || userID == "guest" {
		// 查询 expired_at 字段不为 NULL 的所有文件记录
		if err := config.MySQLDB.Where("expired_at IS NOT NULL").Order("id DESC").Find(&files).Error; err != nil {
			utils.Respond(c, http.StatusInternalServerError, "error", "Failed to retrieve files")
			return
		}
	} else {
		// 查询所有文件记录
		if err := config.MySQLDB.Order("id DESC").Find(&files).Error; err != nil {
			utils.Respond(c, http.StatusInternalServerError, "error", "Failed to retrieve files")
			return
		}
	}

	c.JSON(http.StatusOK, files)
}

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
