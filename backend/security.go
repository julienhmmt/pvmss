package main

import (
	"html"
	"net/http"
	"strings"
	"sync"
	"time"
	
	"pvmss/logger"
)

// CSRF protection
var (
	csrfTokens = make(map[string]time.Time)
	csrfMutex  = sync.RWMutex{}
	csrfTTL    = 30 * time.Minute
)

// validateCSRFToken validates a CSRF token and removes it
func validateCSRFToken(token string) bool {
	if token == "" {
		return false
	}
	
	csrfMutex.Lock()
	defer csrfMutex.Unlock()
	
	expiry, exists := csrfTokens[token]
	if !exists {
		return false
	}
	
	// Check if expired
	if time.Now().After(expiry) {
		delete(csrfTokens, token)
		return false
	}
	
	// Remove token after use (single use)
	delete(csrfTokens, token)
	return true
}

// cleanExpiredTokens removes expired CSRF tokens
func cleanExpiredTokens() {
	csrfMutex.Lock()
	defer csrfMutex.Unlock()
	
	now := time.Now()
	for token, expiry := range csrfTokens {
		if now.After(expiry) {
			delete(csrfTokens, token)
		}
	}
}

// Input validation functions
func validateInput(input string, maxLength int) string {
	// Remove any HTML tags and limit length
	cleaned := html.EscapeString(strings.TrimSpace(input))
	if len(cleaned) > maxLength {
		cleaned = cleaned[:maxLength]
	}
	return cleaned
}

// CSRF middleware
func csrfMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip CSRF for GET requests, health checks, and static files
		if r.Method == "GET" || r.URL.Path == "/health" || 
			strings.HasPrefix(r.URL.Path, "/css/") || 
			strings.HasPrefix(r.URL.Path, "/js/") || 
			strings.HasPrefix(r.URL.Path, "/static/") {
			next.ServeHTTP(w, r)
			return
		}
		
		// Skip CSRF for API GET requests
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Method == "GET" {
			next.ServeHTTP(w, r)
			return
		}
		
		token := r.FormValue("csrf_token")
		if token == "" {
			token = r.Header.Get("X-CSRF-Token")
		}
		
		if !validateCSRFToken(token) {
			logger.Get().Warn().
				Str("ip", r.RemoteAddr).
				Str("path", r.URL.Path).
				Msg("CSRF token validation failed")
			http.Error(w, "Invalid CSRF token", http.StatusForbidden)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// Security headers middleware
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		
		// Content Security Policy
		csp := "default-src 'self'; " +
			"script-src 'self' 'unsafe-inline'; " +
			"style-src 'self' 'unsafe-inline'; " +
			"img-src 'self' data:; " +
			"font-src 'self'; " +
			"connect-src 'self'"
		w.Header().Set("Content-Security-Policy", csp)
		
		next.ServeHTTP(w, r)
	})
}
