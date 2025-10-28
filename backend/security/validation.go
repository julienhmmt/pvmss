package security

import (
	"fmt"
	"os"

	"pvmss/logger"
)

// ValidateRequiredEnvVars checks that all required environment variables are set
// and have appropriate values for security.
func ValidateRequiredEnvVars() error {
	log := logger.Get().With().Str("component", "security_validation").Logger()

	required := map[string]string{
		"SESSION_SECRET":          "Session encryption key",
		"ADMIN_PASSWORD_HASH":     "Admin password hash (bcrypt)",
		"PROXMOX_URL":             "Proxmox server URL",
		"PROXMOX_API_TOKEN_NAME":  "Proxmox API token ID",
		"PROXMOX_API_TOKEN_VALUE": "Proxmox API token secret",
	}

	missing := []string{}

	for key, description := range required {
		if os.Getenv(key) == "" {
			missing = append(missing, fmt.Sprintf("%s (%s)", key, description))
			log.Error().Str("env_var", key).Msg("Required environment variable not set")
		}
	}

	if len(missing) > 0 {
		return fmt.Errorf("missing required environment variables: %v", missing)
	}

	// Validate SESSION_SECRET length (minimum 32 bytes for security)
	sessionSecret := os.Getenv("SESSION_SECRET")
	if len(sessionSecret) < 32 {
		log.Warn().
			Int("length", len(sessionSecret)).
			Msg("SESSION_SECRET is too short (< 32 bytes), security may be compromised")
		return fmt.Errorf("SESSION_SECRET must be at least 32 characters long")
	}

	// Validate ADMIN_PASSWORD_HASH looks like bcrypt
	adminHash := os.Getenv("ADMIN_PASSWORD_HASH")
	if len(adminHash) < 59 || adminHash[:4] != "$2a$" && adminHash[:4] != "$2b$" && adminHash[:4] != "$2y$" {
		log.Warn().Msg("ADMIN_PASSWORD_HASH does not appear to be a valid bcrypt hash")
		return fmt.Errorf("ADMIN_PASSWORD_HASH must be a valid bcrypt hash")
	}

	log.Info().Msg("All required environment variables validated successfully")
	return nil
}
