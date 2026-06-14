package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// chiRouter builds the HTTP handler with all routes.
// It uses chi for API/plots/_next routes and falls back to a static file
// handler for everything else. We avoid chi's wildcard redirect behavior by
// dispatching at the top level based on URL prefix.
func (h *Handler) chiRouter() http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(corsMiddleware(h.allowedOrigins))

	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Printf("[PANIC] %s %s: %v", r.Method, r.URL.Path, rec)
					if strings.HasPrefix(r.URL.Path, "/api/") {
						w.Header().Set("Content-Type", "application/json; charset=utf-8")
						w.WriteHeader(http.StatusInternalServerError)
						json.NewEncoder(w).Encode(map[string]string{
							"error": fmt.Sprintf("Internal server error: %v", rec),
						})
					} else {
						w.Header().Set("Content-Type", "text/html; charset=utf-8")
						w.WriteHeader(http.StatusInternalServerError)
						fmt.Fprint(w, "<html><body><h1>500 Internal Server Error</h1></body></html>")
					}
				}
			}()
			next.ServeHTTP(w, r)
		})
	})

	// Auth middleware
	r.Use(h.authMiddleware)

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Rate-limited auth endpoints (register, login)
		r.Group(func(r chi.Router) {
			r.Use(h.rateLimitMiddleware)
			r.Post("/auth/register", h.handleRegister)
			r.Post("/auth/login", h.handleLogin)
		})
		r.Post("/auth/logout", h.handleLogout)
		r.Get("/auth/me", h.handleMe)
		r.Get("/health", h.handleHealth)
		r.Get("/datasets", h.handleGetDatasets)
		r.Post("/upload", h.handleUpload)
		r.Post("/analyze", h.handleAnalyze)
		r.Post("/connect-source", h.handleConnectSource)
		r.Get("/export/cleaned-csv", h.handleExportCsv)
		r.Get("/pinned-charts", h.handleGetPinnedCharts)
		r.Post("/pin-chart", h.handlePinChart)
		r.Delete("/unpin-chart", h.handleUnpinChart)
		r.Get("/connections", h.handleConnectionList)
		r.Post("/connections/test", h.handleConnectionTest)
		r.Post("/connections", h.handleConnectionCreate)
		r.Delete("/connections", h.handleConnectionDelete)
		r.Get("/python-plot", h.handlePythonPlot)
		r.Post("/refresh-dataset", h.handleRefreshDataset)
		r.Get("/dashboards", h.handleListDashboards)
		r.Post("/dashboards", h.handleCreateDashboard)
		r.Put("/dashboards/{id}", h.handleRenameDashboard)
		r.Delete("/dashboards/{id}", h.handleDeleteDashboard)
		r.Post("/dashboards/{id}/charts", h.handleAddChartToDashboard)
		r.Delete("/dashboards/{id}/charts/{chartId}", h.handleRemoveChartFromDashboard)
		r.Post("/share", h.handleCreateShareLink)
		r.Get("/shared/{token}", h.handleGetSharedDashboard)
	})

	// Generated plots
	r.Get("/plots/{filename}", h.plotService.ServePlot)

	// _next/static/ with long-term caching
	staticDir := h.staticDir()
	if staticDir != "" {
		nextStaticDir := filepath.Join(staticDir, "_next", "static")
		if _, err := os.Stat(nextStaticDir); err == nil {
			fs := http.FileServer(http.Dir(nextStaticDir))
			r.Get("/_next/static/*", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
				http.StripPrefix("/_next/static/", fs).ServeHTTP(w, r)
			})
		}
	}

	// Build the static frontend handler (no http.FileServer for fallback paths
	// to avoid Go's automatic 301 redirects on path cleaning).
	var frontendHandler http.Handler
	if staticDir != "" {
		frontendHandler = h.serveFrontend(staticDir)
	}

	// Top-level dispatcher: chi for known prefixes, frontend for everything else.
	chiRoute := r
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/"),
			strings.HasPrefix(r.URL.Path, "/plots/"),
			strings.HasPrefix(r.URL.Path, "/_next/static/"):
			chiRoute.ServeHTTP(w, r)
		default:
			if frontendHandler != nil {
				frontendHandler.ServeHTTP(w, r)
			} else {
				w.WriteHeader(http.StatusNotFound)
			}
		}
	})
}

// serveFrontend serves the Next.js static export from dirPath.
// It looks up the exact file, tries a .html extension, or falls back to
// index.html for SPA client-side routing. We avoid http.FileServer for
// fallback paths to prevent Go's automatic 301 redirects.
func (h *Handler) serveFrontend(dirPath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Sanitize path: clean and ensure it starts with /
		path := filepath.Clean(r.URL.Path)
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}

		// 1. Try exact file match
		if content, contentType, ok := readFile(dirPath, path); ok {
			w.Header().Set("Content-Type", contentType)
			w.Write(content)
			return
		}

		// 2. Try with .html extension (pre-rendered pages)
		if !strings.HasSuffix(path, ".html") {
			htmlPath := path + ".html"
			if content, contentType, ok := readFile(dirPath, htmlPath); ok {
				w.Header().Set("Content-Type", contentType)
				w.Write(content)
				return
			}
		}

		// 3. Fall back to index.html (SPA client-side routing)
		if content, _, ok := readFile(dirPath, "/index.html"); ok {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(content)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	})
}

// readFile reads a file from dirPath+relPath. Returns content, content type,
// and true if the file exists and is not a directory.
func readFile(dirPath, relPath string) ([]byte, string, bool) {
	// Prevent path traversal
	if strings.Contains(relPath, "..") {
		return nil, "", false
	}

	fullPath := filepath.Join(dirPath, relPath)
	info, err := os.Stat(fullPath)
	if err != nil || info.IsDir() {
		return nil, "", false
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, "", false
	}

	ext := strings.ToLower(filepath.Ext(relPath))
	ct := contentTypeByExtension(ext)
	return content, ct, true
}

// contentTypeByExtension returns the MIME type for common static file extensions.
func contentTypeByExtension(ext string) string {
	switch ext {
	case ".html", ".htm":
		return "text/html; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".svg":
		return "image/svg+xml"
	case ".ico":
		return "image/x-icon"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".eot":
		return "application/vnd.ms-fontobject"
	case ".txt":
		return "text/plain; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}

// staticDir returns the path to the built Next.js frontend output.
func (h *Handler) staticDir() string {
	if dir := os.Getenv("STATIC_DIR"); dir != "" {
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
	}
	if root, err := projectRoot(); err == nil {
		dir := filepath.Join(root, "frontend", "out")
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
	}
	if dir, err := os.Getwd(); err == nil {
		dir = filepath.Join(dir, "frontend", "out")
		if _, err := os.Stat(dir); err == nil {
			return dir
		}
	}
	return ""
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
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
