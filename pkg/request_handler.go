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
}


func NewExecutionService() *ExecutionService {
	return &ExecutionService{
		containers: map[string]string{
			"python": "python-executor",
			"go":     "go-executor",
			"nodejs": "js-executor",
		},
		RateLimiter: NewRateLimiter(),
	}
}


func (s *ExecutionService) HandleExecute(w http.ResponseWriter, r *http.Request) {
	var req ExecutionRequest

	
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}

	
	if err := validateRequest(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	
	response, err := s.ExecuteCode(&req)
	if err != nil {
		handleError(w, err)
		return
	}

	
	sendJSONResponse(w, response)
}


func validateRequest(req *ExecutionRequest) error {
	if req.Language == "" || req.Code == "" || req.Method == "" {
		return ErrInvalidRequest
	}
	return nil
}


func sendJSONResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}


func handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrInvalidRequest),
		errors.Is(err, ErrLanguageNotSupported),
		errors.Is(err, ErrMethodNotSupported):
		http.Error(w, err.Error(), http.StatusBadRequest)
	default:
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}


func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Received request: %s %s", r.Method, r.URL)
		next.ServeHTTP(w, r)
	})
}


func (s *ExecutionService) ExecuteCode(req *ExecutionRequest) (*ExecutionResponse, error) {
	containerName, ok := s.containers[req.Language]
	if !ok {
		return nil, ErrLanguageNotSupported
	}

	
	if req.Method != "docker" {
		return nil, ErrMethodNotSupported
	}

	
	output, err := executeLanguageCode(containerName, req.Language, req.Code)
	if err != nil {
		return &ExecutionResponse{
			Error:         err.Error(),
			StatusMessage: "Runtime Error",
		}, nil
	}

	
	return &ExecutionResponse{
		Output:        output,
		StatusMessage: "Accepted",
	}, nil
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
