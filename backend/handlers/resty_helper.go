package handlers

import (
	"fmt"
	"time"

	"pvmss/proxmox"
)

// getRestyClient creates a resty client with the specified timeout
// This is a convenience function for handlers migrating from the old client
func getRestyClient(timeout time.Duration) (*proxmox.RestyClient, error) {
	client, err := proxmox.NewRestyClientFromEnv(timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to create resty client: %w", err)
	}
	return client, nil
}

// getDefaultRestyClient creates a resty client with a 30-second timeout
func getDefaultRestyClient() (*proxmox.RestyClient, error) {
	return getRestyClient(30 * time.Second)
}
