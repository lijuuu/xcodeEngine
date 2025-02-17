package pkg

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"rce-service/lang"
	"sync"
)

var (
	ErrInvalidRequest       = errors.New("invalid request parameters")
	ErrLanguageNotSupported = errors.New("language not supported")
	ErrMethodNotSupported   = errors.New("method not supported")
)

type ExecutionRequest struct {
	Language string `json:"language"`
	Code     string `json:"code"`
	Method   string `json:"method"`
}

type ExecutionResponse struct {
	Output        string `json:"output"`
	Error         string `json:"error,omitempty"`
	StatusMessage string `json:"status_message"`
}

type ExecutionService struct {
	containers   map[string]string
	RateLimiter  *RateLimiter
	Sanitizer    *CodeSanitizer
	responsePool sync.Pool
}

func NewExecutionService() *ExecutionService {
	return &ExecutionService{
		containers: map[string]string{
			"python": "python-executor",
			"go":     "go-executor",
			"nodejs": "js-executor",
		},
		RateLimiter: NewRateLimiter(),
		Sanitizer:   NewCodeSanitizer(10000),
		responsePool: sync.Pool{
			New: func() interface{} {
				return &ExecutionResponse{}
			},
		},
	}
}

func (s *ExecutionService) HandleExecute(w http.ResponseWriter, r *http.Request) {
	var req ExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Get response object from pool
	resp := s.responsePool.Get().(*ExecutionResponse)
	defer s.responsePool.Put(resp)
	*resp = ExecutionResponse{} // Reset fields

	// Check required fields and method
	if req.Language == "" || req.Code == "" {
		http.Error(w, ErrInvalidRequest.Error(), http.StatusBadRequest)
		return
	}

	// Validate language support
	containerName, supported := s.containers[req.Language]
	if !supported {
		http.Error(w, ErrLanguageNotSupported.Error(), http.StatusBadRequest)
		return
	}

	// Sanitize code
	if err := s.Sanitizer.SanitizeCode(req.Code, req.Language); err != nil {
		resp.Error = err.Error()
		resp.StatusMessage = "Code Sanitization Error"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
		return
	}

	// Execute code
	output, err := executeLanguageCode(containerName, req.Language, req.Code)
	resp.Output = output
	resp.StatusMessage = "Accepted"

	if err != nil {
		resp.Error = err.Error()
		resp.StatusMessage = "Runtime Error"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(resp)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func executeLanguageCode(containerName, language, code string) (string, error) {
	switch language {
	case "python":
		return lang.ExecutePythonCode(containerName, code)
	case "go":
		return lang.ExecuteGoCode(containerName, code)
	case "nodejs":
		return lang.ExecuteJsCode(containerName, code)
	default:
		return "", ErrLanguageNotSupported
	}
}

func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL)
		next.ServeHTTP(w, r)
	})
}
