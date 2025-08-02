package security

import (
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
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

	// Get session secret from environment
	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET environment variable not set")
	}

	// Initialize session manager with enhanced configuration
	sessionManager := scs.New()
	sessionManager.Store = memstore.New()
	sessionManager.Lifetime = 24 * time.Hour
	sessionManager.Cookie = scs.SessionCookie{
		Name:     "pvmss_session",
		HttpOnly: true,
		Secure:   os.Getenv("ENV") == "production",
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	}
	sessionManager.IdleTimeout = 30 * time.Minute
	// Ensure session is persisted even across browser sessions
	sessionManager.Cookie.Persist = true

	logger.Info().Msg("Security components initialized successfully")

	return &SessionManager{
		SessionManager: sessionManager,
	}, nil
}
