package service

import (
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
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

func normalizeLanguage(lang string) string {

	lang = strings.ToLower(lang)

	languageMap := map[string]string{

		"js":          "js",
		"jscript":     "js",
		"javscript":   "js",
		"javsscript":  "js",
		"javascipt":   "js",
		"javasript":   "js",
		"javascript":  "js",
		"java script": "js",
		"jscipt":      "js",

		"python":  "python",
		"pyt":     "python",
		"pyn":     "python",
		"pythn":   "python",
		"phyton":  "python",
		"py":  "python",
		"py thon": "python",
		"pthon":   "python",

		"go":      "go",
		"golang":  "go",
		"gol":     "go",
		"goo":     "go",
		"g o":     "go",
		"golangg": "go",

		"cpp":    "cpp",
		"c++":    "cpp",
		"cp":     "cpp",
		"cppp":   "cpp",
		"c plus": "cpp",
		"cxx":    "cpp",
		"cc":     "cpp",
		"cpp ":   "cpp",
	}

	if normalized, ok := languageMap[lang]; ok {
		return normalized
	}

	return lang
}

func (s *CompilerService) Compile(code string, language string) (*compilergrpc.CompileResponse, error) {
	start := time.Now()

	// Normalize the language string
	language = normalizeLanguage(language)

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

func (s *CompilerService) ExecuteProblemCode(code string, language string) (*compilergrpc.CompileResponse, error) {
	start := time.Now()

	// Normalize the language string
	language = normalizeLanguage(language)

	// fmt.Println("Normalized language:", language)
	// fmt.Println("Code:", code)

	// Sanitize code
	if err := internal.SanitizeCode(code, language, 1000000000000); err != nil {
		return &compilergrpc.CompileResponse{
			Success:       false,
			Output:        "",
			Error:         err.Error(),
			StatusMessage: err.Error(),
		}, nil
	}

	// Execute code using worker pool
	result := s.WorkerPool.ExecuteJob(language, code)
	fmt.Println("Execution result:", result)

	if result.Error != nil {
		return &compilergrpc.CompileResponse{
			Success:       false,
			Error:         result.Error.Error(),
			Output:        result.Output,
			StatusMessage: "Failed to execute code",
		}, nil
	}

	// fmt.Println("Output:", result.Output)

	return &compilergrpc.CompileResponse{
		Success:       true,
		Output:        result.Output,
		ExecutionTime: time.Since(start).String(),
		StatusMessage: "Success",
	}, nil
}
