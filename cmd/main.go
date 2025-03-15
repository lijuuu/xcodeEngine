package main

import (
	"xcodeengine/config"
	"xcodeengine/executor"

	"xcodeengine/natshandler"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	config := config.LoadConfig()

	workerPool, err := executor.NewWorkerPool(config.MaxWorkers, config.JobCount)

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
		logger.Info("Received execution request",
			zap.String("subject", msg.Subject),
			zap.String("reply", msg.Reply),
			zap.String("data", string(msg.Data)))
		natshandler.HandleCompilerRequest(msg, nc,workerPool)
	})

	nc.Subscribe("problems.execute.response", func(msg *nats.Msg) {
		logger.Info("Received execution response",
			zap.String("subject", msg.Subject),
			zap.String("reply", msg.Reply),
			zap.String("data", string(msg.Data)))
	})

	// Keep the service running
	select {}
}
