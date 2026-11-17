package store

import (
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"insightpilot/internal/data"
	"insightpilot/migrations"

	_ "github.com/lib/pq"
)

// ReportRecord represents a persisted scheduled report.
type ReportRecord struct {
	ID           string `json:"id"`
	OwnerID      string `json:"owner_id"`
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
	ID          string  `json:"id"`
	OwnerID     string  `json:"owner_id"`
	Name        string  `json:"name"`
	DatasetID   string  `json:"dataset_id"`
	MetricCol   string  `json:"metric_col"`
	Condition   string  `json:"condition"`
	Threshold   float64 `json:"threshold"`
	Period      string  `json:"period"`
	Emails      string  `json:"emails"`
	SlackHook   string  `json:"slack_hook"`
	Enabled     bool    `json:"enabled"`
	LastChecked string  `json:"last_checked"`
	CreatedAt   string  `json:"created_at"`
}

// LayoutRecord represents a persisted dashboard layout.
type LayoutRecord struct {
	ID        string `json:"id"`
	OwnerID   string `json:"owner_id"`
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
	OwnerID   string    `json:"owner_id"`
	CreatedAt time.Time `json:"created_at"`
	ChartType string    `json:"chart_type"`
	Label     string    `json:"label"`
	Data      any       `json:"data"`
	URL       string    `json:"url"`
}

// runMigrations applies embedded, numbered SQL migrations exactly once and
// records them in schema_migrations. A transaction-scoped advisory lock makes
// concurrent server starts safe.
func (db *DB) runMigrations() error {
	if _, err := db.conn.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	names, err := migrations.Names()
	if err != nil {
		return fmt.Errorf("list migrations: %w", err)
	}

	for _, name := range names {
		tx, err := db.conn.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %s: %w", name, err)
		}

		if _, err := tx.Exec(`SELECT pg_advisory_xact_lock($1)`, int64(872341)); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("lock migration %s: %w", name, err)
		}

		var applied bool
		if err := tx.QueryRow(`SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)`, name).Scan(&applied); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("check migration %s: %w", name, err)
		}
		if applied {
			_ = tx.Rollback()
			continue
		}

		body, err := migrations.Read(name)
		if err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if _, err := tx.Exec(body); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (version) VALUES ($1)`, name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %s: %w", name, err)
		}
	}
	return nil
}

// NewDB creates a new database connection using SUPABASE_URL and SUPABASE_KEY from env.
// It constructs a PostgreSQL connection string from the Supabase URL.
// If connection fails, it returns nil (graceful degradation to in-memory).
func NewDB() *DB {
	// DATABASE_URL takes precedence (local Postgres, compose, managed Postgres).
	if connStr := strings.TrimSpace(os.Getenv("DATABASE_URL")); connStr != "" {
		return openDB(connStr, "PostgreSQL (DATABASE_URL)")
	}

	supabaseURL := os.Getenv("SUPABASE_URL")
	supabaseKey := os.Getenv("SUPABASE_KEY")

	if supabaseURL == "" || supabaseKey == "" {
		log.Println("store: DATABASE_URL and SUPABASE_URL/SUPABASE_KEY not set, using in-memory storage")
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

	return openDB(buildSupabaseConnStr(projectRef, dbPassword), "Supabase PostgreSQL")
}

// openDB connects, tunes the pool, applies migrations, and falls back to
// in-memory storage by returning nil on any failure.
func openDB(connStr, source string) *DB {
	conn, err := sql.Open("postgres", connStr)
	if err != nil {
		log.Printf("store: failed to open database: %v, using in-memory storage", err)
		return nil
	}

	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	conn.SetConnMaxLifetime(5 * time.Minute)

	if err := conn.Ping(); err != nil {
		log.Printf("store: failed to ping database: %v, using in-memory storage", err)
		conn.Close()
		return nil
	}

	db := &DB{conn: conn}
	if err := db.runMigrations(); err != nil {
		log.Printf("store: failed to apply migrations: %v, using in-memory storage", err)
		conn.Close()
		return nil
	}

	log.Printf("store: connected to %s", source)
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

// Close closes the database connection.
func (db *DB) Close() error {
	if db.conn != nil {
		return db.conn.Close()
	}
	return nil
}

// --- Pinned Charts ---

// SavePinnedChart saves a pinned chart to the database.
func (db *DB) SavePinnedChart(id, ownerID, chartType, label string, data any, urlStr string) (string, error) {
	dataJSON, err := json.Marshal(data)
	if err != nil {
		return "", fmt.Errorf("marshal chart data: %w", err)
	}

	var chartID string
	if id == "" {
		err = db.conn.QueryRow(
			`INSERT INTO pinned_charts (owner_id, chart_type, label, data, url) VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			ownerID, chartType, label, dataJSON, urlStr,
		).Scan(&chartID)
	} else {
		chartID = id
		_, err = db.conn.Exec(
			`INSERT INTO pinned_charts (id, owner_id, chart_type, label, data, url) VALUES ($1, $2, $3, $4, $5, $6)
			 ON CONFLICT (id) DO UPDATE SET owner_id=$2, chart_type=$3, label=$4, data=$5, url=$6`,
			id, ownerID, chartType, label, dataJSON, urlStr,
		)
	}
	if err != nil {
		return "", fmt.Errorf("save pinned chart: %w", err)
	}
	return chartID, nil
}

