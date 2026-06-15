package data

import (
	"testing"
)

func makeTestDataset() *Dataset {
	return &Dataset{
		ID:       "test-1",
		Filename: "test.csv",
		Profile: Profile{
			RowCount: 5,
			Columns: []Column{
				{Name: "name", Type: "text", NonEmpty: 5},
				{Name: "region", Type: "text", NonEmpty: 5},
				{Name: "revenue", Type: "number", NonEmpty: 4},
				{Name: "cost", Type: "number", NonEmpty: 4},
				{Name: "month", Type: "date", NonEmpty: 5},
			},
		},
		Rows: []map[string]string{
			{"name": "Alice", "region": "North", "revenue": "100", "cost": "40", "month": "2026-01"},
			{"name": "Bob", "region": "South", "revenue": "200", "cost": "80", "month": "2026-01"},
			{"name": "Charlie", "region": "North", "revenue": "", "cost": "60", "month": "2026-02"},
			{"name": "Diana", "region": "East", "revenue": "150", "cost": "", "month": "2026-02"},
			{"name": "Eve", "region": "West", "revenue": "300", "cost": "120", "month": "2026-03"},
		},
	}
}

func TestPipelineUndoRedo(t *testing.T) {
	p := NewTransformPipeline()
	if p.CanUndo() || p.CanRedo() {
		t.Fatal("fresh pipeline should not allow undo/redo")
	}
	p.AddStep(TransformStep{Type: "filter", Description: "test"})
	if !p.CanUndo() {
		t.Fatal("should allow undo after add")
	}
	if p.CanRedo() {
		t.Fatal("should not allow redo after add")
	}
	if !p.Undo() {
		t.Fatal("undo should succeed")
	}
	if p.CanUndo() {
		t.Fatal("should not allow undo after undo")
	}
	if !p.CanRedo() {
		t.Fatal("should allow redo after undo")
	}
	if !p.Redo() {
		t.Fatal("redo should succeed")
	}
	if p.Len() != 1 {
		t.Fatalf("expected 1 step after redo, got %d", p.Len())
	}
}

func (p *TransformPipeline) Len() int {
	return len(p.Steps)
}

func TestApplyFilter(t *testing.T) {
	ds := makeTestDataset()
	step := TransformStep{
		Type: "filter",
		Params: map[string]interface{}{
			"column":   "region",
			"operator": "eq",
			"value":    "North",
		},
	}
	result := ApplySingle(ds, step)
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 rows (North), got %d", len(result.Rows))
	}
	for _, row := range result.Rows {
		if row["region"] != "North" {
			t.Fatalf("expected region=North, got %s", row["region"])
		}
	}
}

func TestApplyFilterNumeric(t *testing.T) {
	ds := makeTestDataset()
	step := TransformStep{
		Type: "filter",
		Params: map[string]interface{}{
			"column":   "revenue",
			"operator": "gt",
			"value":    "150",
		},
	}
	result := ApplySingle(ds, step)
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 rows (revenue > 150: Bob 200, Eve 300), got %d", len(result.Rows))
	}
	if result.Rows[0]["name"] != "Bob" {
		t.Fatalf("expected first row Bob, got %s", result.Rows[0]["name"])
	}
}

func TestApplyRename(t *testing.T) {
	ds := makeTestDataset()
	step := TransformStep{
		Type: "rename",
		Params: map[string]interface{}{
			"mappings": map[string]interface{}{
				"name": "full_name",
			},
		},
	}
	result := ApplySingle(ds, step)
	if len(result.Rows) != 5 {
		t.Fatalf("expected same row count, got %d", len(result.Rows))
	}
	if _, ok := result.Rows[0]["name"]; ok {
		t.Fatal("'name' column should have been renamed")
	}
	if v := result.Rows[0]["full_name"]; v != "Alice" {
		t.Fatalf("expected 'Alice', got %q", v)
	}
}

func TestApplyDrop(t *testing.T) {
	ds := makeTestDataset()
	step := TransformStep{
		Type: "drop",
		Params: map[string]interface{}{
			"columns": []interface{}{"cost", "month"},
		},
	}
	result := ApplySingle(ds, step)
	if _, ok := result.Rows[0]["cost"]; ok {
		t.Fatal("cost column should have been dropped")
	}
	if _, ok := result.Rows[0]["month"]; ok {
		t.Fatal("month column should have been dropped")
	}
	if v := result.Rows[0]["name"]; v != "Alice" {
		t.Fatalf("expected name preserved, got %q", v)
	}
}

