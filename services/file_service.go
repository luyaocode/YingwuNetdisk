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
	"strconv"
	"strings"
	"time"

	"yingwu/config"
	"yingwu/models"
	"yingwu/utils"

	"database/sql"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
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

func deleteFromMongo(fileID string) error {
	objectID, err := primitive.ObjectIDFromHex(fileID)
	if err != nil {
		return err
	}
	// 开始事务
	session, err := config.MongoClient.StartSession()
	if err != nil {
		log.Printf("Failed to start session: %v", err)
		return err
	}
	defer session.EndSession(context.Background())

	// 运行事务
	err = session.StartTransaction()
	if err != nil {
		log.Printf("Failed to start transaction: %v", err)
		return err
	}

	// 确保 GridFS 存储文件的删除
	bucket, err := gridfs.NewBucket(
		config.MongoClient.Database("yingwu"),
	)
	if err != nil {
		log.Printf("Failed to create GridFS bucket: %v", err)
		session.AbortTransaction(context.Background())
		return err
	}

	// 删除 GridFS 中的文件
	err = bucket.Delete(objectID)
	if err != nil {
		log.Printf("Failed to delete file from GridFS: %v", err)
		session.AbortTransaction(context.Background())
		return err
	}

	// 删除 MongoDB 中的文件记录
	collection := config.MongoClient.Database("yingwu").Collection("fs.files")
	result, err := collection.DeleteOne(
		context.Background(),
		bson.M{"_id": objectID},
	)
	if err != nil {
		log.Printf("Failed to delete file record from MongoDB: %v", err)
		session.AbortTransaction(context.Background())
		return err
	}

	// 如果没有记录删除，仅记录日志，跳过错误返回
	if result.DeletedCount == 0 {
		log.Printf("No file record found with ID: %v. Skipping deletion.", fileID)
	} else {
		log.Printf("Successfully deleted file record with ID: %v", fileID)
	}

	// 提交事务
	err = session.CommitTransaction(context.Background())
	if err != nil {
		log.Printf("Failed to commit transaction: %v", err)
		return err
	}

	return nil
}

func writeMySQL(c *gin.Context, file *multipart.FileHeader, strFileID string,
	nowTime time.Time, hash string) (uint, error) {
	var fid uint = 0
	// 在MySQL中保存文件元信息
	userID, _ := c.Get("userID")
	var expiredTime sql.NullTime
	// 根据 userID 判断是否设置过期时间
	if userID != config.MyGithubID { // 非管理员文件有效期限制
		// 如果是 "guest" 或 userID 为 nil，则设置有效的过期时间
		expiredTime = sql.NullTime{
			Time:  nowTime.Add(config.FileLiveTime),
			Valid: true, // 有效时间
		}
	} else {
		// 如果是管理员，则将 expiredTime 设置为 NULL
		expiredTime = sql.NullTime{
			Valid: false, // 无效（NULL）
		}
	}
	nUserID, _ := utils.AnyToInt64(userID)
	fileRecord := models.File{
		Filename:   file.Filename,
		Size:       file.Size,
		UploadedAt: nowTime,
		UploadedBy: nUserID,
		Hash:       hash,
		FileID:     strFileID,
		ExpiredAt:  expiredTime,
	}
	result := config.MySQLDB.Create(&fileRecord)
	if result.Error != nil {
		log.Printf("Error creating file record: %v", result.Error)
		return fid, result.Error
	}

	// 插入成功
	fid = fileRecord.ID
	log.Printf("MySQL: File record created successfully: %v", file.Filename)
	return fid, nil
}

/**
* 返回文件标识，错误
 */
func writeRedis(fid uint, strFileID string, fileName string, hash string) (string, error) {
	redisKeyShort := "file_" + hash[:6]
	// 将文件 ID 和文件名存储到 Redis 的哈希中
	err := config.RedisClient.HMSet(context.TODO(), redisKeyShort, map[string]interface{}{
		"fid":       fid,       // mysql 主键
		"file_id":   strFileID, // Gridfs id
		"file_name": fileName,
		"hash":      hash,
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
	var fileName string = ""
	var label string = ""
	// 打开文件
	fileContent, err := file.Open()
	if err != nil {
		return fileName, label, err
	}
	defer fileContent.Close()

	// 计算文件的哈希
	hashType := config.HashType
	hash, err := utils.GenerateFileHash(hashType, fileContent)
	if err != nil {
		return fileName, label, err
	}
	// 重置读指针复用fileContent
	if _, err := fileContent.Seek(0, io.SeekStart); err != nil {
		return fileName, label, err
	}

	// // 查询 MySQL 中是否存在该哈希记录
	// var existingFile models.File
	// if err := config.MySQLDB.Where("hash = ?", hash).First(&existingFile).Error; err == nil {
	// 	log.Printf("File already exists: %v", existingFile)
	// 	return "", "", errors.New("文件已存在")
	// }

	nowtime := time.Now()
	fileName = file.Filename
	fileID, err := saveFileToMongo(c, fileContent, fileName, nowtime)
	if err != nil {
		log.Printf("Failed to save file to Mongo: %v", err)
		return fileName, label, err
	}
	strFileID := fileID.Hex()

	fid, err := writeMySQL(c, file, strFileID, nowtime, hash)
	if err != nil {
		log.Printf("Failed to save record to MySQL: %v", err)
		return fileName, label, err
	}
	label, err = writeRedis(fid, strFileID, fileName, hash)
	if err != nil {
		log.Printf("Failed to save record to Redis: %v", err)
		return fileName, label, err
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

func handleDeleteFile(c *gin.Context, hash string) error {
	fileID, hash32, err := getFileIDByHash(c, hash)
	if err != nil {
		return err
	}
	err = deleteFromMongo(fileID)
	if err != nil {
		return err
	}
	// 返回来删除MySQL和Redis记录
	result := config.MySQLDB.Where("hash = ?", hash).
		Delete(&models.File{})
	if result.Error != nil {
		log.Printf("Error deleting file records with hash %s: %v", hash, result.Error)
	}
	if result.RowsAffected == 0 {
		log.Printf("No file records found with hash %s", hash)
	} else {
		log.Printf("Successfully deleted %d file records with hash %s", result.RowsAffected, hash)
	}
	// 使用 Redis 客户端删除指定的键
	redisKeyShort := "file_" + hash32[:6]
	err = config.RedisClient.Del(context.TODO(), hash32).Err()
	if err != nil {
		log.Printf("Failed to delete Redis key %s: %v", redisKeyShort, err)
	} else {
		log.Printf("Successfully deleted Redis key: %s", redisKeyShort)
	}
	return nil
}

func DeleteFile(c *gin.Context) {
	var requestBody struct {
		FileIDs []string `json:"files"`
	}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		utils.Respond(c, http.StatusBadRequest, "error", map[string]string{"message": "Invalid request body"})
		return
	}

	var errorDetails []map[string]string
	var successDetails []map[string]string
	failureCount := 0
	for _, hash := range requestBody.FileIDs {
		err := handleDeleteFile(c, hash)
		if err != nil {
			errorDetail := map[string]string{
				"hash":   hash,
				"reason": err.Error(),
			}
			errorDetails = append(errorDetails, errorDetail)
			failureCount++
		} else {
			successDetail := map[string]string{
				"hash": hash,
			}
			successDetails = append(successDetails, successDetail)
		}
	}

	response := map[string]interface{}{
		"successFiles": successDetails, // 删除成功的文件信息
		"failureFiles": errorDetails,   // 删除失败的文件信息
		"failureCount": failureCount,   // 失败的文件数量
		"message":      "删除处理完成",       // 通用的响应消息
	}

	utils.Respond(c, http.StatusOK, "result", response)
}

func handleLockFile(c *gin.Context, hash string) error {
	status := c.Param("status")
	// 转换 status 为 bool
	var locked bool
	if status == "true" {
		locked = true
	} else if status == "false" {
		locked = false
	} else {
		return fmt.Errorf("invalid status value: %s", status)
	}

	userID, _ := c.Get("userID")
	nUserID, _ := utils.AnyToInt64(userID)

	// 执行更新操作
	if err := config.MySQLDB.Model(&models.File{}).
		Where("hash = ? AND uploaded_by = ?", hash, nUserID).
		Update("locked", locked).Error; err != nil {
		return fmt.Errorf("failed to update locked field: %v", err)
	}

	return nil
}

func handleSetFileInfo(c *gin.Context, hash string, filename, tags string) error {
	userID, _ := c.Get("userID")
	nUserID, _ := utils.AnyToInt64(userID)

	// 执行更新操作
	result := config.MySQLDB.Model(&models.File{}).
		Where("hash = ? AND uploaded_by = ?", hash, nUserID).
		Updates(map[string]interface{}{
			"filename": filename,
			"tags":     tags,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update filename and tags: %v", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("no records found to update")
	}

	return nil
}

func handleSetFileTags(c *gin.Context, hash string, tags string) error {
	userID, _ := c.Get("userID")
	nUserID, _ := utils.AnyToInt64(userID)

	// 执行更新操作
	result := config.MySQLDB.Model(&models.File{}).
		Where("hash = ? AND uploaded_by = ?", hash, nUserID).
		Updates(map[string]interface{}{
			"tags": tags,
		})

	if result.Error != nil {
		return fmt.Errorf("failed to update filename and tags: %v", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("no records found to update")
	}

	return nil
}

// 锁定/解锁文件
func LockFile(c *gin.Context) {
	var requestBody struct {
		FileIDs []string `json:"files"`
	}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		utils.Respond(c, http.StatusBadRequest, "error", map[string]string{"message": "Invalid request body"})
		return
	}

	var errorDetails []map[string]string
	var successDetails []map[string]string
	failureCount := 0
	for _, hash := range requestBody.FileIDs {
		err := handleLockFile(c, hash)
		if err != nil {
			errorDetail := map[string]string{
				"hash":   hash,
				"reason": err.Error(),
			}
			errorDetails = append(errorDetails, errorDetail)
			failureCount++
		} else {
			successDetail := map[string]string{
				"hash": hash,
			}
			successDetails = append(successDetails, successDetail)
		}
	}

	response := map[string]interface{}{
		"successFiles": successDetails, // 操作成功的文件信息
		"failureFiles": errorDetails,   // 操作失败的文件信息
		"failureCount": failureCount,   // 失败的文件数量
		"message":      "操作完成",         // 通用的响应消息
	}

	utils.Respond(c, http.StatusOK, "result", response)
}

// 设置文件信息
func SetFileInfo(c *gin.Context) {
	var info struct {
		Filename string `json:"filename"`
		Tags     string `json:"tags"`
	}

	// 解析 JSON 请求体
	if err := c.ShouldBindJSON(&info); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid input"})
		return
	}
	fileHash := c.Param("hash")
	if err := handleSetFileInfo(c, fileHash, info.Filename, info.Tags); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	utils.Respond(c, http.StatusOK, "message", "ok")
}

// 设置文件标签
func SetFileTags(c *gin.Context) {
	var requestBody struct {
		FileIDs []string `json:"files"`
		Tags    string   `json:"tags"`
	}
	if err := c.ShouldBindJSON(&requestBody); err != nil {
		utils.Respond(c, http.StatusBadRequest, "error", map[string]string{"message": "Invalid request body"})
		return
	}
	var errorDetails []map[string]string
	var successDetails []map[string]string
	failureCount := 0
	for _, hash := range requestBody.FileIDs {
		err := handleSetFileTags(c, hash, requestBody.Tags)
		if err != nil {
			errorDetail := map[string]string{
				"hash":   hash,
				"reason": err.Error(),
			}
			errorDetails = append(errorDetails, errorDetail)
			failureCount++
		} else {
			successDetail := map[string]string{
				"hash": hash,
			}
			successDetails = append(successDetails, successDetail)
		}
	}
	response := map[string]interface{}{
		"successFiles": successDetails, // 操作成功的文件信息
		"failureFiles": errorDetails,   // 操作失败的文件信息
		"failureCount": failureCount,   // 失败的文件数量
		"message":      "操作完成",         // 通用的响应消息
	}

	utils.Respond(c, http.StatusOK, "result", response)
}

func GetAllFileTags(c *gin.Context) {
	// 调用 getTagsFromMySQL 函数来获取标签及其出现次数
	tagCountMap, err := getTagsFromMySQL(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	utils.Respond(c, http.StatusOK, "result", tagCountMap)
}

func getTagsFromMySQL(c *gin.Context) (map[string]int, error) {
	userID, _ := c.Get("userID")
	nUserID, _ := utils.AnyToInt64(userID)
	// 查询所有文件的 tags 字段
	var files []models.File
	err := config.MySQLDB.Model(&models.File{}).Select("tags").
		Where("uploaded_by = ?", nUserID).
		Find(&files).Error
	if err != nil {
		return nil, err
	}
	// 使用 map 统计标签的出现次数
	tagCountMap := make(map[string]int)
	// 遍历所有文件的 tags 字段
	for _, file := range files {
		// 分割 tags 字段中的标签（以空格为分隔符）
		tags := strings.Fields(file.Tags)
		// 统计每个标签的出现次数
		for _, tag := range tags {
			tagCountMap[tag]++
		}
	}

	return tagCountMap, nil
}

func getFileIDByHash(c *gin.Context, hash string) (string, string, error) {
	var fileID string = ""
	var hash32 string = hash
	userID, _ := c.Get("userID")
	nUserID, _ := utils.AnyToInt64(userID)
	if len(hash32) == 6 {
		// 从 Redis 获取上传者信息
		redisKey := "file_" + hash
		fileInfo, err := config.RedisClient.HGetAll(context.TODO(), redisKey).Result()
		if err == redis.Nil {
			log.Printf("Error retrieving key from Redis: %v", err)
			return fileID, hash32, err
		} else if err != nil {
			log.Printf("Failed to get file ID from Redis: %v", err)
			return fileID, hash32, err
		}
		fileID = fileInfo["file_id"]
		hash32 = fileInfo["hash"]
		log.Printf("hash32 and FileID retrieved from Redis for key %s: hash32: %s, FileID: %s", redisKey, hash32, fileID)
	} else if len(hash32) >= 32 {
		var file models.File
		if err := config.MySQLDB.Where("hash = ? AND uploaded_by = ?", hash32, nUserID).
			First(&file).Error; err != nil {
			log.Printf("Failed to retrieve file from MySQL: %v", err)
			return fileID, hash32, err
		}
		fileID = file.FileID
		log.Printf("FileID retrieved from MySQL: FileID: %s", fileID)
	} else {
		log.Printf("Error: Hash length is invalid.")
		return fileID, hash32, errors.New("invalid hash length")
	}
	return fileID, hash32, nil
}

func getFileID(c *gin.Context) (uint, string, string, error) {
	fileHash := c.Param("hash")
	var fid uint = 0
	var fileID string = ""
	var fileName string = ""
	if len(fileHash) == 6 {
		// 从 Redis 获取上传者信息
		redisKey := "file_" + fileHash
		fileInfo, err := config.RedisClient.HGetAll(context.TODO(), redisKey).Result()
		if err == redis.Nil {
			log.Printf("Error retrieving key from Redis: %v", err)
			utils.Respond(c, http.StatusNotFound, "error", "File has expired or does not exist")
			return fid, fileID, fileName, err
		} else if err != nil {
			log.Printf("Failed to get file ID from Redis: %v", err)
			utils.Respond(c, http.StatusInternalServerError, "error", "Failed to retrieve file information")
			return fid, fileID, fileName, err
		}
		fid_uint64, _ := strconv.ParseUint(fileInfo["fid"], 10, 64)
		fid = uint(fid_uint64)
		fileID = fileInfo["file_id"]
		fileName = fileInfo["file_name"]
		log.Printf("File ID retrieved from Redis for key %s: %s, File Name: %s", redisKey, fileID, fileName)
	} else if len(fileHash) >= 32 {
		var file models.File
		if err := config.MySQLDB.Where("hash = ? AND (expired_at > ? OR expired_at IS NULL)", fileHash, time.Now()).
			First(&file).Error; err != nil {
			log.Printf("Failed to retrieve file from MySQL: %v", err)
			utils.Respond(c, http.StatusNotFound, "error", "Resource does not exist")
			return fid, fileID, fileName, err

		}
		fid = file.ID
		fileID = file.FileID     // 获取文件 ID
		fileName = file.Filename // 获取文件名
	} else {
		log.Printf("Error: Hash length is invalid.")
		return fid, fileID, fileName, errors.New("invalid hash length")
	}

	return fid, fileID, fileName, nil
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
	fid, fileID, fileName, err := getFileID(c)
	if err != nil {
		utils.Respond(c, http.StatusInternalServerError, "error", "Failed to retrieve file")
		return
	}
	// 查询文件的锁定状态
	userID, _ := c.Get("userID")
	nUserID, _ := utils.AnyToInt64(userID)
	var file models.File
	err = config.MySQLDB.Where("id = ?", fid).First(&file).Error
	if err != nil {
		utils.Respond(c, http.StatusInternalServerError, "error", "Failed to query file information")
		return
	}

	// 如果文件被锁定且上传者不是当前用户，则返回锁定错误
	if file.Locked && file.UploadedBy != nUserID {
		utils.Respond(c, http.StatusForbidden, "error", "File is locked and you are not allowed to download it.")
		return
	}

	// 处理下载任务
	err = handleDownloadFile(c, fileID, fileName)
	if err != nil {
		utils.Respond(c, http.StatusInternalServerError, "error", "Failed to download file")
		return
	}

	// 将下载记录写入MySQL
	fileRecord := models.DownFile{
		FileID:       fid,
		DownloadedAt: time.Now(),
		DownloadedBy: nUserID,
	}
	result := config.MySQLDB.Create(&fileRecord)
	if result.Error != nil {
		log.Printf("Error creating file record: %v", result.Error)
		return
	}
}

func PreviewFile(c *gin.Context) {
	fid, fileID, fileName, err := getFileID(c)
	if err != nil {
		utils.Respond(c, http.StatusInternalServerError, "error", "Failed to retrieve file")
		return
	}
	// 查询文件的锁定状态
	userID, _ := c.Get("userID")
	nUserID, _ := utils.AnyToInt64(userID)
	var file models.File
	err = config.MySQLDB.Where("id = ?", fid).First(&file).Error
	if err != nil {
		utils.Respond(c, http.StatusInternalServerError, "error", "Failed to query file information")
		return
	}

	// 如果文件被锁定且上传者不是当前用户，则返回锁定错误
	if file.Locked && file.UploadedBy != nUserID {
		utils.Respond(c, http.StatusForbidden, "error", "File is locked and you are not allowed to download it.")
		return
	}

	// 执行下载任务
	err = handleDownloadFile(c, fileID, fileName)
	if err != nil {
		utils.Respond(c, http.StatusInternalServerError, "error", "Failed to preview file")
	}
}

func getForeverFileInfoByHash(hash string) (models.File, error) {
	var file models.File
	if err := config.MySQLDB.Where("expired_at is NULL AND hash = ?", hash).First(&file).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			log.Printf("file with hash %s not found", hash)
		}
		return file, err
	}
	return file, nil
}

func GetNoteInfo(c *gin.Context) {
	hash := c.Param("hash")
	file, err := getForeverFileInfoByHash(hash)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			utils.Respond(c, http.StatusInternalServerError, "error", "Failed to retrieve file")
			return
		}
	}
	if file.NoteID == "" {
		file.NoteID = uuid.New().String()
		// 更新文件的 NoteID
		if err := config.MySQLDB.Model(&file).Where("hash = ?", hash).Update("note_id", file.NoteID).Error; err != nil {
			log.Printf("Error updating file note_id: %v", err)
			utils.Respond(c, http.StatusInternalServerError, "error", "Failed to update file note_id")
			return
		}
	}
	response := map[string]interface{}{
		"noteID":    file.NoteID,
		"noteTitle": file.Filename,
	}
	utils.Respond(c, http.StatusOK, "result", response)
}

