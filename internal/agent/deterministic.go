package agent

import (
	"context"
	"fmt"
	"strings"

	"insightpilot/internal/data"
)

// DeterministicAnalyzer implements Analyzer using the existing
// column-selection + aggregation logic. It never calls external services
// and always returns a valid response.
type DeterministicAnalyzer struct{}

func NewDeterministicAnalyzer() *DeterministicAnalyzer {
	return &DeterministicAnalyzer{}
}

func (a *DeterministicAnalyzer) Analyze(ctx context.Context, req AnalysisRequest) (AnalysisResponse, error) {
	if len(req.Datasets) == 0 {
		return AnalysisResponse{}, fmt.Errorf("no datasets provided")
	}
	primary := req.Datasets[0]

	metricCol := selectMetricColumn(primary.Profile.Columns, req.Prompt)
	dateCol := selectDateColumn(primary.Profile.Columns)
	catCol := selectCategoryColumn(primary.Profile.Columns, metricCol, dateCol)

	kpis := data.BuildKPIs(primary.Rows, metricCol, catCol)
	trend := data.BuildTrend(primary.Rows, dateCol, metricCol)
	segments := data.BuildSegments(primary.Rows, catCol, metricCol)

	assumptions := buildAssumptions(primary, metricCol, catCol, dateCol)
	warnings := buildWarnings(primary, metricCol, catCol, dateCol)
	recommendations := buildRecommendations(metricCol, catCol)

	narrative := buildNarrative(req.Prompt, primary, metricCol, catCol, dateCol)

	// Generate SQL queries if the dataset has a live database connection
	var sqlQueries []string
	if primary.ConnectionString != "" && primary.TableName != "" {
		sqlQueries = generateSQLQueries(primary.TableName, metricCol, catCol, dateCol)
	}

	notebook := []NotebookStep{
		{Title: "Data Profile", Body: fmt.Sprintf("Dataset %q has %d rows and %d columns.", primary.Filename, primary.Profile.RowCount, len(primary.Profile.Columns))},
		{Title: "Column Selection", Body: fmt.Sprintf("Metric: %s, Category: %s, Date: %s", colName(metricCol), colName(catCol), colName(dateCol))},
		{Title: "Analysis", Body: narrative},
	}
	if len(sqlQueries) > 0 {
		notebook = append(notebook, NotebookStep{Title: "Generated SQL", Body: strings.Join(sqlQueries, "\n\n")})
	}

	return AnalysisResponse{
		Question: req.Prompt,
		Dataset: DatasetSummary{
			ID:       primary.ID,
			Filename: primary.Filename,
			RowCount: primary.Profile.RowCount,
		},
		Notebook:  notebook,
		SQLQueries: sqlQueries,
		Dashboard: DashboardSpec{
			Title:           "Insights Board",
			KPIs:            kpis,
			Trend:           trend,
			Segments:        segments,
			Recommendations: recommendations,
		},
		Assumptions:       assumptions,
		Warnings:          warnings,
		UsedDeterministic: true,
	}, nil
}

