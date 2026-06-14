package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

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
	agentCfg := agent.DefaultConfig()
	agentCfg.Enabled = os.Getenv("OPENROUTER_API_KEY") != "" && os.Getenv("OPENROUTER_API_KEY") != "YOUR_OPENROUTER_API_KEY"
	agentCfg.APIKey = os.Getenv("OPENROUTER_API_KEY")
	agentCfg.BaseURL = os.Getenv("OPENROUTER_BASE_URL")
	if v := strings.TrimSpace(os.Getenv("OPENROUTER_MODEL")); v != "" {
		agentCfg.Model = v
	}
	if v := strings.TrimSpace(os.Getenv("OPENROUTER_MAX_TOKENS")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			agentCfg.MaxTokens = n
		}
	}
	if v := strings.TrimSpace(os.Getenv("OPENROUTER_TEMPERATURE")); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			agentCfg.Temperature = f
		}
	}
	if v := strings.TrimSpace(os.Getenv("OPENROUTER_TIMEOUT_SEC")); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			agentCfg.TimeoutSec = n
		}
	}

	if agentCfg.Enabled {
		log.Println("LLM analyzer enabled (OpenRouter API key configured)")
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
