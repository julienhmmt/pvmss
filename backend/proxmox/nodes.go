package proxmox

import (
	"context"
	"fmt"
	"net/url"
	"sort"

	"pvmss/logger"
)

// NodeDetails is a simplified, application-specific struct that holds curated information about a Proxmox node,
// such as its status, resource usage, and hardware specifications.
type NodeDetails struct {
	Node      string  `json:"node"`
	Status    string  `json:"status"`
	CPU       float64 `json:"cpu"`
	MaxCPU    int     `json:"maxcpu"`
	Sockets   int     `json:"sockets"`
	Memory    float64 `json:"memory"`
	MaxMemory float64 `json:"maxmemory"`
	Disk      float64 `json:"disk"`
	MaxDisk   float64 `json:"maxdisk"`
	Uptime    int64   `json:"uptime,omitempty"`
}

// NodeStatus represents the complex, nested structure of the raw JSON response from the Proxmox API's
// `/nodes/{node}/status` endpoint. It is used for unmarshalling the direct API output.
type NodeStatus struct {
	Data struct {
		CPU     float64 `json:"cpu"`
		Uptime  int64   `json:"uptime"`
		CPUInfo struct {
			Cores   int `json:"cores"`
			Sockets int `json:"sockets"`
			Cpus    int `json:"cpus"`
		} `json:"cpuinfo"`
		Memory struct {
			Total int64 `json:"total"`
			Used  int64 `json:"used"`
		} `json:"memory"`
		RootFS struct {
			Total int64 `json:"total"`
			Used  int64 `json:"used"`
		} `json:"rootfs"`
	} `json:"data"`
}

// NodeInfo represents a Proxmox node in the node list
// This is a simplified version of the full node information
type NodeInfo struct {
	Node   string `json:"node"`
	Status string `json:"status"`
	Type   string `json:"type"`
}

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

	// Sort node names alphabetically for consistent ordering
	sort.Strings(nodeNames)

	logger.Get().Info().
		Int("count", len(nodeNames)).
		Strs("nodes", nodeNames).
		Msg("Successfully fetched node names (resty)")

	return nodeNames, nil
}

// GetOnlineNodeNamesResty retrieves the list of online Proxmox nodes only using resty
// This filters out offline/down nodes to prevent API errors during VM creation
func GetOnlineNodeNamesResty(ctx context.Context, client *RestyClient) ([]string, error) {
	var response ListResponse[NodeInfo]

	if err := client.Get(ctx, "/nodes", &response); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get node list from Proxmox API (resty)")
		return nil, fmt.Errorf("failed to get node list: %w", err)
	}

	// Extract only online node names
	onlineNodeNames := make([]string, 0, len(response.Data))
	for _, node := range response.Data {
		if node.Status == "online" {
			onlineNodeNames = append(onlineNodeNames, node.Node)
		} else {
			logger.Get().Info().
				Str("node", node.Node).
				Str("status", node.Status).
				Msg("Skipping offline node for VM operations")
		}
	}

	// Sort online node names alphabetically for consistent ordering
	sort.Strings(onlineNodeNames)

	logger.Get().Info().
		Int("count", len(onlineNodeNames)).
		Strs("nodes", onlineNodeNames).
		Msg("Successfully fetched online node names (resty)")

	return onlineNodeNames, nil
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
