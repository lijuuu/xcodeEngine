package pkg

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"rce-service/lang"
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
	containers  map[string]string
	RateLimiter *RateLimiter
	Sanitizer   *CodeSanitizer
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
	}
}

func (s *ExecutionService) HandleExecute(w http.ResponseWriter, r *http.Request) {
	var req ExecutionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	// Check required fields and method
	if req.Language == "" || req.Code == "" || req.Method != "docker" {
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
		response := ExecutionResponse{
			Error:         err.Error(),
			StatusMessage: "Code Sanitization Error",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Execute code
	output, err := executeLanguageCode(containerName, req.Language, req.Code)
	response := ExecutionResponse{
		Output:        output,
		StatusMessage: "Accepted",
	}

	if err != nil {
		response.Error = err.Error()
		response.StatusMessage = "Runtime Error"
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK) 
		json.NewEncoder(w).Encode(response)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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
