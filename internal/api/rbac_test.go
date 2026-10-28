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

func doJSON(t *testing.T, mux http.Handler, token, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	var rdr io.Reader
	if body != "" {
		rdr = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, rdr)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	return rec
}

func registerUser(t *testing.T, mux http.Handler, email string) (User, string) {
	t.Helper()
	body := `{"email":"` + email + `","password":"secret123","name":"Test"}`
	rec := doJSON(t, mux, "", http.MethodPost, "/api/auth/register", body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register %s status = %d, body = %s", email, rec.Code, rec.Body.String())
	}
	var resp struct {
		User  User   `json:"user"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp.User, resp.Token
}

func uploadCSV(t *testing.T, mux http.Handler, token, filename, content string) string {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(part, content); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/upload", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("upload status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		DatasetID string `json:"datasetId"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	return resp.DatasetID
}

func TestProtectedRouteRequiresAuth(t *testing.T) {
	h := NewHandler(agent.DefaultConfig())
	mux := h.Routes()
	rec := doJSON(t, mux, "", http.MethodGet, "/api/datasets", "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestFirstUserIsAdminThenEditor(t *testing.T) {
	h := NewHandler(agent.DefaultConfig())
	mux := h.Routes()
	first, _ := registerUser(t, mux, "admin@example.com")
	if first.Role != RoleAdmin {
		t.Fatalf("first user role = %q, want admin", first.Role)
	}
	second, _ := registerUser(t, mux, "editor@example.com")
	if second.Role != RoleEditor {
		t.Fatalf("second user role = %q, want editor", second.Role)
	}
}

func TestGuestIsViewerAndCannotUpload(t *testing.T) {
	h := NewHandler(agent.DefaultConfig())
	mux := h.Routes()
	rec := doJSON(t, mux, "", http.MethodPost, "/api/auth/guest", "")
	if rec.Code != http.StatusCreated {
		t.Fatalf("guest login status = %d", rec.Code)
	}
	var resp struct {
		User  User   `json:"user"`
		Token string `json:"token"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatal(err)
	}
	if resp.User.Role != RoleViewer {
		t.Fatalf("guest role = %q, want viewer", resp.User.Role)
	}
	up := doJSON(t, mux, resp.Token, http.MethodPost, "/api/upload", "")
	if up.Code != http.StatusForbidden {
		t.Fatalf("guest upload status = %d, want 403", up.Code)
	}
}

func TestDatasetsAreIsolatedBetweenUsers(t *testing.T) {
	h := NewHandler(agent.DefaultConfig())
	mux := h.Routes()
	_, tokenA := registerUser(t, mux, "alice@example.com")
	_, tokenB := registerUser(t, mux, "bob@example.com")

	dsID := uploadCSV(t, mux, tokenA, "revenue.csv", "month,revenue\n2026-01,100\n2026-02,200\n")

	// Alice can see it, Bob cannot.
	var aliceList struct {
		Datasets []map[string]interface{} `json:"datasets"`
	}
	rec := doJSON(t, mux, tokenA, http.MethodGet, "/api/datasets", "")
	_ = json.Unmarshal(rec.Body.Bytes(), &aliceList)
	if len(aliceList.Datasets) != 1 {
		t.Fatalf("alice datasets = %d, want 1", len(aliceList.Datasets))
	}

	var bobList struct {
		Datasets []map[string]interface{} `json:"datasets"`
	}
	rec = doJSON(t, mux, tokenB, http.MethodGet, "/api/datasets", "")
	_ = json.Unmarshal(rec.Body.Bytes(), &bobList)
	if len(bobList.Datasets) != 0 {
		t.Fatalf("bob datasets = %d, want 0", len(bobList.Datasets))
	}

	// Bob cannot analyze or export Alice's dataset.
	analyze := doJSON(t, mux, tokenB, http.MethodPost, "/api/analyze", `{"datasetId":"`+dsID+`","prompt":"total revenue"}`)
	if analyze.Code != http.StatusNotFound {
		t.Fatalf("cross-user analyze status = %d, want 404", analyze.Code)
	}
	exp := doJSON(t, mux, tokenB, http.MethodGet, "/api/export/cleaned-csv?datasetIds="+dsID, "")
	if exp.Code != http.StatusNotFound {
		t.Fatalf("cross-user export status = %d, want 404", exp.Code)
	}
}

func TestPinnedChartsAreIsolated(t *testing.T) {
	h := NewHandler(agent.DefaultConfig())
	mux := h.Routes()
	_, tokenA := registerUser(t, mux, "pin-a@example.com")
	_, tokenB := registerUser(t, mux, "pin-b@example.com")

	rec := doJSON(t, mux, tokenA, http.MethodPost, "/api/pin-chart", `{"chartType":"bar","label":"A chart","data":[]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("pin status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var pc PinnedChart
	if err := json.Unmarshal(rec.Body.Bytes(), &pc); err != nil {
		t.Fatal(err)
	}

	var bobCharts struct {
		Charts []PinnedChart `json:"pinnedCharts"`
	}
	rec = doJSON(t, mux, tokenB, http.MethodGet, "/api/pinned-charts", "")
	_ = json.Unmarshal(rec.Body.Bytes(), &bobCharts)
	if len(bobCharts.Charts) != 0 {
		t.Fatalf("bob charts = %d, want 0", len(bobCharts.Charts))
	}

	// Bob cannot unpin Alice's chart.
	rec = doJSON(t, mux, tokenB, http.MethodDelete, "/api/unpin-chart?id="+pc.ID, "")
	if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusNotFound {
		t.Fatalf("cross-user unpin status = %d, want error", rec.Code)
	}
}
