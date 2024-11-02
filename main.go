package main

import (
	"yingwu/config"
	"yingwu/routes"

	"github.com/gin-gonic/gin"
)

func main() {
    // 初始化数据库
    config.Init()

    r := gin.Default()
    routes.SetupRoutes(r)

    // 启动服务器
    r.Run(":8080")
}
