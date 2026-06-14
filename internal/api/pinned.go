package api

import (
	"fmt"
	"sync"
	"time"

	"insightpilot/internal/store"
)

// PinnedChart represents a chart that has been pinned to the dashboard.
type PinnedChart struct {
	ID        string `json:"id"`
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
	records, err := ps.db.GetPinnedCharts()
	if err != nil {
		fmt.Printf("Warning: failed to load pinned charts from DB: %v\n", err)
		return
	}
	ps.mu.Lock()
	defer ps.mu.Unlock()
	for _, c := range records {
		ps.charts[c.ID] = &PinnedChart{
			ID:        c.ID,
			CreatedAt: c.CreatedAt.Format(time.RFC3339),
			ChartType: c.ChartType,
			Label:     c.Label,
			Data:      c.Data,
			URL:       c.URL,
		}
	}
	fmt.Printf("Loaded %d pinned charts from database\n", len(records))
}

// GetByIDs returns pinned charts matching the given IDs.
func (ps *PinnedChartService) GetByIDs(ids []string) []*PinnedChart {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	result := make([]*PinnedChart, 0, len(ids))
	for _, id := range ids {
		if pc, ok := ps.charts[id]; ok {
			result = append(result, pc)
		}
	}
	return result
}

// GetAll returns all pinned charts.
func (ps *PinnedChartService) GetAll() []*PinnedChart {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	list := make([]*PinnedChart, 0, len(ps.charts))
	for _, pc := range ps.charts {
		list = append(list, pc)
	}
	return list
}

// Add pins a new chart. If id is empty, one is generated.
// The chart is persisted to the database if available.
func (ps *PinnedChartService) Add(chartType, label string, data any, urlStr string) (*PinnedChart, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	id := newID()
	pc := &PinnedChart{
		ID:        id,
		ChartType: chartType,
		Label:     label,
		Data:      data,
		URL:       urlStr,
	}
	ps.charts[pc.ID] = pc

	if ps.db != nil {
		dbID, err := ps.db.SavePinnedChart(pc.ID, pc.ChartType, pc.Label, pc.Data, pc.URL)
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
func (ps *PinnedChartService) Remove(id string) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	delete(ps.charts, id)

	if ps.db != nil {
		if err := ps.db.DeletePinnedChart(id); err != nil {
			return fmt.Errorf("delete pinned chart from DB: %w", err)
		}
	}
	return nil
}
