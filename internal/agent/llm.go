package agent

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// LLMAnalyzer implements Analyzer by calling an LLM endpoint
// (OpenAI-compatible chat completions API). It sends only dataset
// metadata (schema + statistics) to the LLM — never raw row data.
//
// The analyzer uses a feedback-driven multi-agent loop:
//  1. PlannerAgent generates an analysis plan (column selection, aggregation, chart types)
//  2. Tools execute the plan deterministically against the data
//  3. ValidatorAgent checks results for quality and completeness
//  4. If validation fails, Planner refines the plan (up to 3 iterations)
//  5. Handler executes the final plan locally via execPlan
type LLMAnalyzer struct {
	config        Config
	client        *http.Client
	deterministic *DeterministicAnalyzer
	guard         *Guard
	tools         map[string]Tool
	planner       *PlannerAgent
	validator     *ValidatorAgent
	feedbackLoop  *FeedbackLoop
}

// NewLLMAnalyzer creates an LLM-backed analyzer.
// If the API key is empty, it will always fall back to deterministic.
func NewLLMAnalyzer(cfg Config) *LLMAnalyzer {
	timeout := cfg.TimeoutSec
	if timeout <= 0 {
		timeout = 120
	}
	tools := DefaultTools()
	toolMap := make(map[string]Tool, len(tools))
	for _, t := range tools {
		toolMap[t.Name()] = t
	}

	planner := NewPlannerAgent(cfg)
	validator := NewValidatorAgent(cfg)
	feedbackLoop := NewFeedbackLoop(planner, validator, toolMap)

	return &LLMAnalyzer{
		config:        cfg,
		client:        &http.Client{Timeout: time.Duration(timeout) * time.Second},
		deterministic: NewDeterministicAnalyzer(),
		guard:         NewGuard(),
		tools:         toolMap,
		planner:       planner,
		validator:     validator,
		feedbackLoop:  feedbackLoop,
	}
}

// Analyze tries the LLM first. If the LLM is not configured or fails,
// it falls back to the deterministic analyzer.
func (a *LLMAnalyzer) Analyze(ctx context.Context, req AnalysisRequest) (AnalysisResponse, error) {
	if a.config.APIKey == "" || a.config.BaseURL == "" {
		return a.fallback(ctx, req, "LLM not configured, using deterministic analyzer")
	}

	metas := MetadataFromDatasets(req.Datasets)

	llmResp, err := a.feedbackLoop.Execute(ctx, req, metas)
	if err != nil {
		log.Printf("[LLM] Feedback loop failed: %v", err)
		if a.config.FallbackOnError {
			return a.fallback(ctx, req, fmt.Sprintf("LLM error: %v, falling back", err))
		}
		return AnalysisResponse{}, err
	}

	violations := a.guard.ValidateResponse(llmResp)
	if len(violations) > 0 {
		log.Printf("[LLM] Guardrail violations: %v", violations)
		if a.config.FallbackOnError {
			return a.fallback(ctx, req, fmt.Sprintf("guardrail violations: %v, falling back", violations))
		}
		llmResp.Warnings = append(llmResp.Warnings, violations...)
	}

	if llmResp.Plan != nil {
		log.Printf("[LLM] Returning plan: metric=%q category=%q date=%q agg=%q",
			llmResp.Plan.MetricColumn, llmResp.Plan.CategoryColumn, llmResp.Plan.DateColumn, llmResp.Plan.Aggregation)
	}

	llmResp.UsedDeterministic = false
	return llmResp, nil
}

func (a *LLMAnalyzer) fallback(ctx context.Context, req AnalysisRequest, reason string) (AnalysisResponse, error) {
	resp, err := a.deterministic.Analyze(ctx, req)
	if err != nil {
		return AnalysisResponse{}, err
	}
	resp.Warnings = append(resp.Warnings, reason)
	return resp, nil
}

// --- Helpers ---

func normalizeBearer(key string) string {
	key = strings.TrimSpace(key)
	key = strings.Trim(key, "\"")
	key = strings.Trim(key, "'")
	key = strings.TrimPrefix(key, "Bearer ")
	key = strings.TrimPrefix(key, "bearer ")
	return key
}

func stripMarkdownCodeFences(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "```") {
		if idx := strings.Index(s[3:], "\n"); idx >= 0 {
			s = s[3+idx+1:]
		}
		s = strings.TrimSuffix(s, "```")
		s = strings.TrimSpace(s)
	}
	return s
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
