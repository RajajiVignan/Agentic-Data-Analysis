package api

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"

	"insightpilot/internal/store"
)

type SharedDashboard struct {
	Token     string         `json:"token"`
	CreatedAt time.Time      `json:"created_at"`
	ExpiresAt time.Time      `json:"expires_at"`
	Charts    []*PinnedChart `json:"charts"`
	URL       string         `json:"url"`
}

type ShareService struct {
	shares map[string]*SharedDashboard
	db     *store.DB
	mu     sync.RWMutex
}

func NewShareService(db *store.DB) *ShareService {
	return &ShareService{
		shares: make(map[string]*SharedDashboard),
		db:     db,
	}
}

func generateShareToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *ShareService) Create(ownerID string, charts []*PinnedChart, baseURL string) *SharedDashboard {
	now := time.Now()
	token := generateShareToken()
	sd := &SharedDashboard{
		Token:     token,
		CreatedAt: now,
		ExpiresAt: now.Add(7 * 24 * time.Hour),
		Charts:    charts,
		URL:       baseURL + "/shared/" + token,
	}

	if s.db != nil && !isGuestUser(ownerID) {
		if chartsJSON, err := json.Marshal(charts); err == nil {
			if err := s.db.SaveShare(token, ownerID, sd.CreatedAt, sd.ExpiresAt, chartsJSON, sd.URL); err != nil {
				slog.Warn("Failed to persist share link", "component", "share", "error", err)
			}
		} else {
			slog.Warn("Failed to marshal share charts", "component", "share", "error", err)
		}
		_, _ = s.db.DeleteExpiredShares(now)
	}

	s.mu.Lock()
	s.shares[token] = sd
	s.mu.Unlock()
	return sd
}

func (s *ShareService) Get(token string) *SharedDashboard {
	now := time.Now()
	s.mu.RLock()
	sd, ok := s.shares[token]
	s.mu.RUnlock()
	if ok {
		if now.After(sd.ExpiresAt) {
			s.mu.Lock()
			delete(s.shares, token)
			s.mu.Unlock()
			return nil
		}
		return sd
	}

	if s.db == nil {
		return nil
	}
	rec, err := s.db.GetShare(token)
	if err != nil {
		slog.Warn("Failed to load share link", "component", "share", "error", err)
		return nil
	}
	if rec == nil || now.After(rec.ExpiresAt) {
		return nil
	}
	var charts []*PinnedChart
	if err := json.Unmarshal(rec.Charts, &charts); err != nil {
		slog.Warn("Failed to decode share charts", "component", "share", "error", err)
		return nil
	}
	sd = &SharedDashboard{
		Token:     rec.Token,
		CreatedAt: rec.CreatedAt,
		ExpiresAt: rec.ExpiresAt,
		Charts:    charts,
		URL:       rec.URL,
	}
	s.mu.Lock()
	s.shares[token] = sd
	s.mu.Unlock()
	return sd
}

func (h *Handler) handleCreateShareLink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ChartIDs []string `json:"chartIds"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	ownerID := h.currentUserID(r)
	charts := h.pinnedSvc.GetByIDs(body.ChartIDs, ownerID)
	if len(charts) == 0 {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "No owned charts provided"})
		return
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	baseURL := scheme + "://" + r.Host

	sd := h.shareSvc.Create(ownerID, charts, baseURL)
	h.sendJSON(w, http.StatusCreated, sd)
}

func (h *Handler) handleGetSharedDashboard(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		h.sendJSON(w, http.StatusBadRequest, map[string]string{"error": "Token required"})
		return
	}

	sd := h.shareSvc.Get(token)
	if sd == nil {
		h.sendJSON(w, http.StatusNotFound, map[string]string{"error": "Shared dashboard not found or expired"})
		return
	}

	h.sendJSON(w, http.StatusOK, sd)
}
