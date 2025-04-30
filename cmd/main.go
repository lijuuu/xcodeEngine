package main

import (
	"os/exec"
	"xcodeengine/config"
	"xcodeengine/executor"
	"xcodeengine/natshandler"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"log"
	"strings"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	// Load configuration
	config := config.LoadConfig()

	// Check if the worker image exists
	imageName := "lijuthomas/worker" 
	if !checkIfDockerImageExists(imageName) {
		logger.Fatal("Worker Docker image not found. Exiting...")
	}

	// Initialize worker pool
	workerPool, _ := executor.NewWorkerPool(4, 3, 600, 1000) //worker, jobs, memory, vcpu (4,3,400,1000)

	// Connect to NATS
	nc, err := nats.Connect(config.NatsURL)
	if err != nil {
		logger.Fatal("Failed to connect to NATS",
			zap.String("url", config.NatsURL),
			zap.Error(err))
	}
	defer nc.Close()

	// Subscribe to execution requests
	nc.Subscribe("compiler.execute.request", func(msg *nats.Msg) {
		natshandler.HandleCompilerRequest(msg, nc, workerPool)
	})

	nc.Subscribe("problems.execute.request", func(msg *nats.Msg) {
		natshandler.HandleProblemRunRequest(msg, nc, workerPool)
	})

	// Keep the service running
	select {}
}

// checkIfDockerImageExists checks if a Docker image exists locally
func checkIfDockerImageExists(imageName string) bool {
	cmd := exec.Command("docker", "images", "-q", imageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Println("Error checking Docker image:", err)
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}
