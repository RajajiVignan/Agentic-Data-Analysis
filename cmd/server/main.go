package main

import (
	"context"
	"log/slog"
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
	if err := godotenv.Load(); err != nil {
		slog.Warn("Error loading .env file", "error", err)
	}

	initLogger()

	validateEnv()

	port := getEnv("PORT", "3000")
	host := getEnv("HOST", "127.0.0.1")

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
		slog.Info("LLM analyzer enabled", "provider", "OpenRouter")
	} else {
		slog.Info("LLM analyzer disabled, using deterministic analyzer")
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
		slog.Info("Server starting", "addr", "http://"+host+":"+port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("HTTP server error", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	slog.Info("Shutdown signal received", "signal", sig)

	// Create a context with timeout for shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	// Shutdown handler (stop schedulers, close DB, clean up Python processes)
	handler.Shutdown()

	// Shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server shutdown error", "error", err)
	}

	slog.Info("Server stopped gracefully")
}

func initLogger() {
	level := slog.LevelInfo
	if v := strings.ToLower(os.Getenv("LOG_LEVEL")); v != "" {
		switch v {
		case "debug":
			level = slog.LevelDebug
		case "warn":
			level = slog.LevelWarn
		case "error":
			level = slog.LevelError
		}
	}

	var handler slog.Handler
	if os.Getenv("LOG_FORMAT") == "json" || isProductionEnv() {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	} else {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	}
	slog.SetDefault(slog.New(handler))
}

func validateEnv() {
	missing := false
	optional := map[string]string{
		"SUPABASE_URL":    "Database URL for Supabase",
		"SUPABASE_KEY":    "Database key for Supabase",
		"SMTP_HOST":       "SMTP host for email delivery (reports/alerts)",
		"SMTP_USER":       "SMTP user for email delivery",
		"SMTP_PASSWORD":   "SMTP password for email delivery",
		"OPENROUTER_API_KEY": "API key for LLM-powered analysis",
	}

	for envVar, description := range optional {
		if os.Getenv(envVar) == "" {
			slog.Warn("Missing optional environment variable", "var", envVar, "description", description)
			missing = true
		}
	}

	_ = missing // informational only; app will run with degraded functionality
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func isProductionEnv() bool {
	for _, name := range []string{"APP_ENV", "GO_ENV", "NODE_ENV"} {
		if strings.EqualFold(os.Getenv(name), "production") {
			return true
		}
	}
	return false
}
