package proxmox

import (
	"context"
	"fmt"
	"net/url"

	"pvmss/logger"
)

// VMBR represents a network interface in Proxmox
type VMBR struct {
	Iface       string `json:"iface"`
	IfaceName   string `json:"name"`
	Type        string `json:"type"`
	Method      string `json:"method"`
	Address     string `json:"address"`
	Netmask     string `json:"netmask"`
	Gateway     string `json:"gateway"`
	BridgePorts string `json:"bridge_ports"`
	Comments    string `json:"comments"`
	Active      any    `json:"active"`
	BridgeFD    any    `json:"bridge_fd"`
	BridgeSTP   any    `json:"bridge_stp"`
}

// GetVMBRList fetches network bridge information from a specific Proxmox node.
func GetVMBRs(client ClientInterface, node string) ([]VMBR, error) {
	if node == "" {
		return nil, fmt.Errorf("node name cannot be empty")
	}
	logger.Get().Info().Str("node", node).Msg("Fetching VMBRs from Proxmox")
	ctx, cancel := context.WithTimeout(context.Background(), client.GetTimeout())
	defer cancel()
	return GetVMBRsWithContext(ctx, client, node)
}

// GetVMBRsWithContext fetches the list of network interfaces from the `/nodes/{node}/network` endpoint
// of the Proxmox API using the provided context for timeout and cancellation control.
func GetVMBRsWithContext(ctx context.Context, client ClientInterface, node string) ([]VMBR, error) {
	if node == "" {
		return nil, fmt.Errorf("node name cannot be empty")
	}

	logger.Get().Info().Str("node", node).Msg("Fetching VMBRs with context from Proxmox")

	path := fmt.Sprintf("/nodes/%s/network", url.PathEscape(node))

	// Use the new GetJSON method to directly unmarshal into our typed response
	var response ListResponse[VMBR]
	if err := client.GetJSON(ctx, path, &response); err != nil {
		logger.Get().Error().
			Err(err).
			Str("node", node).
			Msg("Failed to get network interfaces from Proxmox API")
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
		Msg("Successfully fetched network interfaces")

	return bridges, nil
}
