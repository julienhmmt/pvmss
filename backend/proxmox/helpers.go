package proxmox

import (
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"
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

// MakeRestyClientFromEnv creates a RestyClient using environment variables
// This is a convenience function for handlers that need a quick resty client
func MakeRestyClientFromEnv(timeout time.Duration) (*RestyClient, error) {
	proxmoxURL := os.Getenv("PROXMOX_URL")
	tokenID := os.Getenv("PROXMOX_API_TOKEN_NAME")
	tokenValue := os.Getenv("PROXMOX_API_TOKEN_VALUE")
	insecureSkipVerify := os.Getenv("PROXMOX_VERIFY_SSL") == "false"

	if proxmoxURL == "" || tokenID == "" || tokenValue == "" {
		return nil, fmt.Errorf("missing Proxmox environment variables")
	}

	return MakeRestyClient(proxmoxURL, tokenID, tokenValue, insecureSkipVerify, timeout)
}

// MakeRestyClientCookieAuthFromEnv creates a cookie-auth RestyClient using environment variables.
// Only requires PROXMOX_URL and PROXMOX_VERIFY_SSL (no API token needed).
func MakeRestyClientCookieAuthFromEnv(timeout time.Duration) (*RestyClient, error) {
	proxmoxURL := os.Getenv("PROXMOX_URL")
	insecureSkipVerify := os.Getenv("PROXMOX_VERIFY_SSL") == "false"

	if proxmoxURL == "" {
		return nil, fmt.Errorf("PROXMOX_URL environment variable is required")
	}

	return MakeRestyClientCookieAuth(proxmoxURL, insecureSkipVerify, timeout)
}
