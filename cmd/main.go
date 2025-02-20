package main

import (
	"log"
	"os"
	"xcodeengine/executor"
	"xcodeengine/routes"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func setupLogFile(path string) *os.File {
	logFile, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	return logFile
}

func main() {
	// Create a new RateLimiter instance
	// rateLimiter := rate.NewLimiter(1, 3)

	// Create a new Gin router
	router := gin.Default()

	//cors
	router.Use(cors.Default())
	// Use the RateLimiter as middleware
	// router.Use(func(c *gin.Context) {
	// 	if !rateLimiter.Allow() {
	// 		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit exceeded ! Buy some residential proxies and try again"})
	// 		c.Abort()
	// 		return
	// 	}
	// 	c.Next() // Proceed to the next handler
	// })

	logFile1 := setupLogFile("logs/server.log")
	logrus.SetOutput(logFile1)
	logrus.SetFormatter(&logrus.JSONFormatter{})

	routes := routes.NewExecutionService()
	// logFile2 := setupLogFile("/logs/container.log")
	workerPool := executor.NewWorkerPool()
	// API routes
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Go Code Execution Service",
		})
	})
	router.POST("/execute", func(c *gin.Context) {
		routes.HandleExecute(c, workerPool)
	})

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("Starting server on port %s...", port)
	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
