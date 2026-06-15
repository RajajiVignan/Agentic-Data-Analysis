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
	"time"
)

// LLMAnalyzer implements Analyzer by calling an LLM endpoint
// (OpenAI-compatible chat completions API). It sends only dataset
// metadata (schema + statistics) to the LLM — never raw row data.
//
// The analyzer supports a multi-step tool-call loop: the LLM can request
// aggregations, group-bys, and trend computations via tool calls, inspect
// the results, and then produce a final structured plan describing which
// columns to use, what aggregations to compute, and what chart types to
// generate. The handler executes this plan locally against the full dataset.
type LLMAnalyzer struct {
	config        Config
	client        *http.Client
	deterministic *DeterministicAnalyzer
	guard         *Guard
	tools         map[string]Tool
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
	return &LLMAnalyzer{
		config:        cfg,
		client:        &http.Client{Timeout: time.Duration(timeout) * time.Second},
		deterministic: NewDeterministicAnalyzer(),
		guard:         NewGuard(),
		tools:         toolMap,
	}
}

// Analyze tries the LLM first. If the LLM is not configured or fails,
// it falls back to the deterministic analyzer.
func (a *LLMAnalyzer) Analyze(ctx context.Context, req AnalysisRequest) (AnalysisResponse, error) {
	if a.config.APIKey == "" || a.config.BaseURL == "" {
		return a.fallback(ctx, req, "LLM not configured, using deterministic analyzer")
	}

	llmResp, err := a.runToolLoop(ctx, req)
	if err != nil {
		log.Printf("[LLM] runToolLoop failed: %v", err)
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

// --- Tool-call loop ---

const maxToolIterations = 8

// runToolLoop executes the multi-step tool-call conversation with the LLM.
// It starts with the system prompt + user question, then processes tool
// calls iteratively until the LLM produces a final JSON plan.
func (a *LLMAnalyzer) runToolLoop(ctx context.Context, req AnalysisRequest) (AnalysisResponse, error) {
	primary := req.Datasets[0]

	// Build tool definitions for the OpenAI tool-calling API
	toolDefs := make([]map[string]interface{}, 0, len(a.tools))
	for _, t := range a.tools {
		toolDefs = append(toolDefs, map[string]interface{}{
			"type": "function",
			"function": map[string]interface{}{
				"name":        t.Name(),
				"description": t.Description(),
				"parameters":  toolParams(t.Name()),
			},
		})
	}

	isFollowUp := len(req.History) > 0

	systemPrompt := `You are InsightPilot, an AI data analyst. You receive dataset metadata (schema and statistics) but NEVER raw row data.

You have access to tools that can compute aggregations, group data, and build trends on the server side. Use these tools to gather evidence before producing your final analysis.

MANDATORY workflow — you MUST call these tools before responding:
1. First, call get_dataset_profile to understand the available columns.
2. Call aggregate_metric with the user's target numeric column to get sum/avg/min/max.
3. If the user asks about categories or breakdowns, call group_by_dimension with a category column and the metric column.
4. If the user asks about time trends, call build_trend with a date column and the metric column.
5. When you have enough evidence, respond with ONLY valid JSON matching this exact shape (no tool calls, no markdown):

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
  "reasoning": "brief explanation of column choices and what the data showed",
  "filters": [{"column": "col_name", "operator": "eq", "value": "val"}],
  "narrative": "A 2-4 sentence written summary of the key insights in natural language, telling the story behind the data. Use KPI values, trends, and breakdowns."
}

Rules:
- metricColumn MUST be a numeric column from the metadata
- categoryColumn MUST be a text column from the metadata (or "" if none suitable)
- dateColumn MUST be a date column from the metadata (or "" if none suitable)
- aggregation must be one of: sum, avg, count, min, max
- chartTypes must be from: bar, line, pie, scatter, histogram
- filters is an OPTIONAL array of filter clauses. operator can be: eq, neq, gt, gte, lt, lte, contains, in
- Do NOT include markdown, code fences, or explanations outside the JSON
- Base your column choices on actual tool results, not guesses
- IMPORTANT: Always call aggregate_metric at least once with the metric column. Always call build_trend if there is a date column. Always call group_by_dimension if there is a category column.`

	if isFollowUp {
		systemPrompt += "\n\nNOTE: This is a FOLLOW-UP question. Use the conversation history provided below to understand context. If the user says 'filter by X', 'only show Y', 'drill down into Z', 'go back', or 'group by W', respond with the appropriate columns and filters."
	}

	// Build metadata-only prompt (no raw rows)
	metas := MetadataFromDatasets(req.Datasets)
	metasJSON, _ := json.Marshal(metas)

	log.Printf("[LLM] Starting tool-call loop for %d dataset(s), prompt: %q, history: %d turns",
		len(metas), SanitizeForPrompt(req.Prompt, 100), len(req.History))

	messages := []map[string]interface{}{
		{"role": "system", "content": systemPrompt},
	}

	// Insert conversation history as user/assistant message pairs (before current question)
	if isFollowUp {
		for _, turn := range req.History {
			messages = append(messages,
				map[string]interface{}{"role": "user", "content": SanitizeForPrompt(turn.Prompt, 300)},
				map[string]interface{}{"role": "assistant", "content": "I analyzed that query and returned KPIs, charts, and recommendations."},
			)
		}
	}

	// Current user question with dataset metadata
	userContent := fmt.Sprintf("User question: %s\n\nDataset metadata (JSON):\n%s",
		SanitizeForPrompt(req.Prompt, 500), string(metasJSON))
	messages = append(messages, map[string]interface{}{"role": "user", "content": userContent})

	notebook := []NotebookStep{
		{Title: "Data Profile", Body: fmt.Sprintf("Dataset %q has %d rows and %d columns. Analysis uses tool-call loop — no raw data sent to LLM.", primary.Filename, primary.Profile.RowCount, len(primary.Profile.Columns))},
	}

	for i := 0; i < maxToolIterations; i++ {
		resp, err := a.chatCompletion(ctx, messages, toolDefs)
		if err != nil {
			return AnalysisResponse{}, fmt.Errorf("LLM request failed at iteration %d: %w", i, err)
		}

		// Check if the response contains tool calls
		msg := resp.Choices[0].Message
		if len(msg.ToolCalls) > 0 {
			// Process each tool call
			for _, tc := range msg.ToolCalls {
				toolName := tc.Function.Name
				log.Printf("[toolCall] Iteration %d: calling %q with args: %s", i+1, toolName, truncateStr(tc.Function.Arguments, 200))

				tool, ok := a.tools[toolName]
				if ok {
					// Parse tool arguments
					var args map[string]interface{}
					if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
						log.Printf("[toolCall] Failed to parse args for %q: %v", toolName, err)
						args = map[string]interface{}{}
					}
					// Inject dataset reference
					args["dataset"] = primary

					result, execErr := tool.Execute(ctx, args)
					if execErr != nil {
						log.Printf("[toolCall] %q execution failed: %v", toolName, execErr)
						result = map[string]interface{}{"error": execErr.Error()}
					} else {
						log.Printf("[toolCall] %q succeeded: %s", toolName, truncateStr(mustJSON(result), 300))
					}

					// Add assistant message with tool call
					messages = append(messages, map[string]interface{}{
						"role": "assistant",
						"content": "",
						"tool_calls": []map[string]interface{}{
							{
								"id":   tc.ID,
								"type": "function",
								"function": map[string]interface{}{
									"name":      toolName,
									"arguments": tc.Function.Arguments,
								},
							},
						},
					})

					// Add tool result message
					resultJSON, _ := json.Marshal(result)
					messages = append(messages, map[string]interface{}{
						"role":         "tool",
						"tool_call_id": tc.ID,
						"content":      string(resultJSON),
					})

					notebook = append(notebook, NotebookStep{
						Title: fmt.Sprintf("Tool: %s", toolName),
						Body:  fmt.Sprintf("Args: %s\nResult: %s", truncateStr(tc.Function.Arguments, 100), truncateStr(string(resultJSON), 200)),
					})
				} else {
					log.Printf("[toolCall] Unknown tool %q requested by LLM", toolName)
					messages = append(messages, map[string]interface{}{
						"role":         "tool",
						"tool_call_id": tc.ID,
						"content":      fmt.Sprintf(`{"error": "unknown tool: %s"}`, toolName),
					})
				}
			}

			// After the first round of tool calls, remind the LLM to stop
			// calling tools and produce the final JSON plan.
			if i == 0 {
				messages = append(messages, map[string]interface{}{
					"role":    "user",
					"content": "You have gathered enough data. Now respond with ONLY the final JSON plan — no more tool calls. Use the exact JSON shape specified in the system prompt.",
				})
			}

			continue
		}

		// No tool calls — this is the final response
		content := stripMarkdownCodeFences(msg.Content)
		log.Printf("[LLM] Final response after %d iteration(s): %s", i+1, truncateStr(content, 500))

		var plan LLMPlan
		if err := json.Unmarshal([]byte(content), &plan); err != nil {
			return AnalysisResponse{}, fmt.Errorf("parse LLM plan JSON: %w (content: %s)", err, truncateStr(content, 200))
		}

		log.Printf("[LLM] Parsed plan: metric=%q category=%q date=%q agg=%q charts=%v title=%q",
			plan.MetricColumn, plan.CategoryColumn, plan.DateColumn, plan.Aggregation, plan.ChartTypes, plan.Title)

		notebook = append(notebook, NotebookStep{
			Title: "LLM Plan",
			Body:  fmt.Sprintf("Metric: %s, Category: %s, Date: %s, Aggregation: %s, Charts: %v", plan.MetricColumn, plan.CategoryColumn, plan.DateColumn, plan.Aggregation, plan.ChartTypes),
		})
		if plan.Reasoning != "" {
			notebook = append(notebook, NotebookStep{Title: "Reasoning", Body: plan.Reasoning})
		}

		return AnalysisResponse{
			Question: req.Prompt,
			Dataset: DatasetSummary{
				ID:       primary.ID,
				Filename: primary.Filename,
				RowCount: primary.Profile.RowCount,
			},
			Plan:        &plan,
			Notebook:    notebook,
			Assumptions: plan.Assumptions,
		}, nil
	}

	return AnalysisResponse{}, fmt.Errorf("tool-call loop exceeded %d iterations without producing a final plan", maxToolIterations)
}

// --- OpenAI-compatible chat completion ---

// chatCompletionResponse holds the parsed response from the LLM API.
type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			ToolCalls []struct {
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"message"`
	} `json:"choices"`
}

func (a *LLMAnalyzer) chatCompletion(ctx context.Context, messages []map[string]interface{}, tools []map[string]interface{}) (*chatCompletionResponse, error) {
	model := a.config.Model
	if model == "" {
		model = "stepfun-ai/step-3.7-flash"
	}
	maxTokens := a.config.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 4096
	}
	temperature := a.config.Temperature
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
	if len(tools) > 0 {
		payload["tools"] = tools
		payload["tool_choice"] = "auto"
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, a.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+normalizeBearer(a.config.APIKey))

	resp, err := a.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read LLM response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("LLM returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var result chatCompletionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("parse LLM response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("LLM returned no choices")
	}

	return &result, nil
}

// --- Tool parameter schemas ---
// These define the JSON schema for each tool's arguments (OpenAI function calling format).

func toolParams(toolName string) map[string]interface{} {
	switch toolName {
	case "get_dataset_profile":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"dataset": map[string]interface{}{
					"type":        "string",
					"description": "Dataset identifier (injected automatically)",
				},
			},
		}
	case "aggregate_metric":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"column": map[string]interface{}{
					"type":        "string",
					"description": "Name of the numeric column to aggregate",
				},
			},
			"required": []string{"column"},
		}
	case "group_by_dimension":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"category_column": map[string]interface{}{
					"type":        "string",
					"description": "Name of the categorical column to group by",
				},
				"metric_column": map[string]interface{}{
					"type":        "string",
					"description": "Name of the numeric column to sum (optional — defaults to count)",
				},
			},
			"required": []string{"category_column"},
		}
	case "build_trend":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"date_column": map[string]interface{}{
					"type":        "string",
					"description": "Name of the date column for the time axis",
				},
				"metric_column": map[string]interface{}{
					"type":        "string",
					"description": "Name of the numeric column to aggregate over time",
				},
			},
			"required": []string{"date_column", "metric_column"},
		}
	default:
		return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	}
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

func mustJSON(v interface{}) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
