package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"insightpilot/internal/agent"
	"insightpilot/internal/data"
	"insightpilot/internal/store"

	_ "github.com/lib/pq"
)

// Handler is the top-level HTTP handler. It delegates to specialized services
// for pinned charts and plot generation.
type Handler struct {
	datasets          map[string]*data.Dataset
	connections       map[string]data.Connection
	connectionConfigs map[string]*ConnectionConfig
	sessions          map[string]*agent.ConversationSession
	analyzer          agent.Analyzer
	db                *store.DB
	pinnedSvc         *PinnedChartService
	dashboardSvc      *DashboardService
	plotService       *PlotService
	shareSvc          *ShareService
	auth              *AuthService
	reportSvc         *ReportService
	duckdb            *data.DuckDBEngine
	encryptionKey     []byte
	rateLimiter       *rateLimiter
	uploadDir         string
	allowedOrigins    map[string]bool
	stopCh           chan struct{}
	mu                sync.RWMutex
	sessionMu         sync.RWMutex
	pipelines         map[string]*data.TransformPipeline
	dashEditorSvc     *DashboardEditorService
}

// NewHandler creates a new Handler with all services initialized.
func NewHandler(cfg agent.Config) *Handler {
	db := store.NewDB()
	uploadDir := resolveDataDir("UPLOAD_DIR", "uploads")
	plotsDir := filepath.Join(uploadDir, "plots")

	pb := NewPythonBridge(plotsDir)

	// Configure LLM-driven visualization if credentials are available
	llmCfg := LLMConfig{
		Enabled:       cfg.Enabled && cfg.APIKey != "" && cfg.BaseURL != "",
		APIKey:        cfg.APIKey,
		BaseURL:       cfg.BaseURL,
		Model:         cfg.Model,
		MaxTokens:     4096,
		Temperature:   0.3,
		TimeoutSec:    60,
	}
	pb.SetLLMConfig(llmCfg)

	h := &Handler{
		datasets:          make(map[string]*data.Dataset),
		connections:       make(map[string]data.Connection),
		connectionConfigs: make(map[string]*ConnectionConfig),
		sessions:          make(map[string]*agent.ConversationSession),
		analyzer:          agent.NewLLMAnalyzer(cfg),
		db:                db,
		pinnedSvc:         NewPinnedChartService(db),
		dashboardSvc:      NewDashboardService(db),
		plotService:       NewPlotService(plotsDir, uploadDir, pb),
		shareSvc:          NewShareService(),
		auth:              NewAuthService(db),
		reportSvc:         NewReportService(db),
		dashEditorSvc:     NewDashboardEditorService(db),
		duckdb:            data.NewDuckDBEngine(plotsDir),
		pipelines:         make(map[string]*data.TransformPipeline),
		encryptionKey:     generateEncryptionKey(),
		rateLimiter:       newRateLimiter(10, time.Minute),
		uploadDir:         uploadDir,
		stopCh:            make(chan struct{}),
	}

	setDuckDBEngine(h.duckdb)
	h.allowedOrigins = configuredAllowedOrigins()

	// Restore datasets from database on startup
	if h.db != nil {
		if records, err := h.db.LoadDatasets(); err == nil && len(records) > 0 {
			for _, rec := range records {
				profile := data.Profile{}
				if len(rec.Profile) > 0 {
					json.Unmarshal(rec.Profile, &profile)
				}
				h.datasets[rec.ID] = &data.Dataset{
					ID:       rec.ID,
					Filename: rec.Filename,
					FilePath: rec.FilePath,
					OwnerID:  rec.OwnerID,
					Profile:  profile,
					Rows:     rec.Rows,
				}
			}
			slog.Info("Restored datasets from database", "component", "persist", "count", len(records))
		}
	}

	h.plotService.StartCleanup()
	h.reportSvc.Start(h)
	h.startRefreshScheduler()
	h.startGuestCleanup()

	return h
}

