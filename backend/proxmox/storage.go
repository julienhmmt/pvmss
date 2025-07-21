package proxmox

import (
	"context"

	"github.com/rs/zerolog/log"
)

// GetStorages is a convenience function that retrieves the list of all available storages across all nodes.
// It calls GetStoragesWithContext using the client's default timeout.
func GetStorages(client *Client) (interface{}, error) {
	log.Info().Msg("Fetching storage list from Proxmox")
	ctx, cancel := context.WithTimeout(context.Background(), client.Timeout)
	defer cancel()
	return GetStoragesWithContext(ctx, client)
}

// GetStoragesWithContext fetches the list of all storages from the `/storage` endpoint of the Proxmox API
// using the provided context for timeout and cancellation control.
func GetStoragesWithContext(ctx context.Context, client *Client) (interface{}, error) {
	return client.GetWithContext(ctx, "/storage")
}
