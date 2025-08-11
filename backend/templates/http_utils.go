package templates

import (
	"fmt"
	"html/template"
	"net/http"
	"pvmss/security"
)

// csrfToken generates a CSRF token input field for forms
func csrfToken(r *http.Request) template.HTML {
	sessionManager := security.GetSession(r)
	if sessionManager == nil {
		return ""
	}

	// Read-only: do not create/generate tokens in template helpers.
	// CSRF token should be prepared by middleware and available in session/context.
	token, _ := sessionManager.Get(r.Context(), "csrf_token").(string)
	if token == "" {
		return ""
	}

	return template.HTML(fmt.Sprintf(`<input type="hidden" name="csrf_token" value="%s">`, token))
}

// csrfMeta generates a CSRF meta tag for JavaScript
func csrfMeta(r *http.Request) template.HTML {
	sessionManager := security.GetSession(r)
	if sessionManager == nil {
		return ""
	}

	// Read-only: do not create/generate tokens in template helpers.
	token, _ := sessionManager.Get(r.Context(), "csrf_token").(string)
	if token == "" {
		return ""
	}

	return template.HTML(fmt.Sprintf(`<meta name="csrf-token" content="%s">`, token))
}

// isHTTPS checks if the request is using HTTPS
func isHTTPS(r *http.Request) bool {
	if r == nil {
		return false
	}
	if r.TLS != nil {
		return true
	}
	// Proxy-aware checks
	if proto := r.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto == "https"
	}
	if ssl := r.Header.Get("X-Forwarded-Ssl"); ssl != "" {
		return ssl == "on" || ssl == "1"
	}
	return false
}

// getHost returns the host from the request
func getHost(r *http.Request) string {
	if r == nil {
		return ""
	}
	if h := r.Header.Get("X-Forwarded-Host"); h != "" {
		return h
	}
	return r.Host
}
