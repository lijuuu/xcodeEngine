package main

import (
	"xcodeengine/config"

	"github.com/nats-io/nats.go"
	"go.uber.org/zap"
	"xcodeengine/natshandler"
)

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	config := config.LoadConfig()

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
		natshandler.HandleCompilerRequest(msg, nc)
	})
	// Keep the service running
	select {}
}
