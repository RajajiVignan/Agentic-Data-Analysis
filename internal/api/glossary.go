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
	defs, err := s.db.LoadGlossaryDefs("")
	if err != nil || len(defs) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, def := range defs {
		s.defs[def.ID] = def
	}
}

func (s *GlossaryService) List(ownerID string) []*data.MetricDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*data.MetricDefinition, 0, len(s.defs))
	for _, def := range s.defs {
		if ownerID != "" && def.OwnerID != ownerID {
			continue
		}
		out = append(out, def)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func (s *GlossaryService) Get(id, ownerID string) *data.MetricDefinition {
	s.mu.RLock()
	defer s.mu.RUnlock()
	def, ok := s.defs[id]
	if !ok || (ownerID != "" && def.OwnerID != ownerID) {
		return nil
	}
	return def
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

func (s *GlossaryService) Update(def *data.MetricDefinition, ownerID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.defs[def.ID]
	if !ok || (ownerID != "" && existing.OwnerID != ownerID) {
		return false
	}
	def.OwnerID = existing.OwnerID
	def.CreatedAt = existing.CreatedAt
	def.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	s.defs[def.ID] = def
	if s.db != nil {
		_ = s.db.SaveGlossaryDef(def)
	}
	return true
}

func (s *GlossaryService) Delete(id, ownerID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	def, ok := s.defs[id]
	if !ok || (ownerID != "" && def.OwnerID != ownerID) {
		return false
	}
	delete(s.defs, id)
	if s.db != nil {
		_ = s.db.DeleteGlossaryDef(id, ownerID)
	}
	return true
}

// --- HTTP Handlers ---

func (h *Handler) handleGlossaryList(w http.ResponseWriter, r *http.Request) {
	defs := h.glossarySvc.List(h.currentUserID(r))
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
	def.OwnerID = h.currentUserID(r)
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
	if !h.glossarySvc.Update(&def, h.currentUserID(r)) {
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
	if !h.glossarySvc.Delete(id, h.currentUserID(r)) {
		sendNotFound(w, "Definition not found")
		return
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
