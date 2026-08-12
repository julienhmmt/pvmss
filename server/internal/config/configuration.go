package config

import "time"

// Configuration holds the values required for the server to start.
// All required values are loaded from the environment; WebDir is optional
// and will be resolved at startup if omitted. Host defaults to 127.0.0.1.
type Configuration struct {
	Host              string
	Port              int
	DBPath            string
	LogLevel          string
	LogFormat         string
	LogOutput         string
	WebDir            string
	ClusterSource     string
	SessionSecret     string
	AdminPasswordHash string
	// CookieSecure gates the Secure attribute on the session cookie. Defaults
	// to true (production behind TLS-terminating ingress); set
	// PVMSS_COOKIE_SECURE=false only for local plain-HTTP development.
	CookieSecure                      bool
	ProxmoxURL                        string
	ProxmoxAPITokenName               string
	ProxmoxAPITokenValue              string
	InventoryRefreshInterval          time.Duration
	InventoryManualRefreshMinInterval time.Duration
	InventoryRefreshTimeout           time.Duration
	// MaxListPageSize is the upper bound on a VM list request's pageSize —
	// anything larger is rejected, never silently truncated (T04 data-model).
	MaxListPageSize int
}
