package security

import (
	"time"
)

// contextKey is a private type to prevent collisions in context.
// It's a good practice for context keys.
type contextKey string

// Constants for security settings
const (
	// CSRFTokenTTL is the lifetime of CSRF tokens (default 30 minutes).
	CSRFTokenTTL = 30 * time.Minute

	// CSRFTokenContextKey is the key used to store the CSRF token in the request context.
	CSRFTokenContextKey = contextKey("csrf_token")
)
