package data

import (
	"time"
)

// Dataset represents an uploaded or connected dataset.
type Dataset struct {
	ID          string
	Filename    string
	FilePath    string // empty for connected sources
	Profile     Profile
	Rows        []map[string]string
	HealthCheck interface{} // can be any struct, we'll use map[string]interface{} for simplicity
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
	Source     string
	ConnectedAt time.Time
}

// HealthCheckResult represents the result of a health check.
type HealthCheckResult struct {
	Anomalies      []string `json:"anomalies,omitempty"`
	Outliers       []string `json:"outliers,omitempty"`
	MissingValues  []string `json:"missing_values,omitempty"`
	Correlations   []string `json:"correlations,omitempty"`
	DataQualityIssues []string `json:"data_quality_issues,omitempty"`
}