package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"insightpilot/internal/data"
)

type exploreRequest struct {
	DatasetID string             `json:"datasetId"`
	VizType   string             `json:"vizType"` // bar | line | area | pie | scatter | kpi | pivottable | heatmap | sankey | sunburst
	Dimension data.DimensionRef  `json:"dimension"`
	Metric    data.MetricRef     `json:"metric"`
	Sort      string             `json:"sort"`  // asc | desc
	Limit     int                `json:"limit"`
	// Secondary dimension (column / target / level-2) for pivottable, sankey, sunburst.
	Dimension2 data.DimensionRef `json:"dimension2"`
	// Scatter-only numeric columns
	XColumn string `json:"xColumn"`
	YColumn string `json:"yColumn"`
}

func (h *Handler) handleExplore(w http.ResponseWriter, r *http.Request) {
	var req exploreRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.DatasetID == "" {
		sendInvalidRequest(w, "datasetId is required")
		return
	}
	if req.Limit <= 0 || req.Limit > 200 {
		req.Limit = 50
	}

	h.mu.RLock()
	ds, ok := h.datasets[req.DatasetID]
	h.mu.RUnlock()
	if !ok {
		sendNotFound(w, "Dataset not found")
		return
	}

	cols := data.NewColumnSetFromProfile(ds.Profile)

	switch req.VizType {
	case "kpi":
		h.exploreKPI(w, ds, cols, req)
		return
	case "scatter":
		h.exploreScatter(w, ds, cols, req)
		return
	case "pivottable":
		h.explorePivotTable(w, ds, cols, req)
		return
	case "heatmap":
		h.exploreHeatmap(w, ds, cols, req)
		return
	case "sankey":
		h.exploreSankey(w, ds, cols, req)
		return
	case "sunburst":
		h.exploreSunburst(w, ds, cols, req)
		return
	}

	dimExpr, err := h.resolveDimension(req.Dimension, ds, cols)
	if err != nil {
		sendInvalidRequest(w, err.Error())
		return
	}
	metricExpr, err := h.resolveMetric(req.Metric, ds, cols)
	if err != nil {
		sendInvalidRequest(w, err.Error())
		return
	}

	order := "DESC"
	if req.Sort == "asc" {
		order = "ASC"
	}

	sql := fmt.Sprintf(
		`SELECT %s AS label, %s AS value FROM data WHERE %s IS NOT NULL GROUP BY label ORDER BY value %s LIMIT %d`,
		dimExpr, metricExpr, dimExpr, order, req.Limit,
	)

	rows, err := h.cachedExploreSQL(ds, sql)
	if err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Explore query failed: " + err.Error()})
		return
	}

	points := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		var v float64
		if f, e := strconv.ParseFloat(row["value"], 64); e == nil {
			v = f
		}
		points = append(points, map[string]interface{}{
			"label": row["label"],
			"value": v,
		})
	}

	SendJSON(w, http.StatusOK, map[string]interface{}{
		"vizType": req.VizType,
		"points":  points,
	})
}

func (h *Handler) exploreKPI(w http.ResponseWriter, ds *data.Dataset, cols data.ColumnSet, req exploreRequest) {
	baseExpr, isAggField, err := h.resolveMetricBase(req.Metric, ds, cols)
	if err != nil {
		sendInvalidRequest(w, err.Error())
		return
	}
	// A computed metric field of aggregate mode is already aggregated, so we
	// cannot wrap it in SUM/AVG again. Use it directly for total and avg.
	totalExpr, avgExpr := baseExpr, baseExpr
	if !isAggField {
		totalExpr = fmt.Sprintf("SUM(%s)", baseExpr)
		avgExpr = fmt.Sprintf("AVG(%s)", baseExpr)
	}
	sql := fmt.Sprintf(
		`SELECT %s AS total, %s AS avg_val, COUNT(*) AS row_count FROM data`,
		totalExpr, avgExpr,
	)
	rows, err := h.cachedExploreSQL(ds, sql)
	if err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "KPI query failed: " + err.Error()})
		return
	}
	total, avg, cnt := 0.0, 0.0, 0
	if len(rows) > 0 {
		total, _ = strconv.ParseFloat(rows[0]["total"], 64)
		avg, _ = strconv.ParseFloat(rows[0]["avg_val"], 64)
		cnt, _ = strconv.Atoi(rows[0]["row_count"])
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{
		"vizType": "kpi",
		"kpis": []map[string]interface{}{
			{"label": "Total " + h.metricLabel(req.Metric), "value": total, "change": "Sum"},
			{"label": "Average " + h.metricLabel(req.Metric), "value": avg, "change": "Per row"},
			{"label": "Rows", "value": cnt, "change": "Count"},
		},
	})
}

