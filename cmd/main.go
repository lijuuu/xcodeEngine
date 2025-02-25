package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"xcodeengine/executor"
	"xcodeengine/routes"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"golang.org/x/time/rate"
)

type env struct {
	MaxWorkers     int
	JobCount       int
	URL            string
	Ratelimit      int
	RatelimitBurst int
}

func initENV() env {
	if err := godotenv.Load(".env.production"); err != nil {
		log.Fatalf("Error loading .env.production file: %v", err)
	}

	maxWorkers, err := strconv.Atoi(os.Getenv("MAX_WORKERS"))
	if err != nil {
		log.Fatalf("Failed to parse MAX_WORKERS: %v", err)
	}

	jobCount, err := strconv.Atoi(os.Getenv("JOB_COUNT"))
	if err != nil {
		log.Fatalf("Failed to parse JOB_COUNT: %v", err)
	}

	rateLimit, err := strconv.Atoi(os.Getenv("RATE_LIMIT"))
	if err != nil {
		log.Fatalf("Failed to parse RATE_LIMIT: %v", err)
	}

	rateLimitBurst, err := strconv.Atoi(os.Getenv("RATE_LIMIT_BURST"))
	if err != nil {
		log.Fatalf("Failed to parse RATE_LIMIT_BURST: %v", err)
	}

	return env{
		MaxWorkers:     maxWorkers,
		JobCount:       jobCount,
		URL:            os.Getenv("URL"),
		Ratelimit:      rateLimit,
		RatelimitBurst: rateLimitBurst,
	}
}

func main() {

	env := initENV()

	// Create a new RateLimiter instance
	rateLimiter := rate.NewLimiter(rate.Limit(float64(env.Ratelimit)), int(env.RatelimitBurst))

	// Create a new Gin router
	router := gin.Default()
	//cors
	router.Use(cors.Default())
	// Use the RateLimiter as middleware
	router.Use(func(c *gin.Context) {
		if !rateLimiter.Allow() {
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit exceeded !"})
			c.Abort()
			return
		}
		c.Next() // Proceed to the next handler
	})

	logFile1, err := os.OpenFile("logs/server.log", os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	logrus.SetOutput(logFile1)

	routes := routes.NewExecutionService()
	workerPool, err := executor.NewWorkerPool(3, 1)
	if err != nil {
		log.Fatalf("Failed to create worker pool: %v", err)
	}

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
