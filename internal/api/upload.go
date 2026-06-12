package api

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"insightpilot/internal/data"
)

// UploadConfig holds dependencies for the upload handler.
type UploadConfig struct {
	UploadDir string
}

// HandleUpload processes file uploads (CSV/JSON), parses them, stores on disk,
// and registers the dataset in the provided datasets map.
func HandleUpload(w http.ResponseWriter, r *http.Request, cfg UploadConfig, datasets map[string]*data.Dataset, mu *sync.RWMutex) {
	const maxFileSize = 10 << 20 // 10 MB

	if err := r.ParseMultipartForm(maxFileSize); err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Could not parse multipart form (max 10MB)"})
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "File is required (field name 'file')"})
		return
	}
	defer file.Close()

	if header.Size > maxFileSize {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "File exceeds 10MB limit"})
		return
	}

	ct := header.Header.Get("Content-Type")
	allowedTypes := map[string]bool{
		"text/csv": true, "application/csv": true, "application/json": true,
		"text/plain": true, "application/octet-stream": true,
	}
	if ct != "" && !allowedTypes[ct] {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Unsupported file type: " + ct + ". Only CSV and JSON are allowed."})
		return
	}

	safeName := sanitizeFilename(header.Filename)
	ext := strings.ToLower(filepath.Ext(safeName))
	if ext != ".csv" && ext != ".json" {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "File must have .csv or .json extension"})
		return
	}

	content, err := io.ReadAll(io.LimitReader(file, maxFileSize+1))
	if err != nil {
		SendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to read file"})
		return
	}
	if int64(len(content)) > maxFileSize {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "File exceeds 10MB limit"})
		return
	}

	if err := os.MkdirAll(cfg.UploadDir, 0755); err != nil {
		SendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to create upload directory"})
		return
	}

	id := fmt.Sprintf("%d", time.Now().UnixNano())
	filePath := filepath.Join(cfg.UploadDir, fmt.Sprintf("%s-%s", id, safeName))
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		SendJSON(w, http.StatusInternalServerError, map[string]string{"error": "Failed to save file"})
		return
	}

	var rows [][]string
	text := string(content)
	if ext == ".json" {
		rows, err = data.ParseJSONRows(text)
	} else {
		rows, err = data.ParseCSV(text)
	}
	if err != nil {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "Parsing failed: " + err.Error()})
		return
	}
	if len(rows) < 2 {
		SendJSON(w, http.StatusBadRequest, map[string]string{"error": "File must have a header and at least one data row"})
		return
	}

	profile := data.ProfileRows(rows)

	rowObjects := make([]map[string]string, 0, len(rows)-1)
	headers := rows[0]
	for _, row := range rows[1:] {
		obj := make(map[string]string)
		for i, header := range headers {
			if i < len(row) {
				obj[header] = row[i]
			}
		}
		rowObjects = append(rowObjects, obj)
	}

	dataset := &data.Dataset{
		ID: id, Filename: safeName, FilePath: filePath,
		Profile: profile, Rows: rowObjects,
	}

	mu.Lock()
	datasets[id] = dataset
	mu.Unlock()

	SendJSON(w, http.StatusCreated, map[string]interface{}{
		"datasetId": id, "filename": safeName, "profile": profile,
	})
}

func sanitizeFilename(name string) string {
	name = strings.ReplaceAll(name, "\x00", "")
	name = strings.ReplaceAll(name, "/", "_")
	name = strings.ReplaceAll(name, `\`, "_")
	safe := strings.Builder{}
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			safe.WriteRune(r)
		}
	}
	name = safe.String()
	if strings.HasPrefix(name, ".") && filepath.Ext(name) == name {
		name = "upload" + name
	}
	name = strings.TrimLeft(name, ".")
	for strings.Contains(name, "..") {
		name = strings.ReplaceAll(name, "..", ".")
	}
	if name == "" {
		name = "upload"
	}
	return name
}
