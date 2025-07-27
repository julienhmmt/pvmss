package proxmox

import (
	"context"
	"fmt"

	"pvmss/logger"
)

// GetISOList retrieves the list of ISO images from a specific storage on a given Proxmox node.
// It uses the client's default timeout for the API request.
func GetISOList(client *Client, node string, storage string) (map[string]interface{}, error) {
	logger.Get().Info().Str("node", node).Str("storage", storage).Msg("Fetching ISO list from Proxmox")
	ctx, cancel := context.WithTimeout(context.Background(), client.Timeout)
	defer cancel()
	return GetISOListWithContext(ctx, client, node, storage)
}

// GetISOListWithContext retrieves the list of ISO images from a specific storage on a given Proxmox node
// using the provided context for timeout and cancellation control.
// This is the underlying function that performs the actual API call.
func GetISOListWithContext(ctx context.Context, client *Client, node string, storage string) (map[string]interface{}, error) {
	logger.Get().Info().Str("node", node).Str("storage", storage).Msg("Fetching ISO list with context from Proxmox")
	path := fmt.Sprintf("/nodes/%s/storage/%s/content", node, storage)
	return client.GetWithContext(ctx, path)
}
