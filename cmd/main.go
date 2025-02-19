package main

import (
	"log"
	"os"
	"xcodeengine/executor"
	"xcodeengine/routes"

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
	r := gin.Default()
	logFile1 := setupLogFile("/logs/server.log")
	logrus.SetOutput(logFile1)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	
	routes := routes.NewExecutionService()
	logFile2 := setupLogFile("/logs/container.log")
	workerPool, _ := executor.NewWorkerPool(log.New(logFile2, "", log.LstdFlags))
	// API routes
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Go Code Execution Service",
		})
	})
	r.POST("/execute", func(c *gin.Context) {
		routes.HandleExecute(c, workerPool)
	})

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}

	log.Printf("Starting server on port %s...", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Server failed to start: %v", err)
	}
}
