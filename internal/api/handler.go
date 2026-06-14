package api

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
	analyzer          agent.Analyzer
	db                *store.DB
	pinnedSvc         *PinnedChartService
	dashboardSvc      *DashboardService
	plotService       *PlotService
	shareSvc          *ShareService
	auth              *AuthService
	duckdb            *data.DuckDBEngine
	uploadDir         string
	allowedOrigins    map[string]bool
	mu                sync.RWMutex
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
		analyzer:          agent.NewLLMAnalyzer(cfg),
		db:                db,
		pinnedSvc:         NewPinnedChartService(db),
		dashboardSvc:      NewDashboardService(db),
		plotService:       NewPlotService(plotsDir, uploadDir, pb),
		shareSvc:          NewShareService(),
		auth:              NewAuthService(),
		duckdb:            data.NewDuckDBEngine(plotsDir),
		uploadDir:         uploadDir,
	}

	setDuckDBEngine(h.duckdb)
	h.allowedOrigins = configuredAllowedOrigins()

	h.plotService.StartCleanup()

	return h
}

// Routes returns the HTTP handler for all API routes.
func (h *Handler) Routes() http.Handler {
	return h.chiRouter()
}

func (h *Handler) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	SendJSON(w, status, data)
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
	h.mu.RLock()
	defer h.mu.RUnlock()
	list := make([]map[string]interface{}, 0, len(h.datasets))
	for _, d := range h.datasets {
		if d.OwnerID != "" && d.OwnerID != userID {
			continue
		}
		list = append(list, map[string]interface{}{
			"id":             d.ID,
			"filename":       d.Filename,
			"profile":        d.Profile,
			"liveDb":         d.ConnectionString != "",
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
		"application/octet-stream": true,
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
		TimeoutSec: 600,
	}

	ctx, cancel := context.WithTimeout(r.Context(), 600*time.Second)
	defer cancel()

	resp, err := h.analyzer.Analyze(ctx, req)
	if err != nil {
		h.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Analysis failed: " + err.Error()})
		return
	}

	if resp.Plan != nil {
		log.Printf("[execPlan] Executing LLM plan: metric=%q category=%q date=%q agg=%q",
			resp.Plan.MetricColumn, resp.Plan.CategoryColumn, resp.Plan.DateColumn, resp.Plan.Aggregation)
		execPlan(resp.Plan, activeDatasets[0], &resp)
		log.Printf("[execPlan] Done. Dashboard: %d KPIs, %d trend points, %d segments",
			len(resp.Dashboard.KPIs), len(resp.Dashboard.Trend), len(resp.Dashboard.Segments))
	} else {
		log.Printf("[execPlan] No plan (deterministic path). Dashboard: %d KPIs, %d trend points, %d segments",
			len(resp.Dashboard.KPIs), len(resp.Dashboard.Trend), len(resp.Dashboard.Segments))
	}

	// Generate Python plot for the first dataset that has a file path
	var plotURL string
	for _, ds := range activeDatasets {
		if ds.FilePath != "" {
			plotURL = h.plotService.GeneratePlot(ds, body.Prompt)
			if plotURL != "" {
				break
			}
		}
	}

	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"question": resp.Question,
		"dataset":  resp.Dataset,
		"notebook": resp.Notebook,
		"plan":     resp.Plan,
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
		"sqlQueries":         resp.SQLQueries,
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
	if err := writer.Write(headers); err != nil {
		log.Printf("[export] Failed to write headers: %v", err)
		return
	}
	for _, dataset := range selected {
		for _, row := range dataset.Rows {
			record := make([]string, len(headers))
			record[0] = dataset.Filename
			for i := 1; i < len(headers); i++ {
				// Safe access with empty string fallback if column missing
				if val, ok := row[headers[i]]; ok {
					record[i] = val
				} else {
					record[i] = ""
				}
			}
			if err := writer.Write(record); err != nil {
				log.Printf("[export] Failed to write record: %v", err)
				continue
			}
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		log.Printf("[export] Failed to flush: %v", err)
	}
}

func (h *Handler) handleGetPinnedCharts(w http.ResponseWriter, r *http.Request) {
	charts := h.pinnedSvc.GetAll()
	h.sendJSON(w, http.StatusOK, map[string]interface{}{"pinnedCharts": charts})
}

func (h *Handler) handlePinChart(w http.ResponseWriter, r *http.Request) {
	var pc PinnedChart
	if err := json.NewDecoder(r.Body).Decode(&pc); err != nil {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
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

	if ds.ConnectionString == "" || ds.TableName == "" {
		h.mu.Unlock()
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Dataset is not connected to a live database, cannot refresh"})
		return
	}

	connStr := ds.ConnectionString
	tableName := ds.TableName
	h.mu.Unlock()

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

	log.Printf("[refresh] Refreshed dataset %s (%s) — %d rows", id, ds.Filename, len(rows))

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

// --- Helpers (kept in handler.go since they are analysis-specific) ---

func execPlan(plan *agent.LLMPlan, ds *data.Dataset, resp *agent.AnalysisResponse) {
	var metricCol, catCol, dateCol *data.Column
	for i := range ds.Profile.Columns {
		c := &ds.Profile.Columns[i]
		if c.Name == plan.MetricColumn {
			metricCol = c
		}
		if c.Name == plan.CategoryColumn {
			catCol = c
		}
		if c.Name == plan.DateColumn {
			dateCol = c
		}
	}

	log.Printf("[execPlan] Resolved columns: metric=%v category=%v date=%v (from %d total columns)",
		colRes(metricCol), colRes(catCol), colRes(dateCol), len(ds.Profile.Columns))

	if metricCol == nil {
		for i := range ds.Profile.Columns {
			if ds.Profile.Columns[i].Type == "number" {
				metricCol = &ds.Profile.Columns[i]
				break
			}
		}
	}

	if catCol == nil {
		used := map[string]bool{}
		if metricCol != nil {
			used[metricCol.Name] = true
		}
		if dateCol != nil {
			used[dateCol.Name] = true
		}
		for i := range ds.Profile.Columns {
			c := &ds.Profile.Columns[i]
			if c.Type == "text" && !used[c.Name] {
				catCol = c
				break
			}
		}
	}

	if metricCol == nil {
		title := plan.Title
		if title == "" {
			title = "Insights Board"
		}
		resp.Dashboard = agent.DashboardSpec{
			Title:           title,
			KPIs:            []map[string]string{},
			Trend:           []map[string]interface{}{},
			Segments:        []map[string]interface{}{},
			Recommendations: plan.Recommendations,
		}
		if len(plan.Assumptions) > 0 {
			resp.Assumptions = append(plan.Assumptions, resp.Assumptions...)
		}
		return
	}

	// Check if we can use DuckDB (file-backed dataset with engine available)
	duckDB := getDuckDBEngine()
	if ds.FilePath != "" && duckDB != nil {
		kpis, kpiSQL := duckDBKPI(duckDB, ds, metricCol)
		trend, trendSQL := duckDBTrend(duckDB, ds, dateCol, metricCol)
		segments, segSQL := duckDBSegments(duckDB, ds, catCol, metricCol)
		title := plan.Title
		if title == "" {
			title = "Insights Board"
		}
		resp.Dashboard = agent.DashboardSpec{
			Title:           title,
			KPIs:            kpis,
			Trend:           trend,
			Segments:        segments,
			Recommendations: plan.Recommendations,
		}
		sqls := make([]string, 0, 3)
		if kpiSQL != "" {
			sqls = append(sqls, kpiSQL)
		}
		if trendSQL != "" {
			sqls = append(sqls, trendSQL)
		}
		if segSQL != "" {
			sqls = append(sqls, segSQL)
		}
		resp.SQLQueries = append(resp.SQLQueries, sqls...)
		if len(plan.Assumptions) > 0 {
			resp.Assumptions = append(plan.Assumptions, resp.Assumptions...)
		}
		return
	}

	kpis := data.BuildKPIs(ds.Rows, metricCol, catCol)
	trend := data.BuildTrend(ds.Rows, dateCol, metricCol)
	segments := data.BuildSegments(ds.Rows, catCol, metricCol)

	if plan.Aggregation != "" && plan.Aggregation != "sum" {
		kpis = applyAggregation(kpis, plan.Aggregation)
	}

	title := plan.Title
	if title == "" {
		title = "Insights Board"
	}

	recs := plan.Recommendations

	resp.Dashboard = agent.DashboardSpec{
		Title:           title,
		KPIs:            kpis,
		Trend:           trend,
		Segments:        segments,
		Recommendations: recs,
	}

	if len(plan.Assumptions) > 0 {
		resp.Assumptions = append(plan.Assumptions, resp.Assumptions...)
	}
}

var duckDBEngine *data.DuckDBEngine

func setDuckDBEngine(e *data.DuckDBEngine) {
	duckDBEngine = e
}

func getDuckDBEngine() *data.DuckDBEngine {
	return duckDBEngine
}

func duckDBKPI(duckDB *data.DuckDBEngine, ds *data.Dataset, metricCol *data.Column) ([]map[string]string, string) {
	if metricCol == nil {
		return []map[string]string{}, ""
	}
	sql := fmt.Sprintf(`SELECT 
		SUM(CAST("%s" AS DOUBLE)) as total,
		AVG(CAST("%s" AS DOUBLE)) as avg_val,
		MIN(CAST("%s" AS DOUBLE)) as min_val,
		MAX(CAST("%s" AS DOUBLE)) as max_val,
		COUNT(*) as row_count
	FROM data WHERE CAST("%s" AS DOUBLE) IS NOT NULL`,
		metricCol.Name, metricCol.Name, metricCol.Name, metricCol.Name, metricCol.Name)
	results, err := duckDB.QueryKPIs(ds.FilePath, metricCol.Name)
	if err != nil {
		log.Printf("[duckdb] KPI query failed: %v, falling back to in-memory", err)
		return data.BuildKPIs(ds.Rows, metricCol, nil), sql
	}
	kpis := make([]map[string]string, 0)
	for _, row := range results {
		if total, ok := row["total"]; ok && total != "" {
			kpis = append(kpis, map[string]string{"label": "Total", "value": total, "change": "Sum"})
		}
		if avg, ok := row["avg_val"]; ok && avg != "" {
			kpis = append(kpis, map[string]string{"label": "Average", "value": avg, "change": "Per row"})
		}
		if cnt, ok := row["row_count"]; ok && cnt != "" {
			kpis = append(kpis, map[string]string{"label": "Rows", "value": cnt, "change": "Count"})
		}
		break
	}
	return kpis, sql
}

func duckDBTrend(duckDB *data.DuckDBEngine, ds *data.Dataset, dateCol, metricCol *data.Column) ([]map[string]interface{}, string) {
	if metricCol == nil || dateCol == nil {
		return []map[string]interface{}{}, ""
	}
	sql := fmt.Sprintf(`SELECT 
		CAST("%s" AS VARCHAR) as label,
		SUM(CAST("%s" AS DOUBLE)) as value
	FROM data 
	WHERE CAST("%s" AS DOUBLE) IS NOT NULL AND "%s" IS NOT NULL
	GROUP BY "%s"
	ORDER BY label
	LIMIT 20`,
		dateCol.Name, metricCol.Name, metricCol.Name, dateCol.Name, dateCol.Name)
	results, err := duckDB.QueryTrend(ds.FilePath, dateCol.Name, metricCol.Name)
	if err != nil {
		log.Printf("[duckdb] Trend query failed: %v, falling back to in-memory", err)
		return data.BuildTrend(ds.Rows, dateCol, metricCol), sql
	}
	out := make([]map[string]interface{}, len(results))
	for i, r := range results {
		out[i] = make(map[string]interface{})
		for k, v := range r {
			out[i][k] = v
		}
	}
	return out, sql
}

func duckDBSegments(duckDB *data.DuckDBEngine, ds *data.Dataset, catCol, metricCol *data.Column) ([]map[string]interface{}, string) {
	if metricCol == nil || catCol == nil {
		return []map[string]interface{}{}, ""
	}
	sql := fmt.Sprintf(`SELECT 
		CAST("%s" AS VARCHAR) as label,
		SUM(CAST("%s" AS DOUBLE)) as value
	FROM data 
	WHERE CAST("%s" AS DOUBLE) IS NOT NULL AND "%s" IS NOT NULL AND CAST("%s" AS VARCHAR) != ''
	GROUP BY "%s"
	ORDER BY value DESC
	LIMIT 10`,
		catCol.Name, metricCol.Name, metricCol.Name, catCol.Name, catCol.Name, catCol.Name)
	results, err := duckDB.QuerySegments(ds.FilePath, catCol.Name, metricCol.Name)
	if err != nil {
		log.Printf("[duckdb] Segment query failed: %v, falling back to in-memory", err)
		return data.BuildSegments(ds.Rows, catCol, metricCol), sql
	}
	out := make([]map[string]interface{}, len(results))
	for i, r := range results {
		out[i] = make(map[string]interface{})
		for k, v := range r {
			out[i][k] = v
		}
	}
	return out, sql
}

func applyAggregation(kpis []map[string]string, agg string) []map[string]string {
	if len(kpis) == 0 {
		return kpis
	}
	switch agg {
	case "avg":
		for i := range kpis {
			if kpis[i]["label"] == "Total" || (len(kpis[i]["label"]) > 6 && kpis[i]["label"][:6] == "Total ") {
				kpis[i]["label"] = "Average"
				kpis[i]["change"] = "Per row (avg)"
			}
		}
	case "count":
		for i := range kpis {
			kpis[i]["label"] = "Count"
			kpis[i]["change"] = "Total rows"
		}
	case "min", "max":
		for i := range kpis {
			kpis[i]["label"] = strings.ToUpper(agg) + " value"
			kpis[i]["change"] = "Dataset range"
		}
	}
	return kpis
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

func colRes(c *data.Column) string {
	if c == nil {
		return "<nil>"
	}
	return c.Name
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
