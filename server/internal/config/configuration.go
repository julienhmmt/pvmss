package config

import (
	"log/slog"
	"time"
)

// LogValue keeps SessionSecret, the Proxmox token and the admin hash out of
// logs when a whole Configuration is logged.
func (Configuration) LogValue() slog.Value { return slog.StringValue("[redacted]") }

// Configuration holds the values required for the server to start.
// All required values are loaded from the environment; WebDir is optional
// and will be resolved at startup if omitted. Host defaults to 127.0.0.1.
type Configuration struct {
	Host string
	Port int
	// MetricsPort is the separate Prometheus listener port (PVMSS_METRICS_PORT);
	// 0 disables it.
	MetricsPort       int
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
	// MaxListPageSize is the upper bound on a VM list request's pageSize -
	// anything larger is rejected, never silently truncated.
	MaxListPageSize int
	// TrustedProxyHops is the number of trusted reverse-proxy hops in front
	// of the server. It controls X-Forwarded-For parsing in the shared
	// clientIP helper: with N hops, the IP at position len(xff)-N is selected
	// (the first untrusted hop from the right). 0 means no proxy is trusted
	// and RemoteAddr is used directly. Defaults to 1 (a single ingress).
	TrustedProxyHops int
	// RateLimitMax overrides every rate limiter's request ceiling when > 0
	// (PVMSS_RATE_LIMIT_MAX). 0 keeps the built-in per-endpoint defaults.
	// Intended for the e2e suite and load tests: raising it weakens the
	// per-IP login brute-force protection, so it is opt-in and never defaulted.
	RateLimitMax int
	// DeprecatedSSHEnv lists retired SSH publishing variables that were set
	// and are ignored (PVMSS_SSH_KEY_FILE, PVMSS_SSH_USER, PVMSS_SSH_PORT).
	// Startup logs a warning for each: cloud-init files are now pasted by
	// hand on the nodes.
	DeprecatedSSHEnv []string
}
