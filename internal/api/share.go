package api

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
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
	mu     sync.RWMutex
}

func NewShareService() *ShareService {
	return &ShareService{
		shares: make(map[string]*SharedDashboard),
	}
}

func generateShareToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *ShareService) Create(charts []*PinnedChart, baseURL string) *SharedDashboard {
	s.mu.Lock()
	defer s.mu.Unlock()

	token := generateShareToken()
	sd := &SharedDashboard{
		Token:     token,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
		Charts:    charts,
		URL:       baseURL + "/shared/" + token,
	}
	s.shares[token] = sd
	return sd
}

func (s *ShareService) Get(token string) *SharedDashboard {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sd, ok := s.shares[token]
	if !ok {
		return nil
	}
	if time.Now().After(sd.ExpiresAt) {
		return nil
	}
	return sd
}

func (h *Handler) handleCreateShareLink(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ChartIDs []string `json:"chartIds"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}

	charts := h.pinnedSvc.GetByIDs(body.ChartIDs)

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	baseURL := scheme + "://" + r.Host

	sd := h.shareSvc.Create(charts, baseURL)
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
