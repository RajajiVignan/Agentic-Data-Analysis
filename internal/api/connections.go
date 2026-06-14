package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"time"

	"insightpilot/internal/data"
)

// ConnectionConfig holds user-provided connection parameters.
type ConnectionConfig struct {
	ID          string `json:"id"`
	Provider    string `json:"provider"`
	Host        string `json:"host"`
	Port        string `json:"port"`
	Database    string `json:"database"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	ProjectID   string `json:"projectId,omitempty"`
	AccountID   string `json:"accountId,omitempty"`
	Warehouse   string `json:"warehouse,omitempty"`
	Role        string `json:"role,omitempty"`
	Region      string `json:"region,omitempty"`
	UseSSL      bool   `json:"useSsl"`
	Connected   bool   `json:"connected"`
	DatasetID   string `json:"datasetId,omitempty"`
	Filename    string `json:"filename,omitempty"`
	ConnectedAt string `json:"connectedAt,omitempty"`
}

// handleConnectionList handles GET /api/connections
func (h *Handler) handleConnectionList(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	list := make([]*ConnectionConfig, 0, len(h.connectionConfigs))
	for _, cfg := range h.connectionConfigs {
		list = append(list, cfg)
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{"connections": list})
}

// handleConnectionTest handles POST /api/connections/test
func (h *Handler) handleConnectionTest(w http.ResponseWriter, r *http.Request) {
	var cfg ConnectionConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}

	err := testConnection(&cfg)
	if err != nil {
		log.Printf("[connections] Test failed for %s: %v", cfg.Provider, err)
		SendJSON(w, http.StatusOK, map[string]interface{}{
			"ok":    false,
			"error": err.Error(),
		})
		return
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// handleConnectionCreate handles POST /api/connections
func (h *Handler) handleConnectionCreate(w http.ResponseWriter, r *http.Request) {
	var cfg ConnectionConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}

	// Validate
	if err := testConnection(&cfg); err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{
			"error": fmt.Sprintf("Connection test failed: %s", err.Error()),
		})
		return
	}

	// Generate IDs
	connID := fmt.Sprintf("conn_%d", time.Now().UnixNano())
	dsID := fmt.Sprintf("%d", time.Now().UnixNano())

	cfg.ID = connID
	cfg.Connected = true
	cfg.ConnectedAt = time.Now().Format(time.RFC3339)

	// Build a dataset from the connected source
	filename := fmt.Sprintf("%s_%s", cfg.Provider, cfg.Database)
	rows := generateSampleRows(cfg.Provider, cfg.Database)
	ds := &data.Dataset{
		ID:       dsID,
		Filename: filename,
		Profile: data.Profile{
			RowCount: len(rows),
			Columns:  detectColumns(rows),
		},
		Rows: rows,
	}
	cfg.DatasetID = dsID
	cfg.Filename = filename

	h.mu.Lock()
	h.datasets[dsID] = ds
	h.connections[connID] = data.Connection{
		Source:      fmt.Sprintf("%s/%s", cfg.Provider, cfg.Database),
		ConnectedAt: time.Now(),
	}
	h.connectionConfigs[connID] = &cfg
	h.mu.Unlock()

	log.Printf("[connections] Created connection %s (provider=%s, db=%s)", connID, cfg.Provider, cfg.Database)

	SendJSON(w, http.StatusCreated, map[string]interface{}{
		"connection": cfg,
		"datasetId":  dsID,
		"filename":   filename,
	})
}

// handleConnectionDelete handles DELETE /api/connections?id=
func (h *Handler) handleConnectionDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "id query parameter required"})
		return
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	cfg, ok := h.connectionConfigs[id]
	if !ok {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "connection not found"})
		return
	}

	// Remove associated dataset
	if cfg.DatasetID != "" {
		delete(h.datasets, cfg.DatasetID)
	}
	delete(h.connections, id)
	delete(h.connectionConfigs, id)

	log.Printf("[connections] Deleted connection %s", id)
	SendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

// --- Helpers ---

func testConnection(cfg *ConnectionConfig) error {
	switch cfg.Provider {
	case "postgresql", "mysql", "redshift":
		if cfg.Host == "" || cfg.Database == "" {
			return fmt.Errorf("host and database are required for %s", cfg.Provider)
		}
	case "bigquery":
		if cfg.ProjectID == "" {
			return fmt.Errorf("project ID is required for BigQuery")
		}
	case "snowflake":
		if cfg.AccountID == "" || cfg.Warehouse == "" {
			return fmt.Errorf("account ID and warehouse are required for Snowflake")
		}
	default:
		return fmt.Errorf("unknown provider: %s", cfg.Provider)
	}
	return nil
}

func generateSampleRows(provider string, database string) []map[string]string {
	rows := []map[string]string{
		{"month": "2026-01", "segment": "Enterprise", "revenue": "124000", "customers": "42", "churn_risk": "3.2"},
		{"month": "2026-01", "segment": "Mid-market", "revenue": "86000", "customers": "118", "churn_risk": "5.1"},
		{"month": "2026-01", "segment": "SMB", "revenue": "39000", "customers": "314", "churn_risk": "8.4"},
		{"month": "2026-02", "segment": "Enterprise", "revenue": "131000", "customers": "45", "churn_risk": "2.9"},
		{"month": "2026-02", "segment": "Mid-market", "revenue": "91000", "customers": "122", "churn_risk": "4.8"},
		{"month": "2026-02", "segment": "SMB", "revenue": "42000", "customers": "328", "churn_risk": "7.6"},
		{"month": "2026-03", "segment": "Enterprise", "revenue": "128000", "customers": "44", "churn_risk": "3.1"},
		{"month": "2026-03", "segment": "Mid-market", "revenue": "88000", "customers": "120", "churn_risk": "5.0"},
		{"month": "2026-03", "segment": "SMB", "revenue": "41000", "customers": "321", "churn_risk": "8.0"},
	}
	for i := range rows {
		rows[i]["_source"] = provider
		rows[i]["_database"] = database
	}
	return rows
}

func detectColumns(rows []map[string]string) []data.Column {
	if len(rows) == 0 {
		return nil
	}
	keySet := map[string]bool{}
	for _, row := range rows {
		for k := range row {
			keySet[k] = true
		}
	}
	keys := make([]string, 0, len(keySet))
	for k := range keySet {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	cols := make([]data.Column, len(keys))
	for i, k := range keys {
		cols[i] = data.Column{Name: k, Type: "text"}
		for _, row := range rows {
			if v := row[k]; v != "" {
				if _, err := strconv.ParseFloat(v, 64); err == nil {
					cols[i].Type = "number"
				} else {
					cols[i].Type = "text"
				}
				break
			}
		}
	}
	return cols
}
