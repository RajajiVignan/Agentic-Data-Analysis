package data

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ProfileRows analyzes rows and generates a column profile.
func ProfileRows(rows [][]string) Profile {
	if len(rows) == 0 {
		return Profile{}
	}

	headers := make([]string, len(rows[0]))
	for i, h := range rows[0] {
		trimmed := strings.TrimSpace(h)
		if trimmed == "" {
			headers[i] = fmt.Sprintf("column_%d", i+1)
		} else {
			headers[i] = trimmed
		}
	}

	dataRows := rows[1:]
	cols := make([]Column, len(headers))

	for i, name := range headers {
		var values []string
		for _, row := range dataRows {
			if i < len(row) && row[i] != "" {
				values = append(values, row[i])
			}
		}

		cols[i] = Column{
			Name:     name,
			Type:     inferTypeWithName(name, values),
			NonEmpty: len(values),
			Sample:   getSample(values, 3),
		}
	}

	return Profile{
		RowCount: len(dataRows),
		Columns:  cols,
	}
}

func inferType(values []string) string {
	if len(values) == 0 {
		return "empty"
	}

	numericCount := 0
	dateCount := 0

	for _, v := range values {
		if _, err := strconv.ParseFloat(v, 64); err == nil {
			numericCount++
		}
		if _, ok := ParseDateValue(v); ok {
			dateCount++
		}
	}

	ratio := float64(len(values))
	if float64(numericCount)/ratio > 0.8 {
		return "number"
	}
	if float64(dateCount)/ratio > 0.8 {
		return "date"
	}
	return "text"
}

// inferTypeWithName is like inferType but also considers the column name
// as a secondary signal. If the column name looks like a date dimension
// (e.g. "month", "year", "date", "created_at") and a majority of values
// are parseable as dates, classify it as "date" even if the ratio is below
// the strict 0.8 threshold. This helps with small datasets or columns with
// some missing values.
func inferTypeWithName(colName string, values []string) string {
	baseType := inferType(values)
	if baseType != "text" {
		return baseType
	}
	// Only apply name-based heuristic for text-typed columns
	if !looksLikeDateColumnName(colName) {
		return "text"
	}
	// Check if at least 50% of non-empty values parse as dates
	if len(values) == 0 {
		return "text"
	}
	dateCount := 0
	for _, v := range values {
		if _, ok := ParseDateValue(v); ok {
			dateCount++
		}
	}
	if float64(dateCount)/float64(len(values)) >= 0.5 {
		return "date"
	}
	return "text"
}

// looksLikeDateColumnName returns true if the column name suggests
// it contains date/time data.
func looksLikeDateColumnName(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	datePatterns := []string{"date", "month", "year", "week", "day", "time", "period", "quarter", "dt", "created", "updated", "timestamp"}
	for _, p := range datePatterns {
		if strings.Contains(n, p) {
			return true
		}
	}
	return false
}

// ParseDateValue parses a date string using multiple formats.
// Exported for use by the agent tools layer.
func ParseDateValue(value string) (time.Time, bool) {
	v := strings.TrimSpace(value)
	if v == "" {
		return time.Time{}, false
	}

	formats := []string{
		"2006-01-02",
		"2006/01/02",
		"2006-01",
		"2006/01",
		"01/02/2006",
		"01-02-2006",
		"January 2, 2006",
		"Jan 2, 2006",
		"2 Jan 2006",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	}
	for _, format := range formats {
		if t, err := time.Parse(format, v); err == nil {
			return t, true
		}
	}

	yearRegex := regexp.MustCompile(`^\d{4}$`)
	if yearRegex.MatchString(v) {
		if t, err := time.Parse("2006", v); err == nil {
			return t, true
		}
	}

	return time.Time{}, false
}

func getSample(values []string, n int) []string {
	if len(values) <= n {
		return values
	}
	return values[:n]
}

// ParseCSV converts CSV text to a slice of rows.
func ParseCSV(text string) ([][]string, error) {
	r := csv.NewReader(strings.NewReader(text))
	return r.ReadAll()
}

