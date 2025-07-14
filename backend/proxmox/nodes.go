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
	Memory    int64   `json:"memory"`
	MaxMemory int64   `json:"maxmemory"`
	Disk      int64   `json:"disk"`
	MaxDisk   int64   `json:"maxdisk"`
	Uptime    int64   `json:"uptime,omitempty"`
}

// NodeResponse represents the API response structure for node status
type NodeResponse struct {
	Data struct {
		Node  string             `json:"node"`
		CPU   float64            `json:"cpu"`
		CPUs  float64            `json:"cpus"`
		Uptime int64             `json:"uptime,omitempty"`
		Memory map[string]float64 `json:"memory"`
		Rootfs map[string]float64 `json:"rootfs"`
	} `json:"data"`
}

// GetNodeDetails retrieves the hardware details for a specific node with context support.
func GetNodeDetails(client *Client, nodeName string) (*NodeDetails, error) {
	ctx, cancel := context.WithTimeout(context.Background(), client.Timeout)
	defer cancel()

	return GetNodeDetailsWithContext(ctx, client, nodeName)
}

// GetNodeDetailsWithContext retrieves the hardware details for a specific node using the provided context.
func GetNodeDetailsWithContext(ctx context.Context, client *Client, nodeName string) (*NodeDetails, error) {
	if nodeName == "" {
		return nil, fmt.Errorf("node name cannot be empty")
	}

	// We'll use our custom implementation with context support
	log.Debug().Str("node", nodeName).Msg("Getting node details")

	// Use our custom implementation with context support
	status, err := client.GetWithContext(ctx, fmt.Sprintf("/nodes/%s/status", nodeName))
	if err != nil {
		return nil, fmt.Errorf("failed to get status for node %s: %w", nodeName, err)
	}

	// Parse the response into a structured format
	var nodeResp NodeResponse
	respBytes, err := json.Marshal(status)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal node status data: %w", err)
	}

	if err := json.Unmarshal(respBytes, &nodeResp); err != nil {
		// Fallback to manual extraction if structured parsing fails
		log.Debug().Err(err).Msg("Failed to unmarshal node response, falling back to manual extraction")
		return extractNodeDetailsManually(status, nodeName)
	}

	// Create and populate the node details
	details := &NodeDetails{
		Node:      nodeResp.Data.Node,
		Status:    "online", // Default status
		CPU:       nodeResp.Data.CPU,
		MaxCPU:    int(nodeResp.Data.CPUs),
		Memory:    int64(nodeResp.Data.Memory["used"]),
		MaxMemory: int64(nodeResp.Data.Memory["total"]),
		Disk:      int64(nodeResp.Data.Rootfs["used"]),
		MaxDisk:   int64(nodeResp.Data.Rootfs["total"]),
		Uptime:    nodeResp.Data.Uptime,
	}

	// If node name is empty, use the provided one
	if details.Node == "" {
		details.Node = nodeName
	}

	return details, nil
}

// extractNodeDetailsManually extracts node details from the raw API response map.
func extractNodeDetailsManually(status map[string]interface{}, nodeName string) (*NodeDetails, error) {
	// The result is a map. We need to extract the data.
	data, ok := status["data"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("failed to parse node status data for node %s", nodeName)
	}

	details := &NodeDetails{}

	if node, ok := data["node"].(string); ok {
		details.Node = node
	} else {
		// Fallback to the provided nodeName if not in the response
		details.Node = nodeName
	}

	if cpus, ok := data["cpus"].(float64); ok {
		details.MaxCPU = int(cpus)
	}

	if mem, ok := data["memory"].(map[string]interface{}); ok {
		if total, ok := mem["total"].(float64); ok {
			details.MaxMemory = int64(total)
		}
		if used, ok := mem["used"].(float64); ok {
			details.Memory = int64(used)
		}
	}

	if disk, ok := data["rootfs"].(map[string]interface{}); ok {
		if total, ok := disk["total"].(float64); ok {
			details.MaxDisk = int64(total)
		}
		if used, ok := disk["used"].(float64); ok {
			details.Disk = int64(used)
		}
	}
	
	// Default to online status
	details.Status = "online"
	if cpu, ok := data["cpu"].(float64); ok {
		details.CPU = cpu
	}

	if uptime, ok := data["uptime"].(float64); ok {
		details.Uptime = int64(uptime)
	}

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
