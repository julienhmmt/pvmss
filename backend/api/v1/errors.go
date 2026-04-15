package apiv1

import (
	"encoding/json"
	"net/http"
)

// writeError writes a JSON error response with the given HTTP status code.
func writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Code: code, Message: message})
}

// writeJSON writes any value as a JSON response with HTTP 200.
func writeJSON(w http.ResponseWriter, v interface{}) {
	writeJSONStatus(w, http.StatusOK, v)
}

// writeJSONStatus writes any value as a JSON response with the given status code.
// Content-Type is set before WriteHeader to ensure headers are sent correctly.
func writeJSONStatus(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func errUnauthorized(w http.ResponseWriter) {
	writeError(w, http.StatusUnauthorized, "unauthorized", "Authentication required")
}
func errForbidden(w http.ResponseWriter) {
	writeError(w, http.StatusForbidden, "forbidden", "Insufficient permissions")
}
func errBadRequest(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusBadRequest, "bad_request", msg)
}
func errInternal(w http.ResponseWriter) {
	writeError(w, http.StatusInternalServerError, "internal_error", "Internal server error")
}
func errNotFound(w http.ResponseWriter, msg string) {
	writeError(w, http.StatusNotFound, "not_found", msg)
}
func errOffline(w http.ResponseWriter) {
	writeError(w, http.StatusServiceUnavailable, "proxmox_offline", "Proxmox is unavailable")
}
func errNotConfigured(w http.ResponseWriter) {
	writeError(w, http.StatusServiceUnavailable, "not_configured", "API auth is not configured (JWT_SECRET environment variable missing or too short)")
}
