package natshandler

import (
	"encoding/json"
	"log"
	"xcodeengine/executor"
	"xcodeengine/service"

	"xcodeengine/model"

	"github.com/nats-io/nats.go"
)

func HandleCompilerRequest(msg *nats.Msg, nc *nats.Conn, workerPool *executor.WorkerPool) {
	var req model.ExecutionRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		log.Printf("Failed to parse execution request: %v", err)
		return
	}

	compilerService := service.NewCompilerService(workerPool)

	// Execute the code (mock function here)
	res, err := compilerService.Compile(req.Code, req.Language)
	if err != nil {
		log.Printf("Failed to compile code: %v", err)
		return
	}

	// Send response back to API Gateway
	resData, _ := json.Marshal(res)
	nc.Publish(msg.Reply, resData)
}
