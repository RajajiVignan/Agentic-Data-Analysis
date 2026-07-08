// Package cache provides a pluggable result-cache abstraction with an
// in-memory TTL implementation. The interface is intentionally small so a
// Redis-backed implementation can be dropped in later (P1-1: "Redis-ready").
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"time"
)

// Cache is the result-cache contract. Implementations must be safe for
// concurrent use.
type Cache interface {
	// Get returns the cached value for key and whether it was present and live.
	Get(key string) (interface{}, bool)
	// Set stores value under key with the given TTL (0 = use default).
	Set(key string, value interface{}, ttl time.Duration)
	// Delete removes a key.
	Delete(key string)
}

// Entry is a single cache item with an optional expiry.
type Entry struct {
	Value   interface{}
	Expires time.Time
}

// MemoryCache is an in-memory, concurrency-safe TTL cache.
type MemoryCache struct {
	mu           sync.RWMutex
	items        map[string]Entry
	defaultTTL   time.Duration
	sweepEvery   time.Duration
	stopCh       chan struct{}
	stopped      bool
}

// NewMemoryCache creates an in-memory cache. defaultTTL is applied when Set is
// called with a zero TTL; sweepEvery controls how often expired entries are
// reclaimed (use 0 to disable the background sweeper).
func NewMemoryCache(defaultTTL, sweepEvery time.Duration) *MemoryCache {
	if defaultTTL <= 0 {
		defaultTTL = 5 * time.Minute
	}
	if sweepEvery <= 0 {
		sweepEvery = time.Minute
	}
	c := &MemoryCache{
		items:      make(map[string]Entry),
		defaultTTL: defaultTTL,
		sweepEvery: sweepEvery,
		stopCh:     make(chan struct{}),
	}
	go c.sweep()
	return c
}

// Get returns the cached value, if present and not expired.
func (c *MemoryCache) Get(key string) (interface{}, bool) {
	c.mu.RLock()
	entry, ok := c.items[key]
	c.mu.RUnlock()
	if !ok {
		return nil, false
	}
	if !entry.Expires.IsZero() && time.Now().After(entry.Expires) {
		c.Delete(key)
		return nil, false
	}
	return entry.Value, true
}

// Set stores value under key with the given TTL.
func (c *MemoryCache) Set(key string, value interface{}, ttl time.Duration) {
	if ttl <= 0 {
		ttl = c.defaultTTL
	}
	c.mu.Lock()
	c.items[key] = Entry{
		Value:   value,
		Expires: time.Now().Add(ttl),
	}
	c.mu.Unlock()
}

// Delete removes a key.
func (c *MemoryCache) Delete(key string) {
	c.mu.Lock()
	delete(c.items, key)
	c.mu.Unlock()
}

// Len returns the number of live entries (used by tests/metrics).
func (c *MemoryCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.items)
}

// Close stops the background sweeper.
func (c *MemoryCache) Close() {
	c.mu.Lock()
	if c.stopped {
		c.mu.Unlock()
		return
	}
	c.stopped = true
	close(c.stopCh)
	c.mu.Unlock()
}

func (c *MemoryCache) sweep() {
	ticker := time.NewTicker(c.sweepEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			now := time.Now()
			c.mu.Lock()
			for k, e := range c.items {
				if !e.Expires.IsZero() && now.After(e.Expires) {
					delete(c.items, k)
				}
			}
			c.mu.Unlock()
		case <-c.stopCh:
			return
		}
	}
}

// HashKey computes a stable, collision-resistant key from the given parts.
// It is used to build cache keys from (query + dataset version) tuples.
func HashKey(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		_, _ = h.Write([]byte(p))
		_, _ = h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
