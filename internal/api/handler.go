package api

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"insightpilot/internal/agent"
	"insightpilot/internal/data"
	"insightpilot/internal/store"
)

type Handler struct {
	datasets       map[string]*data.Dataset
	connections    map[string]data.Connection
	pinnedCharts   map[string]*PinnedChart
	uploadDir      string
	plotsDir       string
	analyzer       agent.Analyzer
	pythonBridge   *PythonBridge
	db             *store.DB
	allowedOrigins map[string]bool
	mu             sync.RWMutex
}

type PinnedChart struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at,omitempty"`
	ChartType string `json:"chart_type"`
	Label     string `json:"label"`
	Data      any    `json:"data"`
	URL       string `json:"url,omitempty"`
}

func NewHandler(cfg agent.Config) *Handler {
	db := store.NewDB()
	uploadDir := resolveDataDir("UPLOAD_DIR", "uploads")
	plotsDir := filepath.Join(uploadDir, "plots")

	h := &Handler{
		datasets:       make(map[string]*data.Dataset),
		connections:    make(map[string]data.Connection),
		pinnedCharts:   make(map[string]*PinnedChart),
		uploadDir:      uploadDir,
		plotsDir:       plotsDir,
		analyzer:       agent.NewLLMAnalyzer(cfg),
		pythonBridge:   NewPythonBridge(plotsDir),
		db:             db,
		allowedOrigins: configuredAllowedOrigins(),
	}

	// Load pinned charts from database on startup
	if db != nil {
		h.loadPinnedChartsFromDB()
	}

	h.startPlotCleanup()

	return h
}

func resolveDataDir(envName, defaultRel string) string {
	if configured := os.Getenv(envName); configured != "" {
		if abs, err := filepath.Abs(configured); err == nil {
			return abs
		}
		return configured
	}

	root, err := projectRoot()
	if err != nil {
		root, _ = os.Getwd()
	}
	return filepath.Join(root, defaultRel)
}

func projectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found from %s", dir)
		}
		dir = parent
	}
}

func configuredAllowedOrigins() map[string]bool {
	raw := strings.TrimSpace(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if raw == "" {
		raw = strings.TrimSpace(os.Getenv("FRONTEND_ORIGIN"))
	}
	if raw == "" && !isProduction() {
		raw = "http://localhost:3000,http://127.0.0.1:3000,http://localhost:3001,http://127.0.0.1:3001"
	}

	origins := make(map[string]bool)
	for _, origin := range strings.Split(raw, ",") {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins[origin] = true
		}
	}
	return origins
}

func isProduction() bool {
	for _, name := range []string{"APP_ENV", "GO_ENV", "NODE_ENV"} {
		if strings.EqualFold(os.Getenv(name), "production") {
			return true
		}
	}
	return false
}

func (h *Handler) startPlotCleanup() {
	retention := 24 * time.Hour
	if raw := strings.TrimSpace(os.Getenv("PLOT_RETENTION_HOURS")); raw != "" {
		hours, err := strconv.Atoi(raw)
		if err != nil || hours < 0 {
			log.Printf("api: invalid PLOT_RETENTION_HOURS=%q, using %s", raw, retention)
		} else if hours == 0 {
			return
		} else {
			retention = time.Duration(hours) * time.Hour
		}
	}

	h.cleanupPlotArtifacts(retention)

	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			h.cleanupPlotArtifacts(retention)
		}
	}()
}

