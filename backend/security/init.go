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
	scsm := scs.New()
	scsm.Store = memstore.New()
	scsm.Lifetime = 24 * time.Hour
	scsm.Cookie = scs.SessionCookie{
		Name:     "pvmss_session",
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	}
	scsm.IdleTimeout = 30 * time.Minute
	// Ensure session is persisted even across browser sessions
	scsm.Cookie.Persist = true

	// Create our custom session manager
	sessionManager := NewSessionManager(scsm)

	logger.Info().Msg("Security components initialized successfully")

	return sessionManager, nil
}
