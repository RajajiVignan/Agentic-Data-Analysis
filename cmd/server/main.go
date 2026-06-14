package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

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

	// Start server in a goroutine
	go func() {
		log.Printf("InsightPilot running at http://%s:%s", host, port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("Received signal %v, shutting down...", sig)

	// Create a context with timeout for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	// Shutdown handler (stop schedulers, close DB, clean up Python processes)
	handler.Shutdown()

	// Shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP server shutdown error: %v", err)
	}

	log.Println("Server stopped gracefully")
}
