package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"insightpilot/internal/data"
)

type Handler struct {
	datasets    map[string]*data.Dataset
	connections map[string]data.Connection
	uploadDir   string
	mu          sync.RWMutex
}

func NewHandler() *Handler {
	return &Handler{
		datasets:    make(map[string]*data.Dataset),
		connections: make(map[string]data.Connection),
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
	h.sendJSON(w, http.StatusNotImplemented, map[string]string{"error": "Upload porting partially implemented"})
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
	kpis := data.BuildKPIs(primary.Rows, &primary.Profile.Columns[2], &primary.Profile.Columns[1])
	trend := data.BuildTrend(primary.Rows, &primary.Profile.Columns[0], &primary.Profile.Columns[2])
	segments := data.BuildSegments(primary.Rows, &primary.Profile.Columns[1], &primary.Profile.Columns[2])
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
		},
	})
}

func (h *Handler) handleExportCsv(w http.ResponseWriter, r *http.Request) {
	h.sendJSON(w, http.StatusNotImplemented, map[string]string{"error": "Export porting in progress"})
}
