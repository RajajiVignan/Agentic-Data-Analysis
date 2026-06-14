package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSendErrorFormat(t *testing.T) {
	w := httptest.NewRecorder()
	sendError(w, http.StatusBadRequest, ErrInvalidRequest, "test error message")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Fatalf("Content-Type = %q, want application/json", ct)
	}

	var body APIError
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Code != ErrInvalidRequest {
		t.Fatalf("code = %q, want %q", body.Code, ErrInvalidRequest)
	}
	if body.Message != "test error message" {
		t.Fatalf("message = %q, want %q", body.Message, "test error message")
	}
}

func TestSendInvalidRequest(t *testing.T) {
	w := httptest.NewRecorder()
	sendInvalidRequest(w, "bad input")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusBadRequest)
	}
	var body APIError
	json.Unmarshal(w.Body.Bytes(), &body)
	if body.Code != ErrInvalidRequest {
		t.Fatalf("code = %q, want %q", body.Code, ErrInvalidRequest)
	}
}

func TestSendNotFound(t *testing.T) {
	w := httptest.NewRecorder()
	sendNotFound(w, "not found")

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusNotFound)
	}
}

func TestSendUnauthorized(t *testing.T) {
	w := httptest.NewRecorder()
	sendUnauthorized(w, "unauthorized")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestSendConflict(t *testing.T) {
	w := httptest.NewRecorder()
	sendConflict(w, "conflict")

	if w.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusConflict)
	}
}

func TestSendInternalError(t *testing.T) {
	w := httptest.NewRecorder()
	sendInternalError(w, "internal error")

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusInternalServerError)
	}
}
