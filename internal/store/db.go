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

// ReportRecord represents a persisted scheduled report.
type ReportRecord struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	DatasetIDs   string `json:"dataset_ids"`
	Frequency    string `json:"frequency"`
	DayOfWeek    int    `json:"day_of_week"`
	DayOfMonth   int    `json:"day_of_month"`
	Hour         int    `json:"hour"`
	Emails       string `json:"emails"`
	SlackWebhook string `json:"slack_webhook"`
	TeamsWebhook string `json:"teams_webhook"`
	LastSent     string `json:"last_sent"`
	NextRun      string `json:"next_run"`
	Enabled      bool   `json:"enabled"`
	CreatedAt    string `json:"created_at"`
}

// AlertRecord represents a persisted alert rule.
type AlertRecord struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DatasetID   string `json:"dataset_id"`
	MetricCol   string `json:"metric_col"`
	Condition   string `json:"condition"`
	Threshold   float64 `json:"threshold"`
	Period      string `json:"period"`
	Emails      string `json:"emails"`
	SlackHook   string `json:"slack_hook"`
	Enabled     bool   `json:"enabled"`
	LastChecked string `json:"last_checked"`
	CreatedAt   string `json:"created_at"`
}

// LayoutRecord represents a persisted dashboard layout.
type LayoutRecord struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	IsDefault bool   `json:"is_default"`
	Tiles     string `json:"tiles"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

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

func buildSupabaseConnStr(projectRef, dbPassword string) string {
	host := fmt.Sprintf("db.%s.supabase.co", projectRef)
	return fmt.Sprintf("postgresql://postgres:%s@%s:5432/postgres?sslmode=require", dbPassword, host)
}

func pinnedChartsSchemaSQL() string {
	return `
		CREATE TABLE IF NOT EXISTS pinned_charts (
			id TEXT PRIMARY KEY,
			created_at TIMESTAMPTZ DEFAULT now(),
			chart_type TEXT NOT NULL,
			label TEXT,
			data JSONB,
			url TEXT
		)
	`
}

func (db *DB) initSchema() error {
	if _, err := db.conn.Exec(pinnedChartsSchemaSQL()); err != nil {
		return err
	}
	if err := db.initDashboardsTable(); err != nil {
		return err
	}
	if err := db.InitDatasetsTable(); err != nil {
		return err
	}
	if err := db.initReportsTable(); err != nil {
		return err
	}
	if err := db.initAlertsTable(); err != nil {
		return err
	}
	if err := db.initDashboardLayoutsTable(); err != nil {
		return err
	}
	return db.initUsersTable()
}

func (db *DB) initReportsTable() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS scheduled_reports (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			dataset_ids TEXT NOT NULL DEFAULT '[]',
			frequency TEXT NOT NULL DEFAULT 'daily',
			day_of_week INTEGER NOT NULL DEFAULT 0,
			day_of_month INTEGER NOT NULL DEFAULT 1,
			hour INTEGER NOT NULL DEFAULT 9,
			emails TEXT NOT NULL DEFAULT '[]',
			slack_webhook TEXT DEFAULT '',
			teams_webhook TEXT DEFAULT '',
			last_sent TEXT DEFAULT '',
			next_run TEXT DEFAULT '',
			enabled BOOLEAN NOT NULL DEFAULT true,
			created_at TEXT NOT NULL DEFAULT ''
		)
	`)
	return err
}

func (db *DB) initAlertsTable() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS alert_rules (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			dataset_id TEXT NOT NULL,
			metric_col TEXT NOT NULL,
			condition TEXT NOT NULL DEFAULT 'drop',
			threshold DOUBLE PRECISION NOT NULL DEFAULT 10,
			period TEXT DEFAULT '',
			emails TEXT NOT NULL DEFAULT '[]',
			slack_hook TEXT DEFAULT '',
			enabled BOOLEAN NOT NULL DEFAULT true,
			last_checked TEXT DEFAULT '',
			created_at TEXT NOT NULL DEFAULT ''
		)
	`)
	return err
}

func (db *DB) initDashboardLayoutsTable() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS dashboard_layouts (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			is_default BOOLEAN NOT NULL DEFAULT false,
			tiles JSONB NOT NULL DEFAULT '[]'::jsonb,
			created_at TEXT NOT NULL DEFAULT '',
			updated_at TEXT NOT NULL DEFAULT ''
		)
	`)
	return err
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

