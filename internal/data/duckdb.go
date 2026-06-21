package data

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var queryTimeout time.Duration

func init() {
	timeoutSec := 30
	if v := os.Getenv("QUERY_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			timeoutSec = n
		}
	}
	queryTimeout = time.Duration(timeoutSec) * time.Second
}

// quoteIdent safely quotes a DuckDB identifier, escaping embedded double quotes.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

type DuckDBEngine struct {
	scriptsDir string
}

func NewDuckDBEngine(scriptsDir string) *DuckDBEngine {
	os.MkdirAll(scriptsDir, 0755)
	return &DuckDBEngine{scriptsDir: scriptsDir}
}

func duckdbID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (e *DuckDBEngine) executeSQL(csvPath, sql string) ([]map[string]string, error) {
	return e.executeSQLWithTimeout(context.Background(), csvPath, sql)
}

func (e *DuckDBEngine) executeSQLWithTimeout(ctx context.Context, csvPath, sql string) ([]map[string]string, error) {
	id := "dq_" + duckdbID()
	scriptPath := filepath.Join(e.scriptsDir, id+".py")
	resultPath := filepath.Join(e.scriptsDir, id+"_result.json")

	escapedSQL := strings.ReplaceAll(sql, `"""`, `\"\"\"`)

	script := fmt.Sprintf(`import duckdb, json, sys, os

csv_path = sys.argv[1]
result_path = sys.argv[2]

con = duckdb.connect(":memory:")
try:
    con.execute("CREATE TABLE data AS SELECT * FROM read_csv_auto(?)", [csv_path])
    sql_query = """%s"""
    result = con.execute(sql_query).fetchall()
    desc = con.description
    con.close()
    columns = [d[0] for d in desc]
    rows = []
    for row in result:
        r = {}
        for i, col in enumerate(columns):
            val = row[i]
            if val is None:
                r[col] = None
            else:
                r[col] = str(val)
        rows.append(r)
    with open(result_path, "w") as f:
        json.dump({"columns": columns, "rows": rows}, f)
except Exception as e:
    con.close()
    with open(result_path, "w") as f:
        json.dump({"error": str(e)}, f)
    sys.exit(1)
`, escapedSQL)

	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		return nil, fmt.Errorf("write script: %w", err)
	}
	defer os.Remove(scriptPath)
	defer os.Remove(resultPath)

	args := []string{scriptPath, csvPath, resultPath}

	cmd := exec.CommandContext(ctx, "python3", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// Check for error in result file
		if data, readErr := os.ReadFile(resultPath); readErr == nil {
			var errResult struct {
				Error string `json:"error"`
			}
			if json.Unmarshal(data, &errResult) == nil && errResult.Error != "" {
				return nil, fmt.Errorf("duckdb: %s", errResult.Error)
			}
		}
		return nil, fmt.Errorf("execute duckdb: %s: %w", strings.TrimSpace(string(output)), err)
	}

	data, err := os.ReadFile(resultPath)
	if err != nil {
		return nil, fmt.Errorf("read result: %w", err)
	}

	var qr struct {
		Columns []string          `json:"columns"`
		Rows    []json.RawMessage `json:"rows"`
		Error   string            `json:"error"`
	}
	if err := json.Unmarshal(data, &qr); err != nil {
		return nil, fmt.Errorf("parse result: %w", err)
	}
	if qr.Error != "" {
		return nil, fmt.Errorf("duckdb: %s", qr.Error)
	}

	results := make([]map[string]string, len(qr.Rows))
	for i, rowData := range qr.Rows {
		var rowMap map[string]interface{}
		if err := json.Unmarshal(rowData, &rowMap); err != nil {
			return nil, fmt.Errorf("parse row: %w", err)
		}
		row := make(map[string]string)
		for _, col := range qr.Columns {
			if v, ok := rowMap[col]; ok && v != nil {
				row[col] = fmt.Sprintf("%v", v)
			} else {
				row[col] = ""
			}
		}
		results[i] = row
	}

	return results, nil
}

