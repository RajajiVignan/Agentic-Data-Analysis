package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// LLMAnalyzer implements Analyzer by calling an NVIDIA NIM endpoint
// (OpenAI-compatible chat completions API). It falls back to the
// deterministic analyzer on any error if configured to do so.
type LLMAnalyzer struct {
	config    Config
	client    *http.Client
	deterministic *DeterministicAnalyzer
	guard     *Guard
}

// NewLLMAnalyzer creates an LLM-backed analyzer.
// If the API key is empty, it will always fall back to deterministic.
func NewLLMAnalyzer(cfg Config) *LLMAnalyzer {
	timeout := cfg.TimeoutSec
	if timeout <= 0 {
		timeout = 120
	}
	return &LLMAnalyzer{
		config: cfg,
		client: &http.Client{
			Timeout: time.Duration(timeout) * time.Second,
		},
		deterministic: NewDeterministicAnalyzer(),
		guard:         NewGuard(),
	}
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
		// Attach warnings about the violations
		llmResp.Warnings = append(llmResp.Warnings, violations...)
	}

	llmResp.UsedDeterministic = false
	return llmResp, nil
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

func (a *LLMAnalyzer) fallback(ctx context.Context, req AnalysisRequest, reason string) (AnalysisResponse, error) {
	resp, err := a.deterministic.Analyze(ctx, req)
	if err != nil {
		return AnalysisResponse{}, err
	}
	resp.Warnings = append(resp.Warnings, reason)
	return resp, nil
}

func (a *LLMAnalyzer) callLLM(ctx context.Context, req AnalysisRequest) (AnalysisResponse, error) {
	systemPrompt := `You are InsightPilot, an AI data analyst. Analyze the user's data and question. 
Respond with ONLY valid JSON matching this exact shape:
{
  "question": "the user's question",
  "dataset": {"id": "...", "filename": "...", "rowCount": N},
  "notebook": [{"title": "...", "body": "..."}],
  "dashboard": {
    "title": "Insights Board",
    "kpis": [{"label": "...", "value": "...", "change": "..."}],
    "trend": [{"label": "...", "value": N}],
    "segments": [{"label": "...", "value": N}],
    "recommendations": ["..."]
  },
  "assumptions": ["..."],
  "warnings": ["..."]
}
Do not include markdown, code fences, or explanations outside the JSON.`

	userPrompt := a.buildUserPrompt(req)

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

	var result AnalysisResponse
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return AnalysisResponse{}, fmt.Errorf("parse LLM JSON output: %w", err)
	}

	return result, nil
}

func (a *LLMAnalyzer) buildUserPrompt(req AnalysisRequest) string {
	var buf bytes.Buffer

	buf.WriteString("User question: ")
	buf.WriteString(SanitizeForPrompt(req.Prompt, 500))
	buf.WriteString("\n\n")

	for _, ds := range req.Datasets {
		buf.WriteString(fmt.Sprintf("Dataset: %s (ID: %s, %d rows)\n", ds.Filename, ds.ID, ds.Profile.RowCount))
		buf.WriteString("Columns:\n")
		for _, col := range ds.Profile.Columns {
			buf.WriteString(fmt.Sprintf("  - %s (type: %s, non-empty: %d)\n", col.Name, col.Type, col.NonEmpty))
		}
		buf.WriteString("Sample rows (up to 5):\n")
		limit := 5
		if len(ds.Rows) < 5 {
			limit = len(ds.Rows)
		}
		for _, row := range ds.Rows[:limit] {
			rowJSON, _ := json.Marshal(row)
			buf.WriteString(fmt.Sprintf("  %s\n", rowJSON))
		}
		buf.WriteString("\n")
	}

	return buf.String()
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
