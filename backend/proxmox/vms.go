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
	log.Info().Msg("Fetching all VMs from Proxmox")
	log.Debug().Msg("Getting VM list with context")
	
	// Get all nodes first
	nodes, err := GetNodeNamesWithContext(ctx, client)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get node list while fetching VMs")
		return nil, fmt.Errorf("failed to get node list: %w", err)
	}
	
	// Collect VMs from all nodes
	vms := make([]map[string]interface{}, 0)
	
	for _, node := range nodes {
		log.Info().Str("node", node).Msg("Fetching VMs for node")
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
		log.Error().Err(err).Str("node", nodeName).Msg("Failed to get VMs for node from Proxmox API")
		return nil, fmt.Errorf("failed to get VMs for node %s: %w", nodeName, err)
	}
	
	// Extract data from response
	data, ok := response["data"].([]interface{})
	if !ok {
		log.Error().Str("node", nodeName).Msg("Unexpected response format for VMs on node")
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
		log.Error().Err(err).Msg("Failed to get VMs in GetVmList")
		return nil, err
	}
	
	result := map[string]interface{}{
		"data": vms,
	}
	
	log.Info().Int("vm_count", len(vms)).Msg("Returning VM list result")
	return result, nil
}

// GetNextVMID calculates the next available VMID.
func GetNextVMID(ctx context.Context, client *Client) (int, error) {
	vms, err := GetVMsWithContext(ctx, client)
	if err != nil {
		return 0, fmt.Errorf("failed to get VMs to calculate next VMID: %w", err)
	}

	highestVMID := 0
	for _, vm := range vms {
		if vmidFloat, ok := vm["vmid"].(float64); ok {
			vmid := int(vmidFloat)
			if vmid > highestVMID {
				highestVMID = vmid
			}
		}
	}

	return highestVMID + 1, nil
}
