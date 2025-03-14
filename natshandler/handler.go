package natshandler

import (
	"encoding/json"
	"log"
	"xcodeengine/service"

	"github.com/nats-io/nats.go"
	"xcodeengine/model"
)

func HandleCompilerRequest(msg *nats.Msg, nc *nats.Conn) {
	var req model.ExecutionRequest
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		log.Printf("Failed to parse execution request: %v", err)
		return
	}

	compilerService := service.NewCompilerService()

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
