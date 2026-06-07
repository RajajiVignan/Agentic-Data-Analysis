package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
)

// DB is the database persistence layer.
type DB struct {
	conn *sql.DB
}

// PinnedChartRecord represents a row in the pinned_charts table.
type PinnedChartRecord struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	ChartType string    `json:"chart_type"`
	Label     string    `json:"label"`
	Data      any       `json:"data"`
	URL       string    `json:"url"`
}

// NewDB creates a new database connection using SUPABASE_URL and SUPABASE_KEY from env.
// It constructs a PostgreSQL connection string from the Supabase URL.
// If connection fails, it returns nil (graceful degradation to in-memory).
func NewDB() *DB {
	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")

	if supabaseURL == "" || supabaseKey == "" {
		log.Println("store: SUPABASE_URL or SUPABASE_KEY not set, using in-memory storage")
		return nil
	}

	// Supabase URL format: https://xxxx.supabase.co
	// PostgreSQL host: db.xxxx.supabase.co (or xxxx.supabase.co for older projects)
	// We extract the project ref and build the connection string.
	// Standard Supabase pg connection: postgresql://postgres:[password]@db.[project-ref].supabase.co:5432/postgres
	// Since we have the anon key, we use it as the password (Supabase supports this for REST, but for pg we need the actual password).
	// For Supabase, the DB password is set during project creation. We'll try the anon key first,
	// but also support a SUPABASE_DB_PASSWORD env var.

	dbPassword := os.Getenv("SUPABASE_DB_PASSWORD")
	if dbPassword == "" {
		// Try using the publishable key as password (works for some Supabase configurations)
		dbPassword = supabaseKey
	}

	// Extract project ref from URL: https://[ref].supabase.co
	var projectRef string
	_, err := fmt.Sscanf(supabaseURL, "https://%s.supabase.co", &projectRef)
	if err != nil || projectRef == "" {
		// Fallback: try to parse more carefully
		projectRef = extractProjectRef(supabaseURL)
	}

	if projectRef == "" {
		log.Println("store: could not extract project ref from SUPABASE_URL, using in-memory storage")
		return nil
	}

	host := fmt.Sprintf("db.%s.supabase.co", projectRef)
	connStr := fmt.Sprintf("postgresql://postgres:%s@%s:5432/postgres?sslmode=require", dbPassword, host)

	conn, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Printf("store: failed to open database: %v, using in-memory storage", err)
		return nil
	}

	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	// Test the connection
	if err := conn.Ping(); err != nil {
		log.Printf("store: failed to ping database: %v, using in-memory storage", err)
		conn.Close()
		return nil
	}

	db := &DB{conn: conn}
	if err := db.initSchema(); err != nil {
		log.Printf("store: failed to initialize schema: %v, using in-memory storage", err)
		conn.Close()
		return nil
	}

	log.Println("store: connected to Supabase PostgreSQL")
	return db
}

func extractProjectRef(url string) string {
	// Simple extraction: strip https:// and .supabase.co
	var ref string
	for _, prefix := range []string{"https://", "http://"} {
		if len(url) > len(prefix) && url[:len(prefix)] == prefix {
			url = url[len(prefix):]
			break
		}
	}
	for _, suffix := range []string{".supabase.co", ":5432"} {
		if idx := len(url) - len(suffix); idx > 0 && url[idx:] == suffix {
			url = url[:idx]
			break
		}
	}
	ref = url
	return ref
}