func GetAllFiles(c *gin.Context) {
	var files []models.File
	var totalCount int64

	// 获取分页参数，设置默认值
	page := c.DefaultQuery("page", "1")     // 默认页码为1
	limit := c.DefaultQuery("limit", "100") // 默认每页100条记录

	// 获取关键词参数，设置默认值为空字符串
	keyword := c.DefaultQuery("keyword", "")
	tag := c.DefaultQuery("tag", "")

	// 限制关键词最大长度为10个字符
	if len(keyword) > 10 {
		keyword = keyword[:10] // 截取前10个字符
	}

	// 去掉多余空格后拆分关键词为子串
	keyword = strings.TrimSpace(keyword) // 去除前后空格
	words := strings.Fields(keyword)     // 使用 Fields 拆分关键词为子串

	// 将页面和每页记录数转换为整数
	pageNum, err := strconv.Atoi(page)
	if err != nil || pageNum < 1 {
		pageNum = 1 // 页码不能小于1
	}

	limitNum, err := strconv.Atoi(limit)
	if err != nil || limitNum < 1 {
		limitNum = 100 // 每页条数不能小于1
	}

	// 计算偏移量
	offset := (pageNum - 1) * limitNum

	// 获取用户信息
	userID, _ := c.Get("userID")
	if userID == nil || userID == "guest" || userID == "test" { //游客、测试
		// 构建查询条件
		query := config.MySQLDB.Model(&models.File{}).
			Where("expired_at IS NOT NULL AND expired_at > NOW() AND uploaded_by < 0")

		// 如果提供了关键词，增加 Filename 的模糊匹配条件
		if len(words) > 0 {
			// 为每个子串增加模糊匹配条件
			for _, word := range words {
				query = query.Where("filename LIKE ?", "%"+word+"%")
			}
		}

		// 获取总数
		if err := query.Count(&totalCount).Error; err != nil {
			utils.Respond(c, http.StatusInternalServerError, "error", "Failed to retrieve total count")
			return
		}

		// 获取文件列表
		if err := query.Order("id DESC").
			Limit(limitNum).
			Offset(offset).
			Find(&files).Error; err != nil {
			utils.Respond(c, http.StatusInternalServerError, "error", "Failed to retrieve files")
			return
		}
	} else if userID == config.MyGithubID { // 系统管理员
		// 构建查询条件
		query := config.MySQLDB.Model(&models.File{}).
			Where("expired_at IS NULL OR expired_at > NOW()")

		// 如果提供了关键词，增加 Filename 的模糊匹配条件
		if len(words) > 0 {
			// 为每个子串增加模糊匹配条件
			for _, word := range words {
				query = query.Where("filename LIKE ?", "%"+word+"%")
			}
		}

		// 如果提供了tag，增加 tags 的模糊匹配条件
		if tag != "" {
			query = query.Where("tags LIKE ?", "%"+tag+"%")
		}

		// 获取总数
		if err := query.Count(&totalCount).Error; err != nil {
			utils.Respond(c, http.StatusInternalServerError, "error", "Failed to retrieve total count")
			return
		}

		// 获取文件列表
		if err := query.Order("id DESC").
			Limit(limitNum).
			Offset(offset).
			Find(&files).Error; err != nil {
			utils.Respond(c, http.StatusInternalServerError, "error", "Failed to retrieve files")
			return
		}
	} else { // 普通会员
		nUserID, _ := utils.AnyToInt64(userID)
		// 构建查询条件
		query := config.MySQLDB.Model(&models.File{}).
			Where("expired_at IS NOT NULL AND expired_at > NOW() AND (uploaded_by < 0 OR uploaded_by = ?)", nUserID)

		// 如果提供了关键词，增加 Filename 的模糊匹配条件
		if len(words) > 0 {
			// 为每个子串增加模糊匹配条件
			for _, word := range words {
				query = query.Where("filename LIKE ?", "%"+word+"%")
			}
		}

		// 如果提供了tag，增加 tags 的模糊匹配条件
		if tag != "" {
			query = query.Where("tags LIKE ?", "%"+tag+"%")
		}

		// 获取总数
		if err := query.Count(&totalCount).Error; err != nil {
			utils.Respond(c, http.StatusInternalServerError, "error", "Failed to retrieve total count")
			return
		}

		// 获取文件列表
		if err := query.Order("id DESC").
			Limit(limitNum).
			Offset(offset).
			Find(&files).Error; err != nil {
			utils.Respond(c, http.StatusInternalServerError, "error", "Failed to retrieve files")
			return
		}
	}

	// 返回分页数据和总记录数
	c.JSON(http.StatusOK, gin.H{
		"files":      files,
		"totalCount": totalCount,
		"page":       pageNum,
		"limit":      limitNum,
	})
}

