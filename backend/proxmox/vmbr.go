package proxmox

import (
	"context"
	"fmt"
)

// GetVMBRs retrieves the list of network bridges from a specific node.
func GetVMBRs(client *Client, node string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), client.Timeout)
	defer cancel()
	return GetVMBRsWithContext(ctx, client, node)
}

// GetVMBRsWithContext retrieves the list of network bridges with context support.
func GetVMBRsWithContext(ctx context.Context, client *Client, node string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/nodes/%s/network", node)
	return client.GetWithContext(ctx, path)
}