// Routes returns the HTTP handler for all API routes.
func (h *Handler) Routes() http.Handler {
	return h.chiRouter()
}

// Shutdown gracefully stops the handler, cleaning up resources.
func (h *Handler) Shutdown() {
	slog.Info("Shutting down handler services")

	// Stop the refresh scheduler
	h.stopRefreshScheduler()

	// Stop report scheduler
	h.reportSvc.Stop()

	// Stop cleanup ticker
	h.plotService.StopCleanup()

	// Close DuckDB engine
	if h.duckdb != nil {
		h.duckdb.Close()
	}

	// Close database connection
	if h.db != nil {
		h.db.Close()
	}

	slog.Info("Handler shutdown complete")
}

func (h *Handler) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	SendJSON(w, status, data)
}

// decodeJSON reads and decodes a JSON request body, limiting its size to 1 MB.
// Returns false and sends an error response if decoding fails.
func decodeJSON(w http.ResponseWriter, r *http.Request, v interface{}) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		sendInvalidRequest(w, "Invalid request body")
		return false
	}
	return true
}

// SendJSON writes a JSON response with the given status code.
// Package-level so other files in the api package can use it.
func SendJSON(w http.ResponseWriter, status int, data interface{}) {
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
	userID := h.currentUserID(r)
	isGuest := userID != "" && isGuestUser(userID)
	h.mu.RLock()
	defer h.mu.RUnlock()
	list := make([]map[string]interface{}, 0, len(h.datasets))
	for _, d := range h.datasets {
		// Unclaimed datasets (empty OwnerID) are only visible to non-guest authenticated users
		if d.OwnerID == "" {
			if isGuest || userID == "" {
				continue
			}
		} else if d.OwnerID != userID {
			continue
		}
		list = append(list, map[string]interface{}{
			"id":             d.ID,
			"filename":       d.Filename,
			"profile":        d.Profile,
			"liveDb":         d.ConnectionConfigID != "" || d.ConnectionString != "",
			"tableName":      d.TableName,
			"connectionInfo": map[string]string{"table": d.TableName},
		})
	}
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"datasets": list})
}

