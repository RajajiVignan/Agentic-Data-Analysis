package main

import (
	"log"
	"net/http"
	"os"

	"insightpilot/internal/agent"
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

	// Configure the agent layer
	agentCfg := agent.Config{
		Enabled:         os.Getenv("NVIDIA_API_KEY") != "" && os.Getenv("NVIDIA_API_KEY") != "YOUR_NVIDIA_API_KEY",
		NVIDIAAPIKey:    os.Getenv("NVIDIA_API_KEY"),
		NVIDIABaseURL:   os.Getenv("NVIDIA_BASE_URL"),
		TimeoutSec:      30,
		FallbackOnError: true,
	}

	if agentCfg.Enabled {
		log.Println("LLM analyzer enabled (NVIDIA API key configured)")
	} else {
		log.Println("LLM analyzer disabled, using deterministic analyzer")
	}

	// Initialize API handlers
	handler := api.NewHandler(agentCfg)

	// Create HTTP server
	server := &http.Server{
		Addr:    host + ":" + port,
		Handler: handler.Routes(),
	}

	log.Printf("InsightPilot running at http://%s:%s", host, port)
	log.Fatal(server.ListenAndServe())
}
