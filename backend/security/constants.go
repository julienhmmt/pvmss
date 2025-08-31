package security

import (
	"context"
	"time"
)

// csrfContextKey is an unexported type used as a context key. Using an
// unexported type prevents collisions with context keys defined in other packages.
type csrfContextKey struct{}

// WithCSRFToken returns a new context with the provided CSRF token.
func WithCSRFToken(ctx context.Context, token string) context.Context {
	return context.WithValue(ctx, csrfContextKey{}, token)
}

// CSRFTokenFromContext extracts a CSRF token from the given context.
// It returns the token and a boolean indicating whether the token was found.
func CSRFTokenFromContext(ctx context.Context) (string, bool) {
	token, ok := ctx.Value(csrfContextKey{}).(string)
	return token, ok
}

// Constants for security settings.
const (
	// CSRFTokenTTL defines the default lifetime for CSRF tokens. This value can be
	// overridden by the CSRF_TOKEN_TTL environment variable.
	CSRFTokenTTL = 30 * time.Minute
)
