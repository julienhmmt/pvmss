package handlers

import (
	"encoding/json"
	"net/http"

	"pvmss/logger"
)

// CreateHandlerLogger creates a standardized logger for handlers
func CreateHandlerLogger(handlerName string, r *http.Request) logger.Logger {
	logContext := logger.Get().With().Str("handler", handlerName)

	if r != nil {
		logContext = logContext.
			Str("method", r.Method).
			Str("path", r.URL.Path)
	}

	return logContext.Logger()
}

// RenderErrorPage writes a JSON error response with the given status code.
func RenderErrorPage(w http.ResponseWriter, r *http.Request, status int, message string) {
	setNoCacheHeaders(w)
	_ = r // r available for future use (e.g. content-type negotiation)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(JSONErrorResponse{
		Code:    message,
		Message: message,
	})
}
