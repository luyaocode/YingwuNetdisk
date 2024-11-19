package config

import (
	"context"
	"io"
	"log"
	"os"
	"time"
	"yingwu/models"

	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	// 临时文件保留时间
	FileLiveTime = 8 * time.Hour
	HashType     = "md5"

	//
	Role_Test  = -2
	Role_Guest = -3
	Role_Other = -1
)

var (
	MySQLDB     *gorm.DB
	MongoClient *mongo.Client
	RedisClient *redis.Client
	ctx         = context.Background()

	MyGithubID string
)

func Init() {
	var err error

	// 加载配置
	viper.SetConfigName("config") // 配置文件名 (不需要扩展名)
	viper.SetConfigType("yaml")   // 配置文件类型
	viper.AddConfigPath(".")      // 配置文件路径

	if err := viper.ReadInConfig(); err != nil {
		log.Fatalf("Error reading config file, %s", err)
	}

	// 变量初始化
	MyGithubID = viper.GetString("MyGithubID")

	// MySQL 初始化
	mysqlConfig := viper.Sub("mysql")
	mysqlDSN := mysqlConfig.GetString("user") + ":" +
		mysqlConfig.GetString("password") +
		"@tcp(" + mysqlConfig.GetString("host") + ":" +
		mysqlConfig.GetString("port") + ")/" +
		mysqlConfig.GetString("dbname") +
		"?charset=utf8&parseTime=True&loc=Local"

	MySQLDB, err = gorm.Open("mysql", mysqlDSN)
	if err != nil {
		log.Fatal("Failed to connect to MySQL: ", err)
	}
	if !MySQLDB.HasTable(&models.File{}) {
		// 表不存在，创建表
		if err := MySQLDB.AutoMigrate(&models.File{}); err != nil {
			log.Fatalf("failed to migrate database: %v", err)
		}
	}
	if !MySQLDB.HasTable(&models.DownFile{}) {
		// 表不存在，创建表
		if err := MySQLDB.AutoMigrate(&models.DownFile{}); err != nil {
			log.Fatalf("failed to migrate database: %v", err)
		}
	}

	// MongoDB 初始化
	mongoURI := viper.GetString("mongodb.uri")
	mongoOptions := options.Client().ApplyURI(mongoURI)
	MongoClient, err = mongo.Connect(ctx, mongoOptions)
	if err != nil {
		log.Fatal("Failed to connect to MongoDB: ", err)
	}

	// Redis 初始化
	redisOptions := &redis.Options{
		Addr: viper.GetString("redis.address"),
		DB:   viper.GetInt("redis.db"),
	}

	RedisClient = redis.NewClient(redisOptions)
	_, err = RedisClient.Ping(ctx).Result()
	if err != nil {
		log.Fatal("Failed to connect to Redis: ", err)
	}
}

func SetLog() {
	// 配置日志分割
	logFile := &lumberjack.Logger{
		Filename:   "./logs/app.log", // 日志文件路径
		MaxSize:    10,               // 单个日志文件最大大小 (单位：MB)
		MaxBackups: 7,                // 最多保留的旧日志文件个数
		MaxAge:     30,               // 日志文件保存的最大天数
		Compress:   true,             // 是否启用压缩旧日志文件
	}

	// 创建 MultiWriter，用于同时输出到控制台和日志文件
	multiWriter := io.MultiWriter(os.Stdout, logFile)

	// 设置 log 包的默认输出
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile) // 设置日志格式
}
