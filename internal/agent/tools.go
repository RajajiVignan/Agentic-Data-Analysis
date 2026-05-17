package agent

import (
	"context"
	"fmt"
	"insightpilot/internal/data"
)

// Tool defines a narrow, auditable operation the LLM agent can call.
type Tool interface {
	Name() string
	Description() string
	Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error)
}

// ProfileTool returns the column profile of a dataset.
type ProfileTool struct{}

func (t *ProfileTool) Name() string        { return "get_dataset_profile" }
func (t *ProfileTool) Description() string { return "Return column names, types, and row count for a dataset." }

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

// AggregateTool computes sum, avg, min, max for a numeric column.
type AggregateTool struct{}

func (t *AggregateTool) Name() string        { return "aggregate_metric" }
func (t *AggregateTool) Description() string { return "Compute sum, avg, min, max for a numeric column." }

func (t *AggregateTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"status": "not_implemented"}, nil
}

// GroupByTool groups rows by a dimension and aggregates a metric.
type GroupByTool struct{}

func (t *GroupByTool) Name() string        { return "group_by_dimension" }
func (t *GroupByTool) Description() string { return "Group rows by a categorical column and sum a metric." }

func (t *GroupByTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"status": "not_implemented"}, nil
}

// BuildTrendTool aggregates a metric over a date dimension.
type BuildTrendTool struct{}

func (t *BuildTrendTool) Name() string        { return "build_trend" }
func (t *BuildTrendTool) Description() string { return "Aggregate a metric by date for trend analysis." }

func (t *BuildTrendTool) Execute(ctx context.Context, args map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{"status": "not_implemented"}, nil
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
