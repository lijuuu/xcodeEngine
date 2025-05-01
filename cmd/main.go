package main

import (
	"os/exec"
	"xcodeengine/config"
	"xcodeengine/executor"
	"xcodeengine/natshandler"

	"log"
	"strings"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"

	zap_betterstack "xcodeengine/logger"
)

func main() {

	// Load configuration
	log.Println("Loading engine configuration...")
	config := config.LoadConfig()
	log.Printf("Loaded config: %+v\n", config)

	// Initialize Zap logger based on environment
	var logger *zap.Logger
	var err error
	if config.Environment == "development" {
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	if err != nil {
		panic("Failed to initialize Zap logger: " + err.Error())
	}
	defer logger.Sync()

	// Initialize BetterStackLogStreamer
	logStreamer := zap_betterstack.NewBetterStackLogStreamer(
		config.BetterStackSourceToken,
		config.Environment,
		config.BetterStackUploadURL,
		logger,
	)

	log.Println("Prepping Code Execution engine")

	// Check if the worker image exists
	imageName := "lijuthomas/worker"
	log.Printf("Checking if Docker image '%s' exists locally...", imageName)
	if !checkIfDockerImageExists(imageName) {
		logger.Fatal("Worker Docker image not found. Exiting...",
			zap.String("image", imageName))
	}
	log.Printf("Docker image '%s' found.", imageName)

	log.Println("Starting worker pool initialization")
	workerPool, err := executor.NewWorkerPool(2, 3, 200, 500,logStreamer) //workers, jobs, memory, vcpu,logstreamer
	if err != nil {
		logger.Fatal("Failed to initialize worker pool",
			zap.Error(err))
	}
	log.Println("Worker pool initialized successfully")

	// Connect to NATS
	log.Printf("Connecting to NATS at: %s", config.NatsURL)
	nc, err := nats.Connect(config.NatsURL)
	if err != nil {
		logger.Fatal("Failed to connect to NATS",
			zap.String("url", config.NatsURL),
			zap.Error(err))
	}
	defer nc.Close()
	log.Println("Successfully connected to NATS")

	// Subscribe to execution requests
	log.Println("Subscribing to 'compiler.execute.request'")
	_, err = nc.Subscribe("compiler.execute.request", func(msg *nats.Msg) {
		log.Println("Received compiler.execute.request message")
		natshandler.HandleCompilerRequest(msg, nc, workerPool)
	})
	if err != nil {
		logger.Fatal("Failed to subscribe to compiler.execute.request",
			zap.Error(err))
	}

	log.Println("Subscribing to 'problems.execute.request'")
	_, err = nc.Subscribe("problems.execute.request", func(msg *nats.Msg) {
		log.Println("Received problems.execute.request message")
		natshandler.HandleProblemRunRequest(msg, nc, workerPool)
	})
	if err != nil {
		logger.Fatal("Failed to subscribe to problems.execute.request",
			zap.Error(err))
	}

	log.Println("Engine service is up and listening for requests")

	// Keep the service running
	select {}
}

// checkIfDockerImageExists checks if a Docker image exists locally
func checkIfDockerImageExists(imageName string) bool {
	printAllWorkerImages() // print all workers before checking
	cmd := exec.Command("docker", "images", "-q", imageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Println("Error checking Docker image:", err)
		return false
	}
	imageID := strings.TrimSpace(string(output))
	log.Printf("Image ID for '%s': %s", imageName, imageID)
	return imageID != ""
}

// printAllWorkerImages prints all existing images with 'worker' in the name
func printAllWorkerImages() {
	log.Println("Listing all local Docker images containing 'worker':")
	cmd := exec.Command("docker", "images")
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Println("Error listing Docker images:", err)
		return
	}
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "worker") {
			log.Println(line)
		}
	}
}
