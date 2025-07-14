package proxmox

import (
	"context"

	"github.com/rs/zerolog/log"
)

// GetStorages retrieves the list of storages from Proxmox.
func GetStorages(client *Client) (interface{}, error) {
	log.Info().Msg("Fetching storage list from Proxmox")
	ctx, cancel := context.WithTimeout(context.Background(), client.Timeout)
	defer cancel()
	return GetStoragesWithContext(ctx, client)
}

// GetStoragesWithContext retrieves the list of storages from Proxmox with context support.
func GetStoragesWithContext(ctx context.Context, client *Client) (interface{}, error) {
	return client.GetWithContext(ctx, "/storage")
}
