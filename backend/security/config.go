package security

import (
	"os"
	"strconv"
	"sync"
	"time"
)

// Config holds configurable security parameters.
// Values are loaded once from environment variables with sensible defaults.
type Config struct {
	// CSRFTokenTTL controls the maximum lifetime of CSRF tokens before rotation.
	// Default is the value from CSRFTokenTTL constant if not overridden via env.
	CSRFTokenTTL time.Duration

	// BcryptCost controls the hashing cost for passwords.
	// Default 14 if not overridden via env BCRYPT_COST.
	BcryptCost int
}

var (
	config     *Config
	configOnce sync.Once
)

// GetConfig returns the singleton Config, loading from environment on first call.
// Env variables:
// - CSRF_TOKEN_TTL: duration (e.g., "30m", "1h").
// - BCRYPT_COST: integer cost (e.g., 12, 14).
func GetConfig() *Config {
	configOnce.Do(func() {
		cfg := &Config{
			CSRFTokenTTL: CSRFTokenTTL, // default from constants.go
			BcryptCost:   14,
		}

		if v := os.Getenv("CSRF_TOKEN_TTL"); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				cfg.CSRFTokenTTL = d
			}
		}
		if v := os.Getenv("BCRYPT_COST"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 4 && n <= 31 { // bcrypt cost bounds
				cfg.BcryptCost = n
			}
		}

		config = cfg
	})
	return config
}
