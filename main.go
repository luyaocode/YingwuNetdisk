package main

import (
	"flag"
	"log"
	"net/http"
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
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})
	env := flag.String("env", "dev", "set environment (dev or prod)")
	flag.Parse()
	if *env == "prod" {
		log.Println("Starting production server on :8080")
		log.Fatal(http.ListenAndServeTLS(":8080", "./ssl/server.pem",
			"./ssl/server.key", nil))
	} else {
		log.Println("Starting development server on :8080")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}
}
