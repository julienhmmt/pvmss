package proxmox

import (
	"context"
	"time"

	px "github.com/Telmate/proxmox-api-go/proxmox"
)

func GetNodes(client *px.Client) (interface{}, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	vmrefs, err := client.GetNodeList(ctx)
	if err != nil {
		return nil, err
	}

	return vmrefs["data"], nil
}
