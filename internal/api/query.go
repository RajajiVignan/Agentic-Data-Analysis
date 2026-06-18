package api

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"insightpilot/internal/data"
)

func (h *Handler) handleQuery(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DatasetIDs  []string `json:"datasetIds"`
		SQL         string   `json:"sql"`
		Page        int      `json:"page"`
		PageSize    int      `json:"pageSize"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.SQL == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "sql is required"})
		return
	}
	if len(body.DatasetIDs) == 0 {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "datasetIds is required"})
		return
	}
	if body.Page <= 0 {
		body.Page = 1
	}
	if body.PageSize <= 0 || body.PageSize > 1000 {
		body.PageSize = 100
	}

	h.mu.RLock()
	var datasets []*data.Dataset
	for _, id := range body.DatasetIDs {
		if d, ok := h.datasets[id]; ok {
			datasets = append(datasets, d)
		}
	}
	h.mu.RUnlock()

	if len(datasets) == 0 {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "No datasets found"})
		return
	}

	result, columns, err := h.executeSQL(datasets, body.SQL, body.Page, body.PageSize)
	if err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Query failed: " + err.Error()})
		return
	}

	totalRows := len(result)
	hasMore := totalRows >= body.PageSize

	SendJSON(w, http.StatusOK, map[string]interface{}{
		"columns":   columns,
		"rows":      result,
		"page":      body.Page,
		"pageSize":  body.PageSize,
		"hasMore":   hasMore,
		"totalRows": totalRows,
	})
}

func (h *Handler) executeSQL(datasets []*data.Dataset, sql string, page, pageSize int) ([]map[string]string, []string, error) {
	if err := validateSelectOnly(sql); err != nil {
		return nil, nil, err
	}

	timeoutSec := 30
	if v := os.Getenv("QUERY_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeoutSec = n
		}
	}

	duckDB := getDuckDBEngine()
	if duckDB != nil {
		var csvPaths []string
		for _, ds := range datasets {
			if ds.FilePath != "" {
				csvPaths = append(csvPaths, ds.FilePath)
			}
		}
		if len(csvPaths) > 0 {
			ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
			defer cancel()
			return duckDB.ExecuteQueryWithContext(ctx, csvPaths, sql, page, pageSize)
		}
	}

	primary := datasets[0]
	if len(datasets) > 1 {
		primary = data.MergeDatasets(datasets)
	}
	return executeInMemorySQL(primary, sql, page, pageSize)
}

// validateSelectOnly rejects any SQL that is not a SELECT statement.
// This prevents INSERT, UPDATE, DELETE, DROP, and other destructive operations.
func validateSelectOnly(sql string) error {
	trimmed := strings.TrimSpace(sql)
	trimmed = strings.TrimSuffix(trimmed, ";")
	trimmed = strings.TrimSpace(trimmed)

	if trimmed == "" {
		return fmt.Errorf("SQL query is empty")
	}

	lower := strings.ToLower(trimmed)

	// Must start with SELECT
	if !strings.HasPrefix(lower, "select") {
		return fmt.Errorf("only SELECT queries are allowed")
	}

	// Block dangerous keywords anywhere in the query
	blocked := []string{
		"insert ", "update ", "delete ", "drop ",
		"alter ", "create ", "truncate ", "replace ",
		"exec ", "execute ", "call ",
	}
	for _, kw := range blocked {
		if strings.Contains(lower, kw) {
			return fmt.Errorf("SQL contains forbidden keyword: %q", strings.TrimSpace(kw))
		}
	}

	// Block multi-statement (SQL injection via semi-colon)
	if strings.Contains(trimmed, ";") {
		// Allow a single trailing semicolon which was already stripped,
		// but reject if there's still one (2+ statements or mid-query)
		return fmt.Errorf("multi-statement queries are not allowed")
	}

	return nil
}

func (h *Handler) handleQuerySchema(w http.ResponseWriter, r *http.Request) {
	rawIDs := r.URL.Query().Get("datasetIds")
	if rawIDs == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "datasetIds query parameter required"})
		return
	}
	ids := strings.Split(rawIDs, ",")

	h.mu.RLock()
	defer h.mu.RUnlock()

	var schemas []map[string]interface{}
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if d, ok := h.datasets[id]; ok {
			cols := make([]map[string]interface{}, len(d.Profile.Columns))
			for i, c := range d.Profile.Columns {
				cols[i] = map[string]interface{}{
					"name":     c.Name,
					"type":     c.Type,
					"nonEmpty": c.NonEmpty,
				}
			}
			schemas = append(schemas, map[string]interface{}{
				"datasetId":  d.ID,
				"filename":   d.Filename,
				"rowCount":   d.Profile.RowCount,
				"columns":    cols,
				"tableAlias": fmt.Sprintf("ds_%s", strings.Map(func(r rune) rune {
					if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
						return r
					}
					return '_'
				}, strings.ToLower(d.Filename))),
			})
		}
	}

	SendJSON(w, http.StatusOK, map[string]interface{}{"schemas": schemas})
}

func (h *Handler) handleQueryVisualize(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DatasetID string `json:"datasetId"`
		SQL       string `json:"sql"`
		ChartType string `json:"chartType"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.DatasetID == "" || body.SQL == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "datasetId and sql are required"})
		return
	}

	h.mu.RLock()
	ds, ok := h.datasets[body.DatasetID]
	h.mu.RUnlock()
	if !ok {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Dataset not found"})
		return
	}

	result, columns, err := h.executeSQL([]*data.Dataset{ds}, body.SQL, 1, 1000)
	if err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Query failed: " + err.Error()})
		return
	}

	chartType := body.ChartType
	if chartType == "" {
		chartType = "bar"
	}

	SendJSON(w, http.StatusOK, map[string]interface{}{
		"columns":   columns,
		"rows":      result,
		"chartType": chartType,
		"rowCount":  len(result),
	})
}