// GetPinnedCharts retrieves all pinned charts from the database.
func (db *DB) GetPinnedCharts(ownerID string) ([]PinnedChartRecord, error) {
	query := `SELECT id, COALESCE(owner_id,''), created_at, chart_type, label, data, COALESCE(url, '') FROM pinned_charts`
	args := []interface{}{}
	if ownerID != "" {
		query += ` WHERE owner_id = $1`
		args = append(args, ownerID)
	}
	query += ` ORDER BY created_at DESC`
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query pinned charts: %w", err)
	}
	defer rows.Close()

	var charts []PinnedChartRecord
	for rows.Next() {
		var c PinnedChartRecord
		var dataJSON []byte
		if err := rows.Scan(&c.ID, &c.OwnerID, &c.CreatedAt, &c.ChartType, &c.Label, &dataJSON, &c.URL); err != nil {
			return nil, fmt.Errorf("scan pinned chart: %w", err)
		}
		if len(dataJSON) > 0 {
			_ = json.Unmarshal(dataJSON, &c.Data)
		}
		charts = append(charts, c)
	}
	return charts, rows.Err()
}

// DeletePinnedChart removes a pinned chart by ID.
func (db *DB) DeletePinnedChart(id, ownerID string) error {
	if ownerID == "" {
		_, err := db.conn.Exec(`DELETE FROM pinned_charts WHERE id = $1`, id)
		return err
	}
	_, err := db.conn.Exec(`DELETE FROM pinned_charts WHERE id = $1 AND owner_id = $2`, id, ownerID)
	return err
}

// --- Users ---

