package proxmox

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

// GetVMBRs is a convenience function that retrieves the list of network interfaces (including bridges) from a specific node.
// It calls GetVMBRsWithContext using the client's default timeout.
func GetVMBRs(client *Client, node string) (map[string]interface{}, error) {
	log.Info().Str("node", node).Msg("Fetching VMBRs from Proxmox")
	ctx, cancel := context.WithTimeout(context.Background(), client.Timeout)
	defer cancel()
	return GetVMBRsWithContext(ctx, client, node)
}

// GetVMBRsWithContext fetches the list of network interfaces from the `/nodes/{node}/network` endpoint
// of the Proxmox API using the provided context for timeout and cancellation control.
func GetVMBRsWithContext(ctx context.Context, client *Client, node string) (map[string]interface{}, error) {
	log.Info().Str("node", node).Msg("Fetching VMBRs with context from Proxmox")
	path := fmt.Sprintf("/nodes/%s/network", node)
	return client.GetWithContext(ctx, path)
}
