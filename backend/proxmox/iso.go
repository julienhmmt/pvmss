package proxmox

import (
	"context"
	"fmt"
)

// GetISOList retrieves the list of ISO images from a specific storage on a specific node.
func GetISOList(client *Client, node string, storage string) (map[string]interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), client.Timeout)
	defer cancel()
	return GetISOListWithContext(ctx, client, node, storage)
}

// GetISOListWithContext retrieves the list of ISO images with context support.
func GetISOListWithContext(ctx context.Context, client *Client, node string, storage string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/nodes/%s/storage/%s/content", node, storage)
	return client.GetWithContext(ctx, path)
}