func TestApplyNullFill(t *testing.T) {
	ds := makeTestDataset()
	step := TransformStep{
		Type: "null_handle",
		Params: map[string]interface{}{
			"column":   "revenue",
			"strategy": "fill",
			"fillValue": "0",
		},
	}
	result := ApplySingle(ds, step)
	if result.Rows[2]["revenue"] != "0" {
		t.Fatalf("expected revenue filled with 0, got %q", result.Rows[2]["revenue"])
	}
}

func TestApplyNullDropRow(t *testing.T) {
	ds := makeTestDataset()
	step := TransformStep{
		Type: "null_handle",
		Params: map[string]interface{}{
			"strategy": "drop_row",
		},
	}
	result := ApplySingle(ds, step)
	if len(result.Rows) != 3 {
		t.Fatalf("expected 3 rows after dropping rows with nulls, got %d", len(result.Rows))
	}
}

func TestApplyNullFillMean(t *testing.T) {
	ds := makeTestDataset()
	step := TransformStep{
		Type: "null_handle",
		Params: map[string]interface{}{
			"column":   "revenue",
			"strategy": "fill_mean",
		},
	}
	result := ApplySingle(ds, step)
	if result.Rows[2]["revenue"] == "" {
		t.Fatal("revenue should be filled with mean")
	}
	// revenue values for non-empty: 100, 200, 150, 300 = avg 187.5
	if result.Rows[1]["revenue"] != "200" {
		t.Fatalf("existing value 200 should be unchanged, got %q", result.Rows[1]["revenue"])
	}
}

func TestApplyDerive(t *testing.T) {
	ds := makeTestDataset()
	step := TransformStep{
		Type: "derive",
		Params: map[string]interface{}{
			"newColumn":  "profit",
			"expression": "revenue - cost",
		},
	}
	result := ApplySingle(ds, step)
	if _, ok := result.Rows[0]["profit"]; !ok {
		t.Fatal("profit column should exist")
	}
	if result.Rows[0]["profit"] != "60" {
		t.Fatalf("expected profit=100-40=60, got %q", result.Rows[0]["profit"])
	}
	if result.Rows[1]["profit"] != "120" {
		t.Fatalf("expected profit=200-80=120, got %q", result.Rows[1]["profit"])
	}
}

func TestApplyDeriveComplexExpression(t *testing.T) {
	ds := makeTestDataset()
	step := TransformStep{
		Type: "derive",
		Params: map[string]interface{}{
			"newColumn":  "margin",
			"expression": "(revenue - cost) / revenue * 100",
		},
	}
	result := ApplySingle(ds, step)
	if result.Rows[0]["margin"] == "" {
		t.Fatal("margin should be computed")
	}
}

func TestApplyAggregateSum(t *testing.T) {
	ds := makeTestDataset()
	step := TransformStep{
		Type: "aggregate",
		Params: map[string]interface{}{
			"groupBy":         "region",
			"aggregateColumn": "revenue",
			"function":        "sum",
			"newColumnName":   "total_revenue",
		},
	}
	result := ApplySingle(ds, step)
	if len(result.Rows) != 4 {
		t.Fatalf("expected 4 region groups, got %d", len(result.Rows))
	}
	// North: 100 + 0 (empty) = 100
	for _, row := range result.Rows {
		if row["region"] == "North" && row["total_revenue"] != "100" {
			t.Fatalf("expected North total 100, got %s", row["total_revenue"])
		}
	}
}

func TestApplyAggregateCount(t *testing.T) {
	ds := makeTestDataset()
	step := TransformStep{
		Type: "aggregate",
		Params: map[string]interface{}{
			"groupBy":         "region",
			"aggregateColumn": "revenue",
			"function":        "count",
		},
	}
	result := ApplySingle(ds, step)
	for _, row := range result.Rows {
		if row["region"] == "North" && row["count_revenue"] != "1" {
			t.Fatalf("expected North count_revenue=1 (only 1 valid revenue), got %s", row["count_revenue"])
		}
	}
}

func TestApplySort(t *testing.T) {
	ds := makeTestDataset()
	step := TransformStep{
		Type: "sort",
		Params: map[string]interface{}{
			"column": "name",
			"order":  "asc",
		},
	}
	result := ApplySingle(ds, step)
	if result.Rows[0]["name"] != "Alice" {
		t.Fatalf("expected first row Alice, got %s", result.Rows[0]["name"])
	}
	if result.Rows[4]["name"] != "Eve" {
		t.Fatalf("expected last row Eve, got %s", result.Rows[4]["name"])
	}
}

