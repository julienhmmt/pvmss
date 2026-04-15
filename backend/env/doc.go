// Package env loads and validates all application configuration from environment variables.
//
// LoadAndValidate is the sole entry point. It must be called once during application
// startup before any other initialisation. On validation failure it returns a descriptive
// error that the caller should treat as fatal (os.Exit(1) / log.Fatal).
//
// Required variables (startup fails without them):
//
//	JWT_SECRET            — HS256 signing key for /api/v1/ tokens (≥ 32 bytes)
//	SESSION_SECRET        — SCS session encryption key (≥ 32 bytes)
//	ADMIN_PASSWORD_HASH   — bcrypt hash of the admin password (must start with $2)
//
// Required unless PVMSS_OFFLINE=true:
//
//	PROXMOX_URL           — Proxmox VE API base URL (must use https)
//	PROXMOX_API_TOKEN_NAME  — Proxmox API token ID (e.g. "root@pam!pvmss")
//	PROXMOX_API_TOKEN_VALUE — Proxmox API token secret
//
// Optional with defaults:
//
//	PVMSS_DB_PATH   (default "pvmss.db")     — SQLite database file path
//	PVMSS_ENV       (default "development")  — "production" | "development"
//	PVMSS_OFFLINE   (default "false")        — disable all Proxmox API calls
//	PROXMOX_VERIFY_SSL (default "true")      — verify Proxmox TLS certificates
//	LOG_LEVEL       (default "info")         — debug | info | warn | error
//	LOG_OUTPUT      (default "stdout")       — stdout | stderr
//	LOG_FORMAT      (default "json")         — json | pretty
//	TZ              (default "UTC")          — IANA timezone (e.g. "Europe/Paris")
//	PORT            (default "50000")        — TCP port the HTTP server listens on
package env
