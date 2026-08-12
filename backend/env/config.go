package env

// EnvConfig holds all configuration values derived from environment variables.
// The zero value is not valid; use LoadAndValidate to obtain a populated instance.
type EnvConfig struct {
	// JWTSecret is the signing key for /api/v1/ access and refresh tokens.
	// Source: JWT_SECRET (minimum 32 bytes).
	JWTSecret string

	// SessionSecret is the encryption key for server-side sessions (SCS).
	// Source: SESSION_SECRET (minimum 32 bytes).
	SessionSecret string

	// AdminPasswordHash is the bcrypt hash of the built-in admin account password.
	// Source: ADMIN_PASSWORD_HASH (must start with $2).
	AdminPasswordHash string

	// ProxmoxURL is the Proxmox VE API base URL (e.g. "https://pve.example.com:8006").
	// Source: PROXMOX_URL. Not required when Offline is true.
	ProxmoxURL string

	// ProxmoxAPITokenName is the Proxmox API token ID (e.g. "root@pam!pvmss").
	// Source: PROXMOX_API_TOKEN_NAME. Not required when Offline is true.
	ProxmoxAPITokenName string

	// ProxmoxAPITokenValue is the Proxmox API token secret.
	// Source: PROXMOX_API_TOKEN_VALUE. Not required when Offline is true.
	ProxmoxAPITokenValue string

	// ProxmoxSSLVerify controls TLS certificate verification for Proxmox API calls.
	// Source: PROXMOX_VERIFY_SSL. Defaults to true.
	ProxmoxSSLVerify bool

	// DBPath is the file path for the SQLite database.
	// Source: PVMSS_DB_PATH. Defaults to "pvmss.db".
	DBPath string

	// Environment is the deployment environment identifier.
	// Source: PVMSS_ENV. Defaults to "development".
	Environment string

	// Offline disables all Proxmox API calls when true.
	// Source: PVMSS_OFFLINE=true.
	Offline bool

	// LogLevel controls log verbosity (debug, info, warn, error).
	// Source: LOG_LEVEL. Defaults to "info".
	LogLevel string

	// LogOutput is the log destination ("stdout" or "stderr").
	// Source: LOG_OUTPUT. Defaults to "stdout".
	LogOutput string

	// LogFormat is the log encoding ("json" or "pretty").
	// Source: LOG_FORMAT. Defaults to "json".
	LogFormat string

	// Timezone is the IANA timezone name used for log timestamps.
	// Source: TZ. Defaults to "UTC".
	Timezone string

	// Port is the TCP port the HTTP server listens on.
	// Source: PORT. Defaults to "50000".
	Port string

	// TrustedProxies is a comma-separated list of CIDR ranges or IP addresses
	// that are allowed to set X-Forwarded-For / X-Real-IP headers. When empty
	// (the default), the middleware uses r.RemoteAddr directly and ignores
	// forwarding headers, preventing IP spoofing.
	// Source: PVMSS_TRUSTED_PROXIES. Defaults to "" (no trusted proxies).
	TrustedProxies string
}