// UserRecord represents a persisted user.
type UserRecord struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"password_hash"`
	Role         string    `json:"role"`
	CreatedAt    time.Time `json:"created_at"`
}

// SaveDatasetField inserts or updates a semantic field.
func (db *DB) SaveDatasetField(f *data.SemanticField) error {
	_, err := db.conn.Exec(
		`INSERT INTO dataset_fields (id, owner_id, dataset_id, name, kind, config, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		 ON CONFLICT (id) DO UPDATE SET owner_id=$2, dataset_id=$3, name=$4, kind=$5, config=$6, updated_at=$8`,
		f.ID, f.OwnerID, f.DatasetID, f.Name, f.Kind, f.Config, f.CreatedAt, f.UpdatedAt,
	)
	return err
}

// LoadDatasetFields retrieves all semantic fields (optionally filtered by dataset).
func (db *DB) LoadDatasetFields(datasetID, ownerID string) ([]data.SemanticField, error) {
	q := `SELECT id, COALESCE(owner_id,''), dataset_id, name, kind, COALESCE(config,'{}'::jsonb), COALESCE(created_at,''), COALESCE(updated_at,'') FROM dataset_fields`
	conds := []string{}
	args := []interface{}{}
	if datasetID != "" {
		args = append(args, datasetID)
		conds = append(conds, fmt.Sprintf("dataset_id = $%d", len(args)))
	}
	if ownerID != "" {
		args = append(args, ownerID)
		conds = append(conds, fmt.Sprintf("owner_id = $%d", len(args)))
	}
	if len(conds) > 0 {
		q += ` WHERE ` + strings.Join(conds, " AND ")
	}
	q += ` ORDER BY name ASC`
	rows, err := db.conn.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("query dataset_fields: %w", err)
	}
	defer rows.Close()

	var fields []data.SemanticField
	for rows.Next() {
		var f data.SemanticField
		var configJSON []byte
		if err := rows.Scan(&f.ID, &f.OwnerID, &f.DatasetID, &f.Name, &f.Kind, &configJSON, &f.CreatedAt, &f.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan dataset_field: %w", err)
		}
		if len(configJSON) > 0 {
			f.Config = configJSON
		}
		fields = append(fields, f)
	}
	return fields, rows.Err()
}

// DeleteDatasetField removes a semantic field by ID.
func (db *DB) DeleteDatasetField(id, ownerID string) error {
	if ownerID == "" {
		_, err := db.conn.Exec(`DELETE FROM dataset_fields WHERE id = $1`, id)
		return err
	}
	_, err := db.conn.Exec(`DELETE FROM dataset_fields WHERE id = $1 AND owner_id = $2`, id, ownerID)
	return err
}

// --- Saved Queries (SQL Lab) ---

// SaveSavedQuery inserts or updates a saved query.
func (db *DB) SaveSavedQuery(id, ownerID, name, datasetIDs, sql, createdAt, updatedAt string) error {
	_, err := db.conn.Exec(
		`INSERT INTO saved_queries (id, owner_id, name, dataset_ids, sql, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7)
		 ON CONFLICT (id) DO UPDATE SET owner_id=$2, name=$3, dataset_ids=$4, sql=$5, updated_at=$7`,
		id, ownerID, name, datasetIDs, sql, createdAt, updatedAt,
	)
	return err
}

// LoadSavedQueries retrieves all saved queries ordered by most-recently-updated.
func (db *DB) LoadSavedQueries(ownerID string) ([]SavedQueryRecord, error) {
	query := `SELECT id, COALESCE(owner_id,''), name, COALESCE(dataset_ids,'[]'), COALESCE(sql,''), COALESCE(created_at,''), COALESCE(updated_at,'') FROM saved_queries`
	args := []interface{}{}
	if ownerID != "" {
		query += ` WHERE owner_id = $1`
		args = append(args, ownerID)
	}
	query += ` ORDER BY updated_at DESC`
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query saved_queries: %w", err)
	}
	defer rows.Close()

	var records []SavedQueryRecord
	for rows.Next() {
		var r SavedQueryRecord
		if err := rows.Scan(&r.ID, &r.OwnerID, &r.Name, &r.DatasetIDs, &r.SQL, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan saved_query: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

// DeleteSavedQuery removes a saved query by ID.
func (db *DB) DeleteSavedQuery(id, ownerID string) error {
	if ownerID == "" {
		_, err := db.conn.Exec(`DELETE FROM saved_queries WHERE id = $1`, id)
		return err
	}
	_, err := db.conn.Exec(`DELETE FROM saved_queries WHERE id = $1 AND owner_id = $2`, id, ownerID)
	return err
}

// SavedQueryRecord represents a persisted saved query.
type SavedQueryRecord struct {
	ID         string `json:"id"`
	OwnerID    string `json:"owner_id"`
	Name       string `json:"name"`
	DatasetIDs string `json:"dataset_ids"`
	SQL        string `json:"sql"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// SaveUser persists a user to the database.
func (db *DB) SaveUser(id, email, name, passwordHash, role string, createdAt time.Time) error {
	if role == "" {
		role = "editor"
	}
	_, err := db.conn.Exec(
		`INSERT INTO users (id, email, name, password_hash, role, created_at) VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (id) DO UPDATE SET email=$2, name=$3, password_hash=$4, role=$5`,
		id, email, name, passwordHash, role, createdAt,
	)
	return err
}

// LoadUsers retrieves all users from the database.
func (db *DB) LoadUsers() ([]UserRecord, error) {
	rows, err := db.conn.Query(
		`SELECT id, email, name, password_hash, COALESCE(role,'editor'), created_at FROM users ORDER BY created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}
	defer rows.Close()

	var users []UserRecord
	for rows.Next() {
		var u UserRecord
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Role, &u.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// SaveJWTSecret persists the JWT signing secret to the database.
func (db *DB) SaveJWTSecret(secret []byte) error {
	encoded := fmt.Sprintf("%x", secret)
	_, err := db.conn.Exec(
		`INSERT INTO app_config (key, value) VALUES ('jwt_secret', $1)
		 ON CONFLICT (key) DO UPDATE SET value=$1`,
		encoded,
	)
	return err
}

// LoadJWTSecret retrieves the JWT signing secret from the database.
func (db *DB) LoadJWTSecret() ([]byte, error) {
	var encoded string
	err := db.conn.QueryRow(
		`SELECT value FROM app_config WHERE key = 'jwt_secret'`,
	).Scan(&encoded)
	if err != nil {
		return nil, err
	}
	return hex.DecodeString(encoded)
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
			_ = json.Unmarshal(rowsJSON, &d.Rows)
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

// --- Dashboards ---

// DashboardRecord represents a row in the dashboards table.
type DashboardRecord struct {
	ID        string    `json:"id"`
	OwnerID   string    `json:"owner_id"`
	Name      string    `json:"name"`
	ChartIDs  []string  `json:"chart_ids"`
	CreatedAt time.Time `json:"created_at"`
}

// SaveDashboard inserts or updates a dashboard.
func (db *DB) SaveDashboard(id, name, ownerID string, chartIDs []string) error {
	idsJSON, err := json.Marshal(chartIDs)
	if err != nil {
		return fmt.Errorf("marshal chart IDs: %w", err)
	}
	_, err = db.conn.Exec(
		`INSERT INTO dashboards (id, name, owner_id, chart_ids) VALUES ($1, $2, $3, $4)
		 ON CONFLICT (id) DO UPDATE SET name=$2, owner_id=$3, chart_ids=$4`,
		id, name, ownerID, idsJSON,
	)
	return err
}

// GetDashboards retrieves all dashboards from the database.
func (db *DB) GetDashboards(ownerID string) ([]DashboardRecord, error) {
	query := `SELECT id, COALESCE(owner_id,''), name, COALESCE(chart_ids, '[]'::jsonb), created_at FROM dashboards`
	args := []interface{}{}
	if ownerID != "" {
		query += ` WHERE owner_id = $1`
		args = append(args, ownerID)
	}
	query += ` ORDER BY created_at ASC`
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query dashboards: %w", err)
	}
	defer rows.Close()

	var dashboards []DashboardRecord
	for rows.Next() {
		var d DashboardRecord
		var idsJSON []byte
		if err := rows.Scan(&d.ID, &d.OwnerID, &d.Name, &idsJSON, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan dashboard: %w", err)
		}
		if len(idsJSON) > 0 {
			_ = json.Unmarshal(idsJSON, &d.ChartIDs)
		}
		if d.ChartIDs == nil {
			d.ChartIDs = []string{}
		}
		dashboards = append(dashboards, d)
	}
	return dashboards, rows.Err()
}

// DeleteDashboard removes a dashboard by ID.
func (db *DB) DeleteDashboard(id, ownerID string) error {
	if ownerID == "" {
		_, err := db.conn.Exec(`DELETE FROM dashboards WHERE id = $1`, id)
		return err
	}
	_, err := db.conn.Exec(`DELETE FROM dashboards WHERE id = $1 AND owner_id = $2`, id, ownerID)
	return err
}

// --- Scheduled Reports ---

func (db *DB) SaveReport(id, ownerID, name, datasetIDs, frequency string, dayOfWeek, dayOfMonth, hour int, emails, slackWebhook, teamsWebhook, lastSent, nextRun string, enabled bool, createdAt string) error {
	_, err := db.conn.Exec(
		`INSERT INTO scheduled_reports (id, owner_id, name, dataset_ids, frequency, day_of_week, day_of_month, hour, emails, slack_webhook, teams_webhook, last_sent, next_run, enabled, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		 ON CONFLICT (id) DO UPDATE SET owner_id=$2, name=$3, dataset_ids=$4, frequency=$5, day_of_week=$6, day_of_month=$7, hour=$8, emails=$9, slack_webhook=$10, teams_webhook=$11, last_sent=$12, next_run=$13, enabled=$14`,
		id, ownerID, name, datasetIDs, frequency, dayOfWeek, dayOfMonth, hour, emails, slackWebhook, teamsWebhook, lastSent, nextRun, enabled, createdAt,
	)
	return err
}

func (db *DB) LoadReports(ownerID string) ([]ReportRecord, error) {
	query := `SELECT id, COALESCE(owner_id,''), name, dataset_ids, frequency, day_of_week, day_of_month, hour, emails, COALESCE(slack_webhook,''), COALESCE(teams_webhook,''), COALESCE(last_sent,''), COALESCE(next_run,''), enabled, COALESCE(created_at,'') FROM scheduled_reports`
	args := []interface{}{}
	if ownerID != "" {
		query += ` WHERE owner_id = $1`
		args = append(args, ownerID)
	}
	query += ` ORDER BY created_at ASC`
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query reports: %w", err)
	}
	defer rows.Close()

	var records []ReportRecord
	for rows.Next() {
		var r ReportRecord
		if err := rows.Scan(&r.ID, &r.OwnerID, &r.Name, &r.DatasetIDs, &r.Frequency, &r.DayOfWeek, &r.DayOfMonth, &r.Hour, &r.Emails, &r.SlackWebhook, &r.TeamsWebhook, &r.LastSent, &r.NextRun, &r.Enabled, &r.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan report: %w", err)
		}
		records = append(records, r)
	}
	return records, rows.Err()
}

func (db *DB) DeleteReport(id, ownerID string) error {
	if ownerID == "" {
		_, err := db.conn.Exec(`DELETE FROM scheduled_reports WHERE id = $1`, id)
		return err
	}
	_, err := db.conn.Exec(`DELETE FROM scheduled_reports WHERE id = $1 AND owner_id = $2`, id, ownerID)
	return err
}

// --- Alert Rules ---

func (db *DB) SaveAlert(id, ownerID, name, datasetID, metricCol, condition string, threshold float64, period, emails, slackHook string, enabled bool, lastChecked, createdAt string) error {
	_, err := db.conn.Exec(
		`INSERT INTO alert_rules (id, owner_id, name, dataset_id, metric_col, condition, threshold, period, emails, slack_hook, enabled, last_checked, created_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		 ON CONFLICT (id) DO UPDATE SET owner_id=$2, name=$3, dataset_id=$4, metric_col=$5, condition=$6, threshold=$7, period=$8, emails=$9, slack_hook=$10, enabled=$11, last_checked=$12`,
		id, ownerID, name, datasetID, metricCol, condition, threshold, period, emails, slackHook, enabled, lastChecked, createdAt,
	)
	return err
}

func (db *DB) LoadAlerts(ownerID string) ([]AlertRecord, error) {
	query := `SELECT id, COALESCE(owner_id,''), name, dataset_id, metric_col, condition, threshold, COALESCE(period,''), emails, COALESCE(slack_hook,''), enabled, COALESCE(last_checked,''), COALESCE(created_at,'') FROM alert_rules`
	args := []interface{}{}
	if ownerID != "" {
		query += ` WHERE owner_id = $1`
		args = append(args, ownerID)
	}
	query += ` ORDER BY created_at ASC`
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query alerts: %w", err)
	}
	defer rows.Close()

	var records []AlertRecord
	for rows.Next() {
		var a AlertRecord
		if err := rows.Scan(&a.ID, &a.OwnerID, &a.Name, &a.DatasetID, &a.MetricCol, &a.Condition, &a.Threshold, &a.Period, &a.Emails, &a.SlackHook, &a.Enabled, &a.LastChecked, &a.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan alert: %w", err)
		}
		records = append(records, a)
	}
	return records, rows.Err()
}

func (db *DB) DeleteAlert(id, ownerID string) error {
	if ownerID == "" {
		_, err := db.conn.Exec(`DELETE FROM alert_rules WHERE id = $1`, id)
		return err
	}
	_, err := db.conn.Exec(`DELETE FROM alert_rules WHERE id = $1 AND owner_id = $2`, id, ownerID)
	return err
}

// --- Dashboard Layouts ---

func (db *DB) SaveLayout(id, ownerID, name string, isDefault bool, tilesJSON []byte, createdAt, updatedAt string) error {
	_, err := db.conn.Exec(
		`INSERT INTO dashboard_layouts (id, owner_id, name, is_default, tiles, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7)
		 ON CONFLICT (id) DO UPDATE SET owner_id=$2, name=$3, is_default=$4, tiles=$5, updated_at=$7`,
		id, ownerID, name, isDefault, tilesJSON, createdAt, updatedAt,
	)
	return err
}

func (db *DB) LoadLayouts(ownerID string) ([]LayoutRecord, error) {
	query := `SELECT id, COALESCE(owner_id,''), name, is_default, COALESCE(tiles,'[]'::jsonb), COALESCE(created_at,''), COALESCE(updated_at,'') FROM dashboard_layouts`
	args := []interface{}{}
	if ownerID != "" {
		query += ` WHERE owner_id = $1`
		args = append(args, ownerID)
	}
	query += ` ORDER BY created_at ASC`
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query layouts: %w", err)
	}
	defer rows.Close()

	var records []LayoutRecord
	for rows.Next() {
		var l LayoutRecord
		var tilesJSON []byte
		if err := rows.Scan(&l.ID, &l.OwnerID, &l.Name, &l.IsDefault, &tilesJSON, &l.CreatedAt, &l.UpdatedAt); err != nil {
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

func (db *DB) DeleteLayout(id, ownerID string) error {
	if ownerID == "" {
		_, err := db.conn.Exec(`DELETE FROM dashboard_layouts WHERE id = $1`, id)
		return err
	}
	_, err := db.conn.Exec(`DELETE FROM dashboard_layouts WHERE id = $1 AND owner_id = $2`, id, ownerID)
	return err
}

// --- Glossary (Semantic Layer) ---

// SaveGlossaryDef persists a metric definition.
func (db *DB) SaveGlossaryDef(def *data.MetricDefinition) error {
	_, err := db.conn.Exec(
		`INSERT INTO glossary_defs (id, owner_id, name, expression, description, dataset_id, return_type, created_at, updated_at) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		 ON CONFLICT (id) DO UPDATE SET owner_id=$2, name=$3, expression=$4, description=$5, dataset_id=$6, return_type=$7, updated_at=$9`,
		def.ID, def.OwnerID, def.Name, def.Expression, def.Description, def.DatasetID, def.ReturnType, def.CreatedAt, def.UpdatedAt,
	)
	return err
}

// LoadGlossaryDefs retrieves all metric definitions.
func (db *DB) LoadGlossaryDefs(ownerID string) ([]*data.MetricDefinition, error) {
	query := `SELECT id, COALESCE(owner_id,''), name, COALESCE(expression,''), COALESCE(description,''), COALESCE(dataset_id,''), COALESCE(return_type,'number'), COALESCE(created_at,''), COALESCE(updated_at,'') FROM glossary_defs`
	args := []interface{}{}
	if ownerID != "" {
		query += ` WHERE owner_id = $1`
		args = append(args, ownerID)
	}
	query += ` ORDER BY name ASC`
	rows, err := db.conn.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query glossary: %w", err)
	}
	defer rows.Close()

	var defs []*data.MetricDefinition
	for rows.Next() {
		d := &data.MetricDefinition{}
		if err := rows.Scan(&d.ID, &d.OwnerID, &d.Name, &d.Expression, &d.Description, &d.DatasetID, &d.ReturnType, &d.CreatedAt, &d.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan glossary: %w", err)
		}
		defs = append(defs, d)
	}
	return defs, rows.Err()
}

// DeleteGlossaryDef removes a metric definition by ID.
func (db *DB) DeleteGlossaryDef(id, ownerID string) error {
	if ownerID == "" {
		_, err := db.conn.Exec(`DELETE FROM glossary_defs WHERE id = $1`, id)
		return err
	}
	_, err := db.conn.Exec(`DELETE FROM glossary_defs WHERE id = $1 AND owner_id = $2`, id, ownerID)
	return err
}

// --- Shared dashboards (share tokens) ---

// ShareRecord represents a persisted share link.
type ShareRecord struct {
	Token     string
	OwnerID   string
	CreatedAt time.Time
	ExpiresAt time.Time
	Charts    []byte
	URL       string
}

// SaveShare inserts or replaces a share token.
func (db *DB) SaveShare(token, ownerID string, createdAt, expiresAt time.Time, charts []byte, url string) error {
	_, err := db.conn.Exec(
		`INSERT INTO shared_dashboards (token, owner_id, created_at, expires_at, charts, url) VALUES ($1,$2,$3,$4,$5,$6)
		 ON CONFLICT (token) DO UPDATE SET owner_id=$2, expires_at=$4, charts=$5, url=$6`,
		token, ownerID, createdAt, expiresAt, charts, url,
	)
	return err
}

// GetShare returns a share token, or nil when it does not exist.
func (db *DB) GetShare(token string) (*ShareRecord, error) {
	var rec ShareRecord
	err := db.conn.QueryRow(
		`SELECT token, COALESCE(owner_id,''), created_at, expires_at, charts, COALESCE(url,'') FROM shared_dashboards WHERE token = $1`,
		token,
	).Scan(&rec.Token, &rec.OwnerID, &rec.CreatedAt, &rec.ExpiresAt, &rec.Charts, &rec.URL)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get share: %w", err)
	}
	return &rec, nil
}

// DeleteExpiredShares removes expired share links.
func (db *DB) DeleteExpiredShares(now time.Time) (int64, error) {
	res, err := db.conn.Exec(`DELETE FROM shared_dashboards WHERE expires_at <= $1`, now)
	if err != nil {
		return 0, fmt.Errorf("delete expired shares: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// --- Transform pipelines ---

// TransformPipelineRecord represents persisted ETL steps for a dataset.
type TransformPipelineRecord struct {
	DatasetID string
	OwnerID   string
	Steps     []byte
	Undone    []byte
	UpdatedAt time.Time
}

// SaveTransformPipeline upserts the steps for a dataset.
func (db *DB) SaveTransformPipeline(datasetID, ownerID string, steps, undone []byte) error {
	_, err := db.conn.Exec(
		`INSERT INTO transform_pipelines (dataset_id, owner_id, steps, undone, updated_at) VALUES ($1,$2,$3,$4,now())
		 ON CONFLICT (dataset_id) DO UPDATE SET owner_id=$2, steps=$3, undone=$4, updated_at=now()`,
		datasetID, ownerID, steps, undone,
	)
	return err
}

// LoadTransformPipelines returns all persisted pipelines.
func (db *DB) LoadTransformPipelines() ([]TransformPipelineRecord, error) {
	rows, err := db.conn.Query(`SELECT dataset_id, COALESCE(owner_id,''), steps, undone, updated_at FROM transform_pipelines`)
	if err != nil {
		return nil, fmt.Errorf("query transform pipelines: %w", err)
	}
	defer rows.Close()

	var records []TransformPipelineRecord
	for rows.Next() {
		var rec TransformPipelineRecord
		if err := rows.Scan(&rec.DatasetID, &rec.OwnerID, &rec.Steps, &rec.Undone, &rec.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan transform pipeline: %w", err)
		}
		records = append(records, rec)
	}
	return records, rows.Err()
}

// DeleteTransformPipeline removes a dataset pipeline.
func (db *DB) DeleteTransformPipeline(datasetID, ownerID string) error {
	if ownerID == "" {
		_, err := db.conn.Exec(`DELETE FROM transform_pipelines WHERE dataset_id = $1`, datasetID)
		return err
	}
	_, err := db.conn.Exec(`DELETE FROM transform_pipelines WHERE dataset_id = $1 AND owner_id = $2`, datasetID, ownerID)
	return err
}

// --- Conversation sessions ---

// SessionRecord represents a persisted conversation session.
type SessionRecord struct {
	ID            string
	OwnerID       string
	DatasetIDs    []byte
	History       []byte
	ActiveContext []byte
	CreatedAt     time.Time
	LastActiveAt  time.Time
}

// SaveConversationSession upserts a conversation session.
func (db *DB) SaveConversationSession(id, ownerID string, datasetIDs, history, activeContext []byte, createdAt, lastActiveAt time.Time) error {
	_, err := db.conn.Exec(
		`INSERT INTO conversation_sessions (id, owner_id, dataset_ids, history, active_context, created_at, last_active_at) VALUES ($1,$2,$3,$4,$5,$6,$7)
		 ON CONFLICT (id) DO UPDATE SET owner_id=$2, dataset_ids=$3, history=$4, active_context=$5, last_active_at=$7`,
		id, ownerID, datasetIDs, history, activeContext, createdAt, lastActiveAt,
	)
	return err
}

// LoadConversationSession returns a session by ID, or nil when absent.
func (db *DB) LoadConversationSession(id string) (*SessionRecord, error) {
	var rec SessionRecord
	err := db.conn.QueryRow(
		`SELECT id, COALESCE(owner_id,''), dataset_ids, history, active_context, created_at, last_active_at FROM conversation_sessions WHERE id = $1`,
		id,
	).Scan(&rec.ID, &rec.OwnerID, &rec.DatasetIDs, &rec.History, &rec.ActiveContext, &rec.CreatedAt, &rec.LastActiveAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load conversation session: %w", err)
	}
	return &rec, nil
}

// DeleteConversationSession removes a session, scoped to owner when provided.
func (db *DB) DeleteConversationSession(id, ownerID string) error {
	if ownerID == "" {
		_, err := db.conn.Exec(`DELETE FROM conversation_sessions WHERE id = $1`, id)
		return err
	}
	_, err := db.conn.Exec(`DELETE FROM conversation_sessions WHERE id = $1 AND owner_id = $2`, id, ownerID)
	return err
}

// DeleteStaleConversationSessions prunes sessions inactive before the cutoff.
func (db *DB) DeleteStaleConversationSessions(before time.Time) (int64, error) {
	res, err := db.conn.Exec(`DELETE FROM conversation_sessions WHERE last_active_at < $1`, before)
	if err != nil {
		return 0, fmt.Errorf("delete stale sessions: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}
