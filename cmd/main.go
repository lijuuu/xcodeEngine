package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"rce-service/pkg"

	"github.com/gorilla/mux"
)

const DefaultPort = "8080"

func main() {
	service := pkg.NewExecutionService()
	r := mux.NewRouter()

	r.Use(pkg.LoggingMiddleware)
	r.Use(service.RateLimiter.Limit)
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"message": "Hey there, checkout https://github.com/liju-github/SandboxedCodeExecution"})
	})

	r.HandleFunc("/execute", service.HandleExecute).Methods("POST")

	port := os.Getenv("PORT")
	if port == "" {
		port = DefaultPort
	}

	log.Printf("Starting server on port %s...", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Could not start server: %v", err)
	}
}