func (h *Handler) exploreScatter(w http.ResponseWriter, ds *data.Dataset, cols data.ColumnSet, req exploreRequest) {
	if req.XColumn == "" || req.YColumn == "" {
		sendInvalidRequest(w, "xColumn and yColumn are required for scatter")
		return
	}
	xCol, err := quoteIdentSafeForExplore(req.XColumn, cols)
	if err != nil {
		sendInvalidRequest(w, err.Error())
		return
	}
	yCol, err := quoteIdentSafeForExplore(req.YColumn, cols)
	if err != nil {
		sendInvalidRequest(w, err.Error())
		return
	}
	sql := fmt.Sprintf(
		`SELECT CAST(%s AS DOUBLE) AS x, CAST(%s AS DOUBLE) AS y FROM data WHERE %s IS NOT NULL AND %s IS NOT NULL LIMIT %d`,
		xCol, yCol, xCol, yCol, req.Limit,
	)
	rows, err := h.cachedExploreSQL(ds, sql)
	if err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Scatter query failed: " + err.Error()})
		return
	}
	points := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		x, _ := strconv.ParseFloat(row["x"], 64)
		y, _ := strconv.ParseFloat(row["y"], 64)
		points = append(points, map[string]interface{}{"x": x, "y": y, "label": fmt.Sprintf("%.0f", x)})
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{
		"vizType": "scatter",
		"points":  points,
	})
}

// explorePivotTable builds a two-dimensional aggregation (rows x columns) of a
// metric. It returns the row/column labels and a value matrix for the frontend.
func (h *Handler) explorePivotTable(w http.ResponseWriter, ds *data.Dataset, cols data.ColumnSet, req exploreRequest) {
	rowExpr, err := h.resolveDimension(req.Dimension, ds, cols)
	if err != nil {
		sendInvalidRequest(w, err.Error())
		return
	}
	colExpr, err := h.resolveDimension(req.Dimension2, ds, cols)
	if err != nil {
		sendInvalidRequest(w, "dimension2 (column) is required for pivot table: "+err.Error())
		return
	}
	metricExpr, err := h.resolveMetric(req.Metric, ds, cols)
	if err != nil {
		sendInvalidRequest(w, err.Error())
		return
	}
	sql := fmt.Sprintf(
		`SELECT %s AS r, %s AS c, %s AS v FROM data WHERE %s IS NOT NULL AND %s IS NOT NULL GROUP BY r, c ORDER BY r, c LIMIT %d`,
		rowExpr, colExpr, metricExpr, rowExpr, colExpr, req.Limit*req.Limit,
	)
	rows, err := h.cachedExploreSQL(ds, sql)
	if err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Pivot query failed: " + err.Error()})
		return
	}
	rowIndex := make(map[string]int)
	colIndex := make(map[string]int)
	var rowLabels, colLabels []string
	cells := make([][]float64, 0)
	for _, row := range rows {
		var v float64
		if f, e := strconv.ParseFloat(row["v"], 64); e == nil {
			v = f
		}
		ri, ok := rowIndex[row["r"]]
		if !ok {
			ri = len(rowLabels)
			rowIndex[row["r"]] = ri
			rowLabels = append(rowLabels, row["r"])
			cells = append(cells, make([]float64, 0))
		}
		ci, ok := colIndex[row["c"]]
		if !ok {
			ci = len(colLabels)
			colIndex[row["c"]] = ci
			colLabels = append(colLabels, row["c"])
		}
		// ensure each row slice has same length
		for len(cells[ri]) <= ci {
			cells[ri] = append(cells[ri], 0)
		}
		cells[ri][ci] = v
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{
		"vizType":   "pivottable",
		"rowLabels": rowLabels,
		"colLabels": colLabels,
		"cells":     cells,
	})
}

