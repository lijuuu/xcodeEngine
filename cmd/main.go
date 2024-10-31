
package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os/exec"
	"runtime"
	"time"

	"github.com/gorilla/mux"
)

type CodeRequest struct {
	Language string `json:"language"`
	Code     string `json:"code"`
}

type CodeResponse struct {
	Output            string  `json:"output"`
	Error             string  `json:"error,omitempty"`
	ExecutionTime     float64 `json:"execution_time"`      
	MemoryUsage       uint64  `json:"memory_usage"`        
	ExecutionTimeUnit string  `json:"execution_time_unit"` 
	MemoryUsageUnit   string  `json:"memory_usage_unit"`   
}

func executeInContainer(language, code string) (CodeResponse, error) {
	containerName := "executor-" + language + "-" + time.Now().Format("20060102150405")

	var cmd *exec.Cmd
	switch language {
	case "python":
		cmd = exec.Command("docker", "run", "--rm", "--name", containerName,
			"-e", "CODE="+code,
			"python-executor:latest",
			"sh", "-c", "python3 -c \"$CODE\"")
	case "go":
		cmd = exec.Command("docker", "run", "--rm", "--name", containerName,
			"-e", "CODE="+code,
			"go-executor:latest",
			"sh", "-c", "echo \"$CODE\" > /tmp/main.go && go run /tmp/main.go")
	default:
		return CodeResponse{}, errors.New("unsupported language")
	}

	
	startTime := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd = exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...)

	output, err := cmd.CombinedOutput()
	elapsedTime := time.Since(startTime).Seconds() 

	
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)

	response := CodeResponse{
		Output:            string(output),
		ExecutionTime:     elapsedTime,
		MemoryUsage:       memStats.Alloc,
		ExecutionTimeUnit: "seconds",
		MemoryUsageUnit:   "bytes",
	}

	if err != nil {
		response.Error = err.Error()
	}

	return response, nil
}

func handleExecute(w http.ResponseWriter, r *http.Request) {
	var req CodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	response, err := executeInContainer(req.Language, req.Code)
	if err != nil {
		response.Error = err.Error()
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func main() {
	r := mux.NewRouter()
	r.HandleFunc("/execute", handleExecute).Methods("POST")

	log.Println("Server starting on :8080")
	http.ListenAndServe(":8080", r)
}
