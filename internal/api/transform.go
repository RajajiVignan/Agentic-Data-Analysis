package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"insightpilot/internal/data"
)

// getPipeline returns the pipeline for a dataset, creating one if needed and
// recording the untouched dataset as the replay base.
func (h *Handler) getPipeline(datasetID string, ds *data.Dataset) *data.TransformPipeline {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.pipelines == nil {
		h.pipelines = make(map[string]*data.TransformPipeline)
	}
	p, ok := h.pipelines[datasetID]
	if !ok {
		p = data.NewTransformPipeline()
		h.pipelines[datasetID] = p
	}
	p.SetBase(ds)
	return p
}

// persistPipeline stores the steps so transforms survive a restart.
func (h *Handler) persistPipeline(datasetID string, ds *data.Dataset, p *data.TransformPipeline) {
	if h.db == nil || ds == nil || isGuestUser(ds.OwnerID) || p == nil {
		return
	}
	steps, err := json.Marshal(p.Steps)
	if err != nil {
		slog.Warn("Failed to marshal transform steps", "component", "transform", "datasetId", datasetID, "error", err)
		return
	}
	undone, err := json.Marshal(p.Undone)
	if err != nil {
		slog.Warn("Failed to marshal undone transform steps", "component", "transform", "datasetId", datasetID, "error", err)
		return
	}
	if err := h.db.SaveTransformPipeline(datasetID, ds.OwnerID, steps, undone); err != nil {
		slog.Warn("Failed to persist transform pipeline", "component", "transform", "datasetId", datasetID, "error", err)
	}
}

// restoreTransformPipelines replays persisted ETL steps after datasets load.
func (h *Handler) restoreTransformPipelines() {
	if h.db == nil {
		return
	}
	records, err := h.db.LoadTransformPipelines()
	if err != nil {
		slog.Warn("Failed to load transform pipelines", "component", "transform", "error", err)
		return
	}
	for _, rec := range records {
		pipeline := data.NewTransformPipeline()
		if len(rec.Steps) > 0 {
			if err := json.Unmarshal(rec.Steps, &pipeline.Steps); err != nil {
				slog.Warn("Failed to decode transform steps", "component", "transform", "datasetId", rec.DatasetID, "error", err)
				continue
			}
		}
		if len(rec.Undone) > 0 {
			_ = json.Unmarshal(rec.Undone, &pipeline.Undone)
		}

		h.mu.RLock()
		ds, ok := h.datasets[rec.DatasetID]
		h.mu.RUnlock()
		if !ok {
			continue
		}
		if rec.OwnerID != "" && ds.OwnerID != "" && rec.OwnerID != ds.OwnerID {
			continue
		}

		pipeline.SetBase(ds)
		transformed := pipeline.ApplyAll(ds)
		transformed.OwnerID = ds.OwnerID

		h.mu.Lock()
		h.datasets[rec.DatasetID] = transformed
		h.pipelines[rec.DatasetID] = pipeline
		h.mu.Unlock()
	}
	if len(records) > 0 {
		slog.Info("Restored transform pipelines", "component", "transform", "count", len(records))
	}
}

func (h *Handler) handleTransformPreview(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DatasetID string             `json:"datasetId"`
		Step      data.TransformStep `json:"step"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.DatasetID == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "datasetId is required"})
		return
	}

	ds, ok := h.accessibleDataset(r, body.DatasetID)
	if !ok {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Dataset not found"})
		return
	}

	preview := data.ApplySingle(ds, body.Step)
	SendJSON(w, http.StatusOK, map[string]interface{}{
		"rowCount": len(preview.Rows),
		"columns":  preview.Profile.Columns,
		"rows":     preview.Rows[:min(len(preview.Rows), 50)],
	})
}

func (h *Handler) handleTransformApply(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DatasetID string             `json:"datasetId"`
		Step      data.TransformStep `json:"step"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.DatasetID == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "datasetId is required"})
		return
	}

	ds, ok := h.accessibleDataset(r, body.DatasetID)
	if !ok {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Dataset not found"})
		return
	}

	pipeline := h.getPipeline(body.DatasetID, ds)
	pipeline.AddStep(body.Step)

	transformed := pipeline.ApplyAll(ds)
	transformed.OwnerID = ds.OwnerID
	h.persistPipeline(body.DatasetID, ds, pipeline)

	h.mu.Lock()
	h.datasets[body.DatasetID] = transformed
	h.mu.Unlock()

	SendJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"rowCount": len(transformed.Rows),
		"columns":  transformed.Profile.Columns,
		"steps":    pipeline.Steps,
		"canUndo":  pipeline.CanUndo(),
		"canRedo":  pipeline.CanRedo(),
	})
}

