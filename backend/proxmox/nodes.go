package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"
)

// NodeDetails contains detailed information about a Proxmox node.
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

// NodeStatus represents the full API response structure for node status
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

// GetNodeDetails retrieves the hardware details for a specific node with context support.
func GetNodeDetails(client *Client, nodeName string) (*NodeDetails, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return GetNodeDetailsWithContext(ctx, client, nodeName)
}

// GetNodeDetailsWithContext retrieves the hardware details for a specific node with context support.
func GetNodeDetailsWithContext(ctx context.Context, client *Client, nodeName string) (*NodeDetails, error) {
	// Get node status from Proxmox API
	status, err := client.GetWithContext(ctx, fmt.Sprintf("/nodes/%s/status", nodeName))
	if err != nil {
		return nil, fmt.Errorf("failed to get node status for %s: %w", nodeName, err)
	}

	// Log the raw response for debugging
	rawResponse, _ := json.Marshal(status)
	log.Debug().RawJSON("raw_api_response", rawResponse).Msg("Raw Proxmox API response for node status")

	// Marshal the map to JSON bytes
	jsonBytes, err := json.Marshal(status)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal node status: %w", err)
	}

	// Unmarshal into our NodeStatus struct
	var nodeStatus NodeStatus
	if err := json.Unmarshal(jsonBytes, &nodeStatus); err != nil {
		return nil, fmt.Errorf("failed to unmarshal node status: %w", err)
	}

	// Map to our internal NodeDetails struct
	details := &NodeDetails{
		Node:      nodeName,
		Status:    "online",
		CPU:       nodeStatus.Data.CPU,
		Sockets:   nodeStatus.Data.CPUInfo.Sockets,
		Memory:    float64(nodeStatus.Data.Memory.Used),
		MaxMemory: float64(nodeStatus.Data.Memory.Total),
		Disk:      float64(nodeStatus.Data.RootFS.Used),
		MaxDisk:   float64(nodeStatus.Data.RootFS.Total),
		Uptime:    nodeStatus.Data.Uptime,
	}

	// Use the logical core count from cpuinfo.cpus
	if nodeStatus.Data.CPUInfo.Cpus > 0 {
		details.MaxCPU = nodeStatus.Data.CPUInfo.Cpus
	} else {
		// Fallback for older Proxmox versions or unexpected API responses
		details.MaxCPU = nodeStatus.Data.CPUInfo.Cores * nodeStatus.Data.CPUInfo.Sockets
	}

	// If sockets are not reported, default to 1
	if details.Sockets == 0 {
		details.Sockets = 1
	}

	// Log the final computed details that will be sent to the frontend
	log.Info().
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

// GetNodeNames retrieves all node names from Proxmox with context support.
func GetNodeNames(client *Client) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), client.Timeout)
	defer cancel()

	return GetNodeNamesWithContext(ctx, client)
}

// GetNodeNamesWithContext retrieves all node names from Proxmox using the provided context.
func GetNodeNamesWithContext(ctx context.Context, client *Client) ([]string, error) {
	// Using our custom implementation with context support
	log.Debug().Msg("Getting node names")

	// Use the generic Get method to fetch the list of all nodes.
	nodeList, err := client.GetWithContext(ctx, "/nodes")
	if err != nil {
		return nil, fmt.Errorf("failed to get node list: %w", err)
	}

	var nodeNames []string
	// The 'data' key contains a slice of interfaces, where each interface is a map.
	if data, ok := nodeList["data"].([]interface{}); ok {
		log.Info().Int("count", len(data)).Msg("Found nodes in API response")
		nodeNames = make([]string, 0, len(data))
		
		for _, item := range data {
			if nodeItem, ok := item.(map[string]interface{}); ok {
				if name, ok := nodeItem["node"].(string); ok {
					nodeNames = append(nodeNames, name)
				} else {
					log.Warn().Interface("item", nodeItem).Msg("Node item found but 'node' key is not a string or is missing")
				}
			} else {
				log.Warn().Interface("item", item).Msg("Item in node list data is not a map[string]interface{}")
			}
		}
	} else {
		log.Error().Interface("response", nodeList).Msg("Failed to parse node list data: 'data' key is not a slice or is missing")
		return nil, fmt.Errorf("failed to parse node list data")
	}

	return nodeNames, nil
}

// GetNodeStatus checks if a node is online and returns its status.
func GetNodeStatus(client *Client, nodeName string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Try to get node details
	_, err := client.GetWithContext(ctx, fmt.Sprintf("/nodes/%s/status", nodeName))
	if err != nil {
		return "offline", nil
	}

	return "online", nil
}
