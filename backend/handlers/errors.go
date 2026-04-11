package handlers

import (
	"encoding/json"
	"net/http"
	"pvmss/logger"
)

// ErrorResponse represents a standardized error response
type ErrorResponse struct {
	Code    int
	Message string
	Key     string
}

// JSONErrorResponse represents the JSON error response format for API clients
type JSONErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Common error responses (error codes for frontend translation)
var (
	ErrUnauthorized = ErrorResponse{
		Code:    http.StatusUnauthorized,
		Key:     "UNAUTHORIZED",
		Message: "Unauthorized",
	}
	ErrForbidden = ErrorResponse{
		Code:    http.StatusForbidden,
		Key:     "FORBIDDEN",
		Message: "Access denied",
	}
	ErrNotFound = ErrorResponse{
		Code:    http.StatusNotFound,
		Key:     "NOT_FOUND",
		Message: "Resource not found",
	}
	ErrBadRequest = ErrorResponse{
		Code:    http.StatusBadRequest,
		Key:     "BAD_REQUEST",
		Message: "Invalid request",
	}
	ErrInternalServer = ErrorResponse{
		Code:    http.StatusInternalServerError,
		Key:     "INTERNAL_SERVER_ERROR",
		Message: "Internal server error",
	}
	ErrServiceUnavailable = ErrorResponse{
		Code:    http.StatusServiceUnavailable,
		Key:     "SERVICE_UNAVAILABLE",
		Message: "Service temporarily unavailable",
	}
	ErrMethodNotAllowed = ErrorResponse{
		Code:    http.StatusMethodNotAllowed,
		Key:     "METHOD_NOT_ALLOWED",
		Message: "Method not allowed",
	}
	ErrProxmoxConnection = ErrorResponse{
		Code:    http.StatusServiceUnavailable,
		Key:     "PROXMOX_CONNECTION_ERROR",
		Message: "Unable to connect to Proxmox server",
	}
	ErrSessionExpired = ErrorResponse{
		Code:    http.StatusUnauthorized,
		Key:     "SESSION_EXPIRED",
		Message: "Session expired, please log in again",
	}
	ErrInvalidCredentials = ErrorResponse{
		Code:    http.StatusUnauthorized,
		Key:     "INVALID_CREDENTIALS",
		Message: "Invalid credentials",
	}
	ErrCSRFValidation = ErrorResponse{
		Code:    http.StatusBadRequest,
		Key:     "CSRF_VALIDATION_FAILED",
		Message: "Invalid request. Please try again.",
	}
)

// RespondWithError sends a standardized error response with error code as JSON
func RespondWithError(w http.ResponseWriter, r *http.Request, errResp ErrorResponse) {
	logger.Get().Warn().
		Int("status_code", errResp.Code).
		Str("error_key", errResp.Key).
		Str("path", r.URL.Path).
		Msg("Error response sent")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(errResp.Code)
	_ = json.NewEncoder(w).Encode(JSONErrorResponse{
		Code:    errResp.Key,
		Message: errResp.Message,
	})
}

// RespondWithErrorAndLog sends error response and logs with additional context as JSON
func RespondWithErrorAndLog(w http.ResponseWriter, r *http.Request, errResp ErrorResponse, err error, context string) {
	logger.Get().Error().
		Err(err).
		Int("status_code", errResp.Code).
		Str("error_key", errResp.Key).
		Str("context", context).
		Str("path", r.URL.Path).
		Msg("Error occurred")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(errResp.Code)
	_ = json.NewEncoder(w).Encode(JSONErrorResponse{
		Code:    errResp.Key,
		Message: errResp.Message,
	})
}

// RespondWithCustomError sends a custom error message with error code as JSON
func RespondWithCustomError(w http.ResponseWriter, r *http.Request, statusCode int, errorCode string, fallbackMsg string) {
	logger.Get().Warn().
		Int("status_code", statusCode).
		Str("error_key", errorCode).
		Str("path", r.URL.Path).
		Msg("Custom error response sent")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(JSONErrorResponse{
		Code:    errorCode,
		Message: fallbackMsg,
	})
}

// RenderErrorPageWithI18n renders an error page (no i18n)
func RenderErrorPageWithI18n(w http.ResponseWriter, r *http.Request, statusCode int, errorCode string, fallbackMsg string) {
	RenderErrorPage(w, r, statusCode, errorCode)
}

// LocalizeError returns error code directly (no translation)
func LocalizeError(r *http.Request, key string) string {
	return key
}

// LocalizeErrorWithFallback returns error code or fallback
func LocalizeErrorWithFallback(r *http.Request, key string, fallback string) string {
	if key == "" {
		return fallback
	}
	return key
}

// ErrorHelper provides centralized error handling
type ErrorHelper struct {
	Writer  http.ResponseWriter
	Request *http.Request
}

// MakeErrorHelper creates a new error helper
func MakeErrorHelper(w http.ResponseWriter, r *http.Request) *ErrorHelper {
	return &ErrorHelper{
		Writer:  w,
		Request: r,
	}
}

// Send sends a standardized error response
func (e *ErrorHelper) Send(errResp ErrorResponse) {
	RespondWithError(e.Writer, e.Request, errResp)
}

// SendWithLog sends error response with logging
func (e *ErrorHelper) SendWithLog(errResp ErrorResponse, err error, context string) {
	RespondWithErrorAndLog(e.Writer, e.Request, errResp, err, context)
}

// SendCustom sends a custom error message
func (e *ErrorHelper) SendCustom(statusCode int, errorCode string, fallbackMsg string) {
	RespondWithCustomError(e.Writer, e.Request, statusCode, errorCode, fallbackMsg)
}

// RenderPage renders an error page
func (e *ErrorHelper) RenderPage(statusCode int, errorCode string, fallbackMsg string) {
	RenderErrorPageWithI18n(e.Writer, e.Request, statusCode, errorCode, fallbackMsg)
}
