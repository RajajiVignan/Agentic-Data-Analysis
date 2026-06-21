package agent

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strconv"

	"insightpilot/internal/data"
)

// Tool defines a narrow, auditable operation the LLM agent can call.
type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error)
}

// resolveColumn finds a column by name in the dataset profile.
func resolveColumn(ds *data.Dataset, colName string) *data.Column {
	for i := range ds.Profile.Columns {
		if ds.Profile.Columns[i].Name == colName {
			return &ds.Profile.Columns[i]
		}
	}
	return nil
}

// parseFloat safely parses a string to float64.
func parseFloat(s string) (float64, bool) {
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

// --- ProfileTool ---

// ProfileTool returns the column profile of a dataset.
type ProfileTool struct{}

func (t *ProfileTool) Name() string { return "get_dataset_profile" }
func (t *ProfileTool) Description() string {
	return "Return column names, types, and row count for a dataset."
}

func (t *ProfileTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	ds, ok := args["dataset"].(*data.Dataset)
	if !ok {
		return nil, fmt.Errorf("get_dataset_profile: missing dataset argument")
	}
	cols := make([]map[string]interface{}, len(ds.Profile.Columns))
	for i, c := range ds.Profile.Columns {
		cols[i] = map[string]interface{}{
			"name":      c.Name,
			"type":      c.Type,
			"non_empty": c.NonEmpty,
		}
	}
	return map[string]interface{}{
		"id":        ds.ID,
		"filename":  ds.Filename,
		"row_count": ds.Profile.RowCount,
		"columns":   cols,
	}, nil
}

// --- AggregateTool ---

// AggregateTool computes sum, avg, min, max for a numeric column.
type AggregateTool struct{}

func (t *AggregateTool) Name() string { return "aggregate_metric" }
func (t *AggregateTool) Description() string {
	return "Compute sum, avg, min, max for a numeric column. Args: dataset, column (name of numeric column)"
}

func (t *AggregateTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	ds, ok := args["dataset"].(*data.Dataset)
	if !ok {
		return nil, fmt.Errorf("aggregate_metric: missing dataset argument")
	}
	colName, _ := args["column"].(string)
	if colName == "" {
		return nil, fmt.Errorf("aggregate_metric: missing column argument")
	}

	col := resolveColumn(ds, colName)
	if col == nil {
		return nil, fmt.Errorf("aggregate_metric: column %q not found in dataset", colName)
	}

	var vals []float64
	for _, row := range ds.Rows {
		if v, ok := parseFloat(row[col.Name]); ok {
			vals = append(vals, v)
		}
	}

	if len(vals) == 0 {
		return map[string]interface{}{
			"column": colName,
			"count":  0,
			"sum":    0,
			"avg":    0,
			"min":    0,
			"max":    0,
		}, nil
	}

	min, max, sum := vals[0], vals[0], 0.0
	for _, v := range vals {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}
	avg := sum / float64(len(vals))

	return map[string]interface{}{
		"column": colName,
		"count":  len(vals),
		"sum":    math.Round(sum*1000) / 1000,
		"avg":    math.Round(avg*1000) / 1000,
		"min":    math.Round(min*1000) / 1000,
		"max":    math.Round(max*1000) / 1000,
	}, nil
}

// --- GroupByTool ---

// GroupByTool groups rows by a dimension and aggregates a metric.
type GroupByTool struct{}

func (t *GroupByTool) Name() string { return "group_by_dimension" }
func (t *GroupByTool) Description() string {
	return "Group rows by a categorical column and sum a metric. Args: dataset, category_column (group-by column), metric_column (numeric column to sum)"
}

func (t *GroupByTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	ds, ok := args["dataset"].(*data.Dataset)
	if !ok {
		return nil, fmt.Errorf("group_by_dimension: missing dataset argument")
	}
	catColName, _ := args["category_column"].(string)
	metricColName, _ := args["metric_column"].(string)

	if catColName == "" {
		return nil, fmt.Errorf("group_by_dimension: missing category_column argument")
	}

	catCol := resolveColumn(ds, catColName)
	if catCol == nil {
		return nil, fmt.Errorf("group_by_dimension: category column %q not found", catColName)
	}

	var metricCol *data.Column
	if metricColName != "" {
		metricCol = resolveColumn(ds, metricColName)
		if metricCol == nil {
			return nil, fmt.Errorf("group_by_dimension: metric column %q not found", metricColName)
		}
	}

	// Build segments using the same logic as data.BuildSegments
	grouped := make(map[string]float64)
	for _, row := range ds.Rows {
		cat := row[catCol.Name]
		if cat == "" {
			cat = "Unknown"
		}

		val := 1.0
		if metricCol != nil {
			if v, ok := parseFloat(row[metricCol.Name]); ok {
				val = v
			}
		}
		grouped[cat] += val
	}

	type entry struct {
		label string
		value float64
	}
	var entries []entry
	for l, v := range grouped {
		entries = append(entries, entry{l, v})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].value > entries[j].value })

	limit := 10
	if len(entries) < limit {
		limit = len(entries)
	}
	groups := make([]map[string]interface{}, limit)
	for i := 0; i < limit; i++ {
		groups[i] = map[string]interface{}{
			"label": entries[i].label,
			"value": math.Round(entries[i].value*1000) / 1000,
		}
	}

	return map[string]interface{}{
		"category_column": catColName,
		"metric_column":   metricColName,
		"groups":          groups,
		"group_count":     len(entries),
	}, nil
}