func (h *Handler) cleanupPlotArtifacts(retention time.Duration) {
	if removed, err := h.pythonBridge.CleanupOlderThan(retention); err != nil {
		log.Printf("api: plot cleanup failed for %s: %v", h.plotsDir, err)
	} else if removed > 0 {
		log.Printf("api: removed %d stale plot files from %s", removed, h.plotsDir)
	}

	root, err := projectRoot()
	if err != nil {
		return
	}
	legacyPlotsDir := filepath.Join(root, "internal", "api", "uploads", "plots")
	if legacyPlotsDir == h.plotsDir {
		return
	}
	if info, err := os.Stat(legacyPlotsDir); err != nil || !info.IsDir() {
		return
	}
	legacyBridge := NewPythonBridge(legacyPlotsDir)
	if removed, err := legacyBridge.CleanupOlderThan(retention); err != nil {
		log.Printf("api: legacy plot cleanup failed for %s: %v", legacyPlotsDir, err)
	} else if removed > 0 {
		log.Printf("api: removed %d stale legacy plot files from %s", removed, legacyPlotsDir)
	}
}

func (h *Handler) loadPinnedChartsFromDB() {
	charts, err := h.db.GetPinnedCharts()
	if err != nil {
		fmt.Printf("Warning: failed to load pinned charts from DB: %v\n", err)
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, c := range charts {
		h.pinnedCharts[c.ID] = &PinnedChart{
			ID:        c.ID,
			CreatedAt: c.CreatedAt.Format(time.RFC3339),
			ChartType: c.ChartType,
			Label:     c.Label,
			Data:      c.Data,
			URL:       c.URL,
		}
	}
	fmt.Printf("Loaded %d pinned charts from database\n", len(charts))
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	corsHandler := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			h.setCORSHeaders(w, r)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next(w, r)
		}
	}

	mux.HandleFunc("/api/health", corsHandler(h.handleHealth))
	mux.HandleFunc("/api/datasets", corsHandler(h.handleGetDatasets))
	mux.HandleFunc("/api/upload", corsHandler(h.handleUpload))
	mux.HandleFunc("/api/analyze", corsHandler(h.handleAnalyze))
	mux.HandleFunc("/api/connect-source", corsHandler(h.handleConnectSource))
	mux.HandleFunc("/api/export/cleaned-csv", corsHandler(h.handleExportCsv))
	mux.HandleFunc("/api/pinned-charts", corsHandler(h.handleGetPinnedCharts))
	mux.HandleFunc("/api/pin-chart", corsHandler(h.handlePinChart))
	mux.HandleFunc("/api/unpin-chart", corsHandler(h.handleUnpinChart))
	mux.HandleFunc("/api/python-plot", corsHandler(h.handlePythonPlot))

	// Static file server for generated plots
	mux.HandleFunc("/plots/", corsHandler(h.handleServePlot))

	return mux
}

func (h *Handler) setCORSHeaders(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return
	}
	if h.allowedOrigins["*"] || h.allowedOrigins[origin] {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Vary", "Origin")
	}
}

func (h *Handler) handleServePlot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	// Security: prevent directory traversal
	name := filepath.Base(r.URL.Path)
	if name == "" || name == "." {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	plotPath := filepath.Join(h.plotsDir, name)
	info, err := os.Stat(plotPath)
	if err != nil || info.IsDir() {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeFile(w, r, plotPath)
}

func (h *Handler) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	dbStatus := "disabled"
	if h.db != nil {
		dbStatus = "connected"
	}
	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"service":     "InsightPilot API (Go)",
		"datasets":    len(h.datasets),
		"connections": len(h.connections),
		"db":          dbStatus,
	})
}

func (h *Handler) handleGetDatasets(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	list := make([]map[string]interface{}, 0, len(h.datasets))
	for _, d := range h.datasets {
		list = append(list, map[string]interface{}{
			"id":       d.ID,
			"filename": d.Filename,
			"profile":  d.Profile,
		})
	}
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"datasets": list})
}

