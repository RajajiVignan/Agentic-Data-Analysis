package data

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type DuckDBEngine struct {
	scriptsDir string
}

func NewDuckDBEngine(scriptsDir string) *DuckDBEngine {
	os.MkdirAll(scriptsDir, 0755)
	return &DuckDBEngine{scriptsDir: scriptsDir}
}

func (e *DuckDBEngine) executeSQL(csvPath, sql string) ([]map[string]string, error) {
	id := fmt.Sprintf("dq_%d", time.Now().UnixNano())
	scriptPath := filepath.Join(e.scriptsDir, id+".py")
	resultPath := filepath.Join(e.scriptsDir, id+"_result.json")

	escapedSQL := strings.ReplaceAll(sql, `"""`, `\"\"\"`)

	script := fmt.Sprintf(`import duckdb, json, sys, os

csv_path = sys.argv[1]
result_path = sys.argv[2]

con = duckdb.connect(":memory:")
try:
    con.execute("CREATE TABLE data AS SELECT * FROM read_csv_auto('" + csv_path + "')")
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

	cmd := exec.Command("python3", args...)
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

		countSQL := fmt.Sprintf(`SELECT COUNT(*) as non_empty FROM data WHERE "%s" IS NOT NULL AND CAST("%s" AS VARCHAR) != ''`, colName, colName)
		countRows, err := e.executeSQL(csvPath, countSQL)
		var nonEmpty int
		if err == nil && len(countRows) > 0 {
			fmt.Sscanf(countRows[0]["non_empty"], "%d", &nonEmpty)
		}

		sampleSQL := fmt.Sprintf(`SELECT DISTINCT "%s" as val FROM data WHERE "%s" IS NOT NULL AND CAST("%s" AS VARCHAR) != '' LIMIT 5`, colName, colName, colName)
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
	sql := fmt.Sprintf(`SELECT 
		SUM(CAST("%s" AS DOUBLE)) as total,
		AVG(CAST("%s" AS DOUBLE)) as avg_val,
		MIN(CAST("%s" AS DOUBLE)) as min_val,
		MAX(CAST("%s" AS DOUBLE)) as max_val,
		COUNT(*) as row_count
	FROM data WHERE CAST("%s" AS DOUBLE) IS NOT NULL`, metricCol, metricCol, metricCol, metricCol, metricCol)
	return e.executeSQL(csvPath, sql)
}

func (e *DuckDBEngine) QueryTrend(csvPath, dateCol, metricCol string) ([]map[string]string, error) {
	dateExpr := dateCol
	sql := fmt.Sprintf(`SELECT 
		CAST("%s" AS VARCHAR) as label,
		SUM(CAST("%s" AS DOUBLE)) as value
	FROM data 
	WHERE CAST("%s" AS DOUBLE) IS NOT NULL AND "%s" IS NOT NULL
	GROUP BY "%s"
	ORDER BY label
	LIMIT 20`, dateExpr, metricCol, metricCol, dateCol, dateExpr)
	return e.executeSQL(csvPath, sql)
}

func (e *DuckDBEngine) QuerySegments(csvPath, catCol, metricCol string) ([]map[string]string, error) {
	sql := fmt.Sprintf(`SELECT 
		CAST("%s" AS VARCHAR) as label,
		SUM(CAST("%s" AS DOUBLE)) as value
	FROM data 
	WHERE CAST("%s" AS DOUBLE) IS NOT NULL AND "%s" IS NOT NULL AND CAST("%s" AS VARCHAR) != ''
	GROUP BY "%s"
	ORDER BY value DESC
	LIMIT 10`, catCol, metricCol, metricCol, catCol, catCol, catCol)
	return e.executeSQL(csvPath, sql)
}