func (h *Handler) handleConnectSource(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Source string `json:"source"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	source := body.Source
	if source == "" {
		source = "Warehouse"
	}
	safeName := strings.Map(func(r rune) rune {
		if r == ' ' || r == '-' {
			return '_'
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return -1
	}, strings.ToLower(source))
	if safeName == "" {
		safeName = "source"
	}
	filename := fmt.Sprintf("%s_sample", safeName)
	id := "sample-id-123"
	rows := []map[string]string{
		{"month": "2026-01", "segment": "Enterprise", "revenue": "124000", "customers": "42", "churn_risk": "3.2"},
		{"month": "2026-01", "segment": "Mid-market", "revenue": "86000", "customers": "118", "churn_risk": "5.1"},
		{"month": "2026-01", "segment": "SMB", "revenue": "39000", "customers": "314", "churn_risk": "8.4"},
	}
	dataset := &data.Dataset{
		ID:       id,
		Filename: filename,
		OwnerID:  h.currentUserID(r),
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
	h.persistDataset(dataset)
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

	if header.Size > maxFileSize {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "File exceeds 10MB limit"})
		return
	}

	ct := header.Header.Get("Content-Type")
	allowedTypes := map[string]bool{
		"text/csv":                 true,
		"application/csv":          true,
		"application/json":         true,
		"text/plain":               true,
		"application/octet-stream": true, // common fallback; extension + parser provide the real boundary
	}
	if ct != "" && !allowedTypes[ct] {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Unsupported file type: " + ct + ". Only CSV and JSON are allowed."})
		return
	}

	sanitizeFilename := func(name string) string {
		name = strings.ReplaceAll(name, "\x00", "")
		name = strings.ReplaceAll(name, "/", "_")
		name = strings.ReplaceAll(name, `\`, "_")
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
		name = strings.TrimLeft(name, ".")
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

id := newID()
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
		OwnerID:  h.currentUserID(r),
	}

	h.mu.Lock()
	h.datasets[id] = dataset
	h.mu.Unlock()
	h.persistDataset(dataset)

	h.sendJSON(w, http.StatusCreated, map[string]interface{}{
		"datasetId": id,
		"filename":  safeName,
		"profile":   profile,
	})
}

func (h *Handler) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DatasetId    string   `json:"datasetId"`
		DatasetIds   []string `json:"datasetIds"`
		Prompt       string   `json:"prompt"`
		SessionId    string   `json:"sessionId,omitempty"`
		VizType      string   `json:"vizType,omitempty"`
		AccentColor  string   `json:"accentColor,omitempty"`
		ChartScheme  string   `json:"chartScheme,omitempty"`
		FontFamily   string   `json:"fontFamily,omitempty"`
		FontSize     string   `json:"fontSize,omitempty"`
	}
	if !decodeJSON(w, r, &body) {
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

	// Merge multiple datasets into one combined dataset for analysis
	primary := activeDatasets[0]
	if len(activeDatasets) > 1 {
		primary = data.MergeDatasets(activeDatasets)
		slog.Info("Merged datasets into one",
				"component", "analyze",
				"count", len(activeDatasets),
				"rows", primary.Profile.RowCount,
				"cols", len(primary.Profile.Columns))
	}

	// --- Session / Conversation Management ---
	var sessionID string
	var history []agent.ConversationTurn
	var activeContext *agent.ConversationContext

	if body.SessionId != "" {
		h.sessionMu.RLock()
		sess, ok := h.sessions[body.SessionId]
		h.sessionMu.RUnlock()
		if ok {
			sessionID = sess.ID
			history = sess.History
			activeContext = sess.ActiveContext
			slog.Info("Continuing session", "component", "analyze", "sessionId", sessionID, "previousTurns", len(history))
		} else {
			slog.Info("Session not found, starting new", "component", "analyze", "sessionId", body.SessionId)
		}
	}

	if sessionID == "" {
		sessionID = newID()
		h.sessionMu.Lock()
		h.sessions[sessionID] = agent.NewSession(sessionID, targetIds)
		h.sessionMu.Unlock()
		slog.Info("Created new session", "component", "analyze", "sessionId", sessionID)
	}

	req := agent.AnalysisRequest{
		Prompt:     body.Prompt,
		Datasets:   []*data.Dataset{primary},
		TimeoutSec: 600,
		SessionID:  sessionID,
		History:    history,
		Context:    activeContext,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 600*time.Second)
	defer cancel()

	resp, err := h.analyzer.Analyze(ctx, req)
	if err != nil {
		h.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Analysis failed: " + err.Error()})
		return
	}

	// Use context from the response if provided (deterministic sets it), otherwise build from plan
	if resp.Context == nil && resp.Plan != nil {
		resp.Context = &agent.ConversationContext{
			MetricCol:   resp.Plan.MetricColumn,
			CategoryCol: resp.Plan.CategoryColumn,
			DateCol:     resp.Plan.DateColumn,
			Filters:     resp.Plan.Filters,
		}
	}

	if resp.Plan != nil {
		slog.Info("Executing LLM plan",
				"component", "execPlan",
				"metric", resp.Plan.MetricColumn,
				"category", resp.Plan.CategoryColumn,
				"date", resp.Plan.DateColumn,
				"agg", resp.Plan.Aggregation,
				"filters", resp.Plan.Filters)
		execPlan(resp.Plan, primary, &resp)
		slog.Info("Plan execution complete",
				"component", "execPlan",
				"kpis", len(resp.Dashboard.KPIs),
				"trendPoints", len(resp.Dashboard.Trend),
				"segments", len(resp.Dashboard.Segments))
	} else {
		slog.Info("No plan (deterministic path)",
				"component", "execPlan",
				"kpis", len(resp.Dashboard.KPIs),
				"trendPoints", len(resp.Dashboard.Trend),
				"segments", len(resp.Dashboard.Segments))
	}

	// Apply filters from plan (for LLM path) to the dataset before computing
	if resp.Plan != nil && len(resp.Plan.Filters) > 0 && resp.Context != nil {
		resp.Context.Filters = resp.Plan.Filters
	}

	// If dataset has a live DB connection, run SQL against it for real results
	if primary.TableName != "" && hasLiveConnection(primary) {
		metricCol, catCol, dateCol := resolveColumns(primary, resp.Plan)
		if metricCol != nil {
			dash, sqls, err := h.execLiveSQL(primary, metricCol, catCol, dateCol, "")
			if err == nil && dash != nil {
				resp.Dashboard = *dash
				resp.SQLQueries = sqls
			} else if err != nil {
				slog.Error("Failed to execute live SQL", "component", "livedb", "error", err)
			}
		}
	}

	// Determine visualization library
	vizType := body.VizType
	if vizType == "" {
		vizType = VizTypeMatplotlib
	}

	// Build design JSON for Python plot
	designJSON := ""
	if body.AccentColor != "" || body.ChartScheme != "" || body.FontFamily != "" || body.FontSize != "" {
		dc := DesignConfig{
			AccentColor: body.AccentColor,
			ChartScheme: body.ChartScheme,
			FontFamily:  body.FontFamily,
			FontSize:    body.FontSize,
		}
		if b, err := json.Marshal(dc); err == nil {
			designJSON = string(b)
		}
	}

	// Generate Python plot for the first dataset that has a file path
	var plotURL string
	for _, ds := range activeDatasets {
		if ds.FilePath != "" {
			plotURL = h.plotService.GeneratePlot(ds, body.Prompt, vizType, designJSON)
			if plotURL != "" {
				break
			}
		}
	}

	// Store the turn in the session
	resp.SessionID = sessionID
	h.sessionMu.Lock()
	if sess, ok := h.sessions[sessionID]; ok {
		sess.AddTurn(body.Prompt, resp)
		if resp.Context != nil {
			sess.UpdateContext(resp.Context)
		}
	}
	h.sessionMu.Unlock()

	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"question": resp.Question,
		"dataset":  resp.Dataset,
		"notebook": resp.Notebook,
		"plan":     resp.Plan,
		"plotUrl":  plotURL,
		"plotType": vizType,
		"dashboard": map[string]interface{}{
			"title":           resp.Dashboard.Title,
			"kpis":            resp.Dashboard.KPIs,
			"trend":           resp.Dashboard.Trend,
			"segments":        resp.Dashboard.Segments,
			"recommendations": resp.Dashboard.Recommendations,
			"narrative":       resp.Dashboard.Narrative,
			"chartType":       resp.Dashboard.ChartType,
			"chartTypes":      resp.Dashboard.ChartTypes,
			"explanations":    resp.Dashboard.Explanations,
		},
		"assumptions":        resp.Assumptions,
		"warnings":           resp.Warnings,
		"used_deterministic": resp.UsedDeterministic,
		"sqlQueries":         resp.SQLQueries,
		"sessionId":          sessionID,
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

	// Buffer the CSV in memory so we can return error status on failure
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)
	if err := writer.Write(headers); err != nil {
		slog.Error("Failed to write CSV headers", "component", "export", "error", err)
		sendInternalError(w, "Failed to generate CSV headers")
		return
	}

	totalRecords := 0
	failedRecords := 0
	for _, dataset := range selected {
		for _, row := range dataset.Rows {
			record := make([]string, len(headers))
			record[0] = dataset.Filename
			for i := 1; i < len(headers); i++ {
				if val, ok := row[headers[i]]; ok {
					record[i] = val
				} else {
					record[i] = ""
				}
			}
			if err := writer.Write(record); err != nil {
				slog.Error("Failed to write CSV record", "component", "export", "error", err)
				failedRecords++
				continue
			}
			totalRecords++
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		slog.Error("Failed to flush CSV", "component", "export", "error", err)
		sendInternalError(w, "Failed to finalize CSV export")
		return
	}

	if failedRecords > 0 {
		slog.Error("Export failed for some records", "component", "export", "failed", failedRecords, "total", totalRecords+failedRecords)
		sendInternalError(w, fmt.Sprintf("Export failed for %d records", failedRecords))
		return
	}

	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="cleaned-data.csv"`)
	w.WriteHeader(http.StatusOK)
	w.Write(buf.Bytes())
}

