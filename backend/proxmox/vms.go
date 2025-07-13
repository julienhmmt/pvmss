package proxmox

import (
	"context"
)

func GetVmList(c *Client, ctx context.Context) (map[string]interface{}, error) {
	return c.GetVmList(ctx)
}
