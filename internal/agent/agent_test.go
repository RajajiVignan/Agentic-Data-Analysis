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
	if resp.Dataset.RowCount != 3 {
		t.Fatalf("row count = %d, want 3", resp.Dataset.RowCount)
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

// --- Tool tests ---

func TestAggregateToolComputesStats(t *testing.T) {
	ds := &data.Dataset{
		ID:       "agg-test",
		Filename: "agg.csv",
		Profile: data.Profile{
			RowCount: 3,
			Columns: []data.Column{
				{Name: "revenue", Type: "number"},
				{Name: "segment", Type: "text"},
			},
		},
		Rows: []map[string]string{
			{"revenue": "100", "segment": "Enterprise"},
			{"revenue": "200", "segment": "SMB"},
			{"revenue": "300", "segment": "Enterprise"},
		},
	}

	tool := &AggregateTool{}
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"dataset": ds,
		"column":  "revenue",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["column"] != "revenue" {
		t.Fatalf("column = %v, want revenue", result["column"])
	}
	if result["count"] != 3 {
		t.Fatalf("count = %v, want 3", result["count"])
	}
	if result["sum"] != 600.0 {
		t.Fatalf("sum = %v, want 600", result["sum"])
	}
	if result["avg"] != 200.0 {
		t.Fatalf("avg = %v, want 200", result["avg"])
	}
	if result["min"] != 100.0 {
		t.Fatalf("min = %v, want 100", result["min"])
	}
	if result["max"] != 300.0 {
		t.Fatalf("max = %v, want 300", result["max"])
	}
}

func TestAggregateToolMissingColumn(t *testing.T) {
	ds := &data.Dataset{
		ID:       "agg-test",
		Profile:  data.Profile{Columns: []data.Column{{Name: "revenue", Type: "number"}}},
		Rows:     []map[string]string{{"revenue": "100"}},
	}

	tool := &AggregateTool{}
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"dataset": ds,
		"column":  "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for missing column")
	}
}

func TestAggregateToolEmptyData(t *testing.T) {
	ds := &data.Dataset{
		ID:      "agg-empty",
		Profile: data.Profile{Columns: []data.Column{{Name: "revenue", Type: "number"}}},
		Rows:    []map[string]string{},
	}

	tool := &AggregateTool{}
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"dataset": ds,
		"column":  "revenue",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result["count"] != 0 {
		t.Fatalf("count = %v, want 0", result["count"])
	}
}

func TestGroupByToolGroupsAndSums(t *testing.T) {
	ds := &data.Dataset{
		ID:       "group-test",
		Filename: "group.csv",
		Profile: data.Profile{
			RowCount: 4,
			Columns: []data.Column{
				{Name: "revenue", Type: "number"},
				{Name: "segment", Type: "text"},
			},
		},
		Rows: []map[string]string{
			{"revenue": "100", "segment": "Enterprise"},
			{"revenue": "200", "segment": "SMB"},
			{"revenue": "300", "segment": "Enterprise"},
			{"revenue": "50", "segment": "SMB"},
		},
	}

	tool := &GroupByTool{}
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"dataset":         ds,
		"category_column": "segment",
		"metric_column":   "revenue",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	groups, ok := result["groups"].([]map[string]interface{})
	if !ok {
		t.Fatal("groups not returned")
	}
	if len(groups) != 2 {
		t.Fatalf("groups len = %d, want 2", len(groups))
	}

	// Enterprise should be first (400 > 250)
	if groups[0]["label"] != "Enterprise" {
		t.Fatalf("first group = %v, want Enterprise", groups[0]["label"])
	}
	if groups[0]["value"] != 400.0 {
		t.Fatalf("Enterprise value = %v, want 400", groups[0]["value"])
	}
}

