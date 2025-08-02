package security

import (
	"net/http"
	"os"
	"time"

	"github.com/alexedwards/scs/v2"
	"pvmss/logger"
)

// SessionManager wraps scs.SessionManager to provide additional methods
type SessionManager struct {
	*scs.SessionManager
}

// InitSecurity initializes all security components and returns a session manager
func InitSecurity() (*SessionManager, error) {
	logger := logger.Get()
	logger.Info().Msg("Initializing security components")

	// Initialize session manager
	sessionManager := scs.New()
	sessionManager.Lifetime = 24 * time.Hour
	sessionManager.Cookie = scs.SessionCookie{
		Name:     "pvmss_session",
		HttpOnly: true,
		Secure:   os.Getenv("ENV") == "production",
		SameSite: http.SameSiteStrictMode,
		Path:     "/",
	}

	// Set up CSRF protection
	sessionManager.Cookie.Name = "pvmss_csrf"
	sessionManager.IdleTimeout = 30 * time.Minute

	logger.Info().Msg("Security components initialized successfully")

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
