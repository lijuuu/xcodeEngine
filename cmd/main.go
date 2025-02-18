package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"rce-service/pkg"

	"github.com/gorilla/mux"
)

const DefaultPort = "8000"

func main() {
	service := pkg.NewExecutionService()
	r := mux.NewRouter()

	// Middleware - logs each request and increments Prometheus counter
	r.Use(pkg.LoggingMiddleware)

	// Root route ("/") responds with a simple message
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Set content type header and send JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{
			"message": "Hey there, checkout https://github.com/lijuu/SandboxedCodeExecution",
		})
	})

	// POST route "/execute" to handle the execution request
	r.HandleFunc("/execute", service.HandleExecute).Methods("POST")

	// Get the port from the environment variable, or use the default
	port := os.Getenv("PORT")
	if port == "" {
		port = DefaultPort
	}

	// Log that the server is starting and listen for requests
	log.Printf("Starting server on port %s...", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Could not start server: %v", err)
	}
}
