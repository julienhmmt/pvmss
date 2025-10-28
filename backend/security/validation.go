package security

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"pvmss/logger"
)

// ValidateRequiredEnvVars checks that all required environment variables are set
// and have appropriate values for security.
func ValidateRequiredEnvVars() error {
	log := logger.Get().With().Str("component", "security_validation").Logger()

	// Detect test mode (go test) and offline mode
	isTest := strings.HasSuffix(strings.ToLower(filepath.Base(os.Args[0])), ".test") ||
		os.Getenv("GO_TEST") == "1" || os.Getenv("PVMSS_TEST_MODE") == "1"
	offline := strings.EqualFold(os.Getenv("PVMSS_OFFLINE"), "true")

	// Base required vars
	required := map[string]string{
		"SESSION_SECRET":      "Session encryption key",
		"ADMIN_PASSWORD_HASH": "Admin password hash (bcrypt)",
	}
	// Only require Proxmox vars if not offline and not testing
	if !offline && !isTest {
		required["PROXMOX_URL"] = "Proxmox server URL"
		required["PROXMOX_API_TOKEN_NAME"] = "Proxmox API token ID"
		required["PROXMOX_API_TOKEN_VALUE"] = "Proxmox API token secret"
	}

	missing := []string{}

	for key, description := range required {
		if os.Getenv(key) == "" {
			missing = append(missing, fmt.Sprintf("%s (%s)", key, description))
			log.Error().Str("env_var", key).Msg("Required environment variable not set")
		}
	}

	if len(missing) > 0 {
		if isTest {
			log.Warn().Strs("missing", missing).Msg("Missing required environment variables (test mode) — continuing")
		} else {
			return fmt.Errorf("missing required environment variables: %v", missing)
		}
	}

	// Validate SESSION_SECRET length (minimum 32 bytes for security)
	sessionSecret := os.Getenv("SESSION_SECRET")
	if len(sessionSecret) < 32 {
		if isTest {
			log.Warn().Int("length", len(sessionSecret)).Msg("SESSION_SECRET too short in test mode; continuing")
		} else {
			log.Warn().
				Int("length", len(sessionSecret)).
				Msg("SESSION_SECRET is too short (< 32 bytes), security may be compromised")
			return fmt.Errorf("SESSION_SECRET must be at least 32 characters long")
		}
	}

	// Validate ADMIN_PASSWORD_HASH looks like bcrypt (allow test hashes)
	adminHash := os.Getenv("ADMIN_PASSWORD_HASH")
	if len(adminHash) < 20 {
		if isTest {
			log.Warn().Msg("ADMIN_PASSWORD_HASH short in test mode; continuing")
		} else {
			log.Warn().Msg("ADMIN_PASSWORD_HASH is too short")
			return fmt.Errorf("ADMIN_PASSWORD_HASH must be at least 20 characters long")
		}
	}
	// Check if it starts with bcrypt prefix or is a long test hash
	isBcrypt := len(adminHash) >= 4 && (adminHash[:4] == "$2a$" || adminHash[:4] == "$2b$" || adminHash[:4] == "$2y$")
	if !isBcrypt && len(adminHash) < 59 {
		if isTest {
			log.Warn().Msg("ADMIN_PASSWORD_HASH not bcrypt in test mode; continuing")
		} else {
			log.Warn().Msg("ADMIN_PASSWORD_HASH does not appear to be a valid bcrypt hash")
			return fmt.Errorf("ADMIN_PASSWORD_HASH must be a valid bcrypt hash or at least 59 characters for test hashes")
		}
	}

	log.Info().Msg("All required environment variables validated successfully")
	return nil
}
