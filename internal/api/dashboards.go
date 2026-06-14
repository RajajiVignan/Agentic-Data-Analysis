package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"insightpilot/internal/store"

	"github.com/go-chi/chi/v5"
)

// Dashboard represents a named dashboard that holds a collection of pinned charts.
type Dashboard struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	ChartIDs  []string `json:"chartIds,omitempty"`
	CreatedAt string   `json:"created_at"`
}

// DashboardService manages dashboards in memory and syncs to the database.
type DashboardService struct {
	dashboards map[string]*Dashboard
	db         *store.DB
	mu         sync.RWMutex
}

// NewDashboardService creates a new DashboardService, optionally loading
// existing dashboards from the database.
func NewDashboardService(db *store.DB) *DashboardService {
	ds := &DashboardService{
		dashboards: make(map[string]*Dashboard),
		db:         db,
	}
	if db != nil {
		ds.loadFromDB()
	}
	return ds
}

func (ds *DashboardService) loadFromDB() {
	records, err := ds.db.GetDashboards()
	if err != nil {
		log.Printf("Warning: failed to load dashboards from DB: %v", err)
		return
	}
	ds.mu.Lock()
	defer ds.mu.Unlock()
	for _, d := range records {
		ds.dashboards[d.ID] = &Dashboard{
			ID:        d.ID,
			Name:      d.Name,
			ChartIDs:  d.ChartIDs,
			CreatedAt: d.CreatedAt.Format(time.RFC3339),
		}
	}
	log.Printf("Loaded %d dashboards from database", len(records))
}

// GetAll returns all dashboards.
func (ds *DashboardService) GetAll() []*Dashboard {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	list := make([]*Dashboard, 0, len(ds.dashboards))
	for _, d := range ds.dashboards {
		list = append(list, d)
	}
	return list
}

// Get returns a single dashboard by ID.
func (ds *DashboardService) Get(id string) *Dashboard {
	ds.mu.RLock()
	defer ds.mu.RUnlock()
	return ds.dashboards[id]
}

// Create creates a new dashboard with the given name.
func (ds *DashboardService) Create(name string) (*Dashboard, error) {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	id := "db_" + newID()
	d := &Dashboard{
		ID:        id,
		Name:      name,
		ChartIDs:  []string{},
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	ds.dashboards[d.ID] = d

	if ds.db != nil {
		if err := ds.db.SaveDashboard(d.ID, d.Name, d.ChartIDs); err != nil {
			return nil, fmt.Errorf("save dashboard to DB: %w", err)
		}
	}
	return d, nil
}

// Rename updates a dashboard's name.
func (ds *DashboardService) Rename(id, name string) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	d, ok := ds.dashboards[id]
	if !ok {
		return fmt.Errorf("dashboard not found: %s", id)
	}
	d.Name = name

	if ds.db != nil {
		if err := ds.db.SaveDashboard(d.ID, d.Name, d.ChartIDs); err != nil {
			return fmt.Errorf("update dashboard in DB: %w", err)
		}
	}
	return nil
}

// Delete removes a dashboard by ID.
func (ds *DashboardService) Delete(id string) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	delete(ds.dashboards, id)

	if ds.db != nil {
		if err := ds.db.DeleteDashboard(id); err != nil {
			return fmt.Errorf("delete dashboard from DB: %w", err)
		}
	}
	return nil
}

// AddChart adds a chart ID to a dashboard.
func (ds *DashboardService) AddChart(dashboardID, chartID string) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	d, ok := ds.dashboards[dashboardID]
	if !ok {
		return fmt.Errorf("dashboard not found: %s", dashboardID)
	}
	for _, cid := range d.ChartIDs {
		if cid == chartID {
			return nil // already present
		}
	}
	d.ChartIDs = append(d.ChartIDs, chartID)

	if ds.db != nil {
		if err := ds.db.SaveDashboard(d.ID, d.Name, d.ChartIDs); err != nil {
			return fmt.Errorf("update dashboard charts in DB: %w", err)
		}
	}
	return nil
}

// RemoveChart removes a chart ID from a dashboard.
func (ds *DashboardService) RemoveChart(dashboardID, chartID string) error {
	ds.mu.Lock()
	defer ds.mu.Unlock()

	d, ok := ds.dashboards[dashboardID]
	if !ok {
		return fmt.Errorf("dashboard not found: %s", dashboardID)
	}
	updated := make([]string, 0, len(d.ChartIDs))
	for _, cid := range d.ChartIDs {
		if cid != chartID {
			updated = append(updated, cid)
		}
	}
	d.ChartIDs = updated

	if ds.db != nil {
		if err := ds.db.SaveDashboard(d.ID, d.Name, d.ChartIDs); err != nil {
			return fmt.Errorf("update dashboard charts in DB: %w", err)
		}
	}
	return nil
}

// --- HTTP Handlers ---

func (h *Handler) handleListDashboards(w http.ResponseWriter, r *http.Request) {
	dashboards := h.dashboardSvc.GetAll()
	SendJSON(w, http.StatusOK, map[string]interface{}{"dashboards": dashboards})
}

func (h *Handler) handleCreateDashboard(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if body.Name == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Dashboard name is required"})
		return
	}

	d, err := h.dashboardSvc.Create(body.Name)
	if err != nil {
		SendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	SendJSON(w, http.StatusCreated, d)
}

func (h *Handler) handleRenameDashboard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "id path parameter required"})
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if body.Name == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Dashboard name is required"})
		return
	}

	if err := h.dashboardSvc.Rename(id, body.Name); err != nil {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func (h *Handler) handleDeleteDashboard(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "id path parameter required"})
		return
	}
	if err := h.dashboardSvc.Delete(id); err != nil {
		SendJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func (h *Handler) handleAddChartToDashboard(w http.ResponseWriter, r *http.Request) {
	dashboardID := chi.URLParam(r, "id")
	if dashboardID == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "id path parameter required"})
		return
	}
	var body struct {
		ChartID string `json:"chartId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON"})
		return
	}
	if body.ChartID == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "chartId is required"})
		return
	}

	if err := h.dashboardSvc.AddChart(dashboardID, body.ChartID); err != nil {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}

func (h *Handler) handleRemoveChartFromDashboard(w http.ResponseWriter, r *http.Request) {
	dashboardID := chi.URLParam(r, "id")
	chartID := chi.URLParam(r, "chartId")
	if dashboardID == "" || chartID == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "id and chartId path parameters required"})
		return
	}

	if err := h.dashboardSvc.RemoveChart(dashboardID, chartID); err != nil {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	SendJSON(w, http.StatusOK, map[string]interface{}{"ok": true})
}
