package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"insightpilot/internal/agent"
)

func TestShareLinkIsCreatedAndPubliclyReadable(t *testing.T) {
	h := NewHandler(agentConfigForTests())
	mux := h.Routes()
	_, tokenA := registerUser(t, mux, "share-a@example.com")
	_, tokenB := registerUser(t, mux, "share-b@example.com")

	rec := doJSON(t, mux, tokenA, http.MethodPost, "/api/pin-chart", `{"chartType":"bar","label":"Shared chart","data":[]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("pin status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var chart PinnedChart
	if err := json.Unmarshal(rec.Body.Bytes(), &chart); err != nil {
		t.Fatal(err)
	}

	// Another user cannot share a chart they do not own.
	rec = doJSON(t, mux, tokenB, http.MethodPost, "/api/share", `{"chartIds":["`+chart.ID+`"]}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("cross-user share status = %d, want 400", rec.Code)
	}

	rec = doJSON(t, mux, tokenA, http.MethodPost, "/api/share", `{"chartIds":["`+chart.ID+`"]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("share status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var shared SharedDashboard
	if err := json.Unmarshal(rec.Body.Bytes(), &shared); err != nil {
		t.Fatal(err)
	}
	if shared.Token == "" || shared.URL == "" {
		t.Fatalf("share response missing token/url: %#v", shared)
	}

	// Shared links are public (no auth header).
	rec = doJSON(t, mux, "", http.MethodGet, "/api/shared/"+shared.Token, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("shared read status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var got SharedDashboard
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Charts) != 1 || got.Charts[0].Label != "Shared chart" {
		t.Fatalf("shared charts = %#v", got.Charts)
	}
}

func TestTransformPipelineReplaysFromBase(t *testing.T) {
	h := NewHandler(agentConfigForTests())
	mux := h.Routes()
	_, token := registerUser(t, mux, "transform@example.com")
	dsID := uploadCSV(t, mux, token, "sales.csv", "region,revenue,cost\nNorth,100,40\nNorth,200,80\nSouth,150,60\n")

	apply := func(step string) map[string]any {
		rec := doJSON(t, mux, token, http.MethodPost, "/api/transform/apply", `{"datasetId":"`+dsID+`","step":`+step+`}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("apply status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var out map[string]any
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out
	}

	out := apply(`{"type":"filter","params":{"column":"region","operator":"eq","value":"North"}}`)
	if out["rowCount"].(float64) != 2 {
		t.Fatalf("filter rowCount = %v, want 2", out["rowCount"])
	}
	out = apply(`{"type":"derive","params":{"newColumn":"profit","expression":"revenue - cost"}}`)
	if out["rowCount"].(float64) != 2 {
		t.Fatalf("derive rowCount = %v, want 2", out["rowCount"])
	}

	rec := doJSON(t, mux, token, http.MethodPost, "/api/transform/undo", `{"datasetId":"`+dsID+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("undo status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var undoOut map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &undoOut); err != nil {
		t.Fatal(err)
	}
	if undoOut["rowCount"].(float64) != 2 {
		t.Fatalf("undo rowCount = %v, want 2 (filtered base)", undoOut["rowCount"])
	}
	if undoOut["canRedo"] != true {
		t.Fatalf("undo canRedo = %v, want true", undoOut["canRedo"])
	}

	// The reverted dataset must no longer contain the derived column.
	rec = doJSON(t, mux, token, http.MethodGet, "/api/export/cleaned-csv?datasetIds="+dsID, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("export status = %d", rec.Code)
	}
	if body := rec.Body.String(); !strings.Contains(body, "region,revenue,cost") || strings.Contains(body, "profit") {
		t.Fatalf("after undo export = %s", body)
	}

	rec = doJSON(t, mux, token, http.MethodPost, "/api/transform/redo", `{"datasetId":"`+dsID+`"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("redo status = %d, body = %s", rec.Code, rec.Body.String())
	}
}

func TestSessionIsolationAcrossUsers(t *testing.T) {
	h := NewHandler(agentConfigForTests())
	mux := h.Routes()
	_, tokenA := registerUser(t, mux, "sess-a@example.com")
	_, tokenB := registerUser(t, mux, "sess-b@example.com")

	dsA := uploadCSV(t, mux, tokenA, "a.csv", "month,revenue\n2026-01,100\n2026-02,200\n")
	dsB := uploadCSV(t, mux, tokenB, "b.csv", "month,revenue\n2026-01,300\n2026-02,400\n")

	analyze := func(token, dsID, sessionID string) string {
		body := `{"datasetId":"` + dsID + `","prompt":"total revenue"`
		if sessionID != "" {
			body += `,"sessionId":"` + sessionID + `"`
		}
		body += `}`
		rec := doJSON(t, mux, token, http.MethodPost, "/api/analyze", body)
		if rec.Code != http.StatusOK {
			t.Fatalf("analyze status = %d, body = %s", rec.Code, rec.Body.String())
		}
		var out struct {
			SessionID string `json:"sessionId"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
			t.Fatal(err)
		}
		return out.SessionID
	}

	sessionA := analyze(tokenA, dsA, "")
	if sessionA == "" {
		t.Fatal("expected a session id")
	}
	if again := analyze(tokenA, dsA, sessionA); again != sessionA {
		t.Fatalf("session not reused: got %q want %q", again, sessionA)
	}
	if other := analyze(tokenB, dsB, sessionA); other == sessionA {
		t.Fatalf("user B was able to continue user A's session")
	}
}

func agentConfigForTests() agent.Config { return agent.DefaultConfig() }
