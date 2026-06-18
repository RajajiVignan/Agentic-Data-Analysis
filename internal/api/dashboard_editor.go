package api

import (
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"insightpilot/internal/store"
)

type TileType string

const (
	TileChart     TileType = "chart"
	TileText      TileType = "text"
	TileDivider   TileType = "divider"
	TileImage     TileType = "image"
	TileMetric    TileType = "metric"
)

type DashboardTile struct {
	ID        string                 `json:"id"`
	Type      TileType               `json:"type"`
	ChartType string                 `json:"chartType,omitempty"`
	Title     string                 `json:"title,omitempty"`
	Content   string                 `json:"content,omitempty"`
	ImageURL  string                 `json:"imageUrl,omitempty"`
	PinnedID  string                 `json:"pinnedId,omitempty"`
	W         int                    `json:"w"`
	H         int                    `json:"h"`
	X         int                    `json:"x"`
	Y         int                    `json:"y"`
	Data      map[string]interface{} `json:"data,omitempty"`
}

type DashboardLayout struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	IsDefault bool             `json:"isDefault,omitempty"`
	Tiles     []DashboardTile  `json:"tiles"`
	CreatedAt string           `json:"createdAt"`
	UpdatedAt string           `json:"updatedAt"`
}

type DashboardEditorService struct {
	mu        sync.RWMutex
	layouts   map[string]*DashboardLayout
	db        reportStore
}

func NewDashboardEditorService(database *store.DB) *DashboardEditorService {
	var db reportStore
	if database != nil {
		db = newDBStore(database)
	}
	d := &DashboardEditorService{
		layouts: make(map[string]*DashboardLayout),
		db:      db,
	}
	if db != nil {
		if layouts, err := db.LoadLayouts(); err == nil {
			for _, l := range layouts {
				d.layouts[l.ID] = l
			}
			log.Printf("[dashboard] Loaded %d layouts from DB", len(layouts))
		}
	}
	if len(d.layouts) == 0 {
		d.seedDefault()
	}
	return d
}

func (d *DashboardEditorService) seedDefault() {
	d.layouts["default"] = &DashboardLayout{
		ID:        "default",
		Name:      "Default Layout",
		IsDefault: true,
		Tiles:     []DashboardTile{},
		CreatedAt: time.Now().Format(time.RFC3339),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
}

func (d *DashboardEditorService) ListLayouts() []*DashboardLayout {
	d.mu.RLock()
	defer d.mu.RUnlock()
	list := make([]*DashboardLayout, 0, len(d.layouts))
	for _, l := range d.layouts {
		list = append(list, l)
	}
	return list
}

func (d *DashboardEditorService) GetLayout(id string) *DashboardLayout {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.layouts[id]
}

func (d *DashboardEditorService) CreateLayout(name string) *DashboardLayout {
	d.mu.Lock()
	defer d.mu.Unlock()
	layout := &DashboardLayout{
		ID:        "layout_" + newID(),
		Name:      name,
		Tiles:     []DashboardTile{},
		CreatedAt: time.Now().Format(time.RFC3339),
		UpdatedAt: time.Now().Format(time.RFC3339),
	}
	d.layouts[layout.ID] = layout
	if d.db != nil {
		if err := d.db.SaveLayout(layout.ID, layout); err != nil {
			log.Printf("[dashboard] Failed to persist layout %s: %v", layout.ID, err)
		}
	}
	return layout
}

func (d *DashboardEditorService) SaveLayout(id string, layout *DashboardLayout) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	existing, ok := d.layouts[id]
	if !ok {
		return false
	}
	layout.ID = id
	layout.CreatedAt = existing.CreatedAt
	layout.UpdatedAt = time.Now().Format(time.RFC3339)
	d.layouts[id] = layout
	if d.db != nil {
		if err := d.db.SaveLayout(id, layout); err != nil {
			log.Printf("[dashboard] Failed to persist layout %s: %v", id, err)
		}
	}
	return true
}

func (d *DashboardEditorService) DeleteLayout(id string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	_, ok := d.layouts[id]
	if ok {
		delete(d.layouts, id)
		if d.db != nil {
			if err := d.db.DeleteLayout(id); err != nil {
				log.Printf("[dashboard] Failed to delete layout %s: %v", id, err)
			}
		}
	}
	return ok
}

