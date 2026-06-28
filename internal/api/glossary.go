package api

import (
	"net/http"
	"sort"
	"sync"
	"time"

	"insightpilot/internal/data"
	"insightpilot/internal/store"
)

// GlossaryService manages business metric definitions (semantic layer).
type GlossaryService struct {
	mu    sync.RWMutex
	defs  map[string]*data.MetricDefinition
	db    *store.DB
}

func newGlossaryService(db *store.DB) *GlossaryService {
	gs := &GlossaryService{
		defs: make(map[string]*data.MetricDefinition),
		db:   db,
	}
	gs.loadFromDB()
	return gs
}

func (s *GlossaryService) loadFromDB() {
	if s.db == nil {
		return
	}
	defs, err := s.db.LoadGlossaryDefs()
	if err != nil || len(defs) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, def := range defs {
		s.defs[def.ID] = def
	}
}

func (s *GlossaryService) List() []*data.MetricDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*data.MetricDefinition, 0, len(s.defs))
	for _, def := range s.defs {
		out = append(out, def)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func (s *GlossaryService) Get(id string) *data.MetricDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.defs[id]
}

func (s *GlossaryService) Create(def *data.MetricDefinition) *data.MetricDefinition {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC().Format(time.RFC3339)
	def.ID = newID()
	def.CreatedAt = now
	def.UpdatedAt = now
	s.defs[def.ID] = def
	if s.db != nil {
		_ = s.db.SaveGlossaryDef(def)
	}
	return def
}

func (s *GlossaryService) Update(def *data.MetricDefinition) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.defs[def.ID]
	if !ok {
		return false
	}
	def.CreatedAt = existing.CreatedAt
	def.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	s.defs[def.ID] = def
	if s.db != nil {
		_ = s.db.SaveGlossaryDef(def)
	}
	return true
}

func (s *GlossaryService) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.defs[id]
	if !ok {
		return false
	}
	delete(s.defs, id)
	if s.db != nil {
		_ = s.db.DeleteGlossaryDef(id)
	}
	return true
}

// --- HTTP Handlers ---

func (h *Handler) handleGlossaryList(w http.ResponseWriter, r *http.Request) {
	defs := h.glossarySvc.List()
	SendJSON(w, http.StatusOK, map[string]interface{}{"definitions": defs})
}

func (h *Handler) handleGlossaryCreate(w http.ResponseWriter, r *http.Request) {
	var def data.MetricDefinition
	if !decodeJSON(w, r, &def) {
		return
	}
	if def.Name == "" {
		sendInvalidRequest(w, "Name is required")
		return
	}
	created := h.glossarySvc.Create(&def)
	SendJSON(w, http.StatusCreated, created)
}

func (h *Handler) handleGlossaryUpdate(w http.ResponseWriter, r *http.Request) {
	var def data.MetricDefinition
	if !decodeJSON(w, r, &def) {
		return
	}
	if def.ID == "" {
		sendInvalidRequest(w, "ID is required")
		return
	}
	if !h.glossarySvc.Update(&def) {
		sendNotFound(w, "Definition not found")
		return
	}
	SendJSON(w, http.StatusOK, def)
}

func (h *Handler) handleGlossaryDelete(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		sendInvalidRequest(w, "id query parameter required")
		return
	}
	if !h.glossarySvc.Delete(id) {
		sendNotFound(w, "Definition not found")
		return
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