func (h *Handler) handleTransformUndo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DatasetID string `json:"datasetId"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.DatasetID == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "datasetId is required"})
		return
	}

	ds, ok := h.accessibleDataset(r, body.DatasetID)
	if !ok {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Dataset not found"})
		return
	}

	pipeline := h.getPipeline(body.DatasetID, ds)
	if !pipeline.Undo() {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Nothing to undo"})
		return
	}

	transformed := pipeline.ApplyAll(ds)
	transformed.OwnerID = ds.OwnerID
	h.persistPipeline(body.DatasetID, ds, pipeline)

	h.mu.Lock()
	h.datasets[body.DatasetID] = transformed
	h.mu.Unlock()

	SendJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"rowCount": len(transformed.Rows),
		"columns":  transformed.Profile.Columns,
		"steps":    pipeline.Steps,
		"canUndo":  pipeline.CanUndo(),
		"canRedo":  pipeline.CanRedo(),
	})
}

func (h *Handler) handleTransformRedo(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DatasetID string `json:"datasetId"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.DatasetID == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "datasetId is required"})
		return
	}

	ds, ok := h.accessibleDataset(r, body.DatasetID)
	if !ok {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Dataset not found"})
		return
	}

	pipeline := h.getPipeline(body.DatasetID, ds)
	if !pipeline.Redo() {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Nothing to redo"})
		return
	}

	transformed := pipeline.ApplyAll(ds)
	transformed.OwnerID = ds.OwnerID
	h.persistPipeline(body.DatasetID, ds, pipeline)

	h.mu.Lock()
	h.datasets[body.DatasetID] = transformed
	h.mu.Unlock()

	SendJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"rowCount": len(transformed.Rows),
		"columns":  transformed.Profile.Columns,
		"steps":    pipeline.Steps,
		"canUndo":  pipeline.CanUndo(),
		"canRedo":  pipeline.CanRedo(),
	})
}

func (h *Handler) handleTransformHistory(w http.ResponseWriter, r *http.Request) {
	datasetID := r.URL.Query().Get("datasetId")
	if datasetID == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "datasetId query parameter required"})
		return
	}

	ds, ok := h.accessibleDataset(r, datasetID)
	if !ok {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Dataset not found"})
		return
	}

	pipeline := h.getPipeline(datasetID, ds)
	SendJSON(w, http.StatusOK, map[string]interface{}{
		"steps":    pipeline.Steps,
		"undone":   pipeline.Undone,
		"canUndo":  pipeline.CanUndo(),
		"canRedo":  pipeline.CanRedo(),
		"rowCount": len(ds.Rows),
		"columns":  ds.Profile.Columns,
	})
}

func (h *Handler) handleTransformReset(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DatasetID string `json:"datasetId"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.DatasetID == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "datasetId is required"})
		return
	}

	ds, ok := h.accessibleDataset(r, body.DatasetID)
	if !ok {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Dataset not found"})
		return
	}
	h.mu.Lock()
	pipeline := h.pipelines[body.DatasetID]
	delete(h.pipelines, body.DatasetID)
	h.mu.Unlock()

	if h.db != nil && !isGuestUser(ds.OwnerID) {
		if err := h.db.DeleteTransformPipeline(body.DatasetID, ds.OwnerID); err != nil {
			slog.Warn("Failed to delete transform pipeline", "component", "transform", "datasetId", body.DatasetID, "error", err)
		}
	}

	if base := pipeline.BaseDataset(); base != nil {
		base.OwnerID = ds.OwnerID
		h.mu.Lock()
		h.datasets[body.DatasetID] = base
		ds = base
		h.mu.Unlock()
	} else if ds.FilePath != "" {
		// Fallback: re-parse the original file to get back to initial state.
		raw, err := os.ReadFile(ds.FilePath)
		if err == nil {
			var rows [][]string
			if strings.HasSuffix(ds.FilePath, ".json") {
				rows, _ = data.ParseJSONRows(string(raw))
			} else {
				rows, _ = data.ParseCSV(string(raw))
			}
			if len(rows) >= 2 {
				profile := data.ProfileRows(rows)
				rowObjs := rowsToObjects(rows)
				h.mu.Lock()
				h.datasets[body.DatasetID] = &data.Dataset{
					ID:       ds.ID,
					Filename: ds.Filename,
					FilePath: ds.FilePath,
					Profile:  profile,
					Rows:     rowObjs,
					OwnerID:  ds.OwnerID,
				}
				ds = h.datasets[body.DatasetID]
				h.mu.Unlock()
			}
		}
	}

	SendJSON(w, http.StatusOK, map[string]interface{}{
		"ok":       true,
		"rowCount": len(ds.Rows),
		"columns":  ds.Profile.Columns,
	})
}

func rowsToObjects(rows [][]string) []map[string]string {
	if len(rows) < 2 {
		return nil
	}
	headers := rows[0]
	out := make([]map[string]string, len(rows)-1)
	for i, row := range rows[1:] {
		obj := make(map[string]string)
		for j, h := range headers {
			if j < len(row) {
				obj[h] = row[j]
			}
		}
		out[i] = obj
	}
	return out
}
