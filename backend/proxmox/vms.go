package proxmox

import (
	"context"
	"fmt"

	"github.com/rs/zerolog/log"
)

// VMInfo represents basic information about a VM
type VMInfo struct {
	VMID     string `json:"vmid"`
	Name     string `json:"name"`
	Status   string `json:"status"`
	Node     string `json:"node"`
	CPU      int    `json:"cpu"`
	Memory   int64  `json:"memory"`
	Disk     int64  `json:"disk"`
	Template bool   `json:"template"`
}

// GetVMsWithContext retrieves all VMs from Proxmox using the provided context
func GetVMsWithContext(ctx context.Context, client *Client) ([]map[string]interface{}, error) {
	log.Debug().Msg("Getting VM list with context")
	
	// Get all nodes first
	nodes, err := GetNodeNamesWithContext(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("failed to get node list: %w", err)
	}
	
	// Collect VMs from all nodes
	vms := make([]map[string]interface{}, 0)
	
	for _, node := range nodes {
		nodeVMs, err := GetVMsForNodeWithContext(ctx, client, node)
		if err != nil {
			log.Warn().Err(err).Str("node", node).Msg("Failed to get VMs for node")
			continue
		}
		vms = append(vms, nodeVMs...)
	}
	
	return vms, nil
}

// GetVMsForNodeWithContext retrieves VMs for a specific node using the provided context
func GetVMsForNodeWithContext(ctx context.Context, client *Client, nodeName string) ([]map[string]interface{}, error) {
	path := fmt.Sprintf("/nodes/%s/qemu", nodeName)
	
	response, err := client.GetWithContext(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("failed to get VMs for node %s: %w", nodeName, err)
	}
	
	// Extract data from response
	data, ok := response["data"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format for VMs on node %s", nodeName)
	}
	
	// Convert to the expected format
	vms := make([]map[string]interface{}, 0, len(data))
	for _, item := range data {
		if vmData, ok := item.(map[string]interface{}); ok {
			// Add node information to each VM
			vmData["node"] = nodeName
			vms = append(vms, vmData)
		}
	}
	
	return vms, nil
}

// GetVmList is a backward compatibility wrapper for GetVMsWithContext
func GetVmList(c *Client, ctx context.Context) (map[string]interface{}, error) {
	vms, err := GetVMsWithContext(ctx, c)
	if err != nil {
		return nil, err
	}
	
	result := map[string]interface{}{
		"data": vms,
	}
	
	return result, nil
}
