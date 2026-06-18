package api

import (
	"net/http"
	"sync"
	"time"
)

type rateLimiter struct {
	mu       sync.Mutex
	requests map[string]map[string]*slidingWindow
	limit    int
	window   time.Duration
	stopCh   chan struct{}
}

type slidingWindow struct {
	timestamps []time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		requests: make(map[string]map[string]*slidingWindow),
		limit:    limit,
		window:   window,
		stopCh:   make(chan struct{}),
	}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			cutoff := time.Now().Add(-rl.window)
			for ip, paths := range rl.requests {
				for path, sw := range paths {
					sw.trim(cutoff)
					if len(sw.timestamps) == 0 {
						delete(paths, path)
					}
				}
				if len(paths) == 0 {
					delete(rl.requests, ip)
				}
			}
			rl.mu.Unlock()
		case <-rl.stopCh:
			return
		}
	}
}

func (sw *slidingWindow) trim(cutoff time.Time) {
	i := 0
	for ; i < len(sw.timestamps); i++ {
		if sw.timestamps[i].After(cutoff) {
			break
		}
	}
	sw.timestamps = sw.timestamps[i:]
}

func (rl *rateLimiter) allow(ip, path string) bool {
	now := time.Now()
	cutoff := now.Add(-rl.window)

	rl.mu.Lock()
	defer rl.mu.Unlock()

	paths, ok := rl.requests[ip]
	if !ok {
		paths = make(map[string]*slidingWindow)
		rl.requests[ip] = paths
	}
	sw, ok := paths[path]
	if !ok {
		sw = &slidingWindow{}
		paths[path] = sw
	}

	sw.trim(cutoff)
	if len(sw.timestamps) >= rl.limit {
		return false
	}
	sw.timestamps = append(sw.timestamps, now)
	return true
}

func (rl *rateLimiter) stop() {
	close(rl.stopCh)
}

func (h *Handler) rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := r.RemoteAddr
		if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
			ip = forwarded
		}
		if !h.rateLimiter.allow(ip, r.URL.Path) {
			w.Header().Set("Retry-After", "60")
			SendJSON(w, http.StatusTooManyRequests, map[string]string{"error": "Too many requests. Please try again later."})
			return
		}
		next.ServeHTTP(w, r)
	})
}
