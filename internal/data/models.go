package data

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"time"
)

// Dataset represents an uploaded or connected dataset.
type Dataset struct {
	ID       string
	Filename string
	FilePath string // empty for connected sources
	Profile  Profile
	Rows     []map[string]string
}

// Profile represents the column profile of a dataset.
type Profile struct {
	RowCount int
	Columns  []Column
}

// Column represents a column in the dataset.
type Column struct {
	Name     string
	Type     string // "number", "date", "text", "empty"
	NonEmpty int
	Sample   []string
}

// Connection represents a connection to a data source.
type Connection struct {
	Source      string
	ConnectedAt time.Time
}

// ColumnStat holds computed statistics for a single column.
type ColumnStat struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	NonEmpty  int      `json:"nonEmpty"`
	// Number stats
	Min       *float64 `json:"min,omitempty"`
	Max       *float64 `json:"max,omitempty"`
	Avg       *float64 `json:"avg,omitempty"`
	Sum       *float64 `json:"sum,omitempty"`
	// Text stats
	UniqueValues []string `json:"uniqueValues,omitempty"`
	UniqueCount  int      `json:"uniqueCount"`
	// Date stats
	MinDate   string `json:"minDate,omitempty"`
	MaxDate   string `json:"maxDate,omitempty"`
}

// DatasetMetadata is a privacy-safe summary of a dataset.
// It contains schema and statistics but NO raw row data.
type DatasetMetadata struct {
	ID         string        `json:"id"`
	Filename   string        `json:"filename"`
	RowCount   int           `json:"rowCount"`
	Columns    []ColumnStat  `json:"columns"`
}

// ComputeMetadata builds a DatasetMetadata from a Dataset without exposing raw rows.
func ComputeMetadata(ds *Dataset, maxUniqueValues int) DatasetMetadata {
	meta := DatasetMetadata{
		ID:       ds.ID,
		Filename: ds.Filename,
		RowCount: ds.Profile.RowCount,
		Columns:  make([]ColumnStat, len(ds.Profile.Columns)),
	}

	for i, col := range ds.Profile.Columns {
		cs := ColumnStat{
			Name:     col.Name,
			Type:     col.Type,
			NonEmpty: col.NonEmpty,
		}

		switch col.Type {
		case "number":
			var vals []float64
			for _, row := range ds.Rows {
				if v, err := strconv.ParseFloat(row[col.Name], 64); err == nil {
					vals = append(vals, v)
				}
			}
			if len(vals) > 0 {
				min, max, sum := vals[0], vals[0], 0.0
				for _, v := range vals {
					if v < min { min = v }
					if v > max { max = v }
					sum += v
				}
				avg := sum / float64(len(vals))
				// Round to avoid floating-point noise
				min = math.Round(min*1000) / 1000
				max = math.Round(max*1000) / 1000
				avg = math.Round(avg*1000) / 1000
				sum = math.Round(sum*1000) / 1000
				cs.Min = &min
				cs.Max = &max
				cs.Avg = &avg
				cs.Sum = &sum
			}
		case "text":
			uniq := make(map[string]bool)
			for _, row := range ds.Rows {
				if v := row[col.Name]; v != "" {
					uniq[v] = true
				}
			}
			cs.UniqueCount = len(uniq)
			for v := range uniq {
				cs.UniqueValues = append(cs.UniqueValues, v)
			}
			sort.Strings(cs.UniqueValues)
			if len(cs.UniqueValues) > maxUniqueValues {
				cs.UniqueValues = cs.UniqueValues[:maxUniqueValues]
			}
		case "date":
			type dateVal struct {
				str    string
				parsed time.Time
			}
			var dates []dateVal
			for _, row := range ds.Rows {
				tstr := row[col.Name]
				if tstr == "" {
					continue
				}
				t2, err := parseDateStr(tstr)
				if err == nil {
					dates = append(dates, dateVal{str: tstr, parsed: t2})
				}
			}
			if len(dates) > 0 {
				sort.Slice(dates, func(i, j int) bool {
					return dates[i].parsed.Before(dates[j].parsed)
				})
				cs.MinDate = dates[0].str
				cs.MaxDate = dates[len(dates)-1].str
			}
		}

		meta.Columns[i] = cs
	}

	return meta
}

func parseDateStr(s string) (time.Time, error) {
	formats := []string{
		"2006-01-02",
		"2006/01/02",
		"2006-01",
		"2006/01",
		"2006",
		"01/02/2006",
		"01-02-2006",
		"January 2, 2006",
		"Jan 2, 2006",
		"2 Jan 2006",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized date: %s", s)
}
