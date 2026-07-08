package api

import (
	"net/http"
	"sort"
	"sync"
	"time"

	"insightpilot/internal/data"
	"insightpilot/internal/store"
)

// SemanticFieldService manages per-dataset computed fields (semantic layer).
type SemanticFieldService struct {
	mu   sync.RWMutex
	defs map[string]*data.SemanticField
	db   *store.DB
}

func newSemanticFieldService(db *store.DB) *SemanticFieldService {
	s := &SemanticFieldService{
		defs: make(map[string]*data.SemanticField),
		db:   db,
	}
	s.loadFromDB("")
	return s
}

func (s *SemanticFieldService) loadFromDB(datasetID string) {
	if s.db == nil {
		return
	}
	fields, err := s.db.LoadDatasetFields(datasetID)
	if err != nil || len(fields) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range fields {
		s.defs[fields[i].ID] = &fields[i]
	}
}

// List returns fields for a dataset, or all fields when datasetID is empty.
func (s *SemanticFieldService) List(datasetID string) []*data.SemanticField {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*data.SemanticField, 0, len(s.defs))
	for _, f := range s.defs {
		if datasetID == "" || f.DatasetID == datasetID {
			out = append(out, f)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func (s *SemanticFieldService) Get(id string) *data.SemanticField {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.defs[id]
}

func (s *SemanticFieldService) Create(f *data.SemanticField) *data.SemanticField {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	f.ID = newID()
	f.CreatedAt = now
	f.UpdatedAt = now
	s.defs[f.ID] = f
	if s.db != nil {
		_ = s.db.SaveDatasetField(f)
	}
	return f
}

func (s *SemanticFieldService) Update(f *data.SemanticField) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.defs[f.ID]
	if !ok {
		return false
	}
	f.CreatedAt = existing.CreatedAt
	f.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	s.defs[f.ID] = f
	if s.db != nil {
		_ = s.db.SaveDatasetField(f)
	}
	return true
}

func (s *SemanticFieldService) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.defs[id]; !ok {
		return false
	}
	delete(s.defs, id)
	if s.db != nil {
		_ = s.db.DeleteDatasetField(id)
	}
	return true
}

// --- HTTP Handlers ---

func (h *Handler) handleSemanticList(w http.ResponseWriter, r *http.Request) {
	datasetID := r.URL.Query().Get("datasetId")
	fields := h.semanticSvc.List(datasetID)
	SendJSON(w, http.StatusOK, map[string]interface{}{"fields": fields})
}

func (h *Handler) handleSemanticCreate(w http.ResponseWriter, r *http.Request) {
	var f data.SemanticField
	if !decodeJSON(w, r, &f) {
		return
	}
	if f.Name == "" {
		sendInvalidRequest(w, "Name is required")
		return
	}
	if f.Kind != "metric" && f.Kind != "dimension" {
		sendInvalidRequest(w, "kind must be 'metric' or 'dimension'")
		return
	}
	if len(f.Config) == 0 {
		sendInvalidRequest(w, "config is required")
		return
	}
	created := h.semanticSvc.Create(&f)
	SendJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleSemanticUpdate(w http.ResponseWriter, r *http.Request) {
	var f data.SemanticField
	if !decodeJSON(w, r, &f) {
		return
	}
	if f.ID == "" {
		sendInvalidRequest(w, "ID is required")
		return
	}
	if !h.semanticSvc.Update(&f) {
		sendNotFound(w, "Field not found")
		return
	}
	SendJSON(w, http.StatusOK, f)
}

func (h *Handler) handleSemanticDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		sendInvalidRequest(w, "id query parameter required")
		return
	}
	if !h.semanticSvc.Delete(id) {
		sendNotFound(w, "Field not found")
		return
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
