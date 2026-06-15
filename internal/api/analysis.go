package api

import (
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"
	"time"

	"insightpilot/internal/agent"
	"insightpilot/internal/data"
)

// --- Plan execution ---

// applyFiltersToRows filters dataset rows based on the plan's filter clauses.
func applyFiltersToRows(rows []map[string]string, filters []agent.FilterClause) []map[string]string {
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
				v1, _ := strconv.ParseFloat(val, 64)
				v2, _ := strconv.ParseFloat(f.Value, 64)
				if v1 <= v2 {
					include = false
				}
			case "gte":
				v1, _ := strconv.ParseFloat(val, 64)
				v2, _ := strconv.ParseFloat(f.Value, 64)
				if v1 < v2 {
					include = false
				}
			case "lt":
				v1, _ := strconv.ParseFloat(val, 64)
				v2, _ := strconv.ParseFloat(f.Value, 64)
				if v1 >= v2 {
					include = false
				}
			case "lte":
				v1, _ := strconv.ParseFloat(val, 64)
				v2, _ := strconv.ParseFloat(f.Value, 64)
				if v1 > v2 {
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

func execPlan(plan *agent.LLMPlan, ds *data.Dataset, resp *agent.AnalysisResponse) {
	// Apply filters from the plan
	rows := ds.Rows
	if len(plan.Filters) > 0 {
		rows = applyFiltersToRows(rows, plan.Filters)
		log.Printf("[execPlan] Applied %d filter(s): %d rows remaining", len(plan.Filters), len(rows))
	}

	var metricCol, catCol, dateCol *data.Column
	for i := range ds.Profile.Columns {
		c := &ds.Profile.Columns[i]
		if c.Name == plan.MetricColumn {
			metricCol = c
		}
		if c.Name == plan.CategoryColumn {
			catCol = c
		}
		if c.Name == plan.DateColumn {
			dateCol = c
		}
	}

	log.Printf("[execPlan] Resolved columns: metric=%v category=%v date=%v (from %d total columns)",
		colRes(metricCol), colRes(catCol), colRes(dateCol), len(ds.Profile.Columns))

	if metricCol == nil {
		for i := range ds.Profile.Columns {
			if ds.Profile.Columns[i].Type == "number" {
				metricCol = &ds.Profile.Columns[i]
				break
			}
		}
	}

	if catCol == nil {
		used := map[string]bool{}
		if metricCol != nil {
			used[metricCol.Name] = true
		}
		if dateCol != nil {
			used[dateCol.Name] = true
		}
		for i := range ds.Profile.Columns {
			c := &ds.Profile.Columns[i]
			if c.Type == "text" && !used[c.Name] {
				catCol = c
				break
			}
		}
	}

	if metricCol == nil {
		title := plan.Title
		if title == "" {
			title = "Insights Board"
		}
		resp.Dashboard = agent.DashboardSpec{
			Title:           title,
			KPIs:            []map[string]string{},
			Trend:           []map[string]interface{}{},
			Segments:        []map[string]interface{}{},
			Recommendations: plan.Recommendations,
			Narrative:       plan.Narrative,
		}
		if len(plan.Assumptions) > 0 {
			resp.Assumptions = append(plan.Assumptions, resp.Assumptions...)
		}
		return
	}

	duckDB := getDuckDBEngine()
	if ds.FilePath != "" && duckDB != nil && len(plan.Filters) == 0 {
		kpis, kpiSQL := duckDBKPI(duckDB, ds, metricCol)
		trend, trendSQL := duckDBTrend(duckDB, ds, dateCol, metricCol)
		segments, segSQL := duckDBSegments(duckDB, ds, catCol, metricCol)
		title := plan.Title
		if title == "" {
			title = "Insights Board"
		}
		resp.Dashboard = agent.DashboardSpec{
			Title:           title,
			KPIs:            kpis,
			Trend:           trend,
			Segments:        segments,
			Recommendations: plan.Recommendations,
			Narrative:       plan.Narrative,
		}
		sqls := make([]string, 0, 3)
		if kpiSQL != "" {
			sqls = append(sqls, kpiSQL)
		}
		if trendSQL != "" {
			sqls = append(sqls, trendSQL)
		}
		if segSQL != "" {
			sqls = append(sqls, segSQL)
		}
		resp.SQLQueries = append(resp.SQLQueries, sqls...)
		if len(plan.Assumptions) > 0 {
			resp.Assumptions = append(plan.Assumptions, resp.Assumptions...)
		}
		return
	}

	if ds.FilePath != "" && duckDB != nil && len(plan.Filters) > 0 {
		log.Printf("[execPlan] Filters present, falling back to in-memory computation (DuckDB does not support client-side filtering)")
	}

	kpis := data.BuildKPIs(rows, metricCol, catCol)
	trend := data.BuildTrend(rows, dateCol, metricCol)
	segments := data.BuildSegments(rows, catCol, metricCol)

	if plan.Aggregation != "" && plan.Aggregation != "sum" {
		kpis = applyAggregation(kpis, plan.Aggregation)
	}

	title := plan.Title
	if title == "" {
		title = "Insights Board"
	}

	resp.Dashboard = agent.DashboardSpec{
		Title:           title,
		KPIs:            kpis,
		Trend:           trend,
		Segments:        segments,
		Recommendations: plan.Recommendations,
		Narrative:       plan.Narrative,
	}

	if len(plan.Assumptions) > 0 {
		resp.Assumptions = append(plan.Assumptions, resp.Assumptions...)
	}
}

var duckDBEngine *data.DuckDBEngine

func setDuckDBEngine(e *data.DuckDBEngine) {
	duckDBEngine = e
}

func getDuckDBEngine() *data.DuckDBEngine {
	return duckDBEngine
}

func duckDBKPI(duckDB *data.DuckDBEngine, ds *data.Dataset, metricCol *data.Column) ([]map[string]string, string) {
	if metricCol == nil {
		return []map[string]string{}, ""
	}
	results, err := duckDB.QueryKPIs(ds.FilePath, metricCol.Name)
	if err != nil {
		log.Printf("[duckdb] KPI query failed: %v, falling back to in-memory", err)
		q := quoteIdent(metricCol.Name)
		sql := fmt.Sprintf(`SELECT 
			SUM(CAST(%s AS DOUBLE)) as total,
			AVG(CAST(%s AS DOUBLE)) as avg_val,
			MIN(CAST(%s AS DOUBLE)) as min_val,
			MAX(CAST(%s AS DOUBLE)) as max_val,
			COUNT(*) as row_count
		FROM data WHERE CAST(%s AS DOUBLE) IS NOT NULL`, q, q, q, q, q)
		return data.BuildKPIs(ds.Rows, metricCol, nil), sql
	}
	kpis := make([]map[string]string, 0)
	for _, row := range results {
		if total, ok := row["total"]; ok && total != "" {
			kpis = append(kpis, map[string]string{"label": "Total", "value": total, "change": "Sum"})
		}
		if avg, ok := row["avg_val"]; ok && avg != "" {
			kpis = append(kpis, map[string]string{"label": "Average", "value": avg, "change": "Per row"})
		}
		if cnt, ok := row["row_count"]; ok && cnt != "" {
			kpis = append(kpis, map[string]string{"label": "Rows", "value": cnt, "change": "Count"})
		}
		break
	}
	return kpis, ""
}

func duckDBTrend(duckDB *data.DuckDBEngine, ds *data.Dataset, dateCol, metricCol *data.Column) ([]map[string]interface{}, string) {
	if metricCol == nil || dateCol == nil {
		return []map[string]interface{}{}, ""
	}
	qDate := quoteIdent(dateCol.Name)
	qMetric := quoteIdent(metricCol.Name)
	sql := fmt.Sprintf(`SELECT 
		CAST(%s AS VARCHAR) as label,
		SUM(CAST(%s AS DOUBLE)) as value
	FROM data 
	WHERE CAST(%s AS DOUBLE) IS NOT NULL AND %s IS NOT NULL
	GROUP BY %s
	ORDER BY label
	LIMIT 20`, qDate, qMetric, qMetric, qDate, qDate)
	results, err := duckDB.QueryTrend(ds.FilePath, dateCol.Name, metricCol.Name)
	if err != nil {
		log.Printf("[duckdb] Trend query failed: %v, falling back to in-memory", err)
		return data.BuildTrend(ds.Rows, dateCol, metricCol), sql
	}
	out := make([]map[string]interface{}, len(results))
	for i, r := range results {
		out[i] = make(map[string]interface{})
		for k, v := range r {
			out[i][k] = v
		}
	}
	return out, sql
}

func duckDBSegments(duckDB *data.DuckDBEngine, ds *data.Dataset, catCol, metricCol *data.Column) ([]map[string]interface{}, string) {
	if metricCol == nil || catCol == nil {
		return []map[string]interface{}{}, ""
	}
	qCat := quoteIdent(catCol.Name)
	qMetric := quoteIdent(metricCol.Name)
	sql := fmt.Sprintf(`SELECT 
		CAST(%s AS VARCHAR) as label,
		SUM(CAST(%s AS DOUBLE)) as value
	FROM data 
	WHERE CAST(%s AS DOUBLE) IS NOT NULL AND %s IS NOT NULL AND CAST(%s AS VARCHAR) != ''
	GROUP BY %s
	ORDER BY value DESC
	LIMIT 10`, qCat, qMetric, qMetric, qCat, qCat, qCat)
	results, err := duckDB.QuerySegments(ds.FilePath, catCol.Name, metricCol.Name)
	if err != nil {
		log.Printf("[duckdb] Segment query failed: %v, falling back to in-memory", err)
		return data.BuildSegments(ds.Rows, catCol, metricCol), sql
	}
	out := make([]map[string]interface{}, len(results))
	for i, r := range results {
		out[i] = make(map[string]interface{})
		for k, v := range r {
			out[i][k] = v
		}
	}
	return out, sql
}

// execLiveSQL executes SQL queries against a live database connection for the dataset.
func (h *Handler) execLiveSQL(ds *data.Dataset, metricCol, catCol, dateCol *data.Column, agg string) (*agent.DashboardSpec, []string, error) {
	connStr, ok := h.resolveConnStrByConfigID(ds.ConnectionConfigID, ds.ConnectionString)
	if !ok || ds.TableName == "" {
		return nil, nil, fmt.Errorf("dataset has no live database connection")
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}
	defer db.Close()
	db.SetConnMaxLifetime(10 * time.Second)

	var sqlQueries []string
	table := quoteIdent(ds.TableName)

	var kpis []map[string]string
	if metricCol != nil {
		qMetric := quoteIdent(metricCol.Name)
		kpiSQL := fmt.Sprintf(`SELECT 
			SUM(CAST(%s AS DOUBLE PRECISION)) as total,
			AVG(CAST(%s AS DOUBLE PRECISION)) as average,
			MIN(CAST(%s AS DOUBLE PRECISION)) as min_val,
			MAX(CAST(%s AS DOUBLE PRECISION)) as max_val,
			COUNT(*) as row_count
		FROM %s`,
			qMetric, qMetric, qMetric, qMetric, table)
		sqlQueries = append(sqlQueries, "-- KPI\n"+kpiSQL)
		rows, err := db.Query(kpiSQL)
		if err == nil {
			kpis = scanKPIRows(rows)
			rows.Close()
		} else {
			log.Printf("[livedb] KPI query failed: %v", err)
		}
	}

	var trend []map[string]interface{}
	if dateCol != nil && metricCol != nil {
		qDate := quoteIdent(dateCol.Name)
		qMetric := quoteIdent(metricCol.Name)
		trendSQL := fmt.Sprintf(`SELECT 
			CAST(%s AS VARCHAR) as label,
			SUM(CAST(%s AS DOUBLE PRECISION)) as value
		FROM %s
		WHERE %s IS NOT NULL
		GROUP BY %s
		ORDER BY label
		LIMIT 20`,
			qDate, qMetric, table, qDate, qDate)
		sqlQueries = append(sqlQueries, "-- Trend\n"+trendSQL)
		rows, err := db.Query(trendSQL)
		if err == nil {
			trend = scanTrendRows(rows)
			rows.Close()
		} else {
			log.Printf("[livedb] Trend query failed: %v", err)
		}
	}

	var segments []map[string]interface{}
	if catCol != nil && metricCol != nil {
		qCat := quoteIdent(catCol.Name)
		qMetric := quoteIdent(metricCol.Name)
		segSQL := fmt.Sprintf(`SELECT 
			CAST(%s AS VARCHAR) as label,
			SUM(CAST(%s AS DOUBLE PRECISION)) as value
		FROM %s
		WHERE %s IS NOT NULL AND %s != ''
		GROUP BY %s
		ORDER BY value DESC
		LIMIT 10`,
			qCat, qMetric, table, qCat, qCat, qCat)
		sqlQueries = append(sqlQueries, "-- Segments\n"+segSQL)
		rows, err := db.Query(segSQL)
		if err == nil {
			segments = scanSegmentRows(rows)
			rows.Close()
		} else {
			log.Printf("[livedb] Segment query failed: %v", err)
		}
	}

	if len(kpis) == 0 {
		kpis = []map[string]string{{"label": "No Data", "value": "—", "change": ""}}
	}

	dash := &agent.DashboardSpec{
		Title:           fmt.Sprintf("Live Query — %s.%s", ds.Filename, ds.TableName),
		KPIs:            kpis,
		Trend:           trend,
		Segments:        segments,
		Recommendations: []string{"Data queried directly from connected database."},
	}
	return dash, sqlQueries, nil
}

func scanKPIRows(rows *sql.Rows) []map[string]string {
	var results []map[string]string
	for rows.Next() {
		var total, avg sql.NullFloat64
		var rowCnt sql.NullInt64
		if err := rows.Scan(&total, &avg, &sql.NullFloat64{}, &sql.NullFloat64{}, &rowCnt); err != nil {
			continue
		}
		kpis := make([]map[string]string, 0)
		if total.Valid {
			kpis = append(kpis, map[string]string{"label": "Total", "value": fmt.Sprintf("%.0f", total.Float64), "change": "Sum"})
		}
		if avg.Valid {
			kpis = append(kpis, map[string]string{"label": "Average", "value": fmt.Sprintf("%.2f", avg.Float64), "change": "Per row"})
		}
		if rowCnt.Valid {
			kpis = append(kpis, map[string]string{"label": "Rows", "value": fmt.Sprintf("%d", rowCnt.Int64), "change": "Count"})
		}
		results = kpis
		break
	}
	return results
}

func scanTrendRows(rows *sql.Rows) []map[string]interface{} {
	var results []map[string]interface{}
	for rows.Next() {
		var label string
		var value sql.NullFloat64
		if err := rows.Scan(&label, &value); err != nil {
			continue
		}
		r := map[string]interface{}{"label": label}
		if value.Valid {
			r["value"] = value.Float64
		} else {
			r["value"] = 0
		}
		results = append(results, r)
	}
	return results
}

func scanSegmentRows(rows *sql.Rows) []map[string]interface{} {
	var results []map[string]interface{}
	for rows.Next() {
		var label string
		var value sql.NullFloat64
		if err := rows.Scan(&label, &value); err != nil {
			continue
		}
		r := map[string]interface{}{"label": label}
		if value.Valid {
			r["value"] = value.Float64
		} else {
			r["value"] = 0
		}
		results = append(results, r)
	}
	return results
}

func applyAggregation(kpis []map[string]string, agg string) []map[string]string {
	if len(kpis) == 0 {
		return kpis
	}
	switch agg {
	case "avg":
		for i := range kpis {
			if kpis[i]["label"] == "Total" || (len(kpis[i]["label"]) > 6 && kpis[i]["label"][:6] == "Total ") {
				kpis[i]["label"] = "Average"
				kpis[i]["change"] = "Per row (avg)"
			}
		}
	case "count":
		for i := range kpis {
			kpis[i]["label"] = "Count"
			kpis[i]["change"] = "Total rows"
		}
	case "min", "max":
		for i := range kpis {
			kpis[i]["label"] = strings.ToUpper(agg) + " value"
			kpis[i]["change"] = "Dataset range"
		}
	}
	return kpis
}

func exportHeaders(datasets []*data.Dataset) []string {
	seen := map[string]bool{"source_dataset": true}
	headers := []string{"source_dataset"}
	for _, dataset := range datasets {
		for _, column := range dataset.Profile.Columns {
			if !seen[column.Name] {
				seen[column.Name] = true
				headers = append(headers, column.Name)
			}
		}
	}
	if len(headers) == 1 {
		keys := make([]string, 0)
		for _, dataset := range datasets {
			for _, row := range dataset.Rows {
				for key := range row {
					if !seen[key] {
						seen[key] = true
						keys = append(keys, key)
					}
				}
			}
		}
		sort.Strings(keys)
		headers = append(headers, keys...)
	}
	return headers
}

func resolveColumns(ds *data.Dataset, plan *agent.LLMPlan) (metricCol, catCol, dateCol *data.Column) {
	if plan != nil {
		for i := range ds.Profile.Columns {
			c := &ds.Profile.Columns[i]
			if c.Name == plan.MetricColumn {
				metricCol = c
			}
			if c.Name == plan.CategoryColumn {
				catCol = c
			}
			if c.Name == plan.DateColumn {
				dateCol = c
			}
		}
	}
	if metricCol == nil {
		for i := range ds.Profile.Columns {
			if ds.Profile.Columns[i].Type == "number" {
				metricCol = &ds.Profile.Columns[i]
				break
			}
		}
	}
	if catCol == nil {
		used := map[string]bool{}
		if metricCol != nil {
			used[metricCol.Name] = true
		}
		if dateCol != nil {
			used[dateCol.Name] = true
		}
		preferred := []string{"segment", "category", "region", "product", "department", "channel"}
		for _, name := range preferred {
			for i := range ds.Profile.Columns {
				if ds.Profile.Columns[i].Type == "text" && !used[ds.Profile.Columns[i].Name] &&
					strings.EqualFold(ds.Profile.Columns[i].Name, name) {
					catCol = &ds.Profile.Columns[i]
					break
				}
			}
			if catCol != nil {
				break
			}
		}
		if catCol == nil {
			for i := range ds.Profile.Columns {
				if ds.Profile.Columns[i].Type == "text" && !used[ds.Profile.Columns[i].Name] {
					catCol = &ds.Profile.Columns[i]
					break
				}
			}
		}
	}
	if dateCol == nil {
		for i := range ds.Profile.Columns {
			if ds.Profile.Columns[i].Type == "date" {
				dateCol = &ds.Profile.Columns[i]
				break
			}
		}
	}
	return
}

func colRes(c *data.Column) string {
	if c == nil {
		return "<nil>"
	}
	return c.Name
}
