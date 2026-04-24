package main

import (
	"log"

	"github.com/gin-gonic/gin"
	"test_oss/handlers"
	"test_oss/models"
	"test_oss/obsstore"
)

const (
	ak       = "HPUANB4LLQ7LC5QK9DLX"
	sk       = "YXu5c5zzZy2ebwKstB9LxXB2bZQloJKvbctqmOuF"
	endpoint = "obs.cn-east-3.myhuaweicloud.com"
)

func main() {
	models.InitDB()

	if err := obsstore.Init(ak, sk, endpoint); err != nil {
		log.Fatalf("OBS init failed: %v", err)
	}
	log.Println("OBS client initialized")

	r := gin.Default()

	// Serve the test page
	r.Static("/static", "./static")
	r.GET("/", func(c *gin.Context) {
		c.File("./static/index.html")
	})

	api := r.Group("/api")
	{
		api.POST("/upload/init", handlers.InitUpload)
		api.POST("/upload/chunk", handlers.UploadChunk)
		api.POST("/upload/complete", handlers.CompleteUpload)
		api.GET("/upload/status/:fileId", handlers.GetUploadStatus)
	}

	log.Println("server listening on :8080")
	log.Fatal(r.Run(":8080"))
}
