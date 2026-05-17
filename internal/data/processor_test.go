package data

import (
	"reflect"
	"testing"
)

func TestInferTypeRecognizesMonthValues(t *testing.T) {
	got := inferType([]string{"2026-01", "2026-02", "2026-03"})
	if got != "date" {
		t.Fatalf("inferType() = %q, want date", got)
	}
}

func TestBuildTrendAggregatesMonthValues(t *testing.T) {
	rows := []map[string]string{
		{"month": "2026-01", "revenue": "100"},
		{"month": "2026-01", "revenue": "50"},
		{"month": "2026-02", "revenue": "300"},
	}
	dateCol := &Column{Name: "month", Type: "date"}
	metricCol := &Column{Name: "revenue", Type: "number"}

	got := BuildTrend(rows, dateCol, metricCol)
	want := []map[string]interface{}{
		{"label": "2026-01", "value": 150.0},
		{"label": "2026-02", "value": 300.0},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildTrend() = %#v, want %#v", got, want)
	}
}

func TestBuildSegmentsAggregatesByCategory(t *testing.T) {
	rows := []map[string]string{
		{"segment": "Enterprise", "revenue": "100"},
		{"segment": "SMB", "revenue": "50"},
		{"segment": "Enterprise", "revenue": "25"},
	}
	categoryCol := &Column{Name: "segment", Type: "text"}
	metricCol := &Column{Name: "revenue", Type: "number"}

	got := BuildSegments(rows, categoryCol, metricCol)
	want := []map[string]interface{}{
		{"label": "Enterprise", "value": 125.0},
		{"label": "SMB", "value": 50.0},
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("BuildSegments() = %#v, want %#v", got, want)
	}
}