func TestApplySortDesc(t *testing.T) {
	ds := makeTestDataset()
	step := TransformStep{
		Type: "sort",
		Params: map[string]interface{}{
			"column": "revenue",
			"order":  "desc",
		},
	}
	result := ApplySingle(ds, step)
	if result.Rows[0]["name"] != "Eve" {
		t.Fatalf("expected first row Eve (300 revenue), got %s", result.Rows[0]["name"])
	}
}

func TestApplySelect(t *testing.T) {
	ds := makeTestDataset()
	step := TransformStep{
		Type: "select",
		Params: map[string]interface{}{
			"columns": []interface{}{"name", "revenue"},
		},
	}
	result := ApplySingle(ds, step)
	if len(result.Rows[0]) != 2 {
		t.Fatalf("expected 2 columns, got %d", len(result.Rows[0]))
	}
	if _, ok := result.Rows[0]["region"]; ok {
		t.Fatal("region should be excluded")
	}
}

func TestApplyFullPipeline(t *testing.T) {
	ds := makeTestDataset()
	p := NewTransformPipeline()

	// Step 1: Filter to North region
	p.AddStep(TransformStep{
		Type: "filter",
		Params: map[string]interface{}{
			"column": "region", "operator": "eq", "value": "North",
		},
	})

	// Step 2: Derive profit column
	p.AddStep(TransformStep{
		Type: "derive",
		Params: map[string]interface{}{
			"newColumn": "profit", "expression": "revenue - cost",
		},
	})

	result := p.ApplyAll(ds)
	if len(result.Rows) != 2 {
		t.Fatalf("expected 2 North rows, got %d", len(result.Rows))
	}
	// Alice: profit = 100 - 40 = 60
	if result.Rows[0]["profit"] != "60" {
		t.Fatalf("expected Alice profit 60, got %s", result.Rows[0]["profit"])
	}

	// Undo derive step
	p.Undo()
	result2 := p.ApplyAll(ds)
	if _, ok := result2.Rows[0]["profit"]; ok {
		t.Fatal("profit column should be gone after undo")
	}

	// Redo derive step
	p.Redo()
	result3 := p.ApplyAll(ds)
	if _, ok := result3.Rows[0]["profit"]; !ok {
		t.Fatal("profit column should be back after redo")
	}
}

func TestEvalMath(t *testing.T) {
	tests := []struct {
		expr string
		want float64
	}{
		{"1 + 2", 3},
		{"10 - 3", 7},
		{"4 * 5", 20},
		{"20 / 4", 5},
		{"1 + 2 * 3", 7},
		{"(1 + 2) * 3", 9},
		{"10 / 2 + 3", 8},
		{"-5", -5},
		{"-10 + 3", -7},
		{"5 * -3", -15},
	}
	for _, tt := range tests {
		got, err := evalMath(tt.expr)
		if err != nil {
			t.Fatalf("evalMath(%q) unexpected error: %v", tt.expr, err)
		}
		if got != tt.want {
			t.Fatalf("evalMath(%q) = %f, want %f", tt.expr, got, tt.want)
		}
	}
}

func TestApplyNullFillForward(t *testing.T) {
	ds := &Dataset{
		Rows: []map[string]string{
			{"val": "a"},
			{"val": ""},
			{"val": "b"},
			{"val": ""},
		},
	}
	step := TransformStep{
		Type: "null_handle",
		Params: map[string]interface{}{
			"column":   "val",
			"strategy": "fill_forward",
		},
	}
	result := ApplySingle(ds, step)
	if result.Rows[1]["val"] != "a" {
		t.Fatalf("expected forward-fill 'a', got %q", result.Rows[1]["val"])
	}
	if result.Rows[3]["val"] != "b" {
		t.Fatalf("expected forward-fill 'b', got %q", result.Rows[3]["val"])
	}
}

func TestApplyNullFillBackward(t *testing.T) {
	ds := &Dataset{
		Rows: []map[string]string{
			{"val": ""},
			{"val": "a"},
			{"val": ""},
			{"val": "b"},
		},
	}
	step := TransformStep{
		Type: "null_handle",
		Params: map[string]interface{}{
			"column":   "val",
			"strategy": "fill_backward",
		},
	}
	result := ApplySingle(ds, step)
	if result.Rows[0]["val"] != "a" {
		t.Fatalf("expected backward-fill 'a', got %q", result.Rows[0]["val"])
	}
	if result.Rows[2]["val"] != "b" {
		t.Fatalf("expected backward-fill 'b', got %q", result.Rows[2]["val"])
	}
}
