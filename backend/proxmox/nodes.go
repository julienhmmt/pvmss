package proxmox

import (
	"fmt"

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
}

// GetNodeDetails retrieves the hardware details for a specific node.
func GetNodeDetails(client *Client, nodeName string) (*NodeDetails, error) {
	// Use the generic Get method to fetch the status for a specific node.
	status, err := client.Get(fmt.Sprintf("/nodes/%s/status", nodeName))
	if err != nil {
		return nil, fmt.Errorf("failed to get status for node %s: %w", nodeName, err)
	}

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

	return details, nil
}

// GetNodeNames retrieves all node names from Proxmox.
func GetNodeNames(client *Client) ([]string, error) {
	// Use the generic Get method to fetch the list of all nodes.
	nodeList, err := client.Get("/nodes")
	if err != nil {
		return nil, fmt.Errorf("failed to get node list: %w", err)
	}

	var nodeNames []string
	// The 'data' key contains a slice of interfaces, where each interface is a map.
	if data, ok := nodeList["data"].([]interface{}); ok {
		log.Info().Int("count", len(data)).Msg("Found nodes in API response")
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
