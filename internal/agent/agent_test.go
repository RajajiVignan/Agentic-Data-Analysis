package agent

import (
	"context"
	"testing"

	"insightpilot/internal/data"
)

func TestDeterministicAnalyzerReturnsValidResponse(t *testing.T) {
	ds := &data.Dataset{
		ID:       "test-1",
		Filename: "test.csv",
		Profile: data.Profile{
			RowCount: 2,
			Columns: []data.Column{
				{Name: "month", Type: "date"},
				{Name: "segment", Type: "text"},
				{Name: "revenue", Type: "number"},
			},
		},
		Rows: []map[string]string{
			{"month": "2026-01", "segment": "Enterprise", "revenue": "100"},
			{"month": "2026-01", "segment": "SMB", "revenue": "50"},
			{"month": "2026-02", "segment": "Enterprise", "revenue": "200"},
		},
	}

	analyzer := NewDeterministicAnalyzer()
	resp, err := analyzer.Analyze(context.Background(), AnalysisRequest{
		Prompt:   "What is total revenue by segment?",
		Datasets: []*data.Dataset{ds},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Question != "What is total revenue by segment?" {
		t.Fatalf("question = %q", resp.Question)
	}
	if resp.Dataset.ID != "test-1" {
		t.Fatalf("dataset ID = %q", resp.Dataset.ID)
	}
	if resp.Dataset.RowCount != 2 {
		t.Fatalf("row count = %d, want 2", resp.Dataset.RowCount)
	}
	if len(resp.Notebook) == 0 {
		t.Fatal("expected notebook steps")
	}
	if len(resp.Dashboard.KPIs) == 0 {
		t.Fatal("expected KPIs")
	}
	if len(resp.Dashboard.Segments) == 0 {
		t.Fatal("expected segments")
	}
	if len(resp.Dashboard.Recommendations) == 0 {
		t.Fatal("expected recommendations")
	}
	if len(resp.Assumptions) == 0 {
		t.Fatal("expected assumptions")
	}
	if !resp.UsedDeterministic {
		t.Fatal("expected UsedDeterministic to be true")
	}
}

func TestDeterministicAnalyzerNoNumericColumn(t *testing.T) {
	ds := &data.Dataset{
		ID:       "test-2",
		Filename: "text_only.csv",
		Profile: data.Profile{
			RowCount: 1,
			Columns: []data.Column{
				{Name: "name", Type: "text"},
				{Name: "city", Type: "text"},
			},
		},
		Rows: []map[string]string{
			{"name": "Alice", "city": "NYC"},
		},
	}

	analyzer := NewDeterministicAnalyzer()
	resp, err := analyzer.Analyze(context.Background(), AnalysisRequest{
		Prompt:   "analyze this",
		Datasets: []*data.Dataset{ds},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Warnings) == 0 {
		t.Fatal("expected warnings for missing numeric column")
	}
}

func TestDeterministicAnalyzerNoDateColumn(t *testing.T) {
	ds := &data.Dataset{
		ID:       "test-3",
		Filename: "no_date.csv",
		Profile: data.Profile{
			RowCount: 1,
			Columns: []data.Column{
				{Name: "segment", Type: "text"},
				{Name: "revenue", Type: "number"},
			},
		},
		Rows: []map[string]string{
			{"segment": "Enterprise", "revenue": "100"},
		},
	}

	analyzer := NewDeterministicAnalyzer()
	resp, err := analyzer.Analyze(context.Background(), AnalysisRequest{
		Prompt:   "show trend",
		Datasets: []*data.Dataset{ds},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundDateWarning := false
	for _, w := range resp.Warnings {
		if contains(w, "date") {
			foundDateWarning = true
		}
	}
	if !foundDateWarning {
		t.Fatalf("expected a date-related warning, got: %v", resp.Warnings)
	}
}

func TestDeterministicAnalyzerEmptyDatasets(t *testing.T) {
	analyzer := NewDeterministicAnalyzer()
	_, err := analyzer.Analyze(context.Background(), AnalysisRequest{
		Prompt:   "test",
		Datasets: []*data.Dataset{},
	})
	if err == nil {
		t.Fatal("expected error for empty datasets")
	}
}

func TestLLMAnalyzerFallsBackWithoutAPIKey(t *testing.T) {
	ds := &data.Dataset{
		ID:       "test-4",
		Filename: "test.csv",
		Profile: data.Profile{
			RowCount: 1,
			Columns: []data.Column{
				{Name: "revenue", Type: "number"},
			},
		},
		Rows: []map[string]string{
			{"revenue": "100"},
		},
	}

	cfg := DefaultConfig() // No API key set
	analyzer := NewLLMAnalyzer(cfg)
	resp, err := analyzer.Analyze(context.Background(), AnalysisRequest{
		Prompt:   "analyze",
		Datasets: []*data.Dataset{ds},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !resp.UsedDeterministic {
		t.Fatal("expected fallback to deterministic analyzer")
	}
	if len(resp.Warnings) == 0 {
		t.Fatal("expected a warning about LLM not being configured")
	}
}

func TestGuardDetectsLeakedSecrets(t *testing.T) {
	g := NewGuard()
	resp := AnalysisResponse{
		Question: "test",
		Notebook: []NotebookStep{
			{Body: "The API key is NVIDIA_API_KEY=abc123"},
		},
	}
	violations := g.ValidateResponse(resp)
	if len(violations) == 0 {
		t.Fatal("expected guard to detect leaked API key")
	}
}

func TestGuardAllowsCleanResponse(t *testing.T) {
	g := NewGuard()
	resp := AnalysisResponse{
		Question: "What is revenue?",
		Notebook: []NotebookStep{
			{Body: "Revenue increased by 10% month over month."},
		},
		Dashboard: DashboardSpec{
			Recommendations: []string{"Review top segments."},
		},
	}
	violations := g.ValidateResponse(resp)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got: %v", violations)
	}
}

func TestSanitizeForPrompt(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello world", 100, "hello world"},
		{"hello\x00world", 100, "helloworld"},
		{"hello\r\nworld", 100, "hello\nworld"},
		{"abcdef", 3, "abc"},
	}
	for _, tt := range tests {
		got := SanitizeForPrompt(tt.input, tt.maxLen)
		if got != tt.want {
			t.Errorf("SanitizeForPrompt(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
		}
	}
}

func TestStripMarkdownCodeFences(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"```json\n{\"a\":1}\n```", "{\"a\":1}"},
		{"```\n{\"a\":1}\n```", "{\"a\":1}"},
		{"{\"a\":1}", "{\"a\":1}"},
		{"  ```json\n{\"a\":1}\n```  ", "{\"a\":1}"},
	}
	for _, tt := range tests {
		got := stripMarkdownCodeFences(tt.input)
		if got != tt.want {
			t.Errorf("stripMarkdownCodeFences(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
