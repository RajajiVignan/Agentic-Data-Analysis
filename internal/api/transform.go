package api

import (
	"net/http"
	"os"

	"insightpilot/internal/data"
)

type transformHandler struct {
	h *Handler
}

// getPipeline returns the pipeline for a dataset, creating one if needed.
func (h *Handler) getPipeline(datasetID string) *data.TransformPipeline {
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
	return p
}

func (h *Handler) handleTransformPreview(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DatasetID string                 `json:"datasetId"`
		Step      data.TransformStep     `json:"step"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.DatasetID == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "datasetId is required"})
		return
	}

	h.mu.RLock()
	ds, ok := h.datasets[body.DatasetID]
	h.mu.RUnlock()
	if !ok {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Dataset not found"})
		return
	}

	preview := data.ApplySingle(ds, body.Step)
	SendJSON(w, http.StatusOK, map[string]interface{}{
		"rowCount": len(preview.Rows),
		"columns":  preview.Profile.Columns,
		"rows":     preview.Rows[:minInt(len(preview.Rows), 50)],
	})
}

func (h *Handler) handleTransformApply(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DatasetID string                 `json:"datasetId"`
		Step      data.TransformStep     `json:"step"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.DatasetID == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "datasetId is required"})
		return
	}

	h.mu.RLock()
	ds, ok := h.datasets[body.DatasetID]
	h.mu.RUnlock()
	if !ok {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Dataset not found"})
		return
	}

	pipeline := h.getPipeline(body.DatasetID)
	pipeline.AddStep(body.Step)

	transformed := pipeline.ApplyAll(ds)

	h.mu.Lock()
	h.datasets[body.DatasetID] = transformed
	h.mu.Unlock()

	SendJSON(w, http.StatusOK, map[string]interface{}{
		"ok":           true,
		"rowCount":     len(transformed.Rows),
		"columns":      transformed.Profile.Columns,
		"steps":        pipeline.Steps,
		"canUndo":      pipeline.CanUndo(),
		"canRedo":      pipeline.CanRedo(),
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

	h.mu.RLock()
	ds, ok := h.datasets[body.DatasetID]
	h.mu.RUnlock()
	if !ok {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Dataset not found"})
		return
	}

	pipeline := h.getPipeline(body.DatasetID)
	if !pipeline.Undo() {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Nothing to undo"})
		return
	}

	transformed := pipeline.ApplyAll(ds)

	h.mu.Lock()
	h.datasets[body.DatasetID] = transformed
	h.mu.Unlock()

	SendJSON(w, http.StatusOK, map[string]interface{}{
		"ok":           true,
		"rowCount":     len(transformed.Rows),
		"columns":      transformed.Profile.Columns,
		"steps":        pipeline.Steps,
		"canUndo":      pipeline.CanUndo(),
		"canRedo":      pipeline.CanRedo(),
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

	h.mu.RLock()
	ds, ok := h.datasets[body.DatasetID]
	h.mu.RUnlock()
	if !ok {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Dataset not found"})
		return
	}

	pipeline := h.getPipeline(body.DatasetID)
	if !pipeline.Redo() {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Nothing to redo"})
		return
	}

	transformed := pipeline.ApplyAll(ds)

	h.mu.Lock()
	h.datasets[body.DatasetID] = transformed
	h.mu.Unlock()

	SendJSON(w, http.StatusOK, map[string]interface{}{
		"ok":           true,
		"rowCount":     len(transformed.Rows),
		"columns":      transformed.Profile.Columns,
		"steps":        pipeline.Steps,
		"canUndo":      pipeline.CanUndo(),
		"canRedo":      pipeline.CanRedo(),
	})
}

func (h *Handler) handleTransformHistory(w http.ResponseWriter, r *http.Request) {
	datasetID := r.URL.Query().Get("datasetId")
	if datasetID == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "datasetId query parameter required"})
		return
	}

	h.mu.RLock()
	ds, ok := h.datasets[datasetID]
	h.mu.RUnlock()
	if !ok {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Dataset not found"})
		return
	}

	pipeline := h.getPipeline(datasetID)
	SendJSON(w, http.StatusOK, map[string]interface{}{
		"steps":   pipeline.Steps,
		"undone":  pipeline.Undone,
		"canUndo": pipeline.CanUndo(),
		"canRedo": pipeline.CanRedo(),
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

	h.mu.Lock()
	delete(h.pipelines, body.DatasetID)

	// Reload original dataset from store
	ds, ok := h.datasets[body.DatasetID]
	h.mu.Unlock()

	if ok && ds.FilePath != "" {
		// Re-parse the original file to get back to initial state
		raw, err := readFileRaw(ds.FilePath)
		if err == nil {
			var rows [][]string
			if isJSONFile(ds.FilePath) {
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

func readFileRaw(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func isJSONFile(path string) bool {
	return len(path) > 5 && path[len(path)-5:] == ".json"
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

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
