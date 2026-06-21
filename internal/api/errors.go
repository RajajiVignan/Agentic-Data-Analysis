package api

import (
	"net/http"
)

// ErrorCode represents a structured API error code.
type ErrorCode string

const (
	ErrInvalidRequest ErrorCode = "INVALID_REQUEST"
	ErrNotFound       ErrorCode = "NOT_FOUND"
	ErrUnauthorized   ErrorCode = "UNAUTHORIZED"
	ErrForbidden      ErrorCode = "FORBIDDEN"
	ErrConflict       ErrorCode = "CONFLICT"
	ErrRateLimited    ErrorCode = "RATE_LIMITED"
	ErrInternal       ErrorCode = "INTERNAL_ERROR"
	ErrValidation     ErrorCode = "VALIDATION_ERROR"
	ErrUploadFailed   ErrorCode = "UPLOAD_FAILED"
	ErrAnalysisFailed ErrorCode = "ANALYSIS_FAILED"
)

// APIError is a structured error response.
type APIError struct {
	Code    ErrorCode `json:"code"`
	Message string    `json:"message"`
}

func sendError(w http.ResponseWriter, status int, code ErrorCode, message string) {
	SendJSON(w, status, APIError{Code: code, Message: message})
}

func sendInvalidRequest(w http.ResponseWriter, message string) {
	sendError(w, http.StatusBadRequest, ErrInvalidRequest, message)
}

func sendNotFound(w http.ResponseWriter, message string) {
	sendError(w, http.StatusNotFound, ErrNotFound, message)
}

func sendUnauthorized(w http.ResponseWriter, message string) {
	sendError(w, http.StatusUnauthorized, ErrUnauthorized, message)
}

func sendConflict(w http.ResponseWriter, message string) {
	sendError(w, http.StatusConflict, ErrConflict, message)
}

func sendInternalError(w http.ResponseWriter, message string) {
	sendError(w, http.StatusInternalServerError, ErrInternal, message)
}