func (h *Handler) handleConnectSource(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Source string `json:"source"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	source := body.Source
	if source == "" {
		source = "Warehouse"
	}
	filename := fmt.Sprintf("%s_sample", strings.Join(strings.Fields(strings.ToLower(source)), "_"))
	id := "sample-id-123"
	rows := []map[string]string{
		{"month": "2026-01", "segment": "Enterprise", "revenue": "124000", "customers": "42", "churn_risk": "3.2"},
		{"month": "2026-01", "segment": "Mid-market", "revenue": "86000", "customers": "118", "churn_risk": "5.1"},
		{"month": "2026-01", "segment": "SMB", "revenue": "39000", "customers": "314", "churn_risk": "8.4"},
	}
	dataset := &data.Dataset{
		ID:       id,
		Filename: filename,
		Profile: data.Profile{
			RowCount: len(rows),
			Columns: []data.Column{
				{Name: "month", Type: "date"},
				{Name: "segment", Type: "text"},
				{Name: "revenue", Type: "number"},
				{Name: "customers", Type: "number"},
				{Name: "churn_risk", Type: "number"},
			},
		},
		Rows: rows,
	}
	h.mu.Lock()
	h.datasets[id] = dataset
	h.connections[id] = data.Connection{Source: source, ConnectedAt: time.Now()}
	h.mu.Unlock()
	h.sendJSON(w, http.StatusCreated, map[string]interface{}{
		"datasetId": id,
		"filename":  filename,
		"source":    source,
		"profile":   dataset.Profile,
	})
}

func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	const maxFileSize = 10 << 20 // 10 MB
	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Could not parse multipart form (max 10MB)"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "File is required (field name 'file')"})
		return
	}
	defer file.Close()

	// Validate file size from header
	if header.Size > maxFileSize {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "File exceeds 10MB limit"})
		return
	}

	// Validate content type
	ct := header.Header.Get("Content-Type")
	allowedTypes := map[string]bool{
		"text/csv":                 true,
		"application/csv":          true,
		"application/json":         true,
		"text/plain":               true,
		"application/octet-stream": true,
	}
	if ct != "" && !allowedTypes[ct] {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Unsupported file type: " + ct + ". Only CSV and JSON are allowed."})
		return
	}

	// Sanitize filename: strip paths and allow only safe characters.
	// Only alphanumeric, hyphens, underscores, and a single dot for extension are permitted.
	// This prevents Python script injection via crafted filenames.
	sanitizeFilename := func(name string) string {
		name = strings.ReplaceAll(name, "\x00", "")
		name = strings.ReplaceAll(name, "/", "_")
		name = strings.ReplaceAll(name, `\`, "_")
		// Allow only safe characters: alnum, hyphen, underscore, dot
		safe := strings.Builder{}
		for _, r := range name {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
				safe.WriteRune(r)
			}
		}
		name = safe.String()
		if strings.HasPrefix(name, ".") && filepath.Ext(name) == name {
			name = "upload" + name
		}
		// Trim leading dots (hidden files)
		name = strings.TrimLeft(name, ".")
		// Collapse multiple dots to a single one to prevent ".csv" bypass via "....csv"
		for strings.Contains(name, "..") {
			name = strings.ReplaceAll(name, "..", ".")
		}
		if name == "" {
			name = "upload"
		}
		return name
	}

	content, err := io.ReadAll(io.LimitReader(file, maxFileSize+1))
	if err != nil {
		h.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to read file"})
		return
	}
	if int64(len(content)) > maxFileSize {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "File exceeds 10MB limit"})
		return
	}

	if err := os.MkdirAll(h.uploadDir, 0755); err != nil {
		h.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create upload directory"})
		return
	}

	id := fmt.Sprintf("%d", time.Now().UnixNano())
	safeName := sanitizeFilename(header.Filename)
	ext := strings.ToLower(filepath.Ext(safeName))
	if ext != ".csv" && ext != ".json" {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "File must have .csv or .json extension"})
		return
	}

	filePath := filepath.Join(h.uploadDir, fmt.Sprintf("%s-%s", id, safeName))
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		h.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save file"})
		return
	}

	var rows [][]string
	text := string(content)
	if ext == ".json" {
		rows, err = data.ParseJSONRows(text)
	} else {
		rows, err = data.ParseCSV(text)
	}

	if err != nil {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Parsing failed: " + err.Error()})
		return
	}

	if len(rows) < 2 {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "File must have a header and at least one data row"})
		return
	}

	profile := data.ProfileRows(rows)

	rowObjects := make([]map[string]string, 0, len(rows)-1)
	headers := rows[0]
	for _, row := range rows[1:] {
		obj := make(map[string]string)
		for i, header := range headers {
			if i < len(row) {
				obj[header] = row[i]
			}
		}
		rowObjects = append(rowObjects, obj)
	}

	dataset := &data.Dataset{
		ID:       id,
		Filename: safeName,
		FilePath: filePath,
		Profile:  profile,
		Rows:     rowObjects,
	}

	h.mu.Lock()
	h.datasets[id] = dataset
	h.mu.Unlock()

	h.sendJSON(w, http.StatusCreated, map[string]interface{}{
		"datasetId": id,
		"filename":  safeName,
		"profile":   profile,
	})
}

func (h *Handler) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DatasetId  string   `json:"datasetId"`
		DatasetIds []string `json:"datasetIds"`
		Prompt     string   `json:"prompt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	var targetIds []string
	if len(body.DatasetIds) > 0 {
		targetIds = body.DatasetIds
	} else if body.DatasetId != "" {
		targetIds = []string{body.DatasetId}
	}
	if len(targetIds) == 0 {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "No dataset IDs provided"})
		return
	}
	h.mu.RLock()
	var activeDatasets []*data.Dataset
	for _, id := range targetIds {
		if d, ok := h.datasets[id]; ok {
			activeDatasets = append(activeDatasets, d)
		}
	}
	h.mu.RUnlock()
	if len(activeDatasets) == 0 {
		h.sendJSON(w, http.StatusNotFound, map[string]string{"error": "Datasets not found"})
		return
	}

	req := agent.AnalysisRequest{
		Prompt:     body.Prompt,
		Datasets:   activeDatasets,
		TimeoutSec: 120,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
	defer cancel()

	resp, err := h.analyzer.Analyze(ctx, req)
	if err != nil {
		h.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Analysis failed: " + err.Error()})
		return
	}

	// Generate Python plot for the first dataset that has a file path
	var plotURL string
	for _, ds := range activeDatasets {
		if ds.FilePath != "" {
			plotURL = h.generatePythonPlot(ds)
			if plotURL != "" {
				break
			}
		}
	}

	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"question": resp.Question,
		"dataset":  resp.Dataset,
		"notebook": resp.Notebook,
		"plotUrl":  plotURL,
		"dashboard": map[string]interface{}{
			"title":           resp.Dashboard.Title,
			"kpis":            resp.Dashboard.KPIs,
			"trend":           resp.Dashboard.Trend,
			"segments":        resp.Dashboard.Segments,
			"recommendations": resp.Dashboard.Recommendations,
		},
		"assumptions":        resp.Assumptions,
		"warnings":           resp.Warnings,
		"used_deterministic": resp.UsedDeterministic,
	})
}