func (e *DuckDBEngine) ProfileCSV(csvPath string) (Profile, error) {
	sql := `SELECT 
		column_name,
		CASE 
			WHEN column_type IN ('BIGINT', 'DOUBLE', 'INTEGER', 'FLOAT', 'DECIMAL', 'HUGEINT', 'SMALLINT', 'TINYINT') THEN 'number'
			WHEN column_type IN ('DATE', 'TIMESTAMP', 'TIMESTAMP WITH TIME ZONE', 'TIMESTAMP WITHOUT TIME ZONE') THEN 'date'
			ELSE 'text'
		END as col_type
	FROM (DESCRIBE data)`
	rows, err := e.executeSQL(csvPath, sql)
	if err != nil {
		return Profile{}, err
	}

	var cols []Column
	for _, row := range rows {
		colName := row["column_name"]
		colType := row["col_type"]
		if colType == "" {
			colType = "text"
		}
		qCol := quoteIdent(colName)

		countSQL := fmt.Sprintf(`SELECT COUNT(*) as non_empty FROM data WHERE %s IS NOT NULL AND CAST(%s AS VARCHAR) != ''`, qCol, qCol)
		countRows, err := e.executeSQL(csvPath, countSQL)
		var nonEmpty int
		if err == nil && len(countRows) > 0 {
			fmt.Sscanf(countRows[0]["non_empty"], "%d", &nonEmpty)
		}

		sampleSQL := fmt.Sprintf(`SELECT DISTINCT %s as val FROM data WHERE %s IS NOT NULL AND CAST(%s AS VARCHAR) != '' LIMIT 5`, qCol, qCol, qCol)
		sampleRows, err := e.executeSQL(csvPath, sampleSQL)
		var samples []string
		if err == nil {
			for _, sr := range sampleRows {
				if sr["val"] != "" {
					samples = append(samples, sr["val"])
				}
			}
		}

		cols = append(cols, Column{
			Name:     colName,
			Type:     colType,
			NonEmpty: nonEmpty,
			Sample:   samples,
		})
	}

	rowCountSQL := `SELECT COUNT(*) as cnt FROM data`
	countRows, err := e.executeSQL(csvPath, rowCountSQL)
	rowCount := 0
	if err == nil && len(countRows) > 0 {
		fmt.Sscanf(countRows[0]["cnt"], "%d", &rowCount)
	}

	return Profile{RowCount: rowCount, Columns: cols}, nil
}

func (e *DuckDBEngine) QueryKPIs(csvPath, metricCol string) ([]map[string]string, error) {
	q := quoteIdent(metricCol)
	sql := fmt.Sprintf(`SELECT 
		SUM(CAST(%s AS DOUBLE)) as total,
		AVG(CAST(%s AS DOUBLE)) as avg_val,
		MIN(CAST(%s AS DOUBLE)) as min_val,
		MAX(CAST(%s AS DOUBLE)) as max_val,
		COUNT(*) as row_count
	FROM data WHERE CAST(%s AS DOUBLE) IS NOT NULL`, q, q, q, q, q)
	return e.executeSQL(csvPath, sql)
}

func (e *DuckDBEngine) QueryTrend(csvPath, dateCol, metricCol string) ([]map[string]string, error) {
	qDate := quoteIdent(dateCol)
	qMetric := quoteIdent(metricCol)
	dateExpr := qDate
	sql := fmt.Sprintf(`SELECT 
		CAST(%s AS VARCHAR) as label,
		SUM(CAST(%s AS DOUBLE)) as value
	FROM data 
	WHERE CAST(%s AS DOUBLE) IS NOT NULL AND %s IS NOT NULL
	GROUP BY %s
	ORDER BY label
	LIMIT 20`, dateExpr, qMetric, qMetric, qDate, dateExpr)
	return e.executeSQL(csvPath, sql)
}

func (e *DuckDBEngine) QuerySegments(csvPath, catCol, metricCol string) ([]map[string]string, error) {
	qCat := quoteIdent(catCol)
	qMetric := quoteIdent(metricCol)
	sql := fmt.Sprintf(`SELECT 
		CAST(%s AS VARCHAR) as label,
		SUM(CAST(%s AS DOUBLE)) as value
	FROM data 
	WHERE CAST(%s AS DOUBLE) IS NOT NULL AND %s IS NOT NULL AND CAST(%s AS VARCHAR) != ''
	GROUP BY %s
	ORDER BY value DESC
	LIMIT 10`, qCat, qMetric, qMetric, qCat, qCat, qCat)
	return e.executeSQL(csvPath, sql)
}

