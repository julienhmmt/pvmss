// Package security provides security utilities and middleware for the PVMSS application
package security

import (
	"html"
	"strings"
	"time"

	"pvmss/logger"
	"pvmss/state"

	"golang.org/x/crypto/bcrypt"
)

// Security constants
const (
	csrfTTL       = 30 * time.Minute
	maxAttempts   = 5
	lockoutPeriod = 15 * time.Minute
)

// CheckRateLimit checks if an IP has exceeded rate limits
func CheckRateLimit(ip string) bool {
	if ip == "" {
		return false
	}

	// Get state manager
	stateManager := state.GetGlobalState()
	if stateManager == nil {
		logger.Get().Error().Msg("State manager not initialized")
		return false
	}

	// Get login attempts
	attempts, err := stateManager.GetLoginAttempts(ip)
	if err != nil {
		logger.Get().Error().Err(err).Str("ip", ip).Msg("Failed to get login attempts")
		return false
	}

	// Count recent attempts
	recentAttempts := 0
	now := time.Now()
	for _, attempt := range attempts {
		if now.Sub(attempt) < lockoutPeriod {
			recentAttempts++
		}
	}

	return recentAttempts < maxAttempts
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

// RecordLoginAttempt records a login attempt for rate limiting
func RecordLoginAttempt(ip string) {
	if ip == "" {
		return
	}

	// Get state manager
	stateManager := state.GetGlobalState()
	if stateManager == nil {
		logger.Get().Error().Msg("State manager not initialized")
		return
	}

	// Record the attempt
	if err := stateManager.RecordLoginAttempt(ip, time.Now()); err != nil {
		logger.Get().Error().Err(err).Str("ip", ip).Msg("Failed to record login attempt")
	}
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