func TestGroupByToolCountWithoutMetric(t *testing.T) {
	ds := &data.Dataset{
		ID:      "group-count",
		Profile: data.Profile{Columns: []data.Column{{Name: "segment", Type: "text"}}},
		Rows: []map[string]string{
			{"segment": "A"},
			{"segment": "B"},
			{"segment": "A"},
		},
	}

	tool := &GroupByTool{}
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"dataset":         ds,
		"category_column": "segment",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	groups, ok := result["groups"].([]map[string]interface{})
	if !ok {
		t.Fatal("groups not returned or wrong type")
	}
	if len(groups) != 2 {
		t.Fatalf("groups len = %d, want 2", len(groups))
	}
	// A should be first (count 2 > 1)
	if groups[0]["label"] != "A" {
		t.Fatalf("first group = %v, want A", groups[0]["label"])
	}
}

func TestBuildTrendToolAggregatesByDate(t *testing.T) {
	ds := &data.Dataset{
		ID:       "trend-test",
		Filename: "trend.csv",
		Profile: data.Profile{
			RowCount: 4,
			Columns: []data.Column{
				{Name: "month", Type: "date"},
				{Name: "revenue", Type: "number"},
			},
		},
		Rows: []map[string]string{
			{"month": "2026-01", "revenue": "100"},
			{"month": "2026-01", "revenue": "200"},
			{"month": "2026-02", "revenue": "150"},
			{"month": "2026-03", "revenue": "300"},
		},
	}

	tool := &BuildTrendTool{}
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"dataset":       ds,
		"date_column":   "month",
		"metric_column": "revenue",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	points, ok := result["points"].([]map[string]interface{})
	if !ok {
		t.Fatal("points not returned")
	}
	if len(points) != 3 {
		t.Fatalf("points len = %d, want 3", len(points))
	}

	// First point should be 2026-01 with value 300
	if points[0]["label"] != "2026-01" {
		t.Fatalf("first point label = %v, want 2026-01", points[0]["label"])
	}
	if points[0]["value"] != 300.0 {
		t.Fatalf("first point value = %v, want 300", points[0]["value"])
	}
}

func TestBuildTrendToolMissingArgs(t *testing.T) {
	ds := &data.Dataset{
		ID:      "trend-missing",
		Profile: data.Profile{Columns: []data.Column{}},
		Rows:    []map[string]string{},
	}

	tool := &BuildTrendTool{}
	_, err := tool.Execute(context.Background(), map[string]interface{}{
		"dataset": ds,
	})
	if err == nil {
		t.Fatal("expected error for missing date_column and metric_column")
	}
}

func TestDefaultToolsReturnsAllFour(t *testing.T) {
	tools := DefaultTools()
	if len(tools) != 4 {
		t.Fatalf("len(DefaultTools) = %d, want 4", len(tools))
	}

	names := make(map[string]bool)
	for _, t := range tools {
		names[t.Name()] = true
	}

	expected := []string{"get_dataset_profile", "aggregate_metric", "group_by_dimension", "build_trend"}
	for _, name := range expected {
		if !names[name] {
			t.Fatalf("missing tool: %s", name)
		}
	}
}

func TestProfileToolReturnsMetadata(t *testing.T) {
	ds := &data.Dataset{
		ID:       "profile-test",
		Filename: "test.csv",
		Profile: data.Profile{
			RowCount: 5,
			Columns: []data.Column{
				{Name: "revenue", Type: "number", NonEmpty: 5},
				{Name: "segment", Type: "text", NonEmpty: 5},
			},
		},
		Rows: []map[string]string{},
	}

	tool := &ProfileTool{}
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"dataset": ds,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result["id"] != "profile-test" {
		t.Fatalf("id = %v, want profile-test", result["id"])
	}
	if result["row_count"] != 5 {
		t.Fatalf("row_count = %v, want 5", result["row_count"])
	}

	cols := result["columns"].([]map[string]interface{})
	if len(cols) != 2 {
		t.Fatalf("columns len = %d, want 2", len(cols))
	}
}