// ParseJSONRows converts JSON text to a slice of rows.
func ParseJSONRows(text string) ([][]string, error) {
	var raw interface{}
	if err := json.Unmarshal([]byte(text), &raw); err != nil {
		return nil, err
	}

	var records []map[string]interface{}
	switch v := raw.(type) {
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				records = append(records, m)
			}
		}
	case map[string]interface{}:
		if data, ok := v["data"].([]interface{}); ok {
			for _, item := range data {
				if m, ok := item.(map[string]interface{}); ok {
					records = append(records, m)
				}
			}
		} else if rows, ok := v["rows"].([]interface{}); ok {
			for _, item := range rows {
				if m, ok := item.(map[string]interface{}); ok {
					records = append(records, m)
				}
			}
		}
	}

	if len(records) == 0 {
		return nil, fmt.Errorf("JSON uploads must be an array of objects or contain data/rows")
	}

	// Collect unique headers
	headerMap := make(map[string]bool)
	var headers []string
	for _, rec := range records {
		for k := range rec {
			if !headerMap[k] {
				headerMap[k] = true
				headers = append(headers, k)
			}
		}
	}
	sort.Strings(headers)

	result := make([][]string, 0, len(records)+1)
	result = append(result, headers)

	for _, rec := range records {
		row := make([]string, len(headers))
		for i, h := range headers {
			row[i] = fmt.Sprintf("%v", rec[h])
		}
		result = append(result, row)
	}

	return result, nil
}

// BuildKPIs calculates top-level metrics from rows.
func BuildKPIs(rows []map[string]string, metricCol *Column, categoryCol *Column) []map[string]string {
	var total float64
	var count int

	for _, row := range rows {
		if metricCol != nil {
			if val, err := strconv.ParseFloat(row[metricCol.Name], 64); err == nil {
				total += val
				count++
			}
		}
	}

	avg := 0.0
	if count > 0 {
		avg = total / float64(count)
	}

	categoryCount := 0
	if categoryCol != nil {
		uniqueCats := make(map[string]bool)
		for _, row := range rows {
			if cat := row[categoryCol.Name]; cat != "" {
				uniqueCats[cat] = true
			}
		}
		categoryCount = len(uniqueCats)
	}

	label := "Rows analyzed"
	valStr := fmt.Sprintf("%d", len(rows))
	if metricCol != nil {
		label = "Total " + metricCol.Name
		valStr = formatNumber(total)
	}

	return []map[string]string{
		{"label": label, "value": valStr, "change": "Dataset result"},
		{"label": "Average", "value": formatNumber(avg), "change": "Per row"},
		{"label": "Segments", "value": fmt.Sprintf("%d", categoryCount), "change": "Available split"},
	}
}

func formatNumber(val float64) string {
	if math.Abs(val) >= 1000000 {
		return fmt.Sprintf("%.1fM", val/1000000)
	}
	if math.Abs(val) >= 1000 {
		return fmt.Sprintf("%.1fK", val/1000)
	}
	return fmt.Sprintf("%.1f", val)
}

// BuildTrend aggregates data by month.
func BuildTrend(rows []map[string]string, dateCol *Column, metricCol *Column) []map[string]interface{} {
	if dateCol == nil || metricCol == nil {
		return nil
	}

	grouped := make(map[string]float64)
	for _, row := range rows {
		dStr := row[dateCol.Name]
		vStr := row[metricCol.Name]

		t, ok := ParseDateValue(dStr)
		if !ok {
			continue
		}

		val, err := strconv.ParseFloat(vStr, 64)
		if err != nil {
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

	var result []map[string]interface{}
	start := 0
	if len(labels) > 8 {
		start = len(labels) - 8
	}
	for _, l := range labels[start:] {
		result = append(result, map[string]interface{}{"label": l, "value": grouped[l]})
	}
	return result
}

// BuildSegments finds top categories.
func BuildSegments(rows []map[string]string, categoryCol *Column, metricCol *Column) []map[string]interface{} {
	if categoryCol == nil {
		return nil
	}

	grouped := make(map[string]float64)
	for _, row := range rows {
		cat := row[categoryCol.Name]
		if cat == "" {
			cat = "Unknown"
		}

		val := 1.0
		if metricCol != nil {
			if v, err := strconv.ParseFloat(row[metricCol.Name], 64); err == nil {
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

	var result []map[string]interface{}
	limit := 5
	if len(entries) < 5 {
		limit = len(entries)
	}
	for i := 0; i < limit; i++ {
		result = append(result, map[string]interface{}{"label": entries[i].label, "value": entries[i].value})
	}
	return result
}
