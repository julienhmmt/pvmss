package proxmox

import (
	"context"
	"time"
)

// GetStorages retrieves the list of storages from Proxmox.
func GetStorages(client *Client) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return client.GetStorageList(ctx)
}
