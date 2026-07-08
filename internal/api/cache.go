package api

import (
	"os"
	"strconv"
	"time"

	"insightpilot/internal/cache"
	"insightpilot/internal/data"
)

// initResultCache builds the in-memory result cache. The default TTL is
// configurable via CACHE_TTL_SEC (0 or unset => 5 minutes). The Redis-ready
// interface (cache.Cache) means a distributed backend can replace it later.
func initResultCache() *cache.MemoryCache {
	ttl := 5 * time.Minute
	if v := os.Getenv("CACHE_TTL_SEC"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			ttl = time.Duration(n) * time.Second
		}
	}
	return cache.NewMemoryCache(ttl, time.Minute)
}

// datasetVersion returns a stable signature for a dataset that changes when
// its schema or size changes. It is used as part of result-cache keys so that
// edited/re-uploaded data automatically invalidates prior cached results.
func datasetVersion(ds *data.Dataset) string {
	cols := make([]string, 0, len(ds.Profile.Columns))
	for _, c := range ds.Profile.Columns {
		cols = append(cols, c.Name+":"+c.Type)
	}
	path := ds.FilePath
	if path == "" {
		path = ds.Filename
	}
	return cache.HashKey(
		ds.ID,
		path,
		itoa(ds.Profile.RowCount),
		joinStrings(cols, "|"),
	)
}

// cachedExecuteSQL runs a SQL query through the result cache. The key is a hash
// of (sql + dataset versions + pagination), so identical queries against
// unchanged datasets are served from cache.
func (h *Handler) cachedExecuteSQL(datasets []*data.Dataset, sql string, page, pageSize int) ([]map[string]string, []string, error) {
	if h.resultCache == nil {
		return h.executeSQL(datasets, sql, page, pageSize)
	}
	parts := make([]string, 0, len(datasets)+3)
	parts = append(parts, sql, itoa(page), itoa(pageSize))
	for _, d := range datasets {
		parts = append(parts, datasetVersion(d))
	}
	key := cache.HashKey(parts...)

	if v, ok := h.resultCache.Get(key); ok {
		if cached, ok := v.(cachedResult); ok {
			return cached.Rows, cached.Columns, nil
		}
	}

	rows, columns, err := h.executeSQL(datasets, sql, page, pageSize)
	if err != nil {
		return nil, nil, err
	}
	h.resultCache.Set(key, cachedResult{Rows: rows, Columns: columns}, 0)
	return rows, columns, nil
}

// cachedExploreSQL caches a single-dataset explore query result.
func (h *Handler) cachedExploreSQL(ds *data.Dataset, sql string) ([]map[string]string, error) {
	if h.resultCache == nil {
		return h.runExploreSQL(ds, sql)
	}
	key := cache.HashKey("explore", datasetVersion(ds), sql)
	if v, ok := h.resultCache.Get(key); ok {
		if cached, ok := v.(cachedRows); ok {
			return cached.Rows, nil
		}
	}
	rows, err := h.runExploreSQL(ds, sql)
	if err != nil {
		return nil, err
	}
	h.resultCache.Set(key, cachedRows{Rows: rows}, 0)
	return rows, nil
}

type cachedResult struct {
	Rows    []map[string]string
	Columns []string
}

type cachedRows struct {
	Rows []map[string]string
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func joinStrings(parts []string, sep string) string {
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += sep
		}
		out += p
	}
	return out
}
