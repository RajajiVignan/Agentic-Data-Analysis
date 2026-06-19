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
)

// ValidationResult is the structured output from the Validator agent.
// Valid=true means the plan and tool results are satisfactory.
// When Valid=false, Issues and Suggestions guide the refinement.
type ValidationResult struct {
	Valid       bool     `json:"valid"`
	Issues      []string `json:"issues,omitempty"`
	Suggestions []string `json:"suggestions,omitempty"`
}

// ValidatorAgent checks tool results against the analysis plan and
// determines whether the plan is sound or needs refinement.
type ValidatorAgent struct {
	config Config
	client *http.Client
}

// NewValidatorAgent creates a ValidatorAgent with the given config.
func NewValidatorAgent(cfg Config) *ValidatorAgent {
	timeout := cfg.TimeoutSec
	if timeout <= 0 {
		timeout = 60
	}
	return &ValidatorAgent{
		config: cfg,
		client: &http.Client{Timeout: time.Duration(timeout) * time.Second},
	}
}

// Validate sends the plan and tool results to the LLM to assess quality.
func (v *ValidatorAgent) Validate(ctx context.Context, plan *LLMPlan, toolResults map[string]interface{}, prompt string) (*ValidationResult, error) {
	planJSON, _ := json.Marshal(plan)
	resultsJSON, _ := json.Marshal(toolResults)

	systemPrompt := `You are InsightPilot's Validator Agent. Your job is to check whether an analysis plan and its tool results are sound.

The tool results contain actual aggregations, group-bys, and trends computed from the data.

Check each of these and respond with valid JSON only:

1. METRIC: Does the metric have sufficient non-zero data? (more than 0 rows with values)
2. TREND: If a date column was selected, does the trend have at least 2 time periods?
3. CATEGORY: If a category column was selected, are there at least 2 distinct groups?
4. CHARTS: Are the chart types appropriate for the columns?
   - bar: good for comparing categories
   - line: good for trends over time
   - pie: good for parts-of-a-whole (few segments)
   - scatter: good for correlation between two metrics
   - histogram: good for distribution of one metric
5. CONSISTENCY: Do the column names in the plan actually appear in the tool results?

If everything is fine, set "valid": true.
If there are issues, set "valid": false and include brief issues and specific, actionable suggestions.

IMPORTANT: Return ONLY valid JSON, no markdown, no code fences, no extra text.

{
  "valid": true,
  "issues": ["issue description if any"],
  "suggestions": ["actionable suggestion if any"]
}`

	userContent := fmt.Sprintf("User question: %s\n\nAnalysis plan:\n%s\n\nTool results:\n%s",
		SanitizeForPrompt(prompt, 300), string(planJSON), truncateStr(string(resultsJSON), 3000))

	messages := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
		{"role": "user", "content": userContent},
	}

	resp, err := v.chatCompletion(ctx, messages)
	if err != nil {
		log.Printf("[Validator] LLM call failed, assuming valid: %v", err)
		return &ValidationResult{Valid: true}, nil
	}

	content := stripMarkdownCodeFences(resp)
	log.Printf("[Validator] Response: %s", truncateStr(content, 500))

	var result ValidationResult
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		log.Printf("[Validator] Parse error: %v, assuming valid", err)
		return &ValidationResult{Valid: true}, nil
	}

	if result.Valid {
		log.Printf("[Validator] Plan validated successfully")
	} else {
		log.Printf("[Validator] Plan rejected: issues=%v suggestions=%v", result.Issues, result.Suggestions)
	}

	return &result, nil
}

func (v *ValidatorAgent) chatCompletion(ctx context.Context, messages []map[string]interface{}) (string, error) {
	model := v.config.Model
	if model == "" {
		model = "stepfun-ai/step-3.7-flash"
	}
	maxTokens := v.config.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 2048
	}
	temperature := v.config.Temperature
	if temperature == 0 {
		temperature = 0.2
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

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, v.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+normalizeBearer(v.config.APIKey))

	resp, err := v.client.Do(httpReq)
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
