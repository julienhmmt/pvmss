package env

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

const (
	minSecretLen = 32
	defaultPort  = "50000"
)

// LoadAndValidate reads all configuration from environment variables, validates
// required fields, applies defaults for optional fields, and returns an EnvConfig.
//
// The function follows fail-fast semantics: it accumulates all validation errors
// and returns them together so operators can fix every problem in one restart.
//
// Callers should treat a non-nil error as fatal and exit the process immediately.
func LoadAndValidate() (*EnvConfig, error) {
	offline := strings.EqualFold(os.Getenv("PVMSS_OFFLINE"), "true")

	cfg := &EnvConfig{
		JWTSecret:            os.Getenv("JWT_SECRET"),
		SessionSecret:        os.Getenv("SESSION_SECRET"),
		AdminPasswordHash:    os.Getenv("ADMIN_PASSWORD_HASH"),
		ProxmoxURL:           os.Getenv("PROXMOX_URL"),
		ProxmoxAPITokenName:  os.Getenv("PROXMOX_API_TOKEN_NAME"),
		ProxmoxAPITokenValue: os.Getenv("PROXMOX_API_TOKEN_VALUE"),
		Offline:              offline,
		ProxmoxSSLVerify:     parseBoolDefault("PROXMOX_VERIFY_SSL", true),
		DBPath:               envOrDefault("PVMSS_DB_PATH", "pvmss.db"),
		Environment:          envOrDefault("PVMSS_ENV", "development"),
		LogLevel:             envOrDefault("LOG_LEVEL", "info"),
		LogOutput:            envOrDefault("LOG_OUTPUT", "stdout"),
		LogFormat:            envOrDefault("LOG_FORMAT", "json"),
		Timezone:             envOrDefault("TZ", "UTC"),
		Port:                 envOrDefault("PORT", defaultPort),
	}

	if err := validate(cfg); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validate accumulates all constraint violations and returns them as a single error.
func validate(cfg *EnvConfig) error {
	var errs []string

	// JWT_SECRET: required, minimum 32 bytes.
	switch {
	case cfg.JWTSecret == "":
		errs = append(errs, "JWT_SECRET is required")
	case len(cfg.JWTSecret) < minSecretLen:
		errs = append(errs, fmt.Sprintf("JWT_SECRET must be at least %d bytes (got %d)", minSecretLen, len(cfg.JWTSecret)))
	}

	// SESSION_SECRET: required, minimum 32 bytes.
	switch {
	case cfg.SessionSecret == "":
		errs = append(errs, "SESSION_SECRET is required")
	case len(cfg.SessionSecret) < minSecretLen:
		errs = append(errs, fmt.Sprintf("SESSION_SECRET must be at least %d bytes (got %d)", minSecretLen, len(cfg.SessionSecret)))
	}

	// ADMIN_PASSWORD_HASH: required, must be a bcrypt hash (starts with $2).
	switch {
	case cfg.AdminPasswordHash == "":
		errs = append(errs, "ADMIN_PASSWORD_HASH is required")
	case !strings.HasPrefix(cfg.AdminPasswordHash, "$2"):
		errs = append(errs, "ADMIN_PASSWORD_HASH must be a valid bcrypt hash (must start with $2)")
	}

	// PVMSS_DB_PATH: required, cannot be empty.
	if cfg.DBPath == "" {
		errs = append(errs, "PVMSS_DB_PATH is required")
	}

	// Proxmox variables: required unless running in offline mode.
	if !cfg.Offline {
		if cfg.ProxmoxURL == "" {
			errs = append(errs, "PROXMOX_URL is required (set PVMSS_OFFLINE=true to disable Proxmox)")
		} else if err := validateHTTPSURL(cfg.ProxmoxURL); err != nil {
			errs = append(errs, "PROXMOX_URL: "+err.Error())
		}
		if cfg.ProxmoxAPITokenName == "" {
			errs = append(errs, "PROXMOX_API_TOKEN_NAME is required (set PVMSS_OFFLINE=true to disable Proxmox)")
		}
		if cfg.ProxmoxAPITokenValue == "" {
			errs = append(errs, "PROXMOX_API_TOKEN_VALUE is required (set PVMSS_OFFLINE=true to disable Proxmox)")
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("environment configuration errors:\n  - %s", strings.Join(errs, "\n  - "))
}

// validateHTTPSURL ensures raw is a parseable URL with scheme "https" and a non-empty host.
func validateHTTPSURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "https" {
		return fmt.Errorf("must use https scheme (got %q)", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("missing host in URL")
	}
	return nil
}

// envOrDefault returns the value of the named environment variable, or fallback
// when the variable is unset or empty.
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// parseBoolDefault reads a boolean environment variable. Only "true"/"1" and
// "false"/"0" (case-insensitive) are accepted; unset returns the default.
// Any other value is treated as the default with a warning, which is safer
// than silently interpreting typos.
func parseBoolDefault(key string, defaultVal bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	if v == "" {
		return defaultVal
	}
	switch v {
	case "true", "1":
		return true
	case "false", "0":
		return false
	default:
		// Ambiguous value — fall back to the default for safety.
		fmt.Fprintf(os.Stderr, "warning: %s=%q is not a valid boolean (expected true/false); using default %v\n", key, v, defaultVal)
		return defaultVal
	}
}
