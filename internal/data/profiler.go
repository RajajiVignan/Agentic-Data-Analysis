package data

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

// DetailedProfile holds comprehensive column-level statistics.
type DetailedProfile struct {
	DatasetID    string          `json:"datasetId"`
	Filename     string          `json:"filename"`
	RowCount     int             `json:"rowCount"`
	ColumnCount  int             `json:"columnCount"`
	Columns      []ColumnProfile `json:"columns"`
	Correlations []Correlation   `json:"correlations,omitempty"`
	Duplicates   DuplicateInfo   `json:"duplicates"`
	Histograms   []Histogram     `json:"histograms,omitempty"`
}

type ColumnProfile struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	NonEmpty    int      `json:"nonEmpty"`
	NullCount   int      `json:"nullCount"`
	UniqueCount int      `json:"uniqueCount"`
	Min         *float64 `json:"min,omitempty"`
	Max         *float64 `json:"max,omitempty"`
	Mean        *float64 `json:"mean,omitempty"`
	Median      *float64 `json:"median,omitempty"`
	StdDev      *float64 `json:"stdDev,omitempty"`
	Sample      []string `json:"sample,omitempty"`
}

type Histogram struct {
	Column  string   `json:"column"`
	Buckets []Bucket `json:"buckets"`
}

type Bucket struct {
	Label string  `json:"label"`
	Min   float64 `json:"min"`
	Max   float64 `json:"max"`
	Count int     `json:"count"`
}

type Correlation struct {
	Col1 string  `json:"col1"`
	Col2 string  `json:"col2"`
	R    float64 `json:"r"` // Pearson correlation coefficient
}

type DuplicateInfo struct {
	TotalRows     int      `json:"totalRows"`
	DuplicateRows int      `json:"duplicateRows"`
	DuplicateKeys []string `json:"duplicateKeys,omitempty"`
}

// ComputeDetailedProfile computes comprehensive statistics for a dataset.
func ComputeDetailedProfile(ds *Dataset) *DetailedProfile {
	dp := &DetailedProfile{
		DatasetID:   ds.ID,
		Filename:    ds.Filename,
		RowCount:    len(ds.Rows),
		ColumnCount: len(ds.Profile.Columns),
		Columns:     make([]ColumnProfile, 0, len(ds.Profile.Columns)),
	}

	for _, col := range ds.Profile.Columns {
		cp := ColumnProfile{
			Name:      col.Name,
			Type:      col.Type,
			NonEmpty:  col.NonEmpty,
			NullCount: len(ds.Rows) - col.NonEmpty,
			Sample:    col.Sample,
		}
		cp.computeStats(ds.Rows, col)
		dp.Columns = append(dp.Columns, cp)

		// Generate histogram for number columns
		if col.Type == "number" {
			dp.Histograms = append(dp.Histograms, computeHistogram(ds.Rows, col))
		}
	}

	dp.Correlations = computeCorrelations(ds)
	dp.Duplicates = computeDuplicates(ds)

	return dp
}

func (cp *ColumnProfile) computeStats(rows []map[string]string, col Column) {
	switch col.Type {
	case "number":
		var vals []float64
		for _, row := range rows {
			if v, err := strconv.ParseFloat(row[col.Name], 64); err == nil {
				vals = append(vals, v)
			}
		}
		if len(vals) == 0 {
			return
		}
		sort.Float64s(vals)

		min := vals[0]
		max := vals[len(vals)-1]
		var sum float64
		for _, v := range vals {
			sum += v
		}
		mean := sum / float64(len(vals))

		var varianceSum float64
		for _, v := range vals {
			diff := v - mean
			varianceSum += diff * diff
		}
		stdDev := math.Sqrt(varianceSum / float64(len(vals)))

		var median float64
		if len(vals)%2 == 0 {
			median = (vals[len(vals)/2-1] + vals[len(vals)/2]) / 2
		} else {
			median = vals[len(vals)/2]
		}

		cp.Min = &min
		cp.Max = &max
		cp.Mean = &mean
		cp.Median = &median
		cp.StdDev = &stdDev

	case "text":
		uniq := make(map[string]bool)
		for _, row := range rows {
			if v := row[col.Name]; v != "" {
				uniq[v] = true
			}
		}
		cp.UniqueCount = len(uniq)
	}

	// Unique count for all types (approximate)
	uniq := make(map[string]bool)
	for _, row := range rows {
		uniq[row[col.Name]] = true
	}
	if cp.UniqueCount == 0 {
		cp.UniqueCount = len(uniq)
	}
}

