package proxmox

import (
	"context"
	"fmt"
	"net/url"

	"pvmss/logger"
)

// GetNodeNamesResty retrieves the list of all Proxmox nodes using resty
func GetNodeNamesResty(ctx context.Context, client *RestyClient) ([]string, error) {
	var response ListResponse[NodeInfo]

	if err := client.Get(ctx, "/nodes", &response); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get node list from Proxmox API (resty)")
		return nil, fmt.Errorf("failed to get node list: %w", err)
	}

	// Extract node names
	nodeNames := make([]string, 0, len(response.Data))
	for _, node := range response.Data {
		nodeNames = append(nodeNames, node.Node)
	}

	logger.Get().Info().
		Int("count", len(nodeNames)).
		Strs("nodes", nodeNames).
		Msg("Successfully fetched node names (resty)")

	return nodeNames, nil
}

// GetNodeDetailsResty fetches detailed information about a specific node using resty
func GetNodeDetailsResty(ctx context.Context, client *RestyClient, nodeName string) (*NodeDetails, error) {
	// Get node status from Proxmox API
	path := fmt.Sprintf("/nodes/%s/status", url.PathEscape(nodeName))
	var status NodeStatus

	if err := client.Get(ctx, path, &status); err != nil {
		logger.Get().Error().
			Err(err).
			Str("node", nodeName).
			Msg("Failed to get node status from Proxmox API (resty)")
		return nil, fmt.Errorf("failed to get node status for %s: %w", nodeName, err)
	}

	// Log the response for debugging
	logger.Get().Debug().
		Str("node", nodeName).
		Float64("cpu", status.Data.CPU).
		Int64("memory_used", status.Data.Memory.Used).
		Int64("memory_total", status.Data.Memory.Total).
		Msg("Node status response (resty)")

	// Map to our internal NodeDetails struct
	details := &NodeDetails{
		Node:      nodeName,
		Status:    "online",
		CPU:       status.Data.CPU,
		Sockets:   status.Data.CPUInfo.Sockets,
		Memory:    float64(status.Data.Memory.Used),
		MaxMemory: float64(status.Data.Memory.Total),
		Disk:      float64(status.Data.RootFS.Used),
		MaxDisk:   float64(status.Data.RootFS.Total),
		Uptime:    status.Data.Uptime,
	}

	// Determine MaxCPU, preferring the total logical core count 'cpus' when available
	details.MaxCPU = status.Data.CPUInfo.Cpus
	if details.MaxCPU == 0 {
		// Fallback for older Proxmox versions: calculate from cores and sockets
		details.MaxCPU = status.Data.CPUInfo.Cores * status.Data.CPUInfo.Sockets
	}

	// If sockets are not reported, default to 1
	if details.Sockets == 0 {
		details.Sockets = 1
	}

	// Log the final computed details
	logger.Get().Info().
		Str("node", details.Node).
		Int("sockets", details.Sockets).
		Int("final_max_cpu", details.MaxCPU).
		Float64("final_memory_bytes", details.Memory).
		Float64("final_max_memory_bytes", details.MaxMemory).
		Float64("final_disk_bytes", details.Disk).
		Float64("final_max_disk_bytes", details.MaxDisk).
		Msg("Final computed node details (resty)")

	return details, nil
}
