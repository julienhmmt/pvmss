package proxmox

import (
	"context"
	"fmt"
	"net/url"

	"pvmss/logger"
)

// GetVMsResty retrieves a comprehensive list of all VMs across all available Proxmox nodes using resty.
// It first fetches the list of nodes and then iterates through them, calling GetVMsForNodeResty for each.
func GetVMsResty(ctx context.Context, restyClient *RestyClient) ([]VM, error) {
	// Get all nodes first
	nodes, err := GetNodeNamesResty(ctx, restyClient)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get node list while fetching VMs (resty)")
		return nil, fmt.Errorf("failed to get node list: %w", err)
	}

	// Collect VMs from all nodes
	allVMs := make([]VM, 0)

	for _, node := range nodes {
		nodeVMs, err := GetVMsForNodeResty(ctx, restyClient, node)
		if err != nil {
			logger.Get().Warn().Err(err).Str("node", node).Msg("Failed to get VMs for node (resty)")
			continue
		}
		allVMs = append(allVMs, nodeVMs...)
	}

	logger.Get().Info().Int("total_vms", len(allVMs)).Msg("Successfully fetched all VMs (resty)")
	return allVMs, nil
}

// GetVMsForNodeResty fetches all VMs located on a single, specified Proxmox node using resty.
// It calls the `/nodes/{nodeName}/qemu` endpoint and enriches the returned VM data with the node's name.
func GetVMsForNodeResty(ctx context.Context, restyClient *RestyClient, nodeName string) ([]VM, error) {
	path := fmt.Sprintf("/nodes/%s/qemu", url.PathEscape(nodeName))

	var response ListResponse[VM]
	if err := restyClient.Get(ctx, path, &response); err != nil {
		logger.Get().Error().Err(err).Str("node", nodeName).Msg("Failed to get VMs for node from Proxmox API (resty)")
		return nil, fmt.Errorf("failed to get VMs for node %s: %w", nodeName, err)
	}

	// Set the node name for each VM
	for i := range response.Data {
		response.Data[i].Node = nodeName
	}

	logger.Get().Debug().Str("node", nodeName).Int("count", len(response.Data)).Msg("Fetched VMs for node (resty)")
	return response.Data, nil
}

// GetVMConfigResty fetches the VM configuration from Proxmox using resty:
// GET /nodes/{node}/qemu/{vmid}/config
// It returns the raw "data" map as provided by the API so callers can extract
// fields such as description, tags, and network interfaces (net0/net1...).
func GetVMConfigResty(ctx context.Context, restyClient *RestyClient, node string, vmid int) (map[string]interface{}, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid)

	var resp struct {
		Data map[string]interface{} `json:"data"`
	}

	if err := restyClient.Get(ctx, path, &resp); err != nil {
		logger.Get().Error().Err(err).Str("node", node).Int("vmid", vmid).Msg("Failed to get VM config (resty)")
		return nil, fmt.Errorf("failed to get config for vm %d on node %s: %w", vmid, node, err)
	}

	return resp.Data, nil
}

// GetVMCurrentResty fetches the current runtime metrics for a VM using resty
// GET /nodes/{node}/qemu/{vmid}/status/current
func GetVMCurrentResty(ctx context.Context, restyClient *RestyClient, node string, vmid int) (*VMCurrent, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/current", url.PathEscape(node), vmid)

	var resp Response[VMCurrent]
	if err := restyClient.Get(ctx, path, &resp); err != nil {
		logger.Get().Error().Err(err).Str("node", node).Int("vmid", vmid).Msg("Failed to get current VM status (resty)")
		return nil, fmt.Errorf("failed to get current status for vm %d on node %s: %w", vmid, node, err)
	}

	return &resp.Data, nil
}

// UpdateVMConfigResty updates VM configuration fields (e.g., description, tags) using resty
// by POSTing form parameters to:
//
//	POST /nodes/{node}/qemu/{vmid}/config
//
// Params may include keys like "description" and "tags" (semicolon-separated).
func UpdateVMConfigResty(ctx context.Context, restyClient *RestyClient, node string, vmid int, params map[string]string) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid)

	values := make(url.Values)
	for k, v := range params {
		values.Set(k, v)
	}

	var response interface{}
	if err := restyClient.Post(ctx, path, values, &response); err != nil {
		logger.Get().Error().Err(err).Str("node", node).Int("vmid", vmid).Msg("Failed to update VM config (resty)")
		return fmt.Errorf("failed to update config for vm %d on node %s: %w", vmid, node, err)
	}

	logger.Get().Info().Str("node", node).Int("vmid", vmid).Msg("VM config updated successfully (resty)")
	return nil
}

// VMActionResty performs a lifecycle action on a VM via the Proxmox API using resty.
// Supported actions map to the following endpoints:
//
//	POST /nodes/{node}/qemu/{vmid}/status/{action}
//
// Where action is one of: start, stop, shutdown, reboot, reset
// Returns the UPID string on success (for async tasks), or an empty string when not applicable.
func VMActionResty(ctx context.Context, restyClient *RestyClient, node string, vmid string, action string) (string, error) {
	// Validate action
	switch action {
	case "start", "stop", "shutdown", "reboot", "reset":
	default:
		return "", fmt.Errorf("unsupported VM action: %s", action)
	}

	path := fmt.Sprintf("/nodes/%s/qemu/%s/status/%s", url.PathEscape(node), url.PathEscape(vmid), action)

	var response Response[string]
	if err := restyClient.Post(ctx, path, url.Values{}, &response); err != nil {
		logger.Get().Error().Err(err).Str("node", node).Str("vmid", vmid).Str("action", action).Msg("VM action failed (resty)")
		return "", err
	}

	// The task ID (UPID) is returned in the 'data' field.
	if response.Data == "" {
		return "", fmt.Errorf("did not receive a task ID from Proxmox for action '%s' on VM %s", action, vmid)
	}

	logger.Get().Info().Str("node", node).Str("vmid", vmid).Str("action", action).Str("upid", response.Data).Msg("VM action executed (resty)")
	return response.Data, nil
}

// DeleteVMResty deletes a VM from Proxmox using resty.
// This performs a DELETE request to /nodes/{node}/qemu/{vmid}
// Note: The VM must be stopped before deletion. Use VMActionResty to stop it first if needed.
func DeleteVMResty(ctx context.Context, restyClient *RestyClient, node string, vmid int) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d", url.PathEscape(node), vmid)

	var response interface{}
	if err := restyClient.Delete(ctx, path, &response); err != nil {
		logger.Get().Error().Err(err).Str("node", node).Int("vmid", vmid).Msg("VM deletion failed (resty)")
		return fmt.Errorf("failed to delete VM %d on node %s: %w", vmid, node, err)
	}

	logger.Get().Info().Str("node", node).Int("vmid", vmid).Msg("VM deleted successfully (resty)")
	return nil
}

// GetNextVMIDResty determines the next available unique ID for a new VM using resty.
// It fetches all existing VMs, finds the highest current VMID, and returns that value incremented by one.
func GetNextVMIDResty(ctx context.Context, restyClient *RestyClient) (int, error) {
	vms, err := GetVMsResty(ctx, restyClient)
	if err != nil {
		return 0, fmt.Errorf("failed to get VMs to calculate next VMID: %w", err)
	}

	highestVMID := 0
	for _, vm := range vms {
		if vm.VMID > highestVMID {
			highestVMID = vm.VMID
		}
	}

	nextVMID := highestVMID + 1
	logger.Get().Info().Int("next_vmid", nextVMID).Msg("Calculated next VMID (resty)")
	return nextVMID, nil
}
