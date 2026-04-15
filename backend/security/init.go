package security

import (
	"encoding/gob"
	"net/http"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/alexedwards/scs/v2/memstore"

	"pvmss/utils"
)

// RegisterSessionTypes registers custom types with gob for session serialization
func init() {
	gob.Register(map[string]interface{}{})
	gob.Register([]string{})
}

// NewSessionManager creates an SCS session manager for the given environment.
// secret must be at least 32 bytes; env is the PVMSS_ENV value.
func NewSessionManager(secret, env string) (*scs.SessionManager, error) {
	scsm := scs.New()
	scsm.Store = memstore.New()
	scsm.Lifetime = 24 * time.Hour
	scsm.Cookie = scs.SessionCookie{
		Name:     "pvmss_session",
		HttpOnly: true,
		Secure:   utils.IsProduction(env),
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
		Persist:  true,
	}
	scsm.IdleTimeout = 30 * time.Minute
	return scsm, nil
}
