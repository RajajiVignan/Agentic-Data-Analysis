package main

import (
	"log"
	"net/http"
	"os"

	"insightpilot/internal/api"

	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Printf("Warning: Error loading .env file: %v", err)
	}

	// Get configuration from environment
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}
	host := os.Getenv("HOST")
	if host == "" {
		host = "127.0.0.1"
	}

	// Initialize API handlers
	handler := api.NewHandler()

	// Create HTTP server
	server := &http.Server{
		Addr:    host + ":" + port,
		Handler: handler.Routes(),
	}

	log.Printf("InsightPilot running at http://%s:%s", host, port)
	log.Fatal(server.ListenAndServe())
}