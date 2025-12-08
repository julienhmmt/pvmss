package state

import (
	"time"

	"pvmss/logger"
)

// AddCSRFToken adds a new CSRF token with an expiry time.
func (s *appState) AddCSRFToken(token string, expiry time.Time) error {
	s.securityMu.Lock()
	defer s.securityMu.Unlock()
	s.csrfTokens[token] = expiry
	return nil
}

// ValidateAndRemoveCSRFToken validates a CSRF token and removes it if valid.
func (s *appState) ValidateAndRemoveCSRFToken(token string) bool {
	s.securityMu.Lock()
	defer s.securityMu.Unlock()

	expiry, exists := s.csrfTokens[token]
	if !exists {
		return false
	}

	// Remove the token (one-time use)
	delete(s.csrfTokens, token)

	// Check if token is expired
	if time.Now().After(expiry) {
		return false
	}

	return true
}

// CleanExpiredCSRFTokens removes all expired CSRF tokens.
func (s *appState) CleanExpiredCSRFTokens() {
	s.securityMu.Lock()
	defer s.securityMu.Unlock()

	now := time.Now()
	expiredCount := 0
	for token, expiry := range s.csrfTokens {
		if now.After(expiry) {
			delete(s.csrfTokens, token)
			expiredCount++
		}
	}

	if expiredCount > 0 {
		logger.Get().Debug().
			Int("expired_count", expiredCount).
			Int("remaining_count", len(s.csrfTokens)).
			Msg("Cleaned expired CSRF tokens")
	}
}