// generatePythonPlot creates and executes a Python visualization script for the given dataset.
// The CSV file path is passed as a command-line argument to the Python script, preventing
// code injection via crafted filenames.
// Returns the URL path to the generated plot image, or empty string on failure.
func (h *Handler) generatePythonPlot(ds *data.Dataset) string {
	scriptID := fmt.Sprintf("auto_%d", time.Now().UnixNano())
	scriptContent := h.pythonBridge.GeneratePlotScript(scriptID, "")
	plotURL, err := h.pythonBridge.ExecuteScript(scriptID, scriptContent, ds.FilePath)
	if err != nil {
		fmt.Printf("Python plot generation failed for dataset %s: %v\n", ds.ID, err)
		return ""
	}
	return plotURL
}

// handlePythonPlot is an on-demand endpoint to generate a Python plot for a dataset.
func (h *Handler) handlePythonPlot(w http.ResponseWriter, r *http.Request) {
	datasetID := r.URL.Query().Get("datasetId")
	if datasetID == "" {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "datasetId query parameter required"})
		return
	}

	h.mu.RLock()
	ds, ok := h.datasets[datasetID]
	h.mu.RUnlock()
	if !ok {
		h.sendJSON(w, http.StatusNotFound, map[string]string{"error": "Dataset not found"})
		return
	}

	if ds.FilePath == "" {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Dataset has no file path (in-memory only)"})
		return
	}

	plotURL := h.generatePythonPlot(ds)
	if plotURL == "" {
		h.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to generate plot"})
		return
	}

	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"plotUrl": plotURL,
	})
}

