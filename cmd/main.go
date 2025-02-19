package main

import (
	"log"
	"os"
	"xcodeengine/executor"
	"xcodeengine/routes"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func main() {
	r := gin.Default()
	logFile, err := os.OpenFile("server.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	logrus.SetOutput(logFile)
	logrus.SetFormatter(&logrus.JSONFormatter{})
	routes := routes.NewExecutionService()
	workerPool := executor.NewWorkerPool(2)
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
