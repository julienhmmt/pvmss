// Package config loads and validates the server runtime configuration.
//
// The operator contract is environment variables only. [Load] fails fast on a
// missing or malformed required value — there is no flag fallback and no
// implicit cluster source. The authoritative struct is [Configuration];
// [Load] is the only production entry point.
//
// # Required
//
//	PVMSS_PORT            integer 1–65535
//	PVMSS_DB_PATH         SQLite file path (image default /data/pvmss.db)
//	SESSION_SECRET        ≥32 bytes
//	LOG_LEVEL             debug | info | warn | error  (lowercase only)
//	LOG_FORMAT            json | console
//	LOG_OUTPUT            stdout | stderr | a file path
//	PVMSS_CLUSTER_SOURCE  fake | proxmox  (no default — see below)
//
// PVMSS_CLUSTER_SOURCE has no default because fake ships hardcoded demo
// credentials (admin@pve / pvmss-admin). An operator who forgets the
// variable must not silently land in demo mode.
//
// Required when PVMSS_CLUSTER_SOURCE=proxmox:
//
//	PROXMOX_URL
//	PROXMOX_API_TOKEN_NAME
//	PROXMOX_API_TOKEN_VALUE
//
// # Optional
//
//	PVMSS_HOST                                  default 127.0.0.1 (image: 0.0.0.0)
//	PVMSS_WEB_DIR                               resolved relative to the executable
//	ADMIN_PASSWORD_HASH                         empty; if set, must be bcrypt ($2…)
//	PVMSS_COOKIE_SECURE                         default true
//	PVMSS_INVENTORY_REFRESH_INTERVAL            default 30s
//	PVMSS_INVENTORY_MANUAL_REFRESH_MIN_INTERVAL default 5s
//	PVMSS_INVENTORY_REFRESH_TIMEOUT             default 15s
//	PVMSS_MAX_LIST_PAGE_SIZE                    default 100
//
// # Files
//
//	configuration.go  Configuration fields
//	load.go           env parsing and validation
//	log.go            slog construction from LogLevel/LogFormat/LogOutput
//	redact.go         admin app-info view (secrets emptied, never masked)
//
// The checked-in example.env at the repository root is the copy-paste
// operator file. PVMSS_OFFLINE, PVMSS_ENV, JWT_SECRET, PROXMOX_VERIFY_SSL
// and LOG_FILE_PATH belonged to the v0.3 backend and are not read.
package config
