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
	// SSHKeyFile is the private key PVMSS uses to publish admin cloud-init
	// documents to every node (PVMSS_SSH_KEY_FILE). The SSH user, port and
	// pinned host keys are per cluster. Empty: cloud-init documents off.
	SSHKeyFile string
}