func (h *Handler) handleExportCsv(w http.ResponseWriter, r *http.Request) {
	rawIDs := r.URL.Query().Get("datasetIds")
	if rawIDs == "" {
		rawIDs = r.URL.Query().Get("datasetId")
	}
	if rawIDs == "" {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "No dataset IDs provided"})
		return
	}

	ids := strings.Split(rawIDs, ",")
	h.mu.RLock()
	selected := make([]*data.Dataset, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if d, ok := h.datasets[id]; ok {
			selected = append(selected, d)
		}
	}
	h.mu.RUnlock()

	if len(selected) == 0 {
		h.sendJSON(w, http.StatusNotFound, map[string]string{"error": "Datasets not found"})
		return
	}

	headers := exportHeaders(selected)
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="cleaned-data.csv"`)
	w.WriteHeader(http.StatusOK)

	writer := csv.NewWriter(w)
	_ = writer.Write(headers)
	for _, dataset := range selected {
		for _, row := range dataset.Rows {
			record := make([]string, len(headers))
			record[0] = dataset.Filename
			for i := 1; i < len(headers); i++ {
				record[i] = row[headers[i]]
			}
			_ = writer.Write(record)
		}
	}
	writer.Flush()
}

func (h *Handler) handleGetPinnedCharts(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var list []*PinnedChart
	for _, pc := range h.pinnedCharts {
		list = append(list, pc)
	}
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"pinnedCharts": list})
}

func (h *Handler) handlePinChart(w http.ResponseWriter, r *http.Request) {
	var pc PinnedChart
	if err := json.NewDecoder(r.Body).Decode(&pc); err != nil {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}

	if pc.ID == "" {
		pc.ID = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	h.mu.Lock()
	h.pinnedCharts[pc.ID] = &pc
	h.mu.Unlock()

	// Persist to database
	if h.db != nil {
		urlStr := pc.URL
		_, err := h.db.SavePinnedChart(pc.ID, pc.ChartType, pc.Label, pc.Data, urlStr)
		if err != nil {
			fmt.Printf("Warning: failed to save pinned chart to DB: %v\n", err)
		}
	}

	h.sendJSON(w, http.StatusCreated, pc)
}

func (h *Handler) handleUnpinChart(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "id query parameter required"})
		return
	}

	h.mu.Lock()
	delete(h.pinnedCharts, id)
	h.mu.Unlock()

	// Remove from database
	if h.db != nil {
		if err := h.db.DeletePinnedChart(id); err != nil {
			fmt.Printf("Warning: failed to delete pinned chart from DB: %v\n", err)
		}
	}

	h.sendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func exportHeaders(datasets []*data.Dataset) []string {
	seen := map[string]bool{"source_dataset": true}
	headers := []string{"source_dataset"}
	for _, dataset := range datasets {
		for _, column := range dataset.Profile.Columns {
			if !seen[column.Name] {
				seen[column.Name] = true
				headers = append(headers, column.Name)
			}
		}
	}
	if len(headers) == 1 {
		keys := make([]string, 0)
		for _, dataset := range datasets {
			for _, row := range dataset.Rows {
				for key := range row {
					if !seen[key] {
						seen[key] = true
						keys = append(keys, key)
					}
				}
			}
		}
		sort.Strings(keys)
		headers = append(headers, keys...)
	}
	return headers
}
