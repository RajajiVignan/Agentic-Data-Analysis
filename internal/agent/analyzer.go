package agent

import (
	"context"
	"insightpilot/internal/data"
)

// AnalysisRequest is the input to any analyzer implementation.
type AnalysisRequest struct {
	Prompt     string
	Datasets   []*data.Dataset
	TimeoutSec int
}

// AnalysisResponse is the structured output from an analyzer.
// It matches the contract described in Instructions/Agentic_AI.md.
type AnalysisResponse struct {
	Question         string                   `json:"question"`
	Dataset          DatasetSummary           `json:"dataset"`
	Notebook         []NotebookStep           `json:"notebook"`
	Dashboard        DashboardSpec            `json:"dashboard"`
	Assumptions      []string                 `json:"assumptions"`
	Warnings         []string                 `json:"warnings"`
	UsedDeterministic bool                    `json:"used_deterministic"`
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

// DashboardSpec contains the chart-ready output.
type DashboardSpec struct {
	Title           string                   `json:"title"`
	KPIs            []map[string]string      `json:"kpis"`
	Trend           []map[string]interface{} `json:"trend"`
	Segments        []map[string]interface{} `json:"segments"`
	Recommendations []string                 `json:"recommendations"`
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
	NVIDIAAPIKey    string
	NVIDIABaseURL   string
	TimeoutSec      int
	FallbackOnError bool
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		Enabled:         false,
		TimeoutSec:      30,
		FallbackOnError: true,
	}
}
