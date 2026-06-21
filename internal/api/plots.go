package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"insightpilot/internal/data"
)

// PlotService handles Python plot generation, serving, and cleanup.
type PlotService struct {
	plotsDir     string
	uploadDir    string
	bridge       *PythonBridge
	stopCleanup  chan struct{}
}

// NewPlotService creates a new PlotService.
func NewPlotService(plotsDir, uploadDir string, bridge *PythonBridge) *PlotService {
	return &PlotService{
		plotsDir:    plotsDir,
		uploadDir:   uploadDir,
		bridge:      bridge,
		stopCleanup: make(chan struct{}),
	}
}

// GeneratePlot generates a Python visualization for the given dataset using the
// specified visualization library (matplotlib, bokeh, or plotly). It first tries
// LLM-driven code generation (if configured), then falls back to the deterministic
// template. Returns the URL path to the generated plot image, or empty string on failure.
// designJSON is an optional JSON string with visual design settings.
func (ps *PlotService) GeneratePlot(ds *data.Dataset, prompt, vizType, designJSON string) string {
	scriptID := "auto_" + newID()

	profileJSON, _ := json.Marshal(ds.Profile)

	// Try LLM-driven code generation first
	llmScript, err := ps.bridge.GeneratePlotScriptLLM(scriptID, prompt, string(profileJSON), vizType, designJSON)
	if err == nil && llmScript != "" {
		plotURL, execErr := ps.bridge.ExecuteScript(scriptID, llmScript, ds.FilePath, vizType, designJSON)
		if execErr == nil {
			return plotURL
		}
		fmt.Printf("LLM-driven plot execution failed for dataset %s: %v, falling back to template\n", ds.ID, execErr)
	} else if err != nil {
		fmt.Printf("LLM-driven plot generation failed for dataset %s: %v, falling back to template\n", ds.ID, err)
	}

	// Fallback to deterministic template
	scriptContent := ps.bridge.GeneratePlotScript(scriptID, "", vizType, designJSON)
	plotURL, err := ps.bridge.ExecuteScript(scriptID, scriptContent, ds.FilePath, vizType, designJSON)
	if err != nil {
		fmt.Printf("Deterministic plot generation failed for dataset %s: %v\n", ds.ID, err)
		return ""
	}
	return plotURL
}

// ServePlot serves a generated plot image or HTML file by filename.
func (ps *PlotService) ServePlot(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	name := filepath.Base(r.URL.Path)
	if name == "" || name == "." {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	plotPath := filepath.Join(ps.plotsDir, name)
	info, err := os.Stat(plotPath)
	if err != nil || info.IsDir() {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}

	ext := filepath.Ext(name)
	switch ext {
	case ".html":
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	default:
		w.Header().Set("Content-Type", "application/octet-stream")
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	http.ServeFile(w, r, plotPath)
}

// HandlePythonPlot handles the on-demand /api/python-plot endpoint.
// Accepts optional vizType query param (matplotlib, bokeh, plotly; defaults to matplotlib).
func (ps *PlotService) HandlePythonPlot(w http.ResponseWriter, r *http.Request, datasets map[string]*data.Dataset) {
	datasetID := r.URL.Query().Get("datasetId")
	prompt := r.URL.Query().Get("prompt")
	vizType := r.URL.Query().Get("vizType")
	if vizType == "" {
		vizType = VizTypeMatplotlib
	}
	// Build design JSON from query params
	designJSON := "" 
	accent := r.URL.Query().Get("accentColor")
	scheme := r.URL.Query().Get("chartScheme")
	fontFam := r.URL.Query().Get("fontFamily")
	fontSz := r.URL.Query().Get("fontSize")
	if accent != "" || scheme != "" || fontFam != "" || fontSz != "" {
		dc := DesignConfig{
			AccentColor: accent,
			ChartScheme: scheme,
			FontFamily:  fontFam,
			FontSize:    fontSz,
		}
		if b, err := json.Marshal(dc); err == nil {
			designJSON = string(b)
		}
	}

	if datasetID == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "datasetId query parameter required"})
		return
	}

	ds, ok := datasets[datasetID]
	if !ok {
		SendJSON(w, http.StatusNotFound, map[string]string{"error": "Dataset not found"})
		return
	}

	if ds.FilePath == "" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Dataset has no file path (in-memory only)"})
		return
	}

	plotURL := ps.GeneratePlot(ds, prompt, vizType, designJSON)
	if plotURL == "" {
		SendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to generate plot"})
		return
	}

	SendJSON(w, http.StatusOK, map[string]interface{}{
		"plotUrl": plotURL,
		"vizType": vizType,
	})
}

// StartCleanup starts the background goroutine that periodically removes
// stale plot artifacts. It also runs an initial cleanup immediately.
func (ps *PlotService) StartCleanup() {
	retention := 24 * time.Hour
	if raw := strings.TrimSpace(os.Getenv("PLOT_RETENTION_HOURS")); raw != "" {
		hours, err := strconv.Atoi(raw)
		if err != nil || hours < 0 {
			log.Printf("api: invalid PLOT_RETENTION_HOURS=%q, using %s", raw, retention)
		} else if hours == 0 {
			return
		} else {
			retention = time.Duration(hours) * time.Hour
		}
	}

	ps.cleanup(retention)

	ticker := time.NewTicker(time.Hour)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				ps.cleanup(retention)
			case <-ps.stopCleanup:
				return
			}
		}
	}()
}

// StopCleanup stops the background cleanup goroutine.
func (ps *PlotService) StopCleanup() {
	close(ps.stopCleanup)
}

func (ps *PlotService) cleanup(retention time.Duration) {
	if removed, err := ps.bridge.CleanupOlderThan(retention); err != nil {
		log.Printf("api: plot cleanup failed for %s: %v", ps.plotsDir, err)
	} else if removed > 0 {
		log.Printf("api: removed %d stale plot files from %s", removed, ps.plotsDir)
	}

	// Also clean up legacy plots directory if it exists
	root, err := projectRoot()
	if err != nil {
		return
	}
	legacyPlotsDir := filepath.Join(root, "internal", "api", "uploads", "plots")
	if legacyPlotsDir == ps.plotsDir {
		return
	}
	if info, err := os.Stat(legacyPlotsDir); err != nil || !info.IsDir() {
		return
	}
	legacyBridge := NewPythonBridge(legacyPlotsDir)
	if removed, err := legacyBridge.CleanupOlderThan(retention); err != nil {
		log.Printf("api: legacy plot cleanup failed for %s: %v", legacyPlotsDir, err)
	} else if removed > 0 {
		log.Printf("api: removed %d stale legacy plot files from %s", removed, legacyPlotsDir)
	}
}