// exploreHeatmap aggregates a metric over a date/period dimension to feed a
// calendar heatmap (one value per period label).
func (h *Handler) exploreHeatmap(w http.ResponseWriter, ds *data.Dataset, cols data.ColumnSet, req exploreRequest) {
	dimExpr, err := h.resolveDimension(req.Dimension, ds, cols)
	if err != nil {
		sendInvalidRequest(w, err.Error())
		return
	}
	metricExpr, err := h.resolveMetric(req.Metric, ds, cols)
	if err != nil {
		sendInvalidRequest(w, err.Error())
		return
	}
	sql := fmt.Sprintf(
		`SELECT %s AS label, %s AS value FROM data WHERE %s IS NOT NULL GROUP BY label ORDER BY label LIMIT %d`,
		dimExpr, metricExpr, dimExpr, req.Limit,
	)
	rows, err := h.cachedExploreSQL(ds, sql)
	if err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Heatmap query failed: " + err.Error()})
		return
	}
	points := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		var v float64
		if f, e := strconv.ParseFloat(row["value"], 64); e == nil {
			v = f
		}
		points = append(points, map[string]interface{}{"label": row["label"], "value": v})
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{
		"vizType": "heatmap",
		"points":  points,
	})
}

// exploreSankey aggregates a metric between a source and target dimension to
// build flow links for a Sankey diagram.
func (h *Handler) exploreSankey(w http.ResponseWriter, ds *data.Dataset, cols data.ColumnSet, req exploreRequest) {
	srcExpr, err := h.resolveDimension(req.Dimension, ds, cols)
	if err != nil {
		sendInvalidRequest(w, err.Error())
		return
	}
	tgtExpr, err := h.resolveDimension(req.Dimension2, ds, cols)
	if err != nil {
		sendInvalidRequest(w, "dimension2 (target) is required for sankey: "+err.Error())
		return
	}
	metricExpr, err := h.resolveMetric(req.Metric, ds, cols)
	if err != nil {
		sendInvalidRequest(w, err.Error())
		return
	}
	sql := fmt.Sprintf(
		`SELECT %s AS source, %s AS target, %s AS value FROM data WHERE %s IS NOT NULL AND %s IS NOT NULL GROUP BY source, target LIMIT %d`,
		srcExpr, tgtExpr, metricExpr, srcExpr, tgtExpr, req.Limit*req.Limit,
	)
	rows, err := h.cachedExploreSQL(ds, sql)
	if err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Sankey query failed: " + err.Error()})
		return
	}
	links := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		var v float64
		if f, e := strconv.ParseFloat(row["value"], 64); e == nil {
			v = f
		}
		links = append(links, map[string]interface{}{
			"source": row["source"],
			"target": row["target"],
			"value":  v,
		})
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{
		"vizType": "sankey",
		"links":   links,
	})
}

// exploreSunburst aggregates a metric into a two-level hierarchy for a sunburst
// diagram (level 1 = dimension, level 2 = dimension2).
func (h *Handler) exploreSunburst(w http.ResponseWriter, ds *data.Dataset, cols data.ColumnSet, req exploreRequest) {
	l1Expr, err := h.resolveDimension(req.Dimension, ds, cols)
	if err != nil {
		sendInvalidRequest(w, err.Error())
		return
	}
	l2Expr, err := h.resolveDimension(req.Dimension2, ds, cols)
	if err != nil {
		sendInvalidRequest(w, "dimension2 (inner ring) is required for sunburst: "+err.Error())
		return
	}
	metricExpr, err := h.resolveMetric(req.Metric, ds, cols)
	if err != nil {
		sendInvalidRequest(w, err.Error())
		return
	}
	sql := fmt.Sprintf(
		`SELECT %s AS l1, %s AS l2, %s AS value FROM data WHERE %s IS NOT NULL AND %s IS NOT NULL GROUP BY l1, l2 LIMIT %d`,
		l1Expr, l2Expr, metricExpr, l1Expr, l2Expr, req.Limit*req.Limit,
	)
	rows, err := h.cachedExploreSQL(ds, sql)
	if err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Sunburst query failed: " + err.Error()})
		return
	}
	childrenOf := make(map[string][]map[string]interface{})
	for _, row := range rows {
		var v float64
		if f, e := strconv.ParseFloat(row["value"], 64); e == nil {
			v = f
		}
		childrenOf[row["l1"]] = append(childrenOf[row["l1"]], map[string]interface{}{
			"name":  row["l2"],
			"value": v,
		})
	}
	nodes := make([]map[string]interface{}, 0, len(childrenOf))
	for parent, children := range childrenOf {
		nodes = append(nodes, map[string]interface{}{
			"name":     parent,
			"children": children,
		})
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{
		"vizType": "sunburst",
		"nodes":   nodes,
	})
}

// runExploreSQL executes an explore query. It uses the single-CSV DuckDB path
// (which registers the table as `data`) when a file is available, otherwise it
// falls back to the in-memory SQL engine.
func (h *Handler) runExploreSQL(ds *data.Dataset, sql string) ([]map[string]string, error) {
	if err := validateSelectOnly(sql); err != nil {
		return nil, err
	}
	if h.duckdb != nil && ds.FilePath != "" {
		return h.duckdb.ExecuteSingleCSV(ds.FilePath, sql)
	}
	rows, _, err := executeInMemorySQL(ds, sql, 1, 200)
	return rows, err
}

