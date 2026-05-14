package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"insightpilot/internal/data"
)

type Handler struct {
	datasets    map[string]*data.Dataset
	connections map[string]data.Connection
	pinnedCharts map[string]*PinnedChart
	uploadDir   string
	mu          sync.RWMutex
}

type PinnedChart struct {
	ID        string `json:"id"`
	ChartType string `json:"chart_type"`
	Label     string `json:"label"`
	Data      any    `json:"data"`
	URL       string `json:"url"`
}

func NewHandler() *Handler {
	return &Handler{
		datasets:    make(map[string]*data.Dataset),
		connections: make(map[string]data.Connection),
		pinnedCharts: make(map[string]*PinnedChart),
		uploadDir:   "uploads",
	}
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	corsHandler := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
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

	return mux
}

func (h *Handler) sendJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"ok":          true,
		"service":     "InsightPilot API (Go)",
		"datasets":    len(h.datasets),
		"connections": len(h.connections),
	})
}

func (h *Handler) handleGetDatasets(w http.ResponseWriter, r *http.Request) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	var list []map[string]interface{}
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
	filename := fmt.Sprintf("%s_sample", strings.ToLower(source))
	id := "sample-id-123" 
	dataset := &data.Dataset{
		ID:       id,
		Filename: filename,
		Profile: data.Profile{
			RowCount: 0,
			Columns: []data.Column{
				{Name: "month", Type: "date"},
				{Name: "segment", Type: "text"},
				{Name: "revenue", Type: "number"},
				{Name: "customers", Type: "number"},
				{Name: "churn_risk", Type: "number"},
			},
		},
	}
	h.mu.Lock()
	h.datasets[id] = dataset
	h.connections[id] = data.Connection{Source: source}
	h.mu.Unlock()
	h.sendJSON(w, http.StatusCreated, map[string]interface{}{
		"datasetId": id,
		"filename":  filename,
		"source":    source,
		"profile":   dataset.Profile,
	})
}

func (h *Handler) handleUpload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Could not parse multipart form"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "File is required (field name 'file')"})
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)
	if err != nil {
		h.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to read file"})
		return
	}

	if err := os.MkdirAll(h.uploadDir, 0755); err != nil {
		h.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create upload directory"})
		return
	}

	id := fmt.Sprintf("%d", os.Getpid()) 
	safeName := filepath.Base(header.Filename)
	filePath := filepath.Join(h.uploadDir, fmt.Sprintf("%s-%s", id, safeName))
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		h.sendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save file"})
		return
	}

	var rows [][]string
	text := string(content)
	if strings.HasSuffix(strings.ToLower(header.Filename), ".json") {
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
		Filename: header.Filename,
		FilePath: filePath,
		Profile:  profile,
		Rows:     rowObjects,
	}

	h.mu.Lock()
	h.datasets[id] = dataset
	h.mu.Unlock()

	h.sendJSON(w, http.StatusCreated, map[string]interface{}{
		"datasetId": id,
		"filename":  header.Filename,
		"profile":   profile,
	})
}

func (h *Handler) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DatasetId string   `json:"datasetId"`
		DatasetIds []string `json:"datasetIds"`
		Prompt    string   `json:"prompt"`
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
	primary := activeDatasets[0]
	narrative := "The agent analyzed the data. Deterministic logic indicates a strong trend in the primary metric."
	
	var metricCol, catCol *data.Column
	if len(primary.Profile.Columns) > 0 {
		metricCol = &primary.Profile.Columns[0] 
		for i := range primary.Profile.Columns {
			if primary.Profile.Columns[i].Type == "number" {
				metricCol = &primary.Profile.Columns[i]
				break
			}
		}
		for i := range primary.Profile.Columns {
			if primary.Profile.Columns[i].Type == "text" {
				catCol = &primary.Profile.Columns[i]
				break
			}
		}
	}

	kpis := data.BuildKPIs(primary.Rows, metricCol, catCol)
	trend := data.BuildTrend(primary.Rows, &primary.Profile.Columns[0], metricCol)
	segments := data.BuildSegments(primary.Rows, catCol, metricCol)

	h.sendJSON(w, http.StatusOK, map[string]interface{}{
		"question": body.Prompt,
		"dataset": map[string]interface{}{
			"id": primary.ID,
			"filename": primary.Filename,
			"rowCount": primary.Profile.RowCount,
		},
		"notebook": []map[string]string{
			{"title": "Analysis", "body": narrative},
		},
		"dashboard": map[string]interface{}{
			"title": "Insights Board",
			"kpis": kpis,
			"trend": trend,
			"segments": segments,
			"recommendations": []string{
				fmt.Sprintf("Review the top %s groups contributing to %s", 
					func() string { if catCol != nil { return catCol.Name }; return "available" }(), 
					func() string { if metricCol != nil { return metricCol.Name }; return "records" }()),
				"Add business definitions for metrics to ensure consistency.",
				"Publish this board after validating with the data owner.",
			},
		},
	})
}

func (h *Handler) handleExportCsv(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusNotImplemented, map[string]string{"error": "Export porting in progress"})
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
		pc.ID = fmt.Sprintf("%d", os.Getpid()) // Simple ID gen for in-memory
	}
	
	h.mu.Lock()
	h.pinnedCharts[pc.ID] = &pc
	h.mu.Unlock()
	
	h.sendJSON(w, http.StatusCreated, pc)
}
