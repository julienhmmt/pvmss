package proxmox

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

// GetVMBRs retrieves the list of network bridges from a specific node.
func GetVMBRs(client *Client, node string) (map[string]interface{}, error) {
	log.Info().Str("node", node).Msg("Fetching VMBRs from Proxmox")
	ctx, cancel := context.WithTimeout(context.Background(), client.Timeout)
	defer cancel()
	return GetVMBRsWithContext(ctx, client, node)
}

// GetVMBRsWithContext retrieves the list of network bridges with context support.
func GetVMBRsWithContext(ctx context.Context, client *Client, node string) (map[string]interface{}, error) {
	log.Info().Str("node", node).Msg("Fetching VMBRs with context from Proxmox")
	path := fmt.Sprintf("/nodes/%s/network", node)
	return client.GetWithContext(ctx, path)
}
