package proxmox

import (
	"fmt"
	"os"
	"time"
)

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
