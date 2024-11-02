package config

import (
	"context"
	"log"

	"github.com/go-redis/redis/v8"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jinzhu/gorm"
	"github.com/spf13/viper"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var (
    MySQLDB     *gorm.DB
    MongoClient *mongo.Client
    RedisClient *redis.Client
    ctx         = context.Background()
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
