package api

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"insightpilot/internal/agent"
)

func TestHealthReturnsOK(t *testing.T) {
	handler := NewHandler(agent.DefaultConfig())
	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	handler.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["ok"] != true {
		t.Fatalf("ok = %v, want true", body["ok"])
	}
}

func TestDatasetsReturnsEmptyArray(t *testing.T) {
	handler := NewHandler(agent.DefaultConfig())
	req := httptest.NewRequest(http.MethodGet, "/api/datasets", nil)
	rec := httptest.NewRecorder()

	handler.Routes().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if !strings.Contains(rec.Body.String(), `"datasets":[]`) {
		t.Fatalf("body = %s, want empty datasets array", rec.Body.String())
	}
}

func TestUploadAnalyzeAndExportRevenueCSV(t *testing.T) {
	handler := NewHandler(agent.DefaultConfig())
	mux := handler.Routes()

	uploadBody, contentType := multipartBody(t, "file", "revenue.csv", strings.NewReader(`month,segment,revenue,customers,churn_risk
2026-01,Enterprise,124000,42,3.2
2026-01,Mid-market,86000,118,5.1
2026-02,Enterprise,138500,45,2.8
`))
	uploadReq := httptest.NewRequest(http.MethodPost, "/api/upload", uploadBody)
	uploadReq.Header.Set("Content-Type", contentType)
	uploadRec := httptest.NewRecorder()

	mux.ServeHTTP(uploadRec, uploadReq)

	if uploadRec.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, body = %s", uploadRec.Code, uploadRec.Body.String())
	}
	var uploadResp struct {
		DatasetID string `json:"datasetId"`
	}
	if err := json.Unmarshal(uploadRec.Body.Bytes(), &uploadResp); err != nil {
		t.Fatal(err)
	}
	if uploadResp.DatasetID == "" {
		t.Fatal("expected dataset ID")
	}

	analyzeReq := httptest.NewRequest(http.MethodPost, "/api/analyze", strings.NewReader(`{"datasetId":"`+uploadResp.DatasetID+`","prompt":"What is total revenue by segment?"}`))
	analyzeReq.Header.Set("Content-Type", "application/json")
	analyzeRec := httptest.NewRecorder()

	mux.ServeHTTP(analyzeRec, analyzeReq)

	if analyzeRec.Code != http.StatusOK {
		t.Fatalf("analyze status = %d, body = %s", analyzeRec.Code, analyzeRec.Body.String())
	}
	if !strings.Contains(analyzeRec.Body.String(), `"trend":[`) {
		t.Fatalf("expected trend data, body = %s", analyzeRec.Body.String())
	}
	if !strings.Contains(analyzeRec.Body.String(), `"label":"Enterprise"`) {
		t.Fatalf("expected segment grouping, body = %s", analyzeRec.Body.String())
	}

	var analyzeResp struct {
		Dashboard struct {
			Segments []struct {
				Label string  `json:"label"`
				Value float64 `json:"value"`
			} `json:"segments"`
		} `json:"dashboard"`
	}
	if err := json.Unmarshal(analyzeRec.Body.Bytes(), &analyzeResp); err != nil {
		t.Fatal(err)
	}
	if len(analyzeResp.Dashboard.Segments) == 0 || analyzeResp.Dashboard.Segments[0].Label != "Enterprise" {
		t.Fatalf("analyze segments = %#v, want Enterprise first", analyzeResp.Dashboard.Segments)
	}

	exportReq := httptest.NewRequest(http.MethodGet, "/api/export/cleaned-csv?datasetIds="+uploadResp.DatasetID, nil)
	exportRec := httptest.NewRecorder()

	mux.ServeHTTP(exportRec, exportReq)

	if exportRec.Code != http.StatusOK {
		t.Fatalf("export status = %d, body = %s", exportRec.Code, exportRec.Body.String())
	}
	if got := exportRec.Header().Get("Content-Type"); !strings.Contains(got, "text/csv") {
		t.Fatalf("content-type = %q, want text/csv", got)
	}
	if !strings.Contains(exportRec.Body.String(), "source_dataset,month,segment,revenue,customers,churn_risk") {
		t.Fatalf("unexpected csv header: %s", exportRec.Body.String())
	}
	if !strings.Contains(exportRec.Body.String(), "revenue.csv,2026-01,Enterprise,124000,42,3.2") {
		t.Fatalf("unexpected csv rows: %s", exportRec.Body.String())
	}
}

func TestConnectSourceCanBeExported(t *testing.T) {
	handler := NewHandler(agent.DefaultConfig())
	mux := handler.Routes()

	connectReq := httptest.NewRequest(http.MethodPost, "/api/connect-source", strings.NewReader(`{"source":"Export Test"}`))
	connectReq.Header.Set("Content-Type", "application/json")
	connectRec := httptest.NewRecorder()

	mux.ServeHTTP(connectRec, connectReq)

	if connectRec.Code != http.StatusCreated {
		t.Fatalf("connect status = %d, body = %s", connectRec.Code, connectRec.Body.String())
	}

	exportReq := httptest.NewRequest(http.MethodGet, "/api/export/cleaned-csv?datasetIds=sample-id-123", nil)
	exportRec := httptest.NewRecorder()

	mux.ServeHTTP(exportRec, exportReq)

	if exportRec.Code != http.StatusOK {
		t.Fatalf("export status = %d, body = %s", exportRec.Code, exportRec.Body.String())
	}
	if !strings.Contains(exportRec.Body.String(), "export_test_sample,2026-01,Enterprise,124000,42,3.2") {
		t.Fatalf("unexpected connected-source csv: %s", exportRec.Body.String())
	}
}

func multipartBody(t *testing.T, fieldName, filename string, content io.Reader) (*bytes.Buffer, string) {
	t.Helper()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.Copy(part, content); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return body, writer.FormDataContentType()
}
