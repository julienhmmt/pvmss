package proxmox

import (
	"context"
	"fmt"
	"strings"

	"pvmss/logger"
)

// VMInfo is a simplified, application-specific struct that holds curated information about a Virtual Machine.
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

// VM represents a Proxmox virtual machine
type VM struct {
	CPU     float64 `json:"cpu"`
	CPUs    int     `json:"cpus"`
	MaxDisk int64   `json:"maxdisk"`
	MaxMem  int64   `json:"maxmem"`
	Mem     int64   `json:"mem"`
	Name    string  `json:"name"`
	Node    string  `json:"node"`
	Status  string  `json:"status"`
	Uptime  int64   `json:"uptime"`
	VMID    int     `json:"vmid"`
}

// GetVMsWithContext retrieves a comprehensive list of all VMs across all available Proxmox nodes.
// It first fetches the list of nodes and then iterates through them, calling GetVMsForNodeWithContext for each.
func GetVMsWithContext(ctx context.Context, client ClientInterface) ([]VM, error) {
	logger.Get().Info().Msg("Fetching all VMs from Proxmox")

	// Get all nodes first
	nodes, err := GetNodeNamesWithContext(ctx, client)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get node list while fetching VMs")
		return nil, fmt.Errorf("failed to get node list: %w", err)
	}

	// Collect VMs from all nodes
	allVMs := make([]VM, 0)

	for _, node := range nodes {
		logger.Get().Info().Str("node", node).Msg("Fetching VMs for node")
		nodeVMs, err := GetVMsForNodeWithContext(ctx, client, node)
		if err != nil {
			logger.Get().Warn().Err(err).Str("node", node).Msg("Failed to get VMs for node")
			continue
		}
		allVMs = append(allVMs, nodeVMs...)
	}

	logger.Get().Info().Int("total_vms", len(allVMs)).Msg("Successfully fetched all VMs")
	return allVMs, nil
}

// GetVMsForNodeWithContext fetches all VMs located on a single, specified Proxmox node.
// It calls the `/nodes/{nodeName}/qemu` endpoint and enriches the returned VM data with the node's name.
func GetVMsForNodeWithContext(ctx context.Context, client ClientInterface, nodeName string) ([]VM, error) {
	path := fmt.Sprintf("/nodes/%s/qemu", nodeName)

	// Use the new GetJSON method to directly unmarshal into our typed response
	var response ListResponse[VM]
	if err := client.GetJSON(ctx, path, &response); err != nil {
		logger.Get().Error().Err(err).Str("node", nodeName).Msg("Failed to get VMs for node from Proxmox API")
		return nil, fmt.Errorf("failed to get VMs for node %s: %w", nodeName, err)
	}

	// Set the node name for each VM
	for i := range response.Data {
		response.Data[i].Node = nodeName
	}

	logger.Get().Debug().Str("node", nodeName).Int("count", len(response.Data)).Msg("Fetched VMs for node")
	return response.Data, nil
}

// GetVmList is a backward compatibility wrapper. It calls GetVMsWithContext and then wraps
// the resulting slice of VMs into a map with a "data" key to match an older, expected response structure.
func GetVmList(client ClientInterface, ctx context.Context) (map[string]interface{}, error) {
	vms, err := GetVMsWithContext(ctx, client)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get VMs in GetVmList")
		return nil, err
	}

	// Convert to a slice of interfaces for backward compatibility
	vmsInterface := make([]interface{}, len(vms))
	for i, vm := range vms {
		vmsInterface[i] = vm
	}

	result := map[string]interface{}{
		"data": vmsInterface,
	}

	logger.Get().Info().Int("vm_count", len(vms)).Msg("Returning VM list result")
	return result, nil
}

// GetNextVMID determines the next available unique ID for a new VM.
// It fetches all existing VMs, finds the highest current VMID, and returns that value incremented by one.
func GetNextVMID(ctx context.Context, client ClientInterface) (int, error) {
	vms, err := GetVMsWithContext(ctx, client)
	if err != nil {
		return 0, fmt.Errorf("failed to get VMs to calculate next VMID: %w", err)
	}

	highestVMID := 0
	for _, vm := range vms {
		if vm.VMID > highestVMID {
			highestVMID = vm.VMID
		}
	}

	return highestVMID + 1, nil
}

// VMActionWithContext performs a lifecycle action on a VM via the Proxmox API.
// Supported actions map to the following endpoints:
//
//	POST /nodes/{node}/qemu/{vmid}/status/{action}
//
// Where action is one of: start, stop, shutdown, reboot, reset
// Returns the UPID string on success (for async tasks), or an empty string when not applicable.
func VMActionWithContext(ctx context.Context, client ClientInterface, node string, vmid string, action string) (string, error) {
	// Validate action
	switch action {
	case "start", "stop", "shutdown", "reboot", "reset":
	default:
		return "", fmt.Errorf("unsupported VM action: %s", action)
	}

	path := fmt.Sprintf("/nodes/%s/qemu/%s/status/%s", node, vmid, action)

	// Proxmox typically responds with {"data":"UPID:..."}
	raw, err := client.PostFormWithContext(ctx, path, map[string]string{})
	if err != nil {
		logger.Get().Error().Err(err).Str("node", node).Str("vmid", vmid).Str("action", action).Msg("VM action failed")
		return "", err
	}

	// Extract UPID in a tolerant way without strict JSON coupling
	s := string(raw)
	// common response: {"data":"UPID:..."}
	startIdx := strings.Index(s, "UPID:")
	if startIdx >= 0 {
		// trim until closing quote or brace
		endIdx := strings.IndexAny(s[startIdx:], "\"}\n")
		if endIdx > 0 {
			return s[startIdx : startIdx+endIdx], nil
		}
		return s[startIdx:], nil
	}
	return "", nil
}
