package proxmox

import (
	"context"
	"fmt"
	"net/url"

	"pvmss/logger"
)

// GetVMBRsResty fetches the list of network interfaces (bridges) from the `/nodes/{node}/network` endpoint
// of the Proxmox API using resty for better performance.
func GetVMBRsResty(ctx context.Context, restyClient *RestyClient, node string) ([]VMBR, error) {
	if node == "" {
		return nil, fmt.Errorf("node name cannot be empty")
	}

	path := fmt.Sprintf("/nodes/%s/network", url.PathEscape(node))

	var response ListResponse[VMBR]
	if err := restyClient.Get(ctx, path, &response); err != nil {
		logger.Get().Error().
			Err(err).
			Str("node", node).
			Msg("Failed to get network interfaces from Proxmox API (resty)")
		return nil, fmt.Errorf("failed to get network interfaces: %w", err)
	}

	// Filter for bridge interfaces only
	var bridges []VMBR
	for _, iface := range response.Data {
		if iface.Type == "bridge" {
			bridges = append(bridges, iface)
		}
	}

	logger.Get().Info().
		Str("node", node).
		Int("total_interfaces", len(response.Data)).
		Int("bridge_interfaces", len(bridges)).
		Msg("Successfully fetched network interfaces (resty)")

	return bridges, nil
}

// GetAllNetworkInterfacesResty fetches all network interfaces (not just bridges) from a node using resty.
// This is useful if you need to see all interface types (bond, eth, vlan, etc.)
func GetAllNetworkInterfacesResty(ctx context.Context, restyClient *RestyClient, node string) ([]VMBR, error) {
	if node == "" {
		return nil, fmt.Errorf("node name cannot be empty")
	}

	path := fmt.Sprintf("/nodes/%s/network", url.PathEscape(node))

	var response ListResponse[VMBR]
	if err := restyClient.Get(ctx, path, &response); err != nil {
		logger.Get().Error().
			Err(err).
			Str("node", node).
			Msg("Failed to get all network interfaces from Proxmox API (resty)")
		return nil, fmt.Errorf("failed to get all network interfaces: %w", err)
	}

	logger.Get().Info().
		Str("node", node).
		Int("total_interfaces", len(response.Data)).
		Msg("Successfully fetched all network interfaces (resty)")

	return response.Data, nil
}