func (h *Handler) resolveDimension(ref data.DimensionRef, ds *data.Dataset, cols data.ColumnSet) (string, error) {
	if ref.Type == "field" && ref.ID != "" {
		f := h.semanticSvc.Get(ref.ID)
		if f == nil || f.Kind != "dimension" {
			return "", fmt.Errorf("dimension field not found: %s", ref.ID)
		}
		return f.BuildDimensionExpr(cols)
	}
	if ref.Name == "" {
		return "", fmt.Errorf("dimension name is required")
	}
	return quoteIdentSafeForExplore(ref.Name, cols)
}

func (h *Handler) resolveMetric(ref data.MetricRef, ds *data.Dataset, cols data.ColumnSet) (string, error) {
	if ref.Type == "field" && ref.ID != "" {
		f := h.semanticSvc.Get(ref.ID)
		if f == nil || f.Kind != "metric" {
			return "", fmt.Errorf("metric field not found: %s", ref.ID)
		}
		return f.BuildMetricExpr(cols)
	}
	if ref.Name == "" {
		return "", fmt.Errorf("metric name is required")
	}
	agg := strings.ToLower(ref.Agg)
	if agg == "" {
		agg = "sum"
	}
	if !data.AllowedAggFns[agg] {
		return "", fmt.Errorf("unsupported aggregation: %q", ref.Agg)
	}
	if !cols.Has(ref.Name) {
		return "", fmt.Errorf("unknown column: %q", ref.Name)
	}
	col, err := quoteIdentSafeForExplore(ref.Name, cols)
	if err != nil {
		return "", err
	}
	if agg == "count" {
		return fmt.Sprintf("COUNT(%s)", col), nil
	}
	return fmt.Sprintf("%s(CAST(%s AS DOUBLE))", strings.ToUpper(agg), col), nil
}

// resolveMetricBase returns the (non-aggregated) metric expression plus a flag
// indicating whether it is already an aggregate (a computed field of aggregate
// mode). For column metrics it returns CAST(col AS DOUBLE) so callers can apply
// exactly one aggregate; for aggregate-mode fields it returns the pre-aggregated
// expression which must not be wrapped again.
func (h *Handler) resolveMetricBase(ref data.MetricRef, ds *data.Dataset, cols data.ColumnSet) (expr string, isAggField bool, err error) {
	if ref.Type == "field" && ref.ID != "" {
		f := h.semanticSvc.Get(ref.ID)
		if f == nil || f.Kind != "metric" {
			return "", false, fmt.Errorf("metric field not found: %s", ref.ID)
		}
		expr, err = f.BuildMetricExpr(cols)
		if err != nil {
			return "", false, err
		}
		isAggField = f.IsAggregateField()
		return expr, isAggField, nil
	}
	if ref.Name == "" {
		return "", false, fmt.Errorf("metric name is required")
	}
	agg := strings.ToLower(ref.Agg)
	if agg == "" {
		agg = "sum"
	}
	if !data.AllowedAggFns[agg] {
		return "", false, fmt.Errorf("unsupported aggregation: %q", ref.Agg)
	}
	if !cols.Has(ref.Name) {
		return "", false, fmt.Errorf("unknown column: %q", ref.Name)
	}
	col, err := quoteIdentSafeForExplore(ref.Name, cols)
	if err != nil {
		return "", false, err
	}
	if agg == "count" {
		return fmt.Sprintf("COUNT(%s)", col), false, nil
	}
	return fmt.Sprintf("CAST(%s AS DOUBLE)", col), false, nil
}

// quoteIdentSafeForExplore quotes a column name and validates it exists.
func quoteIdentSafeForExplore(name string, cols data.ColumnSet) (string, error) {
	q, err := data.QuoteIdentSafe(name)
	if err != nil {
		return "", err
	}
	if !cols.Has(name) {
		return "", fmt.Errorf("unknown column: %q", name)
	}
	return q, nil
}

func (h *Handler) metricLabel(ref data.MetricRef) string {
	if ref.Type == "field" && ref.ID != "" {
		if f := h.semanticSvc.Get(ref.ID); f != nil {
			return f.Name
		}
	}
	if ref.Name != "" {
		return ref.Name
	}
	return "metric"
}

var _ = time.Now
