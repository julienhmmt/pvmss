package templates

import (
	"fmt"
	"html/template"
	"net/http"
	"pvmss/security"
)

// getCsrfToken retrieves the CSRF token from the user's session.
// It is a read-only operation and does not generate a token.
func getCsrfToken(r *http.Request) string {
	sessionManager := security.GetSession(r)
	if sessionManager == nil {
		return ""
	}
	// CSRF token should be prepared by middleware and available in session/context.
	token, _ := sessionManager.Get(r.Context(), "csrf_token").(string)
	return token
}

// csrfToken generates a CSRF token input field for forms.
// The token value is HTML-escaped for security.
func csrfToken(r *http.Request) template.HTML {
	token := getCsrfToken(r)
	if token == "" {
		return ""
	}
	// Use template.HTMLEscapeString for proper escaping
	return template.HTML(fmt.Sprintf(`<input type="hidden" name="csrf_token" value="%s">`, template.HTMLEscapeString(token)))
}

// csrfMeta generates a CSRF meta tag for JavaScript.
// The token value is HTML-escaped for security.
func csrfMeta(r *http.Request) template.HTML {
	token := getCsrfToken(r)
	if token == "" {
		return ""
	}
	// Use template.HTMLEscapeString for proper escaping
	return template.HTML(fmt.Sprintf(`<meta name="csrf-token" content="%s">`, template.HTMLEscapeString(token)))
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
