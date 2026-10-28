package api

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"insightpilot/internal/store"
)

// PinnedChart represents a chart that has been pinned to the dashboard.
type PinnedChart struct {
	ID        string `json:"id"`
	OwnerID   string `json:"ownerId,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	ChartType string `json:"chart_type"`
	Label     string `json:"label"`
	Data      any    `json:"data"`
	URL       string `json:"url,omitempty"`
}

// PinnedChartService manages pinned charts in memory and syncs to the database.
type PinnedChartService struct {
	charts map[string]*PinnedChart
	db     *store.DB
	mu     sync.RWMutex
}

// NewPinnedChartService creates a new PinnedChartService, optionally loading
// existing charts from the database.
func NewPinnedChartService(db *store.DB) *PinnedChartService {
	ps := &PinnedChartService{
		charts: make(map[string]*PinnedChart),
		db:     db,
	}
	if db != nil {
		ps.loadFromDB()
	}
	return ps
}

func (ps *PinnedChartService) loadFromDB() {
	records, err := ps.db.GetPinnedCharts("")
	if err != nil {
		slog.Warn("Warning: failed to load pinned charts from DB:", "error", err)
		return
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for _, c := range records {
		ps.charts[c.ID] = &PinnedChart{
			ID:        c.ID,
			OwnerID:   c.OwnerID,
			CreatedAt: c.CreatedAt.Format(time.RFC3339),
			ChartType: c.ChartType,
			Label:     c.Label,
			Data:      c.Data,
			URL:       c.URL,
		}
	}
	slog.Info("Loaded %d pinned charts from database", "length of records", len(records))
}

// GetByIDs returns pinned charts matching the given IDs, scoped to ownerID.
func (ps *PinnedChartService) GetByIDs(ids []string, ownerID string) []*PinnedChart {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	result := make([]*PinnedChart, 0, len(ids))
	for _, id := range ids {
		pc, ok := ps.charts[id]
		if !ok || (ownerID != "" && pc.OwnerID != ownerID) {
			continue
		}
		result = append(result, pc)
	}
	return result
}

// GetAll returns pinned charts owned by ownerID.
func (ps *PinnedChartService) GetAll(ownerID string) []*PinnedChart {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	list := make([]*PinnedChart, 0, len(ps.charts))
	for _, pc := range ps.charts {
		if ownerID != "" && pc.OwnerID != ownerID {
			continue
		}
		list = append(list, pc)
	}
	return list
}

// Owned reports whether the chart belongs to ownerID.
func (ps *PinnedChartService) Owned(id, ownerID string) bool {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	pc, ok := ps.charts[id]
	return ok && (ownerID == "" || pc.OwnerID == ownerID)
}

// Add pins a new chart. If id is empty, one is generated.
// The chart is persisted to the database if available.
func (ps *PinnedChartService) Add(ownerID, chartType, label string, data any, urlStr string) (*PinnedChart, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	id := newID()
	pc := &PinnedChart{
		ID:        id,
		OwnerID:   ownerID,
		ChartType: chartType,
		Label:     label,
		Data:      data,
		URL:       urlStr,
	}
	ps.charts[pc.ID] = pc

	if ps.db != nil {
		dbID, err := ps.db.SavePinnedChart(pc.ID, pc.OwnerID, pc.ChartType, pc.Label, pc.Data, pc.URL)
		if err != nil {
			return nil, fmt.Errorf("save pinned chart to DB: %w", err)
		}
		if dbID != "" {
			pc.ID = dbID
		}
	}
	return pc, nil
}

// Remove unpins a chart by ID. Also removes from the database if available.
func (ps *PinnedChartService) Remove(id, ownerID string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if pc, ok := ps.charts[id]; ok {
		if ownerID != "" && pc.OwnerID != ownerID {
			return fmt.Errorf("chart not found")
		}
		delete(ps.charts, id)
	}

	if ps.db != nil {
		if err := ps.db.DeletePinnedChart(id, ownerID); err != nil {
			return fmt.Errorf("delete pinned chart from DB: %w", err)
		}
	}
	return nil
}
