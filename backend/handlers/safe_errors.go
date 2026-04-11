package handlers

import (
	"encoding/json"
	"net/http"

	"pvmss/logger"
)

// RespondWithSafeError returns an error code without exposing internal details as JSON
func RespondWithSafeError(w http.ResponseWriter, r *http.Request, statusCode int, errorCode string, internalErr error) {
	// Log the internal error for debugging
	if internalErr != nil {
		logger.Get().Error().Err(internalErr).Str("error_code", errorCode).Str("path", r.URL.Path).Msg("Internal error occurred")
	}

	// Return error code to client as JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(JSONErrorResponse{
		Code:    errorCode,
		Message: "An error occurred",
	})
}
