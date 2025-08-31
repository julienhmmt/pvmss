package security

import (
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
	"pvmss/logger"
)

// InitSecurity initializes all security components and returns a session manager.
func InitSecurity() (*SessionManager, error) {
	log := logger.Get()
	log.Info().Msg("Initializing security components")

	// Get session secret from environment.
	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET environment variable not set")
	}

	// Determine if running in production for secure cookie settings.
	isProduction := strings.ToLower(os.Getenv("ENV")) == "production"

	// Initialize session manager with enhanced configuration.
	scsm := scs.New()
	scsm.Store = memstore.New()
	scsm.Lifetime = 24 * time.Hour
	scsm.Cookie = scs.SessionCookie{
		Name:     "pvmss_session",
		HttpOnly: true,
		Secure:   isProduction, // Use secure cookies in production.
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	}
	scsm.IdleTimeout = 30 * time.Minute
	// Ensure session is persisted even across browser sessions.
	scsm.Cookie.Persist = true

	if isProduction {
		log.Info().Msg("Secure session cookies enabled for production environment")
	} else {
		log.Warn().Msg("Secure session cookies disabled (not in production environment)")
	}

	// Create our custom session manager.
	sessionManager := NewSessionManager(scsm)

	log.Info().Msg("Security components initialized successfully")

	return sessionManager, nil
}
