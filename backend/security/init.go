package security

import (
	"encoding/gob"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"
	"pvmss/logger"
	"pvmss/utils"
)

// RegisterSessionTypes registers custom types with gob for session serialization
func init() {
	// Register handlers.VMCreateFormData - done via interface to avoid circular import
	// The actual type will be registered when handlers package loads
	gob.Register(map[string]interface{}{})
	gob.Register([]string{})
}

// InitSecurity initializes the session manager and other security components.
// It configures the session store, lifetime, and cookie settings based on the
// environment (production or development).
func InitSecurity() (*scs.SessionManager, error) {
	log := logger.Get()
	log.Info().Msg("Initializing security components")

	// Get session secret from environment.
	sessionSecret := os.Getenv("SESSION_SECRET")
	if sessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET environment variable not set")
	}

	// Determine if running in production for secure cookie settings.
	isProduction := utils.IsProduction()

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

	log.Info().Msg("Security components initialized successfully")

	return scsm, nil
}
