package security

import (
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"pvmss/logger"
)

// SessionManager wraps scs.SessionManager to provide additional methods if needed
type SessionManager struct {
	*scs.SessionManager
}

// InitSecurity initializes all security components and returns a session manager
func InitSecurity() (*SessionManager, error) {
	logger.Get().Info().Msg("Initializing security components")

	// Initialize session manager
	sessionManager := scs.New()
	sessionManager.Lifetime = 24 * time.Hour
	sessionManager.Cookie.Name = "pvmss_session"
	sessionManager.Cookie.HttpOnly = true
	sessionManager.Cookie.Secure = true
	sessionManager.Cookie.SameSite = http.SameSiteLaxMode

	// Set up CSRF protection
	sessionManager.Cookie.Name = "pvmss_csrf"
	sessionManager.IdleTimeout = 30 * time.Minute

	logger.Get().Info().Msg("Security components initialized successfully")

	return &SessionManager{
		SessionManager: sessionManager,
	}, nil
}

// WrapHandlers wraps HTTP handlers with security middleware
func (sm *SessionManager) WrapHandlers(handler http.Handler) http.Handler {
	// Apply security middleware in the correct order
	return HeadersMiddleware(
		sm.SessionManager.LoadAndSave(
			AdminCSRFMiddleware(handler),
		),
	)
}
