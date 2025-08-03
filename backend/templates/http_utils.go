package templates

import (
	"fmt"
	"html/template"
	"net/http"
	"pvmss/security"
	"pvmss/state"
)

// csrfToken generates a CSRF token input field for forms
func csrfToken(r *http.Request) template.HTML {
	stateManager := state.GetGlobalState()
	if stateManager == nil {
		return ""
	}

	sessionManager := stateManager.GetSessionManager()
	if sessionManager == nil {
		return ""
	}

	token, ok := sessionManager.Get(r.Context(), "csrf_token").(string)
	if !ok || token == "" {
		// Generate a new token if one doesn't exist
		newToken, err := security.GenerateCSRFToken()
		if err == nil {
			sessionManager.Put(r.Context(), "csrf_token", newToken)
			token = newToken
		}
	}

	return template.HTML(fmt.Sprintf(`<input type="hidden" name="csrf_token" value="%s">`, token))
}

// csrfMeta generates a CSRF meta tag for JavaScript
func csrfMeta(r *http.Request) template.HTML {
	stateManager := state.GetGlobalState()
	if stateManager == nil {
		return ""
	}

	sessionManager := stateManager.GetSessionManager()
	if sessionManager == nil {
		return ""
	}

	token, ok := sessionManager.Get(r.Context(), "csrf_token").(string)
	if !ok || token == "" {
		// Generate a new token if one doesn't exist
		newToken, err := security.GenerateCSRFToken()
		if err == nil {
			sessionManager.Put(r.Context(), "csrf_token", newToken)
			token = newToken
		}
	}

	return template.HTML(fmt.Sprintf(`<meta name="csrf-token" content="%s">`, token))
}

// isHTTPS checks if the request is using HTTPS
func isHTTPS(r *http.Request) bool {
	return r.TLS != nil
}

// getHost returns the host from the request
func getHost(r *http.Request) string {
	return r.Host
}
