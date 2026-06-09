package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// LLMAnalyzer implements Analyzer by calling an NVIDIA NIM endpoint
// (OpenAI-compatible chat completions API). It sends only dataset
// metadata (schema + statistics) to the LLM — never raw row data.
// The LLM returns a structured plan describing which columns to use,
// what aggregations to compute, and what chart types to generate.
// The handler executes this plan locally against the full dataset.
type LLMAnalyzer struct {
	config        Config
	client        *http.Client
	deterministic *DeterministicAnalyzer
	guard         *Guard
}

// NewLLMAnalyzer creates an LLM-backed analyzer.
// If the API key is empty, it will always fall back to deterministic.
func NewLLMAnalyzer(cfg Config) *LLMAnalyzer {
	timeout := cfg.TimeoutSec
	if timeout <= 0 {
		timeout = 120
	}
	llm := &LLMAnalyzer{
		config:        cfg,
		client:        &http.Client{},
		deterministic: NewDeterministicAnalyzer(),
		guard:         NewGuard(),
	}
	return llm
}

// Analyze tries the LLM first. If the LLM is not configured or fails,
// it falls back to the deterministic analyzer.
func (a *LLMAnalyzer) Analyze(ctx context.Context, req AnalysisRequest) (AnalysisResponse, error) {
	if a.config.NVIDIAAPIKey == "" || a.config.NVIDIABaseURL == "" {
		return a.fallback(ctx, req, "LLM not configured, using deterministic analyzer")
	}

	llmResp, err := a.callLLM(ctx, req)
	if err != nil {
		if a.config.FallbackOnError {
			return a.fallback(ctx, req, fmt.Sprintf("LLM error: %v, falling back", err))
		}
		return AnalysisResponse{}, err
	}

	violations := a.guard.ValidateResponse(llmResp)
	if len(violations) > 0 {
		if a.config.FallbackOnError {
			return a.fallback(ctx, req, fmt.Sprintf("guardrail violations: %v, falling back", violations))
		}
		llmResp.Warnings = append(llmResp.Warnings, violations...)
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

func (a *LLMAnalyzer) callLLM(ctx context.Context, req AnalysisRequest) (AnalysisResponse, error) {
	systemPrompt := `You are InsightPilot, an AI data analyst. You receive dataset metadata (schema and statistics) but NEVER raw row data.

Based on the user's question and the dataset metadata, decide:
1. Which numeric column is the primary metric
2. Which categorical column to segment/group by
3. Which date column to trend over (if any)
4. What aggregation to apply (sum, avg, count, min, max)
5. What chart types best answer the question (bar, line, pie, scatter, histogram)

Respond with ONLY valid JSON matching this exact shape:
{
  "metricColumn": "column_name",
  "categoryColumn": "column_name or empty string",
  "dateColumn": "column_name or empty string",
  "aggregation": "sum",
  "chartTypes": ["bar", "line"],
  "groupBy": "column_name or empty string",
  "title": "short analysis title",
  "recommendations": ["actionable insight 1", "actionable insight 2"],
  "assumptions": ["assumption 1"],
  "reasoning": "brief explanation of column choices"
}

Rules:
- metricColumn MUST be a numeric column from the metadata
- categoryColumn MUST be a text column from the metadata (or "" if none suitable)
- dateColumn MUST be a date column from the metadata (or "" if none suitable)
- aggregation must be one of: sum, avg, count, min, max
- chartTypes must be from: bar, line, pie, scatter, histogram
- Do NOT include markdown, code fences, or explanations outside the JSON
- Do NOT compute any values — you only choose WHAT to compute, not the result`

	// Build metadata-only prompt (no raw rows)
	metas := MetadataFromDatasets(req.Datasets)
	metasJSON, _ := json.Marshal(metas)

	userPrompt := fmt.Sprintf("User question: %s\n\nDataset metadata (JSON):\n%s",
		SanitizeForPrompt(req.Prompt, 500), string(metasJSON))

	log.Printf("[LLM] Sending metadata for %d dataset(s), prompt: %q", len(metas), SanitizeForPrompt(req.Prompt, 100))
	log.Printf("[LLM] Metadata payload: %s", string(metasJSON)[:min(len(metasJSON), 2000)])

	messages := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userPrompt},
	}

	payload := map[string]interface{}{
		"model":       a.config.Model,
		"messages":    messages,
		"max_tokens":  a.config.MaxTokens,
		"temperature": a.config.Temperature,
		"stream":      false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return AnalysisResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.config.NVIDIABaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return AnalysisResponse{}, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+normalizeBearer(a.config.NVIDIAAPIKey))

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return AnalysisResponse{}, fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return AnalysisResponse{}, fmt.Errorf("read LLM response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return AnalysisResponse{}, fmt.Errorf("LLM returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var llmResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &llmResp); err != nil {
		return AnalysisResponse{}, fmt.Errorf("parse LLM response: %w", err)
	}

	if len(llmResp.Choices) == 0 {
		return AnalysisResponse{}, fmt.Errorf("LLM returned no choices")
	}

	content := llmResp.Choices[0].Message.Content
	content = stripMarkdownCodeFences(content)

	log.Printf("[LLM] Raw response: %s", content[:min(len(content), 1000)])

	var plan LLMPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return AnalysisResponse{}, fmt.Errorf("parse LLM plan JSON: %w (content: %s)", err, content[:min(len(content), 200)])
	}

	log.Printf("[LLM] Parsed plan: metric=%q category=%q date=%q agg=%q charts=%v title=%q",
		plan.MetricColumn, plan.CategoryColumn, plan.DateColumn, plan.Aggregation, plan.ChartTypes, plan.Title)

	// Build response with the plan — dashboard will be filled by handler
	primary := req.Datasets[0]
	response := AnalysisResponse{
		Question: req.Prompt,
		Dataset: DatasetSummary{
			ID:       primary.ID,
			Filename: primary.Filename,
			RowCount: primary.Profile.RowCount,
		},
		Plan: &plan,
		Notebook: []NotebookStep{
			{Title: "Data Profile", Body: fmt.Sprintf("Dataset %q has %d rows and %d columns. Analysis based on metadata only — no raw data sent to LLM.", primary.Filename, primary.Profile.RowCount, len(primary.Profile.Columns))},
			{Title: "LLM Plan", Body: fmt.Sprintf("Metric: %s, Category: %s, Date: %s, Aggregation: %s, Charts: %v", plan.MetricColumn, plan.CategoryColumn, plan.DateColumn, plan.Aggregation, plan.ChartTypes)},
			{Title: "Reasoning", Body: plan.Reasoning},
		},
		Assumptions: plan.Assumptions,
	}

	return response, nil
}

// normalizeBearer strips any "Bearer " prefix and surrounding quotes from the token.
func normalizeBearer(key string) string {
	key = strings.TrimSpace(key)
	key = strings.Trim(key, "\"")
	key = strings.Trim(key, "'")
	key = strings.TrimPrefix(key, "Bearer ")
	key = strings.TrimPrefix(key, "bearer ")
	return key
}

// stripMarkdownCodeFences removes ```json ... ``` or ``` ... ``` wrappers.
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
