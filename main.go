package main

import (
	"flag"
	"log"
	"net/http"
	"yingwu/config"
	"yingwu/routes"
	"yingwu/scripts"

	"github.com/gin-gonic/gin"
)

func main() {
	// 设置日志
	config.SetLog()

	// 启动定时任务
	go scripts.CleanChunks()

	env := flag.String("env", "dev", "set environment (dev or prod)")

	// 初始化数据库
	config.Init()

	r := gin.Default()
	routes.SetupRoutes(r, *env)

	// 启动服务器
	flag.Parse()
	if *env == "prod" {
		log.Println("Starting production server on :8080")
		log.Fatal(http.ListenAndServeTLS(":8080", "./ssl/s.pem",
			"./ssl/s.key", r))
	} else {
		log.Println("Starting development server on :8080")
		log.Fatal(http.ListenAndServe(":8080", r))
	}
}
