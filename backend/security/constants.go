package security

import (
	"time"
)

// Constants for security settings
const (
	// CSRFTokenTTL est la durée de vie des tokens CSRF (30 minutes par défaut)
	CSRFTokenTTL = 30 * time.Minute

	// MaxLoginAttempts est le nombre maximum de tentatives de connexion autorisées
	MaxLoginAttempts = 5

	// LockoutPeriod est la période pendant laquelle un utilisateur est verrouillé
	LockoutPeriod = 15 * time.Minute
)
