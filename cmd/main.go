package main

import (
	"net"
	"xcodeengine/config"

	"google.golang.org/grpc"

	"xcodeengine/service"

	compilergrpc "github.com/lijuuu/GlobalProtoXcode/Compiler"
	"go.uber.org/zap"
)

func main() {
	// Initialize zap logger
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	config := config.LoadConfig()

	// // Create a new RateLimiter instance
	// rateLimiter := rate.NewLimiter(rate.Limit(float64(config.Ratelimit)), config.RatelimitBurst)

	// // Create a new Gin router
	// router := gin.Default()
	// //cors
	// router.Use(cors.Default())
	// // Use the RateLimiter as middleware
	// router.Use(func(c *gin.Context) {
	// 	if !rateLimiter.Allow() {
	// 		c.JSON(http.StatusTooManyRequests, gin.H{"error": "Rate limit exceeded !"})
	// 		c.Abort()
	// 		return
	// 	}
	// 	c.Next() // Proceed to the next handler
	// })

	// logFile1, err := os.OpenFile("logs/server.log", os.O_WRONLY|os.O_APPEND, 0666)
	// if err != nil {
	// 	log.Fatalf("Failed to open log file: %v", err)
	// }
	// logrus.SetOutput(logFile1)

	// routes := routes.NewExecutionService()
	// workerPool, err := executor.NewWorkerPool(config.MaxWorkers, config.JobCount)
	// if err != nil {
	// 	log.Fatalf("Failed to create worker pool: %v", err)
	// }

	// // API routes
	// router.GET("/", func(c *gin.Context) {
	// 	c.JSON(200, gin.H{
	// 		"message": "Go Code Execution Service",
	// 	})
	// })
	// router.POST("/execute", func(c *gin.Context) {
	// 	routes.HandleExecute(c, workerPool)
	// })

	// log.Printf("Starting server on port %s...", config.Port)
	// if err := router.Run(":" + config.Port); err != nil {
	// 	log.Fatalf("Server failed to start: %v", err)
	// }

	listener, err := net.Listen("tcp", ":"+config.Port)
	if err != nil {
		logger.Fatal("Failed to listen",
			zap.String("port", config.Port),
			zap.Error(err))
	}
	logger.Info("Server listening",
		zap.String("port", config.Port))

	compilerService := service.NewCompilerService()

	grpcServer := grpc.NewServer()
	compilergrpc.RegisterCompilerServiceServer(grpcServer, compilerService)
	logger.Info("gRPC server registered")

	if err := grpcServer.Serve(listener); err != nil {
		logger.Fatal("Failed to serve",
			zap.Error(err))
	}
}
