package api

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"insightpilot/internal/store"
)

// queryHistoryEntry is one recorded execution in the SQL Lab history.
type queryHistoryEntry struct {
	ID         string   `json:"id"`
	OwnerID    string   `json:"-"`
	DatasetIDs []string `json:"datasetIds"`
	SQL        string   `json:"sql"`
	ExecutedAt string   `json:"executedAt"`
	DurationMs int64    `json:"durationMs"`
	RowCount   int      `json:"rowCount"`
	Error      string   `json:"error,omitempty"`
}

// queryHistoryStore keeps a bounded, in-memory ring of recent query executions.
type queryHistoryStore struct {
	mu      sync.RWMutex
	entries []queryHistoryEntry
	max     int
}

func newQueryHistoryStore(max int) *queryHistoryStore {
	if max <= 0 {
		max = 100
	}
	return &queryHistoryStore{max: max}
}

func (s *queryHistoryStore) record(e queryHistoryEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries = append(s.entries, e)
	if len(s.entries) > s.max {
		s.entries = s.entries[len(s.entries)-s.max:]
	}
}

func (s *queryHistoryStore) list(ownerID string) []queryHistoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]queryHistoryEntry, 0, len(s.entries))
	for _, e := range s.entries {
		if ownerID != "" && e.OwnerID != ownerID {
			continue
		}
		out = append(out, e)
	}
	return out
}

// recordQueryHistory records a SQL execution against the history store.
func (h *Handler) recordQueryHistory(ownerID string, datasetIDs []string, sql string, startedAt time.Time, rowCount int, errMsg string) {
	h.queryHistory.record(queryHistoryEntry{
		ID:         newID(),
		OwnerID:    ownerID,
		DatasetIDs: datasetIDs,
		SQL:        sql,
		ExecutedAt: startedAt.UTC().Format(time.RFC3339),
		DurationMs: time.Since(startedAt).Milliseconds(),
		RowCount:   rowCount,
		Error:      errMsg,
	})
}

// SavedQueryService manages named, persisted SQL queries (SQL Lab).
type SavedQueryService struct {
	mu  sync.RWMutex
	defs map[string]*store.SavedQueryRecord
	db  *store.DB
}

func newSavedQueryService(db *store.DB) *SavedQueryService {
	s := &SavedQueryService{
		defs: make(map[string]*store.SavedQueryRecord),
		db:   db,
	}
	if db != nil {
		if records, err := db.LoadSavedQueries(""); err == nil {
			for i := range records {
				s.defs[records[i].ID] = &records[i]
			}
		}
	}
	return s
}

func (s *SavedQueryService) List(ownerID string) []*store.SavedQueryRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*store.SavedQueryRecord, 0, len(s.defs))
	for _, q := range s.defs {
		if ownerID != "" && q.OwnerID != ownerID {
			continue
		}
		out = append(out, q)
	}
	return out
}

func (s *SavedQueryService) Create(q *store.SavedQueryRecord) *store.SavedQueryRecord {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	q.ID = newID()
	q.CreatedAt = now
	q.UpdatedAt = now
	s.defs[q.ID] = q
	if s.db != nil {
		_ = s.db.SaveSavedQuery(q.ID, q.OwnerID, q.Name, q.DatasetIDs, q.SQL, q.CreatedAt, q.UpdatedAt)
	}
	return q
}

func (s *SavedQueryService) Update(q *store.SavedQueryRecord, ownerID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.defs[q.ID]
	if !ok || (ownerID != "" && existing.OwnerID != ownerID) {
		return false
	}
	existing.Name = q.Name
	existing.DatasetIDs = q.DatasetIDs
	existing.SQL = q.SQL
	existing.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if s.db != nil {
		_ = s.db.SaveSavedQuery(existing.ID, existing.OwnerID, existing.Name, existing.DatasetIDs, existing.SQL, existing.CreatedAt, existing.UpdatedAt)
	}
	return true
}

func (s *SavedQueryService) Delete(id, ownerID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	q, ok := s.defs[id]
	if !ok || (ownerID != "" && q.OwnerID != ownerID) {
		return false
	}
	delete(s.defs, id)
	if s.db != nil {
		_ = s.db.DeleteSavedQuery(id, ownerID)
	}
	return true
}

// --- HTTP Handlers ---

func (h *Handler) handleQueryHistory(w http.ResponseWriter, r *http.Request) {
	SendJSON(w, http.StatusOK, map[string]interface{}{"history": h.queryHistory.list(h.currentUserID(r))})
}

func (h *Handler) handleSavedQueryList(w http.ResponseWriter, r *http.Request) {
	queries := h.savedQuerySvc.List(h.currentUserID(r))
	SendJSON(w, http.StatusOK, map[string]interface{}{"queries": queries})
}

func (h *Handler) handleSavedQueryCreate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name       string   `json:"name"`
		DatasetIDs []string `json:"datasetIds"`
		SQL        string   `json:"sql"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.Name == "" {
		sendInvalidRequest(w, "name is required")
		return
	}
	if req.SQL == "" {
		sendInvalidRequest(w, "sql is required")
		return
	}
	if len(req.DatasetIDs) == 0 {
		sendInvalidRequest(w, "datasetIds is required")
		return
	}
	for _, id := range req.DatasetIDs {
		if !h.canAccessDataset(r, id) {
			sendNotFound(w, "Dataset not found")
			return
		}
	}
	idsJSON, _ := json.Marshal(req.DatasetIDs)
	q := &store.SavedQueryRecord{
		OwnerID:    h.currentUserID(r),
		Name:       req.Name,
		DatasetIDs: string(idsJSON),
		SQL:        req.SQL,
	}
	created := h.savedQuerySvc.Create(q)
	SendJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleSavedQueryUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID         string   `json:"id"`
		Name       string   `json:"name"`
		DatasetIDs []string `json:"datasetIds"`
		SQL        string   `json:"sql"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if req.ID == "" {
		sendInvalidRequest(w, "id is required")
		return
	}
	idsJSON, _ := json.Marshal(req.DatasetIDs)
	q := &store.SavedQueryRecord{
		ID:         req.ID,
		Name:       req.Name,
		DatasetIDs: string(idsJSON),
		SQL:        req.SQL,
	}
	if !h.savedQuerySvc.Update(q, h.currentUserID(r)) {
		sendNotFound(w, "Saved query not found")
		return
	}
	SendJSON(w, http.StatusOK, q)
}

func (h *Handler) handleSavedQueryDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		sendInvalidRequest(w, "id query parameter required")
		return
	}
	if !h.savedQuerySvc.Delete(id, h.currentUserID(r)) {
		sendNotFound(w, "Saved query not found")
		return
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