func GetDownloads(c *gin.Context) {
	var files []models.FileWithDownloadInfo
	var totalCount int64

	// 获取分页参数，设置默认值
	page := c.DefaultQuery("page", "1")     // 默认页码为1
	limit := c.DefaultQuery("limit", "100") // 默认每页100条记录

	// 将页面和每页记录数转换为整数
	pageNum, err := strconv.Atoi(page)
	if err != nil || pageNum < 1 {
		pageNum = 1 // 页码不能小于1
	}

	limitNum, err := strconv.Atoi(limit)
	if err != nil || limitNum < 1 {
		limitNum = 100 // 每页条数不能小于1
	}

	// 计算偏移量
	offset := (pageNum - 1) * limitNum

	// 获取用户信息
	userID, _ := c.Get("userID")
	if userID == nil || userID == "guest" || userID == "test" { //游客、测试
		return
	}
	userIDInt, _ := utils.AnyToInt64(userID)
	err = config.MySQLDB.Table("downloaded_files").
		Select("downloaded_files.downloaded_at, files.id, files.filename, files.size, files.uploaded_at, files.uploaded_by, files.hash, files.file_id, files.expired_at").
		Joins("JOIN files ON downloaded_files.file_id = files.id").
		Where("downloaded_files.downloaded_by = ?", userIDInt).
		Order("downloaded_files.downloaded_at DESC").
		Limit(limitNum).
		Offset(offset).
		Scan(&files).Error

	if err != nil {
		log.Printf("Error querying down files: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error querying download records",
		})
		return
	}

	// 获取总记录数
	err = config.MySQLDB.Table("downloaded_files").
		Joins("JOIN files ON downloaded_files.file_id = files.id").
		Where("downloaded_files.downloaded_by = ?", userID).
		Count(&totalCount).Error

	if err != nil {
		log.Printf("Error counting total down files: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error counting download records",
		})
		return
	}

	// 返回分页数据和总记录数
	c.JSON(http.StatusOK, gin.H{
		"files":      files,
		"totalCount": totalCount,
		"page":       pageNum,
		"limit":      limitNum,
	})
}

