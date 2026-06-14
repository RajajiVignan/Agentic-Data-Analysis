package store

import (
	"strings"
	"testing"
)

func TestBuildSupabaseConnStrUsesPassword(t *testing.T) {
	connStr := buildSupabaseConnStr("abc123", "secret-pass")
	expected := "postgresql://postgres:secret-pass@db.abc123.supabase.co:5432/postgres?sslmode=require"
	if connStr != expected {
		t.Fatalf("unexpected connection string: got %q want %q", connStr, expected)
	}
}

func TestPinnedChartsSchemaUsesTextIDForNumericChartIDs(t *testing.T) {
	schema := pinnedChartsSchemaSQL()
	if !strings.Contains(schema, "id TEXT PRIMARY KEY") {
		t.Fatalf("schema must use TEXT id for numeric string chart IDs: %s", schema)
	}
	if strings.Contains(schema, "id UUID") {
		t.Fatalf("schema must not use UUID id: %s", schema)
	}
}
