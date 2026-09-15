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
	// SSHUser enables SSH snippet delivery when non-empty. Snippet files are
	// written over SSH to the host derived from each cluster's API URL, so
	// PVMSS and Proxmox need no shared filesystem. SSHKeyFile is the path to
	// the private key; SSHPort defaults to 22.
	SSHUser    string
	SSHKeyFile string
	SSHPort    int
}
