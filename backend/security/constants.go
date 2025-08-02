package security

import (
	"time"
)

// Constants for security settings
const (
	// CSRFTokenTTL est la durée de vie des tokens CSRF (30 minutes par défaut)
	CSRFTokenTTL = 30 * time.Minute
)