func (db *DB) initSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS pinned_charts (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at TIMESTAMPTZ DEFAULT now(),
			chart_type TEXT NOT NULL,
			label TEXT,
			data JSONB,
			url TEXT,
			user_id TEXT
		)`,
		`CREATE TABLE IF NOT EXISTS datasets (
			id TEXT PRIMARY KEY,
			filename TEXT NOT NULL,
			file_path TEXT,
			profile JSONB,
			created_at TIMESTAMPTZ DEFAULT now()
		)`,
		`CREATE TABLE IF NOT EXISTS data_sources (
			id TEXT PRIMARY KEY,
			source TEXT NOT NULL,
			connected_at TIMESTAMPTZ DEFAULT now()
		)`,
	}
	for _, q := range queries {
		if _, err := db.conn.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

// Close closes the database connection.
func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

// --- Pinned Charts ---

// SavePinnedChart saves a pinned chart to the database.
func (db *DB) SavePinnedChart(id, chartType, label string, data any, urlStr string) (string, error) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal chart data: %w", err)
	}

	var chartID string
	if id == "" {
		err = db.conn.QueryRow(
			`INSERT INTO pinned_charts (chart_type, label, data, url) VALUES ($1, $2, $3, $4) RETURNING id`,
			chartType, label, dataJSON, urlStr,
		).Scan(&chartID)
	} else {
		chartID = id
		_, err = db.conn.Exec(
			`INSERT INTO pinned_charts (id, chart_type, label, data, url) VALUES ($1, $2, $3, $4, $5)
			 ON CONFLICT (id) DO UPDATE SET chart_type=$2, label=$3, data=$4, url=$5`,
			id, chartType, label, dataJSON, urlStr,
		)
	}
	if err != nil {
		return "", fmt.Errorf("save pinned chart: %w", err)
	}
	return chartID, nil
}

// GetPinnedCharts retrieves all pinned charts from the database.
func (db *DB) GetPinnedCharts() ([]PinnedChartRecord, error) {
	rows, err := db.conn.Query(
		`SELECT id, created_at, chart_type, label, data, COALESCE(url, '') FROM pinned_charts ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query pinned charts: %w", err)
	}
	defer rows.Close()

	var charts []PinnedChartRecord
	for rows.Next() {
		var c PinnedChartRecord
		var dataJSON []byte
		if err := rows.Scan(&c.ID, &c.CreatedAt, &c.ChartType, &c.Label, &dataJSON, &c.URL); err != nil {
			return nil, fmt.Errorf("scan pinned chart: %w", err)
		}
		if len(dataJSON) > 0 {
			json.Unmarshal(dataJSON, &c.Data)
		}
		charts = append(charts, c)
	}
	return charts, rows.Err()
}

// DeletePinnedChart removes a pinned chart by ID.
func (db *DB) DeletePinnedChart(id string) error {
	_, err := db.conn.Exec(`DELETE FROM pinned_charts WHERE id = $1`, id)
	return err
}

// --- Datasets ---

// SaveDataset persists a dataset reference to the database.
func (db *DB) SaveDataset(id, filename, filePath string, profileJSON []byte) error {
	_, err := db.conn.Exec(
		`INSERT INTO datasets (id, filename, file_path, profile) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (id) DO UPDATE SET filename=$2, file_path=$3, profile=$4`,
		id, filename, filePath, profileJSON,
	)
	return err
}

// LoadDatasets retrieves all datasets from the database.
func (db *DB) LoadDatasets() ([]DatasetRecord, error) {
	rows, err := db.conn.Query(
		`SELECT id, filename, COALESCE(file_path, ''), profile FROM datasets ORDER BY created_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query datasets: %w", err)
	}
	defer rows.Close()

	var datasets []DatasetRecord
	for rows.Next() {
		var d DatasetRecord
		var profileJSON []byte
		if err := rows.Scan(&d.ID, &d.Filename, &d.FilePath, &profileJSON); err != nil {
			return nil, fmt.Errorf("scan dataset: %w", err)
		}
		if len(profileJSON) > 0 {
			json.Unmarshal(profileJSON, &d.Profile)
		}
		datasets = append(datasets, d)
	}
	return datasets, rows.Err()
}

// DatasetRecord represents a row in the datasets table.
type DatasetRecord struct {
	ID       string        `json:"id"`
	Filename string        `json:"filename"`
	FilePath string        `json:"file_path"`
	Profile  DatasetProfile `json:"profile"`
}

// DatasetProfile is a lightweight version of data.Profile for DB storage.
type DatasetProfile struct {
	RowCount int              `json:"rowCount"`
	Columns  []DatasetColumn  `json:"columns"`
}

// DatasetColumn represents a column stored in the profile JSON.
type DatasetColumn struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	NonEmpty int      `json:"nonEmpty"`
	Sample   []string `json:"sample"`
}

// SaveDataSource persists a data source connection to the database.
func (db *DB) SaveDataSource(id, source string) error {
	_, err := db.conn.Exec(
		`INSERT INTO data_sources (id, source) VALUES ($1, $2)
		 ON CONFLICT (id) DO UPDATE SET source=$2`,
		id, source,
	)
	return err
}

// LoadDataSources retrieves all data source connections from the database.
func (db *DB) LoadDataSources() (map[string]string, error) {
	rows, err := db.conn.Query(`SELECT id, source FROM data_sources`)
	if err != nil {
		return nil, fmt.Errorf("query data_sources: %w", err)
	}
	defer rows.Close()

	sources := make(map[string]string)
	for rows.Next() {
		var id, source string
		if err := rows.Scan(&id, &source); err != nil {
			return nil, fmt.Errorf("scan data_source: %w", err)
		}
		sources[id] = source
	}
	return sources, rows.Err()
}