func GetUploads(c *gin.Context) {
	var files []models.File
	var totalCount int64

	// 获取分页参数，设置默认值
	page := c.DefaultQuery("page", "1")     // 默认页码为1
	limit := c.DefaultQuery("limit", "100") // 默认每页100条记录

	// 将页面和每页记录数转换为整数
	pageNum, err := strconv.Atoi(page)
	if err != nil || pageNum < 1 {
		pageNum = 1 // 页码不能小于1
	}

	limitNum, err := strconv.Atoi(limit)
	if err != nil || limitNum < 1 {
		limitNum = 100 // 每页条数不能小于1
	}

	// 计算偏移量
	offset := (pageNum - 1) * limitNum

	// 获取用户信息
	userID, _ := c.Get("userID")
	if userID == nil || userID == "guest" || userID == "test" { //游客、测试
		// ..
		return
	}
	userIDInt, _ := utils.AnyToInt64(userID)
	// 查询UploadedBy=userID的记录，按ID逆序排序
	err = config.MySQLDB.Table("files").
		Where("uploaded_by = ?", userIDInt).
		Order("id DESC").
		Limit(limitNum).
		Offset(offset).
		Find(&files).Error

	if err != nil {
		log.Printf("Error querying uploaded files: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error querying upload records",
		})
		return
	}

	// 获取总记录数
	err = config.MySQLDB.Table("files").
		Where("uploaded_by = ?", userIDInt).
		Count(&totalCount).Error

	if err != nil {
		log.Printf("Error counting total uploaded files: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error counting upload records",
		})
		return
	}

	// 返回分页数据和总记录数
	c.JSON(http.StatusOK, gin.H{
		"files":      files,
		"totalCount": totalCount,
		"page":       pageNum,
		"limit":      limitNum,
	})
}

func GetDownFileRank(c *gin.Context) {
	var result []struct {
		FileID        uint   `json:"file_id"`
		Filename      string `json:"filename"`
		DownloadCount int64  `json:"download_count"`
	}

	// 执行联合查询，获取下载量排名
	err := config.MySQLDB.Table("downloaded_files").
		Select("files.id as file_id, files.filename, COUNT(downloaded_files.id) as download_count").
		Joins("JOIN files ON downloaded_files.file_id = files.id").
		Where("files.expired_at IS NOT NULL AND files.expired_at > ?", time.Now()). // 过滤未过期的文件
		Group("files.id").                                                          // 按文件ID分组
		Order("download_count DESC").                                               // 下载量降序排序
		Limit(10).
		Find(&result).Error

	if err != nil {
		log.Printf("Error querying download file rank: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Error querying download file rank",
		})
		return
	}

	// 返回结果
	c.JSON(http.StatusOK, gin.H{
		"files": result,
	})
}
