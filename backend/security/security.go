// Package security provides security utilities and middleware for the PVMSS application
package security

import (
	"crypto/rand"
	"encoding/hex"
	"html"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
	"pvmss/logger"
)

// CSRF protection
var (
	csrfTokens = make(map[string]time.Time)
	csrfMutex  = sync.RWMutex{}
	csrfTTL    = 30 * time.Minute
)

// Rate limiting
var (
	loginAttempts = make(map[string][]time.Time)
	rateMutex     = sync.RWMutex{}
	maxAttempts   = 5
	lockoutPeriod = 15 * time.Minute
)

// GenerateCSRFToken generates a new CSRF token
func GenerateCSRFToken(r *http.Request) string {
	// Generate random token
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to generate CSRF token")
		return ""
	}

	token := hex.EncodeToString(bytes)

	csrfMutex.Lock()
	defer csrfMutex.Unlock()

	// Store token with expiry
	csrfTokens[token] = time.Now().Add(csrfTTL)

	// Clean expired tokens periodically
	go cleanExpiredTokens()

	return token
}

// ValidateCSRFToken validates a CSRF token and removes it
func ValidateCSRFToken(r *http.Request) bool {
	token := r.FormValue("csrf_token")
	if token == "" {
		token = r.Header.Get("X-CSRF-Token")
	}

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

// ValidateInput validates and sanitizes input
func ValidateInput(input string, maxLength int) string {
	// Remove any HTML tags and limit length
	cleaned := html.EscapeString(strings.TrimSpace(input))
	if len(cleaned) > maxLength {
		cleaned = cleaned[:maxLength]
	}
	return cleaned
}

// CheckRateLimit checks if an IP has exceeded rate limits
func CheckRateLimit(ip string) bool {
	rateMutex.Lock()
	defer rateMutex.Unlock()

	now := time.Now()

	// Clean old attempts
	if attempts, exists := loginAttempts[ip]; exists {
		var validAttempts []time.Time
		for _, attempt := range attempts {
			if now.Sub(attempt) < lockoutPeriod {
				validAttempts = append(validAttempts, attempt)
			}
		}
		loginAttempts[ip] = validAttempts
	}

	// Check if limit exceeded
	if len(loginAttempts[ip]) >= maxAttempts {
		return false
	}

	return true
}

// RecordLoginAttempt records a login attempt for rate limiting
func RecordLoginAttempt(ip string) {
	rateMutex.Lock()
	defer rateMutex.Unlock()

	if loginAttempts[ip] == nil {
		loginAttempts[ip] = make([]time.Time, 0)
	}

	loginAttempts[ip] = append(loginAttempts[ip], time.Now())
}

// HashPassword hashes a password using bcrypt
func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

// CheckPasswordHash verifies a password against its hash
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// CSRFMiddleware provides CSRF protection
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip CSRF for GET requests, health checks, and static files
		if r.Method == "GET" ||
			r.URL.Path == "/health" ||
			strings.HasPrefix(r.URL.Path, "/css/") ||
			strings.HasPrefix(r.URL.Path, "/js/") ||
			strings.HasPrefix(r.URL.Path, "/static/") ||
			strings.HasPrefix(r.URL.Path, "/favicon") {
			next.ServeHTTP(w, r)
			return
		}

		// Skip CSRF for API GET requests
		if strings.HasPrefix(r.URL.Path, "/api/") && r.Method == "GET" {
			next.ServeHTTP(w, r)
			return
		}

		if !ValidateCSRFToken(r) {
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

// HeadersMiddleware adds security headers
func HeadersMiddleware(next http.Handler) http.Handler {
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

// AuthMiddleware checks for authentication
func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// This would typically check session or JWT token
		// For now, just pass through - implement as needed
		next.ServeHTTP(w, r)
	})
}
