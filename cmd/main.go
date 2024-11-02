package main

import (
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

	r.HandleFunc("/execute", service.HandleExecute).Methods("POST")
	r.PathPrefix("/swagger/").Handler(http.StripPrefix("/swagger/", http.FileServer(http.Dir("./"))))

	port := os.Getenv("PORT")
	if port == "" {
		port = DefaultPort
	}

	log.Printf("Starting server on port %s...", port)
	if err := http.ListenAndServe(":"+port, r); err != nil {
		log.Fatalf("Could not start server: %v", err)
	}
}
