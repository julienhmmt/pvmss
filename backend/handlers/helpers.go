package handlers

import (
	"encoding/json"
	"net/http"
)

// RenderErrorPage writes a JSON error response with the given status code.
// Used by the legacy router fallback (404/405) and panic-recovery middleware
// in this package. The /api/v1 stack writes its own error responses via
// writeAppError instead.
func RenderErrorPage(w http.ResponseWriter, _ *http.Request, status int, message string) {
	setNoCacheHeaders(w)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(JSONErrorResponse{
		Code:    message,
		Message: message,
	})
}
