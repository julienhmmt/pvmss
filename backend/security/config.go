package security

import (
	"os"
	"pvmss/logger"
	"strconv"
	"sync"
	"time"
)

const (
	// defaultCSRFTokenTTL is the default lifetime for CSRF tokens if not specified via environment variable.
	defaultCSRFTokenTTL = 30 * time.Minute
	// defaultBcryptCost is the default cost for bcrypt password hashing if not specified via environment variable.
	defaultBcryptCost = 14
)

// Config holds configurable security parameters.
// Values are loaded once from environment variables with sensible defaults.
type Config struct {
	// CSRFTokenTTL controls the maximum lifetime of CSRF tokens before rotation.
	CSRFTokenTTL time.Duration

	// BcryptCost controls the hashing cost for passwords.
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
		log := logger.Get().With().Str("component", "security_config").Logger()

		cfg := &Config{
			CSRFTokenTTL: defaultCSRFTokenTTL,
			BcryptCost:   defaultBcryptCost,
		}

		// Load CSRF_TOKEN_TTL from environment, with logging on failure.
		if v := os.Getenv("CSRF_TOKEN_TTL"); v != "" {
			if d, err := time.ParseDuration(v); err == nil {
				cfg.CSRFTokenTTL = d
			} else {
				log.Warn().Err(err).Str("value", v).Msg("Invalid CSRF_TOKEN_TTL format; using default")
			}
		}

		// Load BCRYPT_COST from environment, with validation and logging.
		if v := os.Getenv("BCRYPT_COST"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				if n >= 4 && n <= 31 { // bcrypt cost bounds
					cfg.BcryptCost = n
				} else {
					log.Warn().Int("value", n).Msg("BCRYPT_COST out of range (4-31); using default")
				}
			} else {
				log.Warn().Err(err).Str("value", v).Msg("Invalid BCRYPT_COST format; using default")
			}
		}

		config = cfg
	})
	return config
}
