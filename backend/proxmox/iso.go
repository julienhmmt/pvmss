package proxmox

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

// GetISOList retrieves the list of ISO images from a specific storage on a specific node.
func GetISOList(client *Client, node string, storage string) (map[string]interface{}, error) {
	log.Info().Str("node", node).Str("storage", storage).Msg("Fetching ISO list from Proxmox")
	ctx, cancel := context.WithTimeout(context.Background(), client.Timeout)
	defer cancel()
	return GetISOListWithContext(ctx, client, node, storage)
}

// GetISOListWithContext retrieves the list of ISO images with context support.
func GetISOListWithContext(ctx context.Context, client *Client, node string, storage string) (map[string]interface{}, error) {
	log.Info().Str("node", node).Str("storage", storage).Msg("Fetching ISO list with context from Proxmox")
	path := fmt.Sprintf("/nodes/%s/storage/%s/content", node, storage)
	return client.GetWithContext(ctx, path)
}
