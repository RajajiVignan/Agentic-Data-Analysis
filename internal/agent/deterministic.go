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

	chartType := guessChartType(metricCol, catCol, dateCol, trend, segments)
	chartTypes := []string{chartType}
	if chartType == "line" {
		chartTypes = append(chartTypes, "bar")
	}

	explanations := buildExplanations(metricCol, catCol, dateCol, sqlQueries)

	notebook := []NotebookStep{
		{Title: "Data Profile", Body: fmt.Sprintf("Dataset %q has %d rows and %d columns.", primary.Filename, primary.Profile.RowCount, len(primary.Profile.Columns))},
		{Title: "Column Selection", Body: fmt.Sprintf("Metric: %s, Category: %s, Date: %s", colName(metricCol), colName(catCol), colName(dateCol))},
		{Title: "Visualization", Body: fmt.Sprintf("Recommended chart type: %s", chartType)},
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
		Notebook:   notebook,
		SQLQueries: sqlQueries,
		Dashboard: DashboardSpec{
			Title:           "Insights Board",
			KPIs:            kpis,
			Trend:           trend,
			Segments:        segments,
			Recommendations: recommendations,
			Narrative:       narrative,
			ChartType:       chartType,
			ChartTypes:      chartTypes,
			Explanations:    explanations,
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
	fullNarrative := buildRichNarrative(prompt, ds, metricCol, catCol, dateCol)
	return fullNarrative
}

// buildRichNarrative generates a data-driven story from the available metrics.
func buildRichNarrative(prompt string, ds *data.Dataset, metricCol, catCol, dateCol *data.Column) string {
	parts := []string{}

	if metricCol == nil {
		parts = append(parts, fmt.Sprintf("Analyzed dataset %q containing %d rows across %d columns.", ds.Filename, ds.Profile.RowCount, len(ds.Profile.Columns)))
		if prompt != "" {
			parts = append(parts, fmt.Sprintf("In response to: %q.", prompt))
		}
		return strings.Join(parts, " ")
	}

	// Extract KPI values from the dataset
	var totalVal, avgVal float64
	var totalCount int
	for _, row := range ds.Rows {
		if v, err := strconv.ParseFloat(row[metricCol.Name], 64); err == nil {
			totalVal += v
			totalCount++
		}
	}
	if totalCount > 0 {
		avgVal = totalVal / float64(totalCount)
	}

	// Build the opening statement
	metricName := metricCol.Name
	if totalVal >= 1000000 {
		parts = append(parts, fmt.Sprintf("Total %s is $%.1fM, averaging $%.1fK per row across %d records.",
			metricName, totalVal/1000000, avgVal/1000, ds.Profile.RowCount))
	} else if totalVal >= 1000 {
		parts = append(parts, fmt.Sprintf("Total %s is $%.1fK, averaging $%.1f per row across %d records.",
			metricName, totalVal/1000, avgVal, ds.Profile.RowCount))
	} else {
		parts = append(parts, fmt.Sprintf("Total %s is $%.1f, averaging $%.1f per row across %d records.",
			metricName, totalVal, avgVal, ds.Profile.RowCount))
	}

	// Trend story
	if dateCol != nil {
		trend := data.BuildTrend(ds.Rows, dateCol, metricCol)
		if len(trend) >= 2 {
			first := trend[0]["value"].(float64)
			last := trend[len(trend)-1]["value"].(float64)
			change := last - first
			pctChange := 0.0
			if first != 0 {
				pctChange = (change / first) * 100
			}
			trendDir := "increased"
			if change < 0 {
				trendDir = "decreased"
			}
			absChange := change
			if absChange < 0 {
				absChange = -absChange
			}
			firstLabel := trend[0]["label"].(string)
			lastLabel := trend[len(trend)-1]["label"].(string)

			if absChange >= 1000000 {
				parts = append(parts, fmt.Sprintf("Over time, %s %s by $%.1fM (%.1f%%) from %s to %s.",
					metricName, trendDir, absChange/1000000, pctChange, firstLabel, lastLabel))
			} else if absChange >= 1000 {
				parts = append(parts, fmt.Sprintf("Over time, %s %s by $%.1fK (%.1f%%) from %s to %s.",
					metricName, trendDir, absChange/1000, pctChange, firstLabel, lastLabel))
			} else {
				parts = append(parts, fmt.Sprintf("Over time, %s %s by $%.1f (%.1f%%) from %s to %s.",
					metricName, trendDir, absChange, pctChange, firstLabel, lastLabel))
			}

			// Highlight best/worst periods
			bestVal := trend[0]["value"].(float64)
			bestLabel := trend[0]["label"].(string)
			worstVal := trend[0]["value"].(float64)
			worstLabel := trend[0]["label"].(string)
			for _, t := range trend {
				v := t["value"].(float64)
				if v > bestVal {
					bestVal = v
					bestLabel = t["label"].(string)
				}
				if v < worstVal {
					worstVal = v
					worstLabel = t["label"].(string)
				}
			}
			if bestLabel != worstLabel {
				parts = append(parts, fmt.Sprintf("Peak performance was in %s ($%.1fK), while the lowest point was %s ($%.1fK).",
					bestLabel, bestVal/1000, worstLabel, worstVal/1000))
			}
		} else if len(trend) == 1 {
			parts = append(parts, fmt.Sprintf("Data is available for %s with %s of $%.1f.", trend[0]["label"], metricName, trend[0]["value"]))
		}
	}

	// Segment story
	if catCol != nil {
		segments := data.BuildSegments(ds.Rows, catCol, metricCol)
		if len(segments) > 0 {
			top := segments[0]
			topName := top["label"].(string)
			topVal := top["value"].(float64)
			topPct := 0.0
			if totalVal > 0 {
				topPct = (topVal / totalVal) * 100
			}
			parts = append(parts, fmt.Sprintf("The leading segment is %q, contributing $%.1fK (%.1f%% of total).", topName, topVal/1000, topPct))

			if len(segments) > 1 {
				// Show other segments
				otherNames := make([]string, 0, len(segments)-1)
				for i := 1; i < len(segments); i++ {
					otherNames = append(otherNames, fmt.Sprintf("%q (%d%%)", segments[i]["label"], int(segments[i]["value"].(float64)/totalVal*100)))
				}
				if len(otherNames) == 1 {
					parts = append(parts, fmt.Sprintf("Other contributors include %s.", otherNames[0]))
				} else if len(otherNames) > 1 {
					parts = append(parts, fmt.Sprintf("Other contributors include %s.", strings.Join(otherNames, ", ")))
				}
			}
		}
	}

	if prompt != "" {
		parts = append(parts, fmt.Sprintf("This analysis was generated in response to: %q.", prompt))
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

// guessChartType picks an appropriate chart type based on available columns and data.
func guessChartType(metricCol, catCol, dateCol *data.Column, trend, segments []map[string]interface{}) string {
	if dateCol != nil && len(trend) > 1 {
		return "line"
	}
	if catCol != nil && len(segments) > 1 {
		return "pie"
	}
	if metricCol != nil {
		return "bar"
	}
	return "bar"
}

// buildExplanations creates per-chart explanation entries.
func buildExplanations(metricCol, catCol, dateCol *data.Column, sqlQueries []string) []map[string]string {
	var exps []map[string]string

	if metricCol != nil {
		sql := ""
		if len(sqlQueries) > 0 {
			sql = sqlQueries[0]
		}
		cols := metricCol.Name
		warn := ""
		if metricCol.NonEmpty < 3 {
			warn = "Very few data points — results may not be statistically meaningful"
		}
		exps = append(exps, map[string]string{
			"chart":    "kpi",
			"columns":  cols,
			"sql":      sql,
			"warning":  warn,
			"grouping": "none",
		})
	}

	if dateCol != nil && metricCol != nil {
		sql := ""
		if len(sqlQueries) >= 3 {
			sql = sqlQueries[2]
		} else if len(sqlQueries) >= 2 {
			sql = sqlQueries[1]
		}
		warn := ""
		if dateCol.NonEmpty < 3 {
			warn = "Only 3 or fewer time periods — trend may be unreliable"
		}
		exps = append(exps, map[string]string{
			"chart":    "trend",
			"columns":  fmt.Sprintf("%s by %s", metricCol.Name, dateCol.Name),
			"sql":      sql,
			"warning":  warn,
			"grouping": dateCol.Name,
		})
	}

	if catCol != nil && metricCol != nil {
		sql := ""
		if len(sqlQueries) >= 2 {
			sql = sqlQueries[1]
		} else if len(sqlQueries) >= 1 {
			sql = sqlQueries[0]
		}
		warn := ""
		if catCol.NonEmpty < 2 {
			warn = "Very few unique categories — consider a different dimension"
		}
		exps = append(exps, map[string]string{
			"chart":    "segment",
			"columns":  fmt.Sprintf("%s by %s", metricCol.Name, catCol.Name),
			"sql":      sql,
			"warning":  warn,
			"grouping": catCol.Name,
		})
	}

	return exps
}