// --- BuildTrendTool ---

// BuildTrendTool aggregates a metric over a date dimension.
type BuildTrendTool struct{}

func (t *BuildTrendTool) Name() string { return "build_trend" }
func (t *BuildTrendTool) Description() string {
	return "Aggregate a metric by date for trend analysis. Args: dataset, date_column, metric_column"
}

func (t *BuildTrendTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	ds, ok := args["dataset"].(*data.Dataset)
	if !ok {
		return nil, fmt.Errorf("build_trend: missing dataset argument")
	}
	dateColName, _ := args["date_column"].(string)
	metricColName, _ := args["metric_column"].(string)

	if dateColName == "" {
		return nil, fmt.Errorf("build_trend: missing date_column argument")
	}
	if metricColName == "" {
		return nil, fmt.Errorf("build_trend: missing metric_column argument")
	}

	dateCol := resolveColumn(ds, dateColName)
	if dateCol == nil {
		return nil, fmt.Errorf("build_trend: date column %q not found", dateColName)
	}
	metricCol := resolveColumn(ds, metricColName)
	if metricCol == nil {
		return nil, fmt.Errorf("build_trend: metric column %q not found", metricColName)
	}

	// Use the same logic as data.BuildTrend
	grouped := make(map[string]float64)
	for _, row := range ds.Rows {
		dStr := row[dateCol.Name]
		vStr := row[metricCol.Name]

		t, ok := data.ParseDateValue(dStr)
		if !ok {
			continue
		}

		val, ok := parseFloat(vStr)
		if !ok {
			continue
		}

		label := t.Format("2006-01")
		grouped[label] += val
	}

	var labels []string
	for l := range grouped {
		labels = append(labels, l)
	}
	sort.Strings(labels)

	var points []map[string]interface{}
	start := 0
	if len(labels) > 8 {
		start = len(labels) - 8
	}
	for _, l := range labels[start:] {
		points = append(points, map[string]interface{}{
			"label": l,
			"value": math.Round(grouped[l]*1000) / 1000,
		})
	}

	return map[string]interface{}{
		"date_column":   dateColName,
		"metric_column": metricColName,
		"points":        points,
		"point_count":   len(points),
	}, nil
}

// DefaultTools returns the standard set of tools available to the agent.
func DefaultTools() []Tool {
	return []Tool{
		&ProfileTool{},
		&AggregateTool{},
		&GroupByTool{},
		&BuildTrendTool{},
	}
}
