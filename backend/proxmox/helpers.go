package proxmox

import (
	"fmt"
	"net/url"
	"strings"
	"sync"
	"time"

	envpkg "pvmss/env"
)

// envCfgMu guards the package-level environment configuration.
var envCfgMu sync.RWMutex

// envCfg holds the validated environment configuration set at startup.
var envCfg *envpkg.EnvConfig

// SetEnvConfig stores the validated environment configuration for use by
// FromEnv convenience functions. Must be called once during startup.
func SetEnvConfig(cfg *envpkg.EnvConfig) {
	envCfgMu.Lock()
	defer envCfgMu.Unlock()
	envCfg = cfg
}

// getEnvConfig returns the stored environment configuration.
func getEnvConfig() *envpkg.EnvConfig {
	envCfgMu.RLock()
	defer envCfgMu.RUnlock()
	return envCfg
}

// normalizeBaseURL ensures the Proxmox API URL is correctly formatted.
func normalizeBaseURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %w", err)
	}

	if u.Scheme == "" {
		u.Scheme = "https"
	}

	if u.Path == "" || u.Path == "/" {
		u.Path = "/api2/json"
	} else if !strings.HasSuffix(u.Path, "/api2/json") {
		u.Path = strings.TrimSuffix(u.Path, "/") + "/api2/json"
	}

	return u.String(), nil
}

// MakeRestyClientFromEnv creates a RestyClient using the stored EnvConfig.
// Deprecated: Use MakeRestyClientFromEnvConfig when the config is available.
func MakeRestyClientFromEnv(timeout time.Duration) (*RestyClient, error) {
	cfg := getEnvConfig()
	if cfg == nil {
		return nil, fmt.Errorf("environment configuration not initialised (call SetEnvConfig first)")
	}
	return MakeRestyClientFromEnvConfig(cfg, timeout)
}

// MakeRestyClientCookieAuthFromEnv creates a cookie-auth RestyClient using the stored EnvConfig.
// Deprecated: Use MakeRestyClientCookieAuthFromEnvConfig when the config is available.
func MakeRestyClientCookieAuthFromEnv(timeout time.Duration) (*RestyClient, error) {
	cfg := getEnvConfig()
	if cfg == nil {
		return nil, fmt.Errorf("environment configuration not initialised (call SetEnvConfig first)")
	}
	return MakeRestyClientCookieAuthFromEnvConfig(cfg, timeout)
}

// MakeRestyClientFromEnvConfig creates a RestyClient using the validated EnvConfig.
func MakeRestyClientFromEnvConfig(cfg *envpkg.EnvConfig, timeout time.Duration) (*RestyClient, error) {
	if cfg.ProxmoxURL == "" || cfg.ProxmoxAPITokenName == "" || cfg.ProxmoxAPITokenValue == "" {
		return nil, fmt.Errorf("missing Proxmox configuration")
	}

	return MakeRestyClient(cfg.ProxmoxURL, cfg.ProxmoxAPITokenName, cfg.ProxmoxAPITokenValue, !cfg.ProxmoxSSLVerify, timeout)
}

// MakeRestyClientCookieAuthFromEnvConfig creates a cookie-auth RestyClient using the validated EnvConfig.
func MakeRestyClientCookieAuthFromEnvConfig(cfg *envpkg.EnvConfig, timeout time.Duration) (*RestyClient, error) {
	if cfg.ProxmoxURL == "" {
		return nil, fmt.Errorf("PROXMOX_URL is required")
	}

	return MakeRestyClientCookieAuth(cfg.ProxmoxURL, !cfg.ProxmoxSSLVerify, timeout)
}
