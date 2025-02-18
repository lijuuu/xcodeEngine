package pkg

import (
	"errors"
	"rce-service/lang"

	"github.com/gin-gonic/gin"
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
}

type ExecutionService struct {
	containerName map[string]string
	maxCodeLen    int
}

func NewExecutionService() *ExecutionService {
	return &ExecutionService{
		containerName: map[string]string{
			"go":     "worker",
			"python": "worker",
			"js":     "worker",
			"cpp":    "worker",
		},
		maxCodeLen: 10000,
	}
}

func (s *ExecutionService) HandleExecute(c *gin.Context) {
	var req ExecutionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, ExecutionResponse{
			Error:         err.Error(),
			StatusMessage: "Invalid Request Format",
		})
		return
	}

	// Check code length
	if len(req.Code) > s.maxCodeLen {
		c.JSON(400, ExecutionResponse{
			Error:         ErrCodeTooLong.Error(),
			StatusMessage: "Code Too Long",
		})
		return
	}

	// Execute code
	output, err := lang.Execute(s.containerName[req.Language], req.Language, req.Code)
	if err != nil {
		c.JSON(400, ExecutionResponse{
			Error:         err.Error(),
			StatusMessage: "Runtime Error",
			Output:        output, // Include output even if there's an error
		})
		return
	}

	c.JSON(200, ExecutionResponse{
		Output:        output,
		StatusMessage: "Success",
	})
}