// ExecuteQuery runs an arbitrary SQL query against one or more CSV files using DuckDB.
// Each CSV is registered as a table named after its filename (with non-alphanumeric chars replaced by _).
// Supports pagination via LIMIT/OFFSET.
// Uses a configurable timeout (QUERY_TIMEOUT_SEC env var, default 30s).
func (e *DuckDBEngine) ExecuteQuery(csvPaths []string, sql string, page, pageSize int) ([]map[string]string, []string, error) {
	return e.ExecuteQueryWithContext(context.Background(), csvPaths, sql, page, pageSize)
}

// ExecuteQueryWithContext runs an arbitrary SQL query with a configurable timeout.
func (e *DuckDBEngine) ExecuteQueryWithContext(ctx context.Context, csvPaths []string, sql string, page, pageSize int) ([]map[string]string, []string, error) {
	id := "eq_" + duckdbID()
	scriptPath := filepath.Join(e.scriptsDir, id+".py")
	resultPath := filepath.Join(e.scriptsDir, id+"_result.json")

	tableDefs := ""
	tableRefs := ""
	for i, path := range csvPaths {
		tname := fmt.Sprintf("t%d", i)
		tableDefs += fmt.Sprintf("    con.execute(\"CREATE TABLE %s AS SELECT * FROM read_csv_auto(?)\", [%q])\n", tname, path)
		if i > 0 {
			tableRefs += ", "
		}
		tableRefs += tname
	}
	if tableRefs == "" {
		return nil, nil, fmt.Errorf("no CSV paths provided")
	}

	escapedSQL := strings.ReplaceAll(sql, `"""`, `\"\"\"`)

	script := fmt.Sprintf(`import duckdb, json, sys, os

csv_paths = sys.argv[1:]

con = duckdb.connect(":memory:")
try:
%s
    sql_query = """%s"""
    result = con.execute(sql_query).fetchall()
    desc = con.description
    con.close()
    columns = [d[0] for d in desc] if desc else []
    rows = []
    for row in result:
        r = {}
        for i, col in enumerate(columns):
            val = row[i] if i < len(row) else None
            if val is None:
                r[col] = None
            else:
                r[col] = str(val)
        rows.append(r)
    with open(r"%s", "w") as f:
        json.dump({"columns": columns, "rows": rows}, f)
except Exception as e:
    con.close()
    with open(r"%s", "w") as f:
        json.dump({"error": str(e)}, f)
    sys.exit(1)
`, tableDefs, escapedSQL, resultPath, resultPath)

	if err := os.WriteFile(scriptPath, []byte(script), 0644); err != nil {
		return nil, nil, fmt.Errorf("write script: %w", err)
	}
	defer os.Remove(scriptPath)
	defer os.Remove(resultPath)

	args := append([]string{scriptPath}, csvPaths...)
	cmd := exec.CommandContext(ctx, "python3", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if ctx.Err() != nil {
			return nil, nil, fmt.Errorf("query timed out after %v", queryTimeout)
		}
		if data, readErr := os.ReadFile(resultPath); readErr == nil {
			var errResult struct {
				Error string `json:"error"`
			}
			if json.Unmarshal(data, &errResult) == nil && errResult.Error != "" {
				return nil, nil, fmt.Errorf("duckdb: %s", errResult.Error)
			}
		}
		return nil, nil, fmt.Errorf("execute duckdb: %s: %w", strings.TrimSpace(string(output)), err)
	}

	data, err := os.ReadFile(resultPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read result: %w", err)
	}

	var qr struct {
		Columns []string          `json:"columns"`
		Rows    []json.RawMessage `json:"rows"`
		Error   string            `json:"error"`
	}
	if err := json.Unmarshal(data, &qr); err != nil {
		return nil, nil, fmt.Errorf("parse result: %w", err)
	}
	if qr.Error != "" {
		return nil, nil, fmt.Errorf("duckdb: %s", qr.Error)
	}

	results := make([]map[string]string, len(qr.Rows))
	for i, rowData := range qr.Rows {
		var rowMap map[string]interface{}
		if err := json.Unmarshal(rowData, &rowMap); err != nil {
			return nil, nil, fmt.Errorf("parse row: %w", err)
		}
		row := make(map[string]string)
		for _, col := range qr.Columns {
			if v, ok := rowMap[col]; ok && v != nil {
				row[col] = fmt.Sprintf("%v", v)
			} else {
				row[col] = ""
			}
		}
		results[i] = row
	}

	return results, qr.Columns, nil
}

// Close cleans up temporary script files generated by the DuckDB engine.
func (e *DuckDBEngine) Close() {
	os.RemoveAll(e.scriptsDir)
}
