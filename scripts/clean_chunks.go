package scripts

/**
* 定时清理GridFS过期文件
 */

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// cleanChunks 删除 `fs.chunks` 中那些对应 `fs.files` 中没有记录的文件数据
func cleanChunks(mongoClient *mongo.Client, databaseName string) {
	// 获取 fs.files 和 fs.chunks 集合
	filesCollection := mongoClient.Database(databaseName).Collection("fs.files")
	chunksCollection := mongoClient.Database(databaseName).Collection("fs.chunks")

	// 查找所有 fs.chunks 中的文件 ID
	cursor, err := chunksCollection.Find(context.Background(), bson.M{})
	if err != nil {
		log.Printf("Failed to find chunks: %v", err)
		return
	}
	defer cursor.Close(context.Background())

	// 遍历 chunks，删除那些在 fs.files 中不存在的文件 ID
	for cursor.Next(context.Background()) {
		var chunk bson.M
		if err := cursor.Decode(&chunk); err != nil {
			log.Printf("Failed to decode chunk: %v", err)
			continue
		}

		// 获取 chunk 中的文件 ID
		filesID, ok := chunk["files_id"].(primitive.ObjectID)
		if !ok {
			log.Printf("Invalid files_id in chunk: %v", chunk["_id"])
			continue
		}

		// 检查 fs.files 是否包含该文件 ID
		count, err := filesCollection.CountDocuments(context.Background(), bson.M{
			"_id": filesID,
		})
		if err != nil {
			log.Printf("Failed to count files for files_id %v: %v", filesID, err)
			continue
		}

		// 如果 fs.files 中没有该文件记录，删除 fs.chunks 中的对应 chunk
		if count == 0 {
			_, err := chunksCollection.DeleteOne(context.Background(), bson.M{
				"_id": chunk["_id"],
			})
			if err != nil {
				log.Printf("Failed to delete chunk: %v", err)
			} else {
				log.Printf("Deleted chunk with _id %v as its file does not exist", chunk["_id"])
			}
		}
	}

	if err := cursor.Err(); err != nil {
		log.Printf("Cursor iteration error: %v", err)
	}
}

func CleanChunks() {
	// 设置 MongoDB URI 和数据库名称
	mongoURI := "mongodb://luyao:27017@localhost:27017/yingwu"
	databaseName := "yingwu"

	// MongoDB 客户端初始化
	clientOptions := options.Client().ApplyURI(mongoURI)
	mongoClient, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatalf("Failed to connect to MongoDB: %v", err)
	}
	defer mongoClient.Disconnect(context.Background())

	// 清理文件和数据块
	cleanChunks(mongoClient, databaseName)

	// 计算距离下一个凌晨4点的时间间隔
	now := time.Now()
	nextCleanTime := time.Date(now.Year(), now.Month(), now.Day(), 4, 0, 0, 0, now.Location())

	// 如果当前时间已经过了凌晨4点，设置下一个执行时间为明天凌晨4点
	if now.After(nextCleanTime) {
		nextCleanTime = nextCleanTime.Add(24 * time.Hour)
	}

	// 计算等待时间
	waitDuration := nextCleanTime.Sub(now)
	log.Printf("Waiting until %s to clean chunks. Duration: %v", nextCleanTime, waitDuration)

	// 等待直到凌晨4点
	time.Sleep(waitDuration)

	// 执行清理操作
	for {
		// 执行清理任务
		log.Println("Cleaning chunks...")
		cleanChunks(mongoClient, databaseName)

		// 每24小时执行一次
		time.Sleep(24 * time.Hour)
	}
}
