package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"insightpilot/internal/agent"
)

func TestConnectionTestMySQL(t *testing.T) {
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
	if resp["ok"] != true {
		t.Fatalf("ok = %v, want true", resp["ok"])
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
