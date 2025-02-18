package main

import (
	"log"
	"os"
	"rce-service/pkg"

	"github.com/gin-gonic/gin"
)

func main() {
	service := pkg.NewExecutionService()
	r := gin.Default()

	// API routes
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": "Go Code Execution Service",
		})
	})

	r.POST("/execute", service.HandleExecute)

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
