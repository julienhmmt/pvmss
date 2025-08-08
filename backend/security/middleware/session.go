package middleware

import (
	"context"
	"net/http"

	"github.com/alexedwards/scs/v2"
	"pvmss/logger"
	"pvmss/security"
)

// sessionKey is a custom type for context keys
type sessionKey string

// String implements the Stringer interface
func (k sessionKey) String() string {
	return "session_key_" + string(k)
}

// SessionKey is the context key for the session manager
var SessionKey = sessionKey("session_manager")

// SessionMiddleware ensures the session manager is attached to the request context
func SessionMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log := logger.Get().With().
			Str("middleware", "SessionMiddleware").
			Str("path", r.URL.Path).
			Logger()

		// Get the session manager
		sessionManager := security.GetSession(r)
		if sessionManager == nil {
			log.Warn().Msg("No session manager available")
			next.ServeHTTP(w, r)
			return
		}

		// Add the session manager to the context
		ctx := r.Context()
		ctx = context.WithValue(ctx, SessionKey, sessionManager)
		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}

// GetSessionManagerFromContext retrieves the session manager from the context
func GetSessionManagerFromContext(ctx context.Context) *scs.SessionManager {
	if sm, ok := ctx.Value(SessionKey).(*scs.SessionManager); ok {
		return sm
	}
	return nil
}