// --- Users ---

// UserRecord represents a persisted user.
type UserRecord struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"password_hash"`
	CreatedAt    time.Time `json:"created_at"`
}

func (db *DB) initUsersTable() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL,
			password_hash TEXT NOT NULL,
			created_at TIMESTAMPTZ DEFAULT now()
		)
	`)
	if err != nil {
		return err
	}
	_, err = db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS app_config (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)
	`)
	return err
}

// SaveUser persists a user to the database.
func (db *DB) SaveUser(id, email, name, passwordHash string, createdAt time.Time) error {
	_, err := db.conn.Exec(
		`INSERT INTO users (id, email, name, password_hash, created_at) VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (id) DO UPDATE SET email=$2, name=$3, password_hash=$4`,
		id, email, name, passwordHash, createdAt,
	)
	return err
}

// LoadUsers retrieves all users from the database.
func (db *DB) LoadUsers() ([]UserRecord, error) {
	rows, err := db.conn.Query(
		`SELECT id, email, name, password_hash, created_at FROM users ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	var users []UserRecord
	for rows.Next() {
		var u UserRecord
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// SaveJWTSecret persists the JWT signing secret to the database.
func (db *DB) SaveJWTSecret(secret []byte) error {
	_, err := db.conn.Exec(
		`INSERT INTO app_config (key, value) VALUES ('jwt_secret', $1)
		 ON CONFLICT (key) DO UPDATE SET value=$1`,
		string(secret),
	)
	return err
}

// LoadJWTSecret retrieves the JWT signing secret from the database.
func (db *DB) LoadJWTSecret() ([]byte, error) {
	var value string
	err := db.conn.QueryRow(
		`SELECT value FROM app_config WHERE key = 'jwt_secret'`,
	).Scan(&value)
	if err != nil {
		return nil, err
	}
	return []byte(value), nil
}

// --- Datasets ---

// SaveDataset persists a dataset reference to the database.
func (db *DB) SaveDataset(id, filename, filePath, ownerID string, profileJSON []byte) error {
	_, err := db.conn.Exec(
		`INSERT INTO datasets (id, filename, file_path, owner_id, profile) VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (id) DO UPDATE SET filename=$2, file_path=$3, owner_id=$4, profile=$5`,
		id, filename, filePath, ownerID, profileJSON,
	)
	return err
}

// SaveDatasetRows persists the row data for a dataset.
func (db *DB) SaveDatasetRows(id string, rows []map[string]string) error {
	rowsJSON, err := json.Marshal(rows)
	if err != nil {
		return fmt.Errorf("marshal rows: %w", err)
	}
	_, err = db.conn.Exec(
		`UPDATE datasets SET rows_data = $1 WHERE id = $2`,
		rowsJSON, id,
	)
	return err
}

// DatasetRecord represents a persisted dataset with its row data.
type DatasetRecord struct {
	ID       string              `json:"id"`
	Filename string              `json:"filename"`
	FilePath string              `json:"file_path"`
	OwnerID  string              `json:"owner_id"`
	Profile  json.RawMessage     `json:"profile"`
	Rows     []map[string]string `json:"rows"`
}

// LoadDatasets retrieves all datasets from the database.
func (db *DB) LoadDatasets() ([]DatasetRecord, error) {
	rows, err := db.conn.Query(
		`SELECT id, filename, COALESCE(file_path, ''), COALESCE(owner_id, ''), COALESCE(profile, '{}'::jsonb), COALESCE(rows_data, '[]'::jsonb) FROM datasets ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query datasets: %w", err)
	}
	defer rows.Close()

	var datasets []DatasetRecord
	for rows.Next() {
		var d DatasetRecord
		var rowsJSON []byte
		if err := rows.Scan(&d.ID, &d.Filename, &d.FilePath, &d.OwnerID, &d.Profile, &rowsJSON); err != nil {
			return nil, fmt.Errorf("scan dataset: %w", err)
		}
		if len(rowsJSON) > 0 {
			json.Unmarshal(rowsJSON, &d.Rows)
		}
		if d.Rows == nil {
			d.Rows = []map[string]string{}
		}
		datasets = append(datasets, d)
	}
	return datasets, rows.Err()
}

// DeleteDataset removes a dataset by ID.
func (db *DB) DeleteDataset(id string) error {
	_, err := db.conn.Exec(`DELETE FROM datasets WHERE id = $1`, id)
	return err
}

// InitDatasetsTable creates the datasets table if it doesn't exist.
func (db *DB) InitDatasetsTable() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS datasets (
			id TEXT PRIMARY KEY,
			filename TEXT NOT NULL,
			file_path TEXT,
			owner_id TEXT DEFAULT '',
			profile JSONB,
			rows_data JSONB DEFAULT '[]'::jsonb,
			created_at TIMESTAMPTZ DEFAULT now()
		)
	`)
	if err != nil {
		return err
	}
	// Add owner_id column for existing tables (best-effort, ignore if already exists)
	db.conn.Exec(`ALTER TABLE datasets ADD COLUMN IF NOT EXISTS owner_id TEXT DEFAULT ''`)
	return nil
}

// --- Dashboards ---

// DashboardRecord represents a row in the dashboards table.
type DashboardRecord struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ChartIDs  []string  `json:"chart_ids"`
	CreatedAt time.Time `json:"created_at"`
}

