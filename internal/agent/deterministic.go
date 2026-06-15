package agent

import (
	"context"
	"fmt"
	"strconv"
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

// parseFollowUpFilters extracts filter clauses and context changes from follow-up prompts.
func parseFollowUpFilters(prompt string, prevContext *ConversationContext) *ConversationContext {
	ctx := &ConversationContext{}
	if prevContext != nil {
		ctx = prevContext.Clone()
	}
	lower := strings.ToLower(strings.TrimSpace(prompt))

	// "reset filters" or "clear filters" or "show everything"
	if strings.Contains(lower, "reset") || strings.Contains(lower, "clear") || strings.Contains(lower, "show all") || strings.Contains(lower, "everything") {
		ctx.Filters = nil
		return ctx
	}

	// "remove filter on <col>" or "remove <col>"
	if strings.Contains(lower, "remove filter") || strings.Contains(lower, "remove ") {
		parts := strings.Split(lower, "remove ")
		if len(parts) > 1 {
			colName := strings.TrimSpace(parts[len(parts)-1])
			var kept []FilterClause
			for _, f := range ctx.Filters {
				if !strings.EqualFold(f.Column, colName) {
					kept = append(kept, f)
				}
			}
			ctx.Filters = kept
		}
		return ctx
	}

	// "filter by <col> = <val>" or "filter where <col> is <val>"
	filterPatterns := []struct {
		prefix   string
		operator string
	}{
		{"filter ", ""},
		{"only show ", ""},
		{"where ", ""},
	}

	var filterCol, filterVal, filterOp string
	for _, fp := range filterPatterns {
		idx := strings.Index(lower, fp.prefix)
		if idx >= 0 {
			rest := lower[idx+len(fp.prefix):]
			// Try "col = val", "col is val", "col contains val", "col > val", etc.
			for _, op := range []struct {
				sep    string
				opName string
			}{
				{" = ", "eq"}, {" == ", "eq"}, {" is ", "eq"},
				{" != ", "neq"}, {" <> ", "neq"},
				{" > ", "gt"}, {" >= ", "gte"},
				{" < ", "lt"}, {" <= ", "lte"},
				{" contains ", "contains"},
			} {
				if strings.Contains(rest, op.sep) {
					parts := strings.SplitN(rest, op.sep, 2)
					filterCol = strings.TrimSpace(parts[0])
					filterVal = strings.TrimSpace(parts[1])
					filterOp = op.opName
					goto found
				}
			}
		}
	}
found:

	if filterCol != "" && filterVal != "" {
		if filterOp == "" {
			filterOp = "eq"
		}
		filterVal = strings.Trim(filterVal, "\"'")
		// Only add if not already present
		exists := false
		for _, f := range ctx.Filters {
			if strings.EqualFold(f.Column, filterCol) && f.Operator == filterOp && f.Value == filterVal {
				exists = true
				break
			}
		}
		if !exists {
			ctx.Filters = append(ctx.Filters, FilterClause{Column: filterCol, Operator: filterOp, Value: filterVal})
		}
	}

	// "drill down into <val>" -> if we have a category column, treat as filter on it
	if strings.Contains(lower, "drill down") && prevContext != nil && prevContext.CategoryCol != "" {
		parts := strings.Split(lower, "drill down")
		if len(parts) > 1 {
			rest := strings.TrimSpace(parts[len(parts)-1])
			rest = strings.TrimPrefix(rest, "into ")
			rest = strings.TrimPrefix(rest, "on ")
			rest = strings.Trim(rest, "\"' ")
			if rest != "" {
				filterVal = rest
				filterCol = prevContext.CategoryCol
				filterOp = "eq"
				exists := false
				for _, f := range ctx.Filters {
					if strings.EqualFold(f.Column, filterCol) && f.Operator == filterOp && f.Value == filterVal {
						exists = true
						break
					}
				}
				if !exists {
					ctx.Filters = append(ctx.Filters, FilterClause{Column: filterCol, Operator: filterOp, Value: filterVal})
				}
			}
		}
	}

	return ctx
}

func (a *DeterministicAnalyzer) Analyze(ctx context.Context, req AnalysisRequest) (AnalysisResponse, error) {
	if len(req.Datasets) == 0 {
		return AnalysisResponse{}, fmt.Errorf("no datasets provided")
	}
	primary := req.Datasets[0]

	isFollowUp := len(req.History) > 0

	// For follow-ups, use previous context to guide column selection
	var metricCol, dateCol, catCol *data.Column
	if isFollowUp && req.Context != nil {
		if req.Context.MetricCol != "" {
			metricCol = selectColumnByName(primary.Profile.Columns, req.Context.MetricCol)
		}
		if req.Context.CategoryCol != "" {
			catCol = selectColumnByName(primary.Profile.Columns, req.Context.CategoryCol)
		}
		if req.Context.DateCol != "" {
			dateCol = selectColumnByName(primary.Profile.Columns, req.Context.DateCol)
		}
	}
	if metricCol == nil {
		metricCol = selectMetricColumn(primary.Profile.Columns, req.Prompt)
	}
	if dateCol == nil {
		dateCol = selectDateColumn(primary.Profile.Columns)
	}
	if catCol == nil {
		catCol = selectCategoryColumn(primary.Profile.Columns, metricCol, dateCol)
	}

	// Parse follow-up context changes
	var activeCtx *ConversationContext
	if isFollowUp {
		activeCtx = parseFollowUpFilters(req.Prompt, req.Context)
	} else {
		activeCtx = &ConversationContext{
			MetricCol:   colNameStr(metricCol),
			CategoryCol: colNameStr(catCol),
			DateCol:     colNameStr(dateCol),
		}
	}

	// Apply filters to dataset rows
	filteredRows := primary.Rows
	if len(activeCtx.Filters) > 0 {
		filteredRows = applyFilters(primary.Rows, activeCtx.Filters)
	}

	// Compute results on filtered data
	kpis := data.BuildKPIs(filteredRows, metricCol, catCol)
	trend := data.BuildTrend(filteredRows, dateCol, metricCol)
	segments := data.BuildSegments(filteredRows, catCol, metricCol)

	assumptions := buildAssumptions(primary, metricCol, catCol, dateCol)
	warnings := buildWarnings(primary, metricCol, catCol, dateCol)
	warnings = append(warnings, buildFilterWarnings(activeCtx, primary.Profile.RowCount, len(filteredRows))...)
	recommendations := buildRecommendations(metricCol, catCol)

	narrative := buildNarrative(req.Prompt, primary, metricCol, catCol, dateCol)

	var sqlQueries []string
	if primary.TableName != "" && (primary.ConnectionConfigID != "" || primary.ConnectionString != "") {
		sqlQueries = generateSQLQueries(primary.TableName, metricCol, catCol, dateCol)
	}

	notebook := []NotebookStep{
		{Title: "Data Profile", Body: fmt.Sprintf("Dataset %q has %d rows and %d columns.", primary.Filename, primary.Profile.RowCount, len(primary.Profile.Columns))},
		{Title: "Column Selection", Body: fmt.Sprintf("Metric: %s, Category: %s, Date: %s", colName(metricCol), colName(catCol), colName(dateCol))},
		{Title: "Analysis", Body: narrative},
	}
	if len(activeCtx.Filters) > 0 {
		filterDesc := make([]string, len(activeCtx.Filters))
		for i, f := range activeCtx.Filters {
			filterDesc[i] = fmt.Sprintf("%s %s %s", f.Column, f.Operator, f.Value)
		}
		notebook = append(notebook, NotebookStep{Title: "Active Filters", Body: strings.Join(filterDesc, ", ")})
	}
	if len(sqlQueries) > 0 {
		notebook = append(notebook, NotebookStep{Title: "Generated SQL", Body: strings.Join(sqlQueries, "\n\n")})
	}

	return AnalysisResponse{
		Question: req.Prompt,
		Dataset: DatasetSummary{
			ID:       primary.ID,
			Filename: primary.Filename,
			RowCount: len(filteredRows),
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
		Context:           activeCtx,
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

func selectColumnByName(columns []data.Column, name string) *data.Column {
	for i := range columns {
		if columns[i].Name == name {
			return &columns[i]
		}
	}
	return nil
}

func colNameStr(c *data.Column) string {
	if c == nil {
		return ""
	}
	return c.Name
}

func applyFilters(rows []map[string]string, filters []FilterClause) []map[string]string {
	if len(filters) == 0 {
		return rows
	}
	var out []map[string]string
	for _, row := range rows {
		include := true
		for _, f := range filters {
			val, ok := row[f.Column]
			if !ok {
				include = false
				break
			}
			switch f.Operator {
			case "eq":
				if !strings.EqualFold(val, f.Value) {
					include = false
				}
			case "neq":
				if strings.EqualFold(val, f.Value) {
					include = false
				}
			case "contains":
				if !strings.Contains(strings.ToLower(val), strings.ToLower(f.Value)) {
					include = false
				}
			case "gt":
				if !compareNumeric(val, f.Value, func(a, b float64) bool { return a > b }) {
					include = false
				}
			case "gte":
				if !compareNumeric(val, f.Value, func(a, b float64) bool { return a >= b }) {
					include = false
				}
			case "lt":
				if !compareNumeric(val, f.Value, func(a, b float64) bool { return a < b }) {
					include = false
				}
			case "lte":
				if !compareNumeric(val, f.Value, func(a, b float64) bool { return a <= b }) {
					include = false
				}
			}
			if !include {
				break
			}
		}
		if include {
			out = append(out, row)
		}
	}
	return out
}

func compareNumeric(a, b string, cmp func(float64, float64) bool) bool {
	af, errA := strconv.ParseFloat(a, 64)
	bf, errB := strconv.ParseFloat(b, 64)
	if errA != nil || errB != nil {
		return strings.Compare(a, b) == 0
	}
	return cmp(af, bf)
}

func buildFilterWarnings(ctx *ConversationContext, totalRows, filteredRows int) []string {
	if len(ctx.Filters) == 0 {
		return nil
	}
	warnings := []string{}
	var descs []string
	for _, f := range ctx.Filters {
		descs = append(descs, fmt.Sprintf("%s %s %s", f.Column, f.Operator, f.Value))
	}
	warnings = append(warnings, fmt.Sprintf("Filters applied: %s (%d of %d rows match)", strings.Join(descs, ", "), filteredRows, totalRows))
	if filteredRows == 0 {
		warnings = append(warnings, "No rows match the current filters. Try removing or changing filters.")
	}
	return warnings
}
