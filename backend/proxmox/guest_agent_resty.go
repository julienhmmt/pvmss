package proxmox

import (
	"context"
	"fmt"
	"net/url"
)

// GetGuestAgentNetworkInterfacesResty fetches network information from the QEMU guest agent
// using the Resty HTTP client. Returns nil if guest agent is not available or not running.
//
// GET /nodes/{node}/qemu/{vmid}/agent/network-get-interfaces
func GetGuestAgentNetworkInterfacesResty(ctx context.Context, restyClient *RestyClient, node string, vmid int) ([]GuestAgentNetworkInterface, error) {
	if restyClient == nil {
		return nil, fmt.Errorf("restyClient is nil")
	}
	if node == "" || vmid <= 0 {
		return nil, fmt.Errorf("node and valid vmid are required")
	}

	path := fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", url.PathEscape(node), vmid)

	var resp Response[struct {
		Result []GuestAgentNetworkInterface `json:"result"`
	}]

	if err := restyClient.Get(ctx, path, &resp); err != nil {
		return nil, fmt.Errorf("failed to get guest agent network for node %s, vmid %d: %w", node, vmid, err)
	}

	return resp.Data.Result, nil
}
