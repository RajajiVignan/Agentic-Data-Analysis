package api

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	"insightpilot/internal/data"
)

// asyncQueryJob is a single background SQL query execution.
type asyncQueryJob struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"` // "running" | "done" | "error"
	Columns   []string  `json:"columns"`
	Rows      []map[string]string `json:"rows"`
	Page      int       `json:"page"`
	PageSize  int       `json:"pageSize"`
	HasMore   bool      `json:"hasMore"`
	TotalRows int       `json:"totalRows"`
	Error     string    `json:"error"`
	CreatedAt time.Time `json:"createdAt"`
}

// asyncQueryManager tracks in-flight and completed async jobs.
type asyncQueryManager struct {
	mu     sync.RWMutex
	jobs   map[string]*asyncQueryJob
	stopCh chan struct{}
}

func newAsyncQueryManager() *asyncQueryManager {
	m := &asyncQueryManager{
		jobs:   make(map[string]*asyncQueryJob),
		stopCh: make(chan struct{}),
	}
	go m.sweep()
	return m
}

func (m *asyncQueryManager) create() *asyncQueryJob {
	job := &asyncQueryJob{
		ID:        newID(),
		Status:    "running",
		CreatedAt: time.Now(),
	}
	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()
	return job
}

func (m *asyncQueryManager) get(id string) (*asyncQueryJob, bool) {
	m.mu.RLock()
	job, ok := m.jobs[id]
	m.mu.RUnlock()
	return job, ok
}

func (m *asyncQueryManager) finish(job *asyncQueryJob, rows []map[string]string, columns []string, err error) {
	job.Columns = columns
	job.Rows = rows
	job.TotalRows = len(rows)
	job.HasMore = len(rows) >= job.PageSize
	if err != nil {
		job.Status = "error"
		job.Error = err.Error()
	} else {
		job.Status = "done"
	}
	m.mu.Lock()
	m.jobs[job.ID] = job
	m.mu.Unlock()
}

func (m *asyncQueryManager) sweep() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			cutoff := time.Now().Add(-30 * time.Minute)
			m.mu.Lock()
			for id, job := range m.jobs {
				if job.CreatedAt.Before(cutoff) {
					delete(m.jobs, id)
				}
			}
			m.mu.Unlock()
		case <-m.stopCh:
			return
		}
	}
}

func (m *asyncQueryManager) close() {
	close(m.stopCh)
}

// asyncQueryTimeout returns the timeout for background async queries.
func asyncQueryTimeout() time.Duration {
	sec := 600
	if v := os.Getenv("QUERY_ASYNC_TIMEOUT_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			sec = n
		}
	}
	return time.Duration(sec) * time.Second
}

// handleQueryAsync kicks off a long-running SQL query and returns a job ID
// that the client can poll with GET /api/query-job.
func (h *Handler) handleQueryAsync(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DatasetIDs []string `json:"datasetIds"`
		SQL        string   `json:"sql"`
		Page       int      `json:"page"`
		PageSize   int      `json:"pageSize"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.SQL == "" {
		sendInvalidRequest(w, "sql is required")
		return
	}
	if len(body.DatasetIDs) == 0 {
		sendInvalidRequest(w, "datasetIds is required")
		return
	}
	if body.Page <= 0 {
		body.Page = 1
	}
	if body.PageSize <= 0 || body.PageSize > 1000 {
		body.PageSize = 100
	}

	h.mu.RLock()
	var datasets []*data.Dataset
	for _, id := range body.DatasetIDs {
		if d, ok := h.datasets[id]; ok {
			datasets = append(datasets, d)
		}
	}
	h.mu.RUnlock()

	if len(datasets) == 0 {
		sendNotFound(w, "No datasets found")
		return
	}

	job := h.asyncMgr.create()
	job.Page = body.Page
	job.PageSize = body.PageSize

	datasetsCopy := make([]*data.Dataset, len(datasets))
	copy(datasetsCopy, datasets)

	go func() {
		rows, columns, err := h.runAsyncSQL(datasetsCopy, body.SQL, body.Page, body.PageSize, int(asyncQueryTimeout().Seconds()))
		h.asyncMgr.finish(job, rows, columns, err)
	}()

	SendJSON(w, http.StatusAccepted, map[string]interface{}{
		"jobId": job.ID,
		"status": "running",
	})
}

// handleQueryJob polls the status/result of an async query job.
func (h *Handler) handleQueryJob(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		sendInvalidRequest(w, "id query parameter required")
		return
	}
	job, ok := h.asyncMgr.get(id)
	if !ok {
		sendNotFound(w, "Job not found")
		return
	}
	SendJSON(w, http.StatusOK, job)
}

// runAsyncSQL executes a SQL query with a caller-supplied timeout, so long
// queries are not cut off by the synchronous QUERY_TIMEOUT_SEC limit.
func (h *Handler) runAsyncSQL(datasets []*data.Dataset, sql string, page, pageSize, timeoutSec int) ([]map[string]string, []string, error) {
	if err := validateSelectOnly(sql); err != nil {
		return nil, nil, err
	}

	duckDB := getDuckDBEngine()
	if duckDB != nil {
		var csvPaths []string
		for _, ds := range datasets {
			if ds.FilePath != "" {
				csvPaths = append(csvPaths, ds.FilePath)
			}
		}
		if len(csvPaths) > 0 {
			timeout := time.Duration(timeoutSec) * time.Second
			if timeout <= 0 {
				timeout = 10 * time.Minute
			}
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			defer cancel()
			return duckDB.ExecuteQueryWithContext(ctx, csvPaths, sql, page, pageSize)
		}
	}

	primary := datasets[0]
	if len(datasets) > 1 {
		primary = data.MergeDatasets(datasets)
	}
	return executeInMemorySQL(primary, sql, page, pageSize)
}