func computeHistogram(rows []map[string]string, col Column) Histogram {
	h := Histogram{Column: col.Name}

	var vals []float64
	for _, row := range rows {
		if v, err := strconv.ParseFloat(row[col.Name], 64); err == nil {
			vals = append(vals, v)
		}
	}
	if len(vals) == 0 {
		return h
	}

	sort.Float64s(vals)
	min := vals[0]
	max := vals[len(vals)-1]

	if max == min {
		// All same value
		h.Buckets = []Bucket{{Label: fmt.Sprintf("%.2f", min), Min: min, Max: max, Count: len(vals)}}
		return h
	}

	// Sturges' rule for number of bins
	nBins := int(math.Ceil(1 + math.Log2(float64(len(vals)))))
	if nBins > 20 {
		nBins = 20
	}
	if nBins < 3 {
		nBins = 3
	}

	binWidth := (max - min) / float64(nBins)
	buckets := make([]Bucket, nBins)
	for i := 0; i < nBins; i++ {
		bMin := min + float64(i)*binWidth
		bMax := bMin + binWidth
		if i == nBins-1 {
			bMax = max
		}
		buckets[i] = Bucket{
			Label: fmt.Sprintf("%.1f-%.1f", bMin, bMax),
			Min:   bMin,
			Max:   bMax,
		}
	}

	for _, v := range vals {
		idx := int((v - min) / binWidth)
		if idx >= nBins {
			idx = nBins - 1
		}
		if idx < 0 {
			idx = 0
		}
		buckets[idx].Count++
	}

	h.Buckets = buckets
	return h
}

func computeCorrelations(ds *Dataset) []Correlation {
	// Find numeric columns
	numCols := make([]Column, 0)
	for _, c := range ds.Profile.Columns {
		if c.Type == "number" {
			numCols = append(numCols, c)
		}
	}

	if len(numCols) < 2 {
		return nil
	}

	var cors []Correlation
	for i := 0; i < len(numCols); i++ {
		for j := i + 1; j < len(numCols); j++ {
			r := pearsonCorrelation(ds.Rows, numCols[i].Name, numCols[j].Name)
			if !math.IsNaN(r) {
				r = math.Round(r*1000) / 1000
				cors = append(cors, Correlation{Col1: numCols[i].Name, Col2: numCols[j].Name, R: r})
			}
		}
	}
	return cors
}

func pearsonCorrelation(rows []map[string]string, col1, col2 string) float64 {
	var xVals, yVals []float64
	for _, row := range rows {
		x, errX := strconv.ParseFloat(row[col1], 64)
		y, errY := strconv.ParseFloat(row[col2], 64)
		if errX == nil && errY == nil {
			xVals = append(xVals, x)
			yVals = append(yVals, y)
		}
	}

	n := len(xVals)
	if n < 3 {
		return math.NaN()
	}

	var sumX, sumY, sumXY, sumX2, sumY2 float64
	for i := 0; i < n; i++ {
		x := xVals[i]
		y := yVals[i]
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
		sumY2 += y * y
	}

	num := float64(n)*sumXY - sumX*sumY
	den := math.Sqrt((float64(n)*sumX2 - sumX*sumX) * (float64(n)*sumY2 - sumY*sumY))
	if den == 0 {
		return math.NaN()
	}
	return num / den
}

func computeDuplicates(ds *Dataset) DuplicateInfo {
	di := DuplicateInfo{TotalRows: len(ds.Rows)}

	if len(ds.Rows) < 2 {
		return di
	}

	// Build column order from profile
	cols := make([]string, len(ds.Profile.Columns))
	for i, c := range ds.Profile.Columns {
		cols[i] = c.Name
	}

	seen := make(map[string]bool)
	var dupKeys []string
	dupCount := 0

	for _, row := range ds.Rows {
		var parts []string
		for _, c := range cols {
			parts = append(parts, row[c])
		}
		key := strings.Join(parts, "\x00")
		if seen[key] {
			dupCount++
			if len(dupKeys) < 5 {
				dupKeys = append(dupKeys, parts[0])
			}
		}
		seen[key] = true
	}

	di.DuplicateRows = dupCount
	di.DuplicateKeys = dupKeys
	return di
}