func (h *Handler) handleClearSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SessionId string `json:"sessionId"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.SessionId == "" {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "sessionId is required"})
		return
	}
	h.sessionMu.Lock()
	delete(h.sessions, body.SessionId)
	h.sessionMu.Unlock()
	slog.Info("Cleared session", "component", "session", "sessionId", body.SessionId)
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func (h *Handler) handleGetPinnedCharts(w http.ResponseWriter, r *http.Request) {
	charts := h.pinnedSvc.GetAll()
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"pinnedCharts": charts})
}

func (h *Handler) handlePinChart(w http.ResponseWriter, r *http.Request) {
	var pc PinnedChart
	if !decodeJSON(w, r, &pc) {
		return
	}

	saved, err := h.pinnedSvc.Add(pc.ChartType, pc.Label, pc.Data, pc.URL)
	if err != nil {
		h.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to pin chart: " + err.Error()})
		return
	}

	h.sendJSON(w, http.StatusCreated, saved)
}

func (h *Handler) handleUnpinChart(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "id query parameter required"})
		return
	}

	if err := h.pinnedSvc.Remove(id); err != nil {
		h.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to unpin chart: " + err.Error()})
		return
	}

	h.sendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func (h *Handler) handleRefreshDataset(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "id query parameter required"})
		return
	}

	h.mu.Lock()
	ds, ok := h.datasets[id]
	if !ok {
		h.mu.Unlock()
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Dataset not found"})
		return
	}

	tableName := ds.TableName
	configID := ds.ConnectionConfigID
	legacyConnStr := ds.ConnectionString
	h.mu.Unlock()

	if tableName == "" || (configID == "" && legacyConnStr == "") {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Dataset is not connected to a live database, cannot refresh"})
		return
	}

	connStr, ok := h.resolveConnStrByConfigID(configID, legacyConnStr)
	if !ok {
		SendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to resolve connection string"})
		return
	}

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		SendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to open database: " + err.Error()})
		return
	}
	defer db.Close()
	db.SetConnMaxLifetime(10 * time.Second)

	rows, _, err := fetchTableData(db, tableName, 1000)
	if err != nil {
		SendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to fetch data: " + err.Error()})
		return
	}

	if len(rows) == 0 {
		SendJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "Table is empty", "rowCount": 0})
		return
	}

	cols := detectColumns(rows)

	newDS := &data.Dataset{
		ID:               ds.ID,
		Filename:         ds.Filename,
		FilePath:         ds.FilePath,
		Profile:          data.Profile{RowCount: len(rows), Columns: cols},
		Rows:             rows,
		ConnectionString: connStr,
		TableName:        tableName,
	}
	if ds.FilePath != "" {
		newDS.FilePath = ds.FilePath
	}

	h.mu.Lock()
	h.datasets[id] = newDS
	h.mu.Unlock()
	h.persistDataset(newDS)

	slog.Info("Refreshed dataset", "component", "refresh", "datasetId", id, "filename", ds.Filename, "rows", len(rows))

	SendJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"rowCount": len(rows),
	})
}

func (h *Handler) handlePythonPlot(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	datasets := h.datasets
	h.mu.RUnlock()
	h.plotService.HandlePythonPlot(w, r, datasets)
}

func (h *Handler) handleDatasetProfile(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "id query parameter required"})
		return
	}

	h.mu.RLock()
	ds, ok := h.datasets[id]
	h.mu.RUnlock()
	if !ok {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Dataset not found"})
		return
	}

	profile := data.ComputeDetailedProfile(ds)
	SendJSON(w, http.StatusOK, profile)
}

// persistDataset saves a dataset to the database for recovery on restart.
// Guest datasets are never persisted (they are temporary).
func (h *Handler) persistDataset(ds *data.Dataset) {
	if h.db == nil || ds == nil {
		return
	}
	if isGuestUser(ds.OwnerID) {
		return
	}
	profileJSON, err := json.Marshal(ds.Profile)
	if err != nil {
		slog.Error("Failed to marshal profile for dataset", "component", "persist", "datasetId", ds.ID, "error", err)
		return
	}
	if err := h.db.SaveDataset(ds.ID, ds.Filename, ds.FilePath, ds.OwnerID, profileJSON); err != nil {
		slog.Error("Failed to save dataset", "component", "persist", "datasetId", ds.ID, "error", err)
		return
	}
	if len(ds.Rows) > 0 {
		if err := h.db.SaveDatasetRows(ds.ID, ds.Rows); err != nil {
			slog.Error("Failed to save rows for dataset", "component", "persist", "datasetId", ds.ID, "error", err)
		}
	}
}

// --- Helpers moved to analysis.go ---

// startRefreshScheduler starts a background goroutine that periodically
// refreshes datasets connected to live databases. Interval is configured
// via REFRESH_INTERVAL_MIN env var (default 15 minutes).
func (h *Handler) startRefreshScheduler() {
	interval := 15
	if v := os.Getenv("REFRESH_INTERVAL_MIN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			interval = n
		}
	}
	if interval <= 0 {
		return
	}
	ticker := time.NewTicker(time.Duration(interval) * time.Minute)
	go func() {
		for {
			select {
			case <-ticker.C:
				h.refreshConnectedDatasets()
			case <-h.stopCh:
				ticker.Stop()
				return
			}
		}
	}()
	slog.Info("Started background refresh scheduler", "component", "refresh", "intervalMin", interval)
}

// stopRefreshScheduler signals the refresh scheduler to stop.
func (h *Handler) stopRefreshScheduler() {
	close(h.stopCh)
}

// refreshConnectedDatasets refreshes all datasets that have a live database connection.
func (h *Handler) refreshConnectedDatasets() {
	h.mu.RLock()
	type refreshTask struct {
		id             string
		tableName      string
		filename       string
		configID       string
		legacyConnStr  string
	}
	tasks := make([]refreshTask, 0)
	for id, ds := range h.datasets {
		if ds.TableName == "" || !hasLiveConnection(ds) {
			continue
		}
		tasks = append(tasks, refreshTask{
			id:            id,
			tableName:     ds.TableName,
			filename:      ds.Filename,
			configID:      ds.ConnectionConfigID,
			legacyConnStr: ds.ConnectionString,
		})
	}
	h.mu.RUnlock()

	for _, t := range tasks {
		connStr, ok := h.resolveConnStrByConfigID(t.configID, t.legacyConnStr)
		if !ok {
			continue
		}
		db, err := sql.Open("postgres", connStr)
		if err != nil {
			slog.Error("Failed to open connection for dataset refresh", "component", "refresh", "datasetId", t.id, "error", err)
			continue
		}

		rows, _, err := fetchTableData(db, t.tableName, 1000)
		db.Close()

		if err != nil {
			slog.Error("Failed to fetch data for dataset refresh", "component", "refresh", "datasetId", t.id, "error", err)
			continue
		}

		if len(rows) == 0 {
			continue
		}

		cols := detectColumns(rows)

		h.mu.Lock()
		if existing, ok := h.datasets[t.id]; ok {
			existing.Rows = rows
			existing.Profile = data.Profile{RowCount: len(rows), Columns: cols}
		}
		h.mu.Unlock()

		slog.Info("Refreshed dataset", "component", "refresh", "datasetId", t.id, "filename", t.filename, "rows", len(rows))
	}
}

func (h *Handler) getDataset(id string) *data.Dataset {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.datasets[id]
}

// --- Package-level helpers ---

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
