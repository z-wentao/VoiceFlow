package main

import (
    "log"
    "net/http"

    "github.com/gin-gonic/gin"
)

func main() {
    // Create a Gin router with default middleware (logger and recovery)
    r := gin.Default()

    // 静态文件
    r.StaticFile("/", "./web/index.html")

    // Define a simple GET endpoint
    r.GET("/ping", func(c *gin.Context) {
	// Return JSON response
	c.JSON(http.StatusOK, gin.H{
	    "message": "pong",
	})
    })

    // Start server on port 8080 (default)
    // Server will listen on 0.0.0.0:8080 (localhost:8080 on Windows)
    log.Println("🚀 服务器启动在 http://localhost:8080")
    r.Run()
}
