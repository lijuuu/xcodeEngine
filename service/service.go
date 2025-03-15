package service

import (
	"encoding/base64"
	"errors"
	"time"
	"xcodeengine/executor"
	"xcodeengine/internal"

	compilergrpc "github.com/lijuuu/GlobalProtoXcode/Compiler"
)

var (
	ErrInvalidRequest = errors.New("invalid request parameters")
	ErrCodeTooLong    = errors.New("code exceeds maximum length")
)

type CompilerRequest struct {
	Code     string `json:"code" binding:"required"`
	Language string `json:"language" binding:"required"`
}

type CompilerResponse struct {
	Output        string `json:"output"`
	Error         string `json:"error,omitempty"`
	StatusMessage string `json:"status_message"`
	Success       bool   `json:"success"`
	ExecutionTime string `json:"execution_time,omitempty"`
}

type CompilerService struct {
	WorkerPool *executor.WorkerPool
}

func NewCompilerService(workerPool *executor.WorkerPool) *CompilerService {
	return &CompilerService{
		WorkerPool: workerPool,
	}
}

func (s *CompilerService) Compile(code string, language string) (*compilergrpc.CompileResponse, error) {
	start := time.Now()

	codeBytes, err := base64.StdEncoding.DecodeString(code)
	if err != nil {
		return &compilergrpc.CompileResponse{
			Success:       false,
			Error:         err.Error(),
			StatusMessage: "Failed to decode base64",
		}, nil
	}

	code = string(codeBytes)

	// Sanitize code
	if err := internal.SanitizeCode(code, language, 10000); err != nil {
		return &compilergrpc.CompileResponse{
			Success:       false,
			Error:         err.Error(),
			StatusMessage: err.Error(),
		}, nil
	}

	// Execute code using worker pool
	result := s.WorkerPool.ExecuteJob(language, code)

	if result.Error != nil {
		return &compilergrpc.CompileResponse{
			Success:       false,
			Error:         result.Error.Error(),
			Output:        result.Output,
			StatusMessage: "Failed to execute code",
		}, nil
	}

	return &compilergrpc.CompileResponse{
		Success:       true,
		Output:        result.Output,
		ExecutionTime: time.Since(start).String(),
		StatusMessage: "Success",
	}, nil
}
