package data

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// SemanticField is a user-defined computed field (metric or dimension) attached
// to a dataset. Fields are BUILDER-COMPOSED (safe): the config is a structured
// description, never raw SQL, so we can whitelist every token when generating SQL.
type SemanticField struct {
	ID        string          `json:"id"`
	DatasetID string          `json:"datasetId"`
	Name      string          `json:"name"`
	Kind      string          `json:"kind"` // "metric" | "dimension"
	Config    json.RawMessage `json:"config"`
	CreatedAt string          `json:"createdAt"`
	UpdatedAt string          `json:"updatedAt"`
}

// AllowedAggFns and AllowedOps constrain what a computed field may contain.
var AllowedAggFns = map[string]bool{
	"sum": true, "avg": true, "min": true, "max": true, "count": true,
}

var allowedOps = map[string]bool{
	"+": true, "-": true, "*": true, "/": true,
}

var allowedTransforms = map[string]bool{
	"month": true, "year": true, "day": true, "upper": true, "lower": true,
}

// QuoteIdentSafe safely quotes a SQL identifier, rejecting anything non-identifier.
func QuoteIdentSafe(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty identifier")
	}
	for _, r := range name {
		if !(r == '_' || r == '$' || (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9')) {
			return "", fmt.Errorf("invalid identifier: %q", name)
		}
	}
	return `"` + name + `"`, nil
}

// ColumnSet is the set of valid column names for a dataset.
type ColumnSet map[string]bool

// NewColumnSetFromProfile builds a ColumnSet from a dataset profile.
func NewColumnSetFromProfile(p Profile) ColumnSet {
	cs := make(ColumnSet)
	for _, c := range p.Columns {
		cs[c.Name] = true
	}
	return cs
}

// Has reports whether name is a known column (empty set = allow all, used for
// live-DB datasets whose schema we may not have profiled).
func (cs ColumnSet) Has(name string) bool { return cs[name] || len(cs) == 0 }

// IsAggregateField reports whether the metric field's config is an
// aggregate-mode field (e.g. SUM of a column), which must not be wrapped in
// another aggregate when used by the KPI explore path.
func (f *SemanticField) IsAggregateField() bool {
	var cfg struct {
		Mode string `json:"mode"`
	}
	if err := json.Unmarshal(f.Config, &cfg); err != nil {
		return false
	}
	return cfg.Mode == "aggregate"
}

// BuildMetricExpr returns a safe SQL expression for a metric field.
// For aggregate mode: agg(CAST(col AS DOUBLE)).
// For expression mode: left op right (recursively), where each side is a
// whitelisted column or a numeric literal.
func (f *SemanticField) BuildMetricExpr(cols ColumnSet) (string, error) {
	var cfg struct {
		Mode   string          `json:"mode"`
		Agg    string          `json:"agg"`
		Column string          `json:"column"`
		Left   json.RawMessage `json:"left"`
		Op     string          `json:"op"`
		Right  json.RawMessage `json:"right"`
	}
	if err := json.Unmarshal(f.Config, &cfg); err != nil {
		return "", fmt.Errorf("invalid metric config: %w", err)
	}
	switch cfg.Mode {
	case "aggregate":
		agg := strings.ToLower(cfg.Agg)
		if !AllowedAggFns[agg] {
			return "", fmt.Errorf("unsupported aggregation: %q", cfg.Agg)
		}
		col, err := QuoteIdentSafe(cfg.Column)
		if err != nil {
			return "", err
		}
		if !cols.Has(cfg.Column) {
			return "", fmt.Errorf("unknown column: %q", cfg.Column)
		}
		if agg == "count" {
			return fmt.Sprintf("COUNT(%s)", col), nil
		}
		return fmt.Sprintf("%s(CAST(%s AS DOUBLE))", strings.ToUpper(agg), col), nil
	case "expression":
		if !allowedOps[cfg.Op] {
			return "", fmt.Errorf("unsupported operator: %q", cfg.Op)
		}
		left, err := buildOperand(cfg.Left, cols)
		if err != nil {
			return "", err
		}
		right, err := buildOperand(cfg.Right, cols)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("(%s %s %s)", left, cfg.Op, right), nil
	default:
		return "", fmt.Errorf("unknown metric mode: %q", cfg.Mode)
	}
}

// BuildDimensionExpr returns a safe SQL expression for a dimension field.
func (f *SemanticField) BuildDimensionExpr(cols ColumnSet) (string, error) {
	var cfg struct {
		Mode      string `json:"mode"`
		Column    string `json:"column"`
		Transform string `json:"transform"`
	}
	if err := json.Unmarshal(f.Config, &cfg); err != nil {
		return "", fmt.Errorf("invalid dimension config: %w", err)
	}
	col, err := QuoteIdentSafe(cfg.Column)
	if err != nil {
		return "", err
	}
	if !cols.Has(cfg.Column) {
		return "", fmt.Errorf("unknown column: %q", cfg.Column)
	}
	switch cfg.Mode {
	case "column", "":
		return col, nil
	case "transform":
		t := strings.ToLower(cfg.Transform)
		if !allowedTransforms[t] {
			return "", fmt.Errorf("unsupported transform: %q", cfg.Transform)
		}
		switch t {
		case "month", "year", "day":
			return fmt.Sprintf("DATE_TRUNC('%s', CAST(%s AS DATE))", t, col), nil
		case "upper":
			return fmt.Sprintf("UPPER(%s)", col), nil
		case "lower":
			return fmt.Sprintf("LOWER(%s)", col), nil
		}
	}
	return col, nil
}

// operand is either {kind:"column",name:"x"} or {kind:"number",value:1.5}.
func buildOperand(raw json.RawMessage, cols ColumnSet) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("missing operand")
	}
	var op struct {
		Kind  string  `json:"kind"`
		Name  string  `json:"name"`
		Value float64 `json:"value"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return "", fmt.Errorf("invalid operand: %w", err)
	}
	switch op.Kind {
	case "column":
		col, err := QuoteIdentSafe(op.Name)
		if err != nil {
			return "", err
		}
		if !cols.Has(op.Name) {
			return "", fmt.Errorf("unknown column: %q", op.Name)
		}
		return fmt.Sprintf("CAST(%s AS DOUBLE)", col), nil
	case "number":
		return strconv.FormatFloat(op.Value, 'f', -1, 64), nil
	default:
		return "", fmt.Errorf("unknown operand kind: %q", op.Kind)
	}
}

// MetricRef describes a metric selection in an explore request.
type MetricRef struct {
	Type string `json:"type"` // "column" | "field"
	Name string `json:"name"` // column name (when type=column)
	Agg  string `json:"agg"`  // aggregation (when type=column)
	ID   string `json:"id"`   // semantic field id (when type=field)
}

// DimensionRef describes a dimension selection in an explore request.
type DimensionRef struct {
	Type string `json:"type"` // "column" | "field"
	Name string `json:"name"` // column name (when type=column)
	ID   string `json:"id"`   // semantic field id (when type=field)
}
