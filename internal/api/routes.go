package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// chiRouter builds the chi router with all API routes.
// This replaces the raw http.NewServeMux with proper path parameter support.
func (h *Handler) chiRouter() http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware(h.allowedOrigins))

	// API routes
	r.Route("/api", func(r chi.Router) {
		r.Get("/health", h.handleHealth)
		r.Get("/datasets", h.handleGetDatasets)
		r.Post("/upload", h.handleUpload)
		r.Post("/analyze", h.handleAnalyze)
		r.Post("/connect-source", h.handleConnectSource)
		r.Get("/export/cleaned-csv", h.handleExportCsv)

		r.Get("/pinned-charts", h.handleGetPinnedCharts)
		r.Post("/pin-chart", h.handlePinChart)
		r.Delete("/unpin-chart", h.handleUnpinChart)

		r.Get("/python-plot", h.handlePythonPlot)
	})

	// Static file server for generated plots — chi handles path params properly
	r.Get("/plots/{filename}", h.plotService.ServePlot)

	return r
}

// corsMiddleware returns a chi middleware that sets CORS headers based on
// the configured allowed origins.
func corsMiddleware(allowedOrigins map[string]bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" {
				if allowedOrigins["*"] || allowedOrigins[origin] {
					w.Header().Set("Access-Control-Allow-Origin", origin)
					w.Header().Set("Vary", "Origin")
				}
			}
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
