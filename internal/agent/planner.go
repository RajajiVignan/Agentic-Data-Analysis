package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"insightpilot/internal/data"
)

// PlannerAgent creates focused analysis plans from dataset metadata.
// It uses an LLM call to decide which columns to analyze, what aggregations
// to apply, and what chart types to generate — without executing any tools.
type PlannerAgent struct {
	config Config
	client *http.Client
}

// NewPlannerAgent creates a PlannerAgent with the given config.
func NewPlannerAgent(cfg Config) *PlannerAgent {
	timeout := cfg.TimeoutSec
	if timeout <= 0 {
		timeout = 60
	}
	return &PlannerAgent{
		config: cfg,
		client: &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}
}

// Plan sends dataset metadata + user question to the LLM and returns
// a column specification (metric, category, date, aggregation, chart types, filters).
func (p *PlannerAgent) Plan(ctx context.Context, prompt string, metas []data.DatasetMetadata, history []ConversationTurn) (*LLMPlan, []NotebookStep, error) {
	metasJSON, _ := json.Marshal(metas)
	isFollowUp := len(history) > 0

	systemPrompt := `You are InsightPilot's Planner Agent. Your role is to choose which columns to analyze.

Given dataset metadata (schema + statistics — NO raw rows) and a user question, produce a focused analysis plan.

Respond with ONLY valid JSON matching this exact shape — no markdown, no code fences, no extra text:

{
  "metricColumn": "column_name or empty string",
  "categoryColumn": "column_name or empty string",
  "dateColumn": "column_name or empty string",
  "aggregation": "sum",
  "chartTypes": ["bar"],
  "title": "short analysis title",
  "filters": [{"column": "col", "operator": "eq", "value": "val"}],
  "reasoning": "brief explanation of column choices"
}

Rules:
- metricColumn MUST be a numeric column name from the metadata (use "" if none)
- categoryColumn MUST be a text column name or ""
- dateColumn MUST be a date column name or ""
- aggregation: one of sum, avg, count, min, max
- chartTypes: subset of bar, line, pie, scatter, histogram
- filters is OPTIONAL — only include if the question specifies a filter
- If you cannot determine the metric, use ""
- Base column choices on column names, types, and statistics from the metadata
- IMPORTANT: Prefer columns whose names semantically match the user's question`

	if isFollowUp {
		systemPrompt += "\n\nNOTE: This is a FOLLOW-UP question. Consider the conversation history for context. If the user drills down or applies filters, reflect that in your plan."
	}

	userContent := fmt.Sprintf("Dataset metadata:\n%s\n\nUser question: %s", string(metasJSON), SanitizeForPrompt(prompt, 500))

	messages := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
	}
	if isFollowUp {
		for _, turn := range history {
			messages = append(messages,
				map[string]interface{}{"role": "user", "content": SanitizeForPrompt(turn.Prompt, 300)},
				map[string]interface{}{"role": "assistant", "content": "I analyzed that query and returned KPIs, charts, and recommendations."},
			)
		}
	}
	messages = append(messages, map[string]interface{}{"role": "user", "content": userContent})

	resp, err := p.chatCompletion(ctx, messages)
	if err != nil {
		return nil, nil, fmt.Errorf("planner LLM call failed: %w", err)
	}

	content := stripMarkdownCodeFences(resp)
	log.Printf("[Planner] LLM response: %s", truncateStr(content, 500))

	var plan LLMPlan
	if err := json.Unmarshal([]byte(content), &plan); err != nil {
		return nil, nil, fmt.Errorf("planner parse JSON: %w (content: %s)", err, truncateStr(content, 200))
	}

	notebook := []NotebookStep{
		{
			Title: "Analysis Plan",
			Body:  fmt.Sprintf("Metric: %s | Category: %s | Date: %s | Aggregation: %s | Charts: %v", plan.MetricColumn, plan.CategoryColumn, plan.DateColumn, plan.Aggregation, plan.ChartTypes),
		},
	}
	if plan.Reasoning != "" {
		notebook = append(notebook, NotebookStep{Title: "Planner Reasoning", Body: plan.Reasoning})
	}

	log.Printf("[Planner] Plan produced: metric=%q category=%q date=%q agg=%q", plan.MetricColumn, plan.CategoryColumn, plan.DateColumn, plan.Aggregation)
	return &plan, notebook, nil
}

func (p *PlannerAgent) chatCompletion(ctx context.Context, messages []map[string]interface{}) (string, error) {
	model := p.config.Model
	if model == "" {
		model = "stepfun-ai/step-3.7-flash"
	}
	maxTokens := p.config.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	temperature := p.config.Temperature
	if temperature == 0 {
		temperature = 0.3
	}

	payload := map[string]interface{}{
		"model":       model,
		"messages":    messages,
		"max_tokens":  maxTokens,
		"temperature": temperature,
		"stream":      false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+normalizeBearer(p.config.APIKey))

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read LLM response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("LLM returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse LLM response: %w", err)
	}

	if len(result.Choices) == 0 {
		return "", fmt.Errorf("LLM returned no choices")
	}

	return result.Choices[0].Message.Content, nil
}

// SanitizeForPrompt is already defined in guardrails.go — no redefinition needed.
