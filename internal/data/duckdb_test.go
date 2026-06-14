package data

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDuckDBEngineBasicQuery(t *testing.T) {
	dir := t.TempDir()
	engine := NewDuckDBEngine(filepath.Join(dir, "scripts"))

	csvPath := filepath.Join(dir, "test.csv")
	csvContent := `month,segment,revenue,customers
2026-01,Enterprise,124000,42
2026-01,Mid-market,86000,118
2026-02,Enterprise,138500,45
2026-02,Mid-market,92000,122
`
	if err := os.WriteFile(csvPath, []byte(csvContent), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("ProfileCSV", func(t *testing.T) {
		profile, err := engine.ProfileCSV(csvPath)
		if err != nil {
			t.Fatalf("ProfileCSV failed: %v", err)
		}
		if profile.RowCount != 4 {
			t.Fatalf("row count = %d, want 4", profile.RowCount)
		}
		if len(profile.Columns) == 0 {
			t.Fatal("no columns profiled")
		}
	})

	t.Run("QueryKPIs", func(t *testing.T) {
		results, err := engine.QueryKPIs(csvPath, "revenue")
		if err != nil {
			t.Fatalf("QueryKPIs failed: %v", err)
		}
		if len(results) == 0 {
			t.Fatal("no KPI results")
		}
		if results[0]["total"] == "" {
			t.Fatal("total should not be empty")
		}
	})

	t.Run("QuerySegments", func(t *testing.T) {
		results, err := engine.QuerySegments(csvPath, "segment", "revenue")
		if err != nil {
			t.Fatalf("QuerySegments failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("segment count = %d, want 2", len(results))
		}
	})

	t.Run("QueryTrend", func(t *testing.T) {
		results, err := engine.QueryTrend(csvPath, "month", "revenue")
		if err != nil {
			t.Fatalf("QueryTrend failed: %v", err)
		}
		if len(results) != 2 {
			t.Fatalf("trend points = %d, want 2", len(results))
		}
	})
}

func TestDuckDBEngineInvalidFile(t *testing.T) {
	dir := t.TempDir()
	engine := NewDuckDBEngine(filepath.Join(dir, "scripts"))

	_, err := engine.ProfileCSV("/nonexistent/file.csv")
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
}
