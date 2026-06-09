package proxmox

import (
	"fmt"
	"net/url"
	"strings"
	"time"

	envpkg "pvmss/env"
)

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
