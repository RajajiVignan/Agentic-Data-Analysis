package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"insightpilot/internal/agent"
)

func TestConnectionTestMySQLUnsupported(t *testing.T) {
	handler := NewHandler(agent.DefaultConfig())
	mux := handler.Routes()

	body := `{"provider":"mysql","host":"localhost","database":"testdb","username":"root","password":"secret"}`
	req := httptest.NewRequest(http.MethodPost, "/api/connections/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	t.Logf("Status: %d", rec.Code)
	t.Logf("Body: %s", rec.Body.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json parse error: %v, body=%s", err, rec.Body.String())
	}
	// MySQL is now rejected with an error since we don't have a MySQL driver
	if resp["ok"] != false {
		t.Fatalf("ok = %v, want false (MySQL not supported without driver)", resp["ok"])
	}
	if resp["error"] == nil {
		t.Fatal("expected error message for unsupported MySQL")
	}
}

func TestConnectionTestMySQLMissingHost(t *testing.T) {
	handler := NewHandler(agent.DefaultConfig())
	mux := handler.Routes()

	body := `{"provider":"mysql","database":"testdb"}`
	req := httptest.NewRequest(http.MethodPost, "/api/connections/test", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	t.Logf("Status: %d", rec.Code)
	t.Logf("Body: %s", rec.Body.String())

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json parse error: %v, body=%s", err, rec.Body.String())
	}
	if resp["ok"] != false {
		t.Fatalf("ok = %v, want false", resp["ok"])
	}
	if resp["error"] == nil {
		t.Fatal("expected error message")
	}
}
