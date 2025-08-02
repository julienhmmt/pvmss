package proxmox

import (
	"context"
	"fmt"
	"time"

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

// GetNodeDetails is a convenience function that retrieves hardware details for a specific node.
// It calls GetNodeDetailsWithContext using a default timeout.
func GetNodeDetails(client ClientInterface, nodeName string) (*NodeDetails, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return GetNodeDetailsWithContext(ctx, client, nodeName)
}

// GetNodeDetailsWithContext fetches the status of a specific node from the Proxmox API using the provided context.
// It then unmarshals the raw response into the NodeStatus struct and maps the relevant data
// into the cleaner, application-friendly NodeDetails struct.
func GetNodeDetailsWithContext(ctx context.Context, client ClientInterface, nodeName string) (*NodeDetails, error) {
	logger.Get().Info().Str("node", nodeName).Msg("Fetching node details")

	// Get node status from Proxmox API
	path := fmt.Sprintf("/nodes/%s/status", nodeName)
	var status NodeStatus
	if err := client.GetJSON(ctx, path, &status); err != nil {
		logger.Get().Error().Err(err).Str("node", nodeName).Msg("Failed to get node status from Proxmox API")
		return nil, fmt.Errorf("failed to get node status for %s: %w", nodeName, err)
	}

	// Log the response for debugging
	logger.Get().Debug().
		Str("node", nodeName).
		Float64("cpu", status.Data.CPU).
		Int64("memory_used", status.Data.Memory.Used).
		Int64("memory_total", status.Data.Memory.Total).
		Msg("Node status response")

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

	// Use the logical core count from cpuinfo.cpus
	if status.Data.CPUInfo.Cpus > 0 {
		details.MaxCPU = status.Data.CPUInfo.Cpus
	} else {
		// Fallback for older Proxmox versions or unexpected API responses
		details.MaxCPU = status.Data.CPUInfo.Cores * status.Data.CPUInfo.Sockets
	}

	// If sockets are not reported, default to 1
	if details.Sockets == 0 {
		details.Sockets = 1
	}

	// Log the final computed details that will be sent to the frontend
	logger.Get().Info().
		Str("node", details.Node).
		Int("sockets", details.Sockets).
		Int("final_max_cpu", details.MaxCPU).
		Float64("final_memory_bytes", details.Memory).
		Float64("final_max_memory_bytes", details.MaxMemory).
		Float64("final_disk_bytes", details.Disk).
		Float64("final_max_disk_bytes", details.MaxDisk).
		Msg("Final computed node details")

	return details, nil
}

// GetNodeNames is a convenience function that retrieves the names of all available Proxmox nodes.
// It calls GetNodeNamesWithContext using the client's default timeout.
func GetNodeNames(client ClientInterface) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), client.GetTimeout())
	defer cancel()

	return GetNodeNamesWithContext(ctx, client)
}

// NodeInfo represents a Proxmox node in the node list
// This is a simplified version of the full node information
type NodeInfo struct {
	Node   string `json:"node"`
	Status string `json:"status"`
	Type   string `json:"type"`
}

// GetNodeNamesWithContext fetches the list of all configured nodes from the `/nodes` endpoint of the Proxmox API.
// It parses the response to extract and return a simple slice of node names.
func GetNodeNamesWithContext(ctx context.Context, client ClientInterface) ([]string, error) {
	logger.Get().Info().Msg("Fetching node names")

	// Use the new GetJSON method to directly unmarshal into our typed response
	var response ListResponse[NodeInfo]
	if err := client.GetJSON(ctx, "/nodes", &response); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get node list from Proxmox API")
		return nil, fmt.Errorf("failed to get node list: %w", err)
	}

	// Extract node names
	nodeNames := make([]string, 0, len(response.Data))
	for _, node := range response.Data {
		nodeNames = append(nodeNames, node.Node)
	}

	logger.Get().Info().Int("count", len(nodeNames)).Msg("Successfully fetched node names")
	return nodeNames, nil
}

// GetNodeStatus provides a simple health check for a given node.
// It attempts to fetch the node's status and returns 'online' on success or 'offline' on failure.
func GetNodeStatus(client ClientInterface, nodeName string) (string, error) {
	// Use a short timeout since this is a health check
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var status NodeStatus
	err := client.GetJSON(ctx, fmt.Sprintf("/nodes/%s/status", nodeName), &status)
	if err != nil {
		logger.Get().Debug().
			Err(err).
			Str("node", nodeName).
			Msg("Node status check failed, marking as offline")
		return "offline", nil // Return offline without error for connection issues
	}

	logger.Get().Debug().
		Str("node", nodeName).
		Msg("Node status check successful, marking as online")

	return "online", nil
}
