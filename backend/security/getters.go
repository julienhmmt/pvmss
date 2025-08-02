package security

import (
	"net/http"
)

// GetCSRFToken retrieves the CSRF token from the request context.
// It returns an empty string if the token is not found.
func GetCSRFToken(r *http.Request) string {
	if token, ok := r.Context().Value(CSRFTokenContextKey).(string); ok {
		return token
	}
	return ""
}
