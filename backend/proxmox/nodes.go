package proxmox

import (
	"context"
	"time"
)

func GetNodes(client *Client) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	return client.GetNodeList(ctx)
}
