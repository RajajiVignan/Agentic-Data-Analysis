package store

import (
	"strings"
	"testing"

	"insightpilot/migrations"
)

func TestBuildSupabaseConnStrUsesPassword(t *testing.T) {
	connStr := buildSupabaseConnStr("abc123", "secret-pass")
	expected := "postgresql://postgres:secret-pass@db.abc123.supabase.co:5432/postgres?sslmode=require"
	if connStr != expected {
		t.Fatalf("unexpected connection string: got %q want %q", connStr, expected)
	}
}

func TestCoreMigrationUsesTextIDsForChartTables(t *testing.T) {
	body, err := migrations.Read("0001_core_tables.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(body, "id TEXT PRIMARY KEY") {
		t.Fatalf("schema must use TEXT ids for numeric string chart IDs")
	}
	if strings.Contains(body, "id UUID") {
		t.Fatalf("schema must not use UUID ids")
	}
}

func TestMigrationsCreateEveryTenantTable(t *testing.T) {
	names, err := migrations.Names()
	if err != nil {
		t.Fatal(err)
	}
	var all strings.Builder
	for _, name := range names {
		body, err := migrations.Read(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		all.WriteString(body)
		all.WriteString("\n")
	}

	schema := all.String()
	tables := []string{
		"pinned_charts", "dashboards", "datasets", "scheduled_reports",
		"alert_rules", "dashboard_layouts", "glossary_defs", "dataset_fields",
		"saved_queries", "users", "app_config", "shared_dashboards",
		"transform_pipelines", "conversation_sessions",
	}
	for _, table := range tables {
		if !strings.Contains(schema, "CREATE TABLE IF NOT EXISTS "+table) {
			t.Errorf("migration set does not create %s", table)
		}
	}
}