func executeInMemorySQL(ds *data.Dataset, sql string, page, pageSize int) ([]map[string]string, []string, error) {
	lower := strings.ToLower(strings.TrimSpace(sql))

	var columns []string
	for _, c := range ds.Profile.Columns {
		columns = append(columns, c.Name)
	}

	if strings.HasPrefix(lower, "select") {
		result := simpleInMemorySelect(ds, sql, columns)
		return paginateResult(result, columns, page, pageSize)
	}
	if strings.HasPrefix(lower, "count") || strings.Contains(lower, "count(") {
		count := len(ds.Rows)
		row := map[string]string{"count": fmt.Sprintf("%d", count)}
		return []map[string]string{row}, []string{"count"}, nil
	}

	log.Printf("[query] Falling back to returning all rows for unsupported SQL: %s", sql)
	return paginateResult(rowsToMapSlice(ds), columns, page, pageSize)
}

func simpleInMemorySelect(ds *data.Dataset, sql string, columns []string) []map[string]string {
	lower := strings.ToLower(sql)
	var results []map[string]string

	limit := 1000
	if idx := strings.LastIndex(lower, "limit"); idx >= 0 {
		after := strings.TrimSpace(lower[idx+5:])
		after = strings.TrimSuffix(after, ";")
		fmt.Sscanf(after, "%d", &limit)
	}

	orderCol := ""
	orderDesc := false
	if idx := strings.Index(lower, "order by"); idx >= 0 {
		after := strings.TrimSpace(lower[idx+8:])
		if end := strings.Index(after, "limit"); end >= 0 {
			after = strings.TrimSpace(after[:end])
		}
		parts := strings.Fields(after)
		if len(parts) > 0 {
			orderCol = strings.Trim(parts[0], "\"`")
			if len(parts) > 1 && strings.ToLower(parts[1]) == "desc" {
				orderDesc = true
			}
		}
	}

	var whereCol, whereOp, whereVal string
	if idx := strings.Index(lower, "where"); idx >= 0 {
		after := strings.TrimSpace(lower[idx+5:])
		end := len(after)
		if oidx := strings.Index(after, "order"); oidx >= 0 {
			end = oidx
		}
		if lidx := strings.Index(after, "limit"); lidx >= 0 && lidx < end {
			end = lidx
		}
		clause := strings.TrimSpace(after[:end])
		parts := strings.Fields(clause)
		if len(parts) >= 3 {
			whereCol = strings.Trim(parts[0], "\"`")
			whereOp = strings.ToLower(parts[1])
			whereVal = strings.Trim(parts[2], "'\"")
		}
	}

	results = make([]map[string]string, 0, len(ds.Rows))
	for _, row := range ds.Rows {
		if whereCol != "" {
			val := row[whereCol]
			switch whereOp {
			case "=", "==", "eq":
				if !strings.EqualFold(val, whereVal) {
					continue
				}
			case "!=", "<>", "neq":
				if strings.EqualFold(val, whereVal) {
					continue
				}
			case ">":
				if !isGreater(val, whereVal) {
					continue
				}
			case "<":
				if !isLess(val, whereVal) {
					continue
				}
			case ">=":
				if isLess(val, whereVal) {
					continue
				}
			case "<=":
				if isGreater(val, whereVal) {
					continue
				}
			}
		}
		newRow := make(map[string]string, len(columns))
		for _, c := range columns {
			newRow[c] = row[c]
		}
		results = append(results, newRow)
	}

	if orderCol != "" {
		sortRows(results, orderCol, orderDesc)
	}

	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}

func sortRows(rows []map[string]string, col string, desc bool) {
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			less := rows[i][col] < rows[j][col]
			if desc {
				less = rows[i][col] > rows[j][col]
			}
			if less {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
}

func isGreater(a, b string) bool {
	var fa, fb float64
	if _, err := fmt.Sscanf(a, "%f", &fa); err == nil {
		if _, err := fmt.Sscanf(b, "%f", &fb); err == nil {
			return fa > fb
		}
	}
	return a > b
}

func isLess(a, b string) bool {
	var fa, fb float64
	if _, err := fmt.Sscanf(a, "%f", &fa); err == nil {
		if _, err := fmt.Sscanf(b, "%f", &fb); err == nil {
			return fa < fb
		}
	}
	return a < b
}

func paginateResult(rows []map[string]string, columns []string, page, pageSize int) ([]map[string]string, []string, error) {
	start := (page - 1) * pageSize
	if start >= len(rows) {
		return []map[string]string{}, columns, nil
	}
	end := start + pageSize
	if end > len(rows) {
		end = len(rows)
	}
	return rows[start:end], columns, nil
}

func rowsToMapSlice(ds *data.Dataset) []map[string]string {
	return ds.Rows
}
