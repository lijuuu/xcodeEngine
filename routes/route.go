package routes

import (
	"encoding/base64"
	"errors"
	"fmt"
	"time"
	"xcodeengine/executor"
	"xcodeengine/internal"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var (
	ErrInvalidRequest = errors.New("invalid request parameters")
	ErrCodeTooLong    = errors.New("code exceeds maximum length")
)

type ExecutionRequest struct {
	Code     string `json:"code" binding:"required"`
	Language string `json:"language" binding:"required"`
}

type ExecutionResponse struct {
	Output        string `json:"output"`
	Error         string `json:"error,omitempty"`
	StatusMessage string `json:"status_message"`
	Success       bool   `json:"success"`
	ExecutionTime string `json:"execution_time,omitempty"`
}

type ExecutionService struct{}

func NewExecutionService() *ExecutionService {
	return &ExecutionService{}
}

func (s *ExecutionService) HandleExecute(c *gin.Context, workerPool *executor.WorkerPool) {
	var req ExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, ExecutionResponse{
			Error:         err.Error(),
			StatusMessage: "API request failed",
			Success:       false,
		})
		return
	}

	start := time.Now()
	codeBytes, err := base64.StdEncoding.DecodeString(req.Code)
	if err != nil {
		c.JSON(400, ExecutionResponse{
			Error:         err.Error(),
			StatusMessage: "API request failed",
			Success:       false,
		})
		return
	}

	code := string(codeBytes)
	fmt.Println("Time taken to decode base64: ", time.Since(start))

	// Sanitize code
	if err := internal.SanitizeCode(code, req.Language, 10000); err != nil {
		c.JSON(400, ExecutionResponse{
			Error:         err.Error(),
			StatusMessage: "API request failed",
			Success:       false,
		})
		return
	}

	// Execute code using worker pool
	result := workerPool.ExecuteJob(req.Language, code)
	logrus.Println("Request: ", req, "Response: ", result)

	fmt.Println("Result: ", result.Output)

	if result.Error != nil {
		c.JSON(400, ExecutionResponse{
			Output:        result.Output,
			Error:         result.Output,
			StatusMessage: "API request failed",
			Success:       false,
		})
		return
	}

	c.JSON(200, ExecutionResponse{
		Output:        result.Output,
		Error:         "",
		StatusMessage: "API request successful",
		Success:       true,
		ExecutionTime: result.ExecutionTime,
	})
}
