package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"insightpilot/internal/data"
)

// AnalysisRequest is the input to any analyzer implementation.
type AnalysisRequest struct {
	Prompt     string
	Datasets   []*data.Dataset
	TimeoutSec int
	SessionID  string                `json:"sessionId,omitempty"`
	History    []ConversationTurn    `json:"history,omitempty"`
	Context    *ConversationContext  `json:"context,omitempty"`
}

// AnalysisResponse is the structured output from an analyzer.
// LLM analyzers produce a Plan (instructions); the handler fills
// in Dashboard from local execution. Deterministic analyzers
// fill Dashboard directly.
type AnalysisResponse struct {
	Question          string                   `json:"question"`
	Dataset           DatasetSummary           `json:"dataset"`
	Notebook          []NotebookStep           `json:"notebook"`
	Plan              *LLMPlan                 `json:"plan,omitempty"`
	Dashboard         DashboardSpec            `json:"dashboard"`
	Assumptions       []string                 `json:"assumptions"`
	Warnings          []string                 `json:"warnings"`
	UsedDeterministic bool                     `json:"used_deterministic"`
	SQLQueries        []string                 `json:"sqlQueries,omitempty"`
	SessionID         string                   `json:"sessionId,omitempty"`
	Context           *ConversationContext     `json:"context,omitempty"`
}

// DatasetSummary describes the primary dataset used in analysis.
type DatasetSummary struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	RowCount int    `json:"rowCount"`
}

// NotebookStep is a single step in the agent's reasoning trace.
type NotebookStep struct {
	Title string `json:"title"`
	Body  string `json:"body,omitempty"`
	Code  string `json:"code,omitempty"`
}

// LLMPlan contains the structured instructions the LLM returns
// after inspecting dataset metadata. The backend executes this
// plan locally — raw data never leaves the server.
type LLMPlan struct {
	MetricColumn    string         `json:"metricColumn"`
	CategoryColumn  string         `json:"categoryColumn"`
	DateColumn      string         `json:"dateColumn"`
	Aggregation     string         `json:"aggregation"`   // "sum", "avg", "count", "min", "max"
	ChartTypes      []string       `json:"chartTypes"`     // e.g. ["bar", "line", "pie"]
	GroupBy         string         `json:"groupBy"`
	Title           string         `json:"title"`
	Recommendations []string       `json:"recommendations"`
	Assumptions     []string       `json:"assumptions"`
	Reasoning       string         `json:"reasoning"`
	Filters         []FilterClause `json:"filters,omitempty"`
	Narrative       string         `json:"narrative,omitempty"`
}

// UnmarshalJSON handles the LLM returning assumptions or recommendations as a
// single string instead of an array of strings.
func (p *LLMPlan) UnmarshalJSON(data []byte) error {
	type alias LLMPlan
	raw := struct {
		Assumptions     json.RawMessage `json:"assumptions"`
		Recommendations json.RawMessage `json:"recommendations"`
		*alias
	}{alias: (*alias)(p)}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	if raw.Assumptions != nil {
		if err := json.Unmarshal(raw.Assumptions, &p.Assumptions); err != nil {
			var s string
			if err2 := json.Unmarshal(raw.Assumptions, &s); err2 != nil || s == "" {
				return fmt.Errorf("assumptions: %w (also tried string: %v)", err, err2)
			}
			p.Assumptions = []string{s}
		}
	}
	if raw.Recommendations != nil {
		if err := json.Unmarshal(raw.Recommendations, &p.Recommendations); err != nil {
			var s string
			if err2 := json.Unmarshal(raw.Recommendations, &s); err2 != nil || s == "" {
				return fmt.Errorf("recommendations: %w (also tried string: %v)", err, err2)
			}
			p.Recommendations = []string{s}
		}
	}
	return nil
}

// DashboardSpec contains the chart-ready output.
type DashboardSpec struct {
	Title           string                   `json:"title"`
	KPIs            []map[string]string      `json:"kpis"`
	Trend           []map[string]interface{} `json:"trend"`
	Segments        []map[string]interface{} `json:"segments"`
	Recommendations []string                 `json:"recommendations"`
	Narrative       string                   `json:"narrative,omitempty"`
}

// Analyzer is the interface that all analysis implementations must satisfy.
// The deterministic analyzer is always available as a fallback.
// The LLM analyzer is used when an API key is configured.
type Analyzer interface {
	Analyze(ctx context.Context, req AnalysisRequest) (AnalysisResponse, error)
}

// Config holds configuration for the agent layer.
type Config struct {
	Enabled         bool
	APIKey          string
	BaseURL         string
	Model           string
	MaxTokens       int
	Temperature     float64
	TimeoutSec      int
	FallbackOnError bool
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:         false,
		Model:           "stepfun-ai/step-3.7-flash",
		MaxTokens:       16384,
		Temperature:     1.0,
		TimeoutSec:      120,
		FallbackOnError: true,
	}
}

// MetadataFromDatasets extracts privacy-safe metadata from datasets.
func MetadataFromDatasets(datasets []*data.Dataset) []data.DatasetMetadata {
	var metas []data.DatasetMetadata
	for _, ds := range datasets {
		metas = append(metas, data.ComputeMetadata(ds, 20))
	}
	return metas
}