func (d *DashboardEditorService) AddTile(layoutID string, tile DashboardTile) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	layout, ok := d.layouts[layoutID]
	if !ok {
		return false
	}
	tile.ID = "tile_" + newID()
	layout.Tiles = append(layout.Tiles, tile)
	layout.UpdatedAt = time.Now().Format(time.RFC3339)
	return true
}

func (d *DashboardEditorService) UpdateTile(layoutID, tileID string, tile DashboardTile) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	layout, ok := d.layouts[layoutID]
	if !ok {
		return false
	}
	for i, t := range layout.Tiles {
		if t.ID == tileID {
			tile.ID = tileID
			layout.Tiles[i] = tile
			layout.UpdatedAt = time.Now().Format(time.RFC3339)
			return true
		}
	}
	return false
}

func (d *DashboardEditorService) RemoveTile(layoutID, tileID string) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	layout, ok := d.layouts[layoutID]
	if !ok {
		return false
	}
	for i, t := range layout.Tiles {
		if t.ID == tileID {
			layout.Tiles = append(layout.Tiles[:i], layout.Tiles[i+1:]...)
			layout.UpdatedAt = time.Now().Format(time.RFC3339)
			return true
		}
	}
	return false
}

// --- HTTP Handlers ---

func (h *Handler) handleListLayouts(w http.ResponseWriter, r *http.Request) {
	layouts := h.dashEditorSvc.ListLayouts()
	SendJSON(w, http.StatusOK, map[string]interface{}{"layouts": layouts})
}

func (h *Handler) handleGetLayout(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "id path parameter required"})
		return
	}
	layout := h.dashEditorSvc.GetLayout(id)
	if layout == nil {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Layout not found"})
		return
	}
	SendJSON(w, http.StatusOK, layout)
}

func (h *Handler) handleCreateLayout(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Name == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}
	layout := h.dashEditorSvc.CreateLayout(body.Name)
	SendJSON(w, http.StatusCreated, layout)
}

func (h *Handler) handleSaveLayout(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "id path parameter required"})
		return
	}
	var layout DashboardLayout
	if !decodeJSON(w, r, &layout) {
		return
	}
	if !h.dashEditorSvc.SaveLayout(id, &layout) {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Layout not found"})
		return
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func (h *Handler) handleDeleteLayout(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "id path parameter required"})
		return
	}
	if !h.dashEditorSvc.DeleteLayout(id) {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Layout not found"})
		return
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func (h *Handler) handleAddTile(w http.ResponseWriter, r *http.Request) {
	layoutID := chi.URLParam(r, "id")
	if layoutID == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "id path parameter required"})
		return
	}
	var tile DashboardTile
	if !decodeJSON(w, r, &tile) {
		return
	}
	if tile.Type == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "type is required (chart, text, divider, image, metric)"})
		return
	}
	if !h.dashEditorSvc.AddTile(layoutID, tile) {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Layout not found"})
		return
	}
	layout := h.dashEditorSvc.GetLayout(layoutID)
	SendJSON(w, http.StatusCreated, layout)
}

func (h *Handler) handleUpdateTile(w http.ResponseWriter, r *http.Request) {
	layoutID := chi.URLParam(r, "id")
	tileID := chi.URLParam(r, "tileId")
	if layoutID == "" || tileID == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "id and tileId path parameters required"})
		return
	}
	var tile DashboardTile
	if !decodeJSON(w, r, &tile) {
		return
	}
	if !h.dashEditorSvc.UpdateTile(layoutID, tileID, tile) {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Layout or tile not found"})
		return
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func (h *Handler) handleRemoveTile(w http.ResponseWriter, r *http.Request) {
	layoutID := chi.URLParam(r, "id")
	tileID := chi.URLParam(r, "tileId")
	if layoutID == "" || tileID == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "id and tileId path parameters required"})
		return
	}
	if !h.dashEditorSvc.RemoveTile(layoutID, tileID) {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Layout or tile not found"})
		return
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
