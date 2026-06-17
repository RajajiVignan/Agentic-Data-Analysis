package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"insightpilot/internal/data"
)

func (h *Handler) handleJoin(w http.ResponseWriter, r *http.Request) {
	var body struct {
		LeftDatasetID  string `json:"leftDatasetId"`
		RightDatasetID string `json:"rightDatasetId"`
		LeftKey        string `json:"leftKey"`
		RightKey       string `json:"rightKey"`
		JoinType       string `json:"joinType"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if body.LeftDatasetID == "" || body.RightDatasetID == "" || body.LeftKey == "" || body.RightKey == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "leftDatasetId, rightDatasetId, leftKey, rightKey are required"})
		return
	}
	joinType := body.JoinType
	if joinType == "" {
		joinType = "inner"
	}
	if joinType != "inner" && joinType != "left" && joinType != "right" && joinType != "outer" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "joinType must be one of: inner, left, right, outer"})
		return
	}

	h.mu.RLock()
	leftDS, leftOk := h.datasets[body.LeftDatasetID]
	rightDS, rightOk := h.datasets[body.RightDatasetID]
	h.mu.RUnlock()

	if !leftOk {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Left dataset not found"})
		return
	}
	if !rightOk {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Right dataset not found"})
		return
	}

	if !columnExists(leftDS, body.LeftKey) {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Left key column %q not found in dataset %q. Available columns: %v", body.LeftKey, leftDS.Filename, columnNames(leftDS))})
		return
	}
	if !columnExists(rightDS, body.RightKey) {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("Right key column %q not found in dataset %q. Available columns: %v", body.RightKey, rightDS.Filename, columnNames(rightDS))})
		return
	}

	joined := data.JoinDatasets(leftDS, rightDS, body.LeftKey, body.RightKey, joinType)
	if joined == nil {
		SendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Join failed"})
		return
	}

	joined.OwnerID = h.currentUserID(r)

	h.mu.Lock()
	h.datasets[joined.ID] = joined
	h.mu.Unlock()

	h.persistDataset(joined)

	SendJSON(w, http.StatusCreated, map[string]interface{}{
		"datasetId": joined.ID,
		"filename":  joined.Filename,
		"profile":   joined.Profile,
		"rowCount":  len(joined.Rows),
	})
}

func columnExists(ds *data.Dataset, colName string) bool {
	for _, c := range ds.Profile.Columns {
		if c.Name == colName {
			return true
		}
	}
	return false
}

func columnNames(ds *data.Dataset) []string {
	names := make([]string, len(ds.Profile.Columns))
	for i, c := range ds.Profile.Columns {
		names[i] = c.Name
	}
	return names
}