// generateSQLQueries generates SQL that matches the deterministic analysis logic.
func generateSQLQueries(tableName string, metricCol, catCol, dateCol *data.Column) []string {
	var queries []string

	if metricCol != nil {
		queries = append(queries,
			fmt.Sprintf("-- Total metric\nSELECT SUM(%s) AS total, AVG(%s) AS average, MIN(%s) AS min, MAX(%s) AS max\nFROM %s;",
				quoteCol(metricCol.Name), quoteCol(metricCol.Name), quoteCol(metricCol.Name), quoteCol(metricCol.Name), quoteIdent(tableName)))
	}

	if catCol != nil && metricCol != nil {
		queries = append(queries,
			fmt.Sprintf("-- Breakdown by %s\nSELECT %s, SUM(%s) AS total\nFROM %s\nGROUP BY %s\nORDER BY total DESC;",
				catCol.Name, quoteCol(catCol.Name), quoteCol(metricCol.Name), quoteIdent(tableName), quoteCol(catCol.Name)))
	}

	if dateCol != nil && metricCol != nil {
		queries = append(queries,
			fmt.Sprintf("-- Trend over %s\nSELECT %s, SUM(%s) AS total\nFROM %s\nGROUP BY %s\nORDER BY %s;",
				dateCol.Name, quoteCol(dateCol.Name), quoteCol(metricCol.Name), quoteIdent(tableName), quoteCol(dateCol.Name), quoteCol(dateCol.Name)))
	}

	return queries
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func quoteCol(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

func buildNarrative(prompt string, ds *data.Dataset, metricCol, catCol, dateCol *data.Column) string {
	parts := []string{}
	parts = append(parts, fmt.Sprintf("Analyzed %q (%d rows).", ds.Filename, ds.Profile.RowCount))
	if metricCol != nil {
		parts = append(parts, fmt.Sprintf("Primary metric: %s.", metricCol.Name))
	}
	if catCol != nil {
		parts = append(parts, fmt.Sprintf("Grouped by %s.", catCol.Name))
	}
	if dateCol != nil {
		parts = append(parts, fmt.Sprintf("Trended over %s.", dateCol.Name))
	}
	if prompt != "" {
		parts = append(parts, fmt.Sprintf("User question: %q.", prompt))
	}
	return strings.Join(parts, " ")
}

func buildAssumptions(ds *data.Dataset, metricCol, catCol, dateCol *data.Column) []string {
	assumptions := []string{
		fmt.Sprintf("Dataset %q is representative of the business question.", ds.Filename),
	}
	if metricCol != nil {
		assumptions = append(assumptions, fmt.Sprintf("Column %q is the most relevant metric for the analysis.", metricCol.Name))
	}
	if catCol != nil {
		assumptions = append(assumptions, fmt.Sprintf("Column %q provides meaningful segmentation.", catCol.Name))
	}
	return assumptions
}

func buildWarnings(ds *data.Dataset, metricCol, catCol, dateCol *data.Column) []string {
	warnings := []string{}
	if metricCol == nil {
		warnings = append(warnings, "No numeric metric column was found. KPIs may be limited.")
	}
	if catCol == nil {
		warnings = append(warnings, "No category column was found. Segment analysis is unavailable.")
	}
	if dateCol == nil {
		warnings = append(warnings, "No date column was found. Trend analysis is unavailable.")
	}
	if ds.Profile.RowCount < 10 {
		warnings = append(warnings, "Small dataset size may affect statistical reliability.")
	}
	return warnings
}

func buildRecommendations(metricCol, catCol *data.Column) []string {
	recs := []string{}
	if catCol != nil && metricCol != nil {
		recs = append(recs, fmt.Sprintf("Review the top %s groups contributing to %s.", catCol.Name, metricCol.Name))
	}
	recs = append(recs,
		"Add business definitions for metrics to ensure consistency.",
		"Publish this board after validating with the data owner.",
	)
	return recs
}

func colName(c *data.Column) string {
	if c == nil {
		return "none"
	}
	return c.Name
}

// The following are copies of the selection helpers from handler.go,
// kept here so the deterministic analyzer is self-contained.

func selectMetricColumn(columns []data.Column, prompt string) *data.Column {
	prompt = strings.ToLower(prompt)
	for i := range columns {
		if columns[i].Type == "number" && prompt != "" && strings.Contains(prompt, strings.ToLower(columns[i].Name)) {
			return &columns[i]
		}
	}
	for i := range columns {
		if columns[i].Type == "number" {
			return &columns[i]
		}
	}
	return nil
}

func selectCategoryColumn(columns []data.Column, metricCol *data.Column, dateCol *data.Column) *data.Column {
	used := make(map[string]bool)
	if metricCol != nil {
		used[metricCol.Name] = true
	}
	if dateCol != nil {
		used[dateCol.Name] = true
	}
	preferredNames := []string{"segment", "category", "region", "product", "department", "channel"}
	for _, name := range preferredNames {
		for i := range columns {
			if columns[i].Type == "text" && !used[columns[i].Name] && strings.EqualFold(columns[i].Name, name) {
				return &columns[i]
			}
		}
	}
	for i := range columns {
		if columns[i].Type == "text" && !used[columns[i].Name] && !looksLikeDateDimension(columns[i].Name) {
			return &columns[i]
		}
	}
	return nil
}

func selectDateColumn(columns []data.Column) *data.Column {
	for i := range columns {
		if columns[i].Type == "date" {
			return &columns[i]
		}
	}
	return nil
}

func looksLikeDateDimension(name string) bool {
	n := strings.ToLower(name)
	return strings.Contains(n, "date") || strings.Contains(n, "month") || strings.Contains(n, "year") || strings.Contains(n, "week")
}