func (db *DB) initDashboardsTable() error {
	_, err := db.conn.Exec(`
		CREATE TABLE IF NOT EXISTS dashboards (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL,
			chart_ids JSONB DEFAULT '[]'::jsonb,
			created_at TIMESTAMPTZ DEFAULT now()
		)
	`)
	return err
}

// SaveDashboard inserts or updates a dashboard.
func (db *DB) SaveDashboard(id, name string, chartIDs []string) error {
	idsJSON, err := json.Marshal(chartIDs)
	if err != nil {
		return fmt.Errorf("marshal chart IDs: %w", err)
	}
	_, err = db.conn.Exec(
		`INSERT INTO dashboards (id, name, chart_ids) VALUES ($1, $2, $3)
		 ON CONFLICT (id) DO UPDATE SET name=$2, chart_ids=$3`,
		id, name, idsJSON,
	)
	return err
}

// GetDashboards retrieves all dashboards from the database.
func (db *DB) GetDashboards() ([]DashboardRecord, error) {
	rows, err := db.conn.Query(
		`SELECT id, name, COALESCE(chart_ids, '[]'::jsonb), created_at FROM dashboards ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query dashboards: %w", err)
	}
	defer rows.Close()

	var dashboards []DashboardRecord
	for rows.Next() {
		var d DashboardRecord
		var idsJSON []byte
		if err := rows.Scan(&d.ID, &d.Name, &idsJSON, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan dashboard: %w", err)
		}
		if len(idsJSON) > 0 {
			json.Unmarshal(idsJSON, &d.ChartIDs)
		}
		if d.ChartIDs == nil {
			d.ChartIDs = []string{}
		}
		dashboards = append(dashboards, d)
	}
	return dashboards, rows.Err()
}

// DeleteDashboard removes a dashboard by ID.
func (db *DB) DeleteDashboard(id string) error {
	_, err := db.conn.Exec(`DELETE FROM dashboards WHERE id = $1`, id)
	return err
}

// --- Scheduled Reports ---

func (db *DB) SaveReport(id, name, datasetIDs, frequency string, dayOfWeek, dayOfMonth, hour int, emails, slackWebhook, teamsWebhook, lastSent, nextRun string, enabled bool, createdAt string) error {
	_, err := db.conn.Exec(
		`INSERT INTO scheduled_reports (id, name, dataset_ids, frequency, day_of_week, day_of_month, hour, emails, slack_webhook, teams_webhook, last_sent, next_run, enabled, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		 ON CONFLICT (id) DO UPDATE SET name=$2, dataset_ids=$3, frequency=$4, day_of_week=$5, day_of_month=$6, hour=$7, emails=$8, slack_webhook=$9, teams_webhook=$10, last_sent=$11, next_run=$12, enabled=$13`,
		id, name, datasetIDs, frequency, dayOfWeek, dayOfMonth, hour, emails, slackWebhook, teamsWebhook, lastSent, nextRun, enabled, createdAt,
	)
	return err
}

func (db *DB) LoadReports() ([]ReportRecord, error) {
	rows, err := db.conn.Query(`SELECT id, name, dataset_ids, frequency, day_of_week, day_of_month, hour, emails, COALESCE(slack_webhook,''), COALESCE(teams_webhook,''), COALESCE(last_sent,''), COALESCE(next_run,''), enabled, COALESCE(created_at,'') FROM scheduled_reports ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("query reports: %w", err)
	}
	defer rows.Close()

	var records []ReportRecord
	for rows.Next() {
		var r ReportRecord
		if err := rows.Scan(&r.ID, &r.Name, &r.DatasetIDs, &r.Frequency, &r.DayOfWeek, &r.DayOfMonth, &r.Hour, &r.Emails, &r.SlackWebhook, &r.TeamsWebhook, &r.LastSent, &r.NextRun, &r.Enabled, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan report: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (db *DB) DeleteReport(id string) error {
	_, err := db.conn.Exec(`DELETE FROM scheduled_reports WHERE id = $1`, id)
	return err
}

// --- Alert Rules ---

func (db *DB) SaveAlert(id, name, datasetID, metricCol, condition string, threshold float64, period, emails, slackHook string, enabled bool, lastChecked, createdAt string) error {
	_, err := db.conn.Exec(
		`INSERT INTO alert_rules (id, name, dataset_id, metric_col, condition, threshold, period, emails, slack_hook, enabled, last_checked, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		 ON CONFLICT (id) DO UPDATE SET name=$2, dataset_id=$3, metric_col=$4, condition=$5, threshold=$6, period=$7, emails=$8, slack_hook=$9, enabled=$10, last_checked=$11`,
		id, name, datasetID, metricCol, condition, threshold, period, emails, slackHook, enabled, lastChecked, createdAt,
	)
	return err
}

func (db *DB) LoadAlerts() ([]AlertRecord, error) {
	rows, err := db.conn.Query(`SELECT id, name, dataset_id, metric_col, condition, threshold, COALESCE(period,''), emails, COALESCE(slack_hook,''), enabled, COALESCE(last_checked,''), COALESCE(created_at,'') FROM alert_rules ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("query alerts: %w", err)
	}
	defer rows.Close()

	var records []AlertRecord
	for rows.Next() {
		var a AlertRecord
		if err := rows.Scan(&a.ID, &a.Name, &a.DatasetID, &a.MetricCol, &a.Condition, &a.Threshold, &a.Period, &a.Emails, &a.SlackHook, &a.Enabled, &a.LastChecked, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan alert: %w", err)
		}
		records = append(records, a)
	}
	return records, rows.Err()
}

func (db *DB) DeleteAlert(id string) error {
	_, err := db.conn.Exec(`DELETE FROM alert_rules WHERE id = $1`, id)
	return err
}

// --- Dashboard Layouts ---

func (db *DB) SaveLayout(id, name string, isDefault bool, tilesJSON []byte, createdAt, updatedAt string) error {
	_, err := db.conn.Exec(
		`INSERT INTO dashboard_layouts (id, name, is_default, tiles, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (id) DO UPDATE SET name=$2, is_default=$3, tiles=$4, updated_at=$6`,
		id, name, isDefault, tilesJSON, createdAt, updatedAt,
	)
	return err
}

func (db *DB) LoadLayouts() ([]LayoutRecord, error) {
	rows, err := db.conn.Query(`SELECT id, name, is_default, COALESCE(tiles,'[]'::jsonb), COALESCE(created_at,''), COALESCE(updated_at,'') FROM dashboard_layouts ORDER BY created_at ASC`)
	if err != nil {
		return nil, fmt.Errorf("query layouts: %w", err)
	}
	defer rows.Close()

	var records []LayoutRecord
	for rows.Next() {
		var l LayoutRecord
		var tilesJSON []byte
		if err := rows.Scan(&l.ID, &l.Name, &l.IsDefault, &tilesJSON, &l.CreatedAt, &l.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan layout: %w", err)
		}
		if len(tilesJSON) > 0 {
			l.Tiles = string(tilesJSON)
		} else {
			l.Tiles = "[]"
		}
		records = append(records, l)
	}
	return records, rows.Err()
}

func (db *DB) DeleteLayout(id string) error {
	_, err := db.conn.Exec(`DELETE FROM dashboard_layouts WHERE id = $1`, id)
	return err
}
