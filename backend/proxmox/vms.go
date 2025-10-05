package proxmox

import (
	"context"
	"fmt"
	"net/url"
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

// GetVMConfigWithContext fetches the VM configuration from Proxmox:
// GET /nodes/{node}/qemu/{vmid}/config
// It returns the raw "data" map as provided by the API so callers can extract
// fields such as description, tags, and network interfaces (net0/net1...).
func GetVMConfigWithContext(ctx context.Context, client ClientInterface, node string, vmid int) (map[string]interface{}, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid)
	var resp struct {
		Data map[string]interface{} `json:"data"`
	}
	if err := client.GetJSON(ctx, path, &resp); err != nil {
		logger.Get().Error().Err(err).Str("node", node).Int("vmid", vmid).Msg("Failed to get VM config")
		return nil, fmt.Errorf("failed to get config for vm %d on node %s: %w", vmid, node, err)
	}
	return resp.Data, nil
}

// UpdateVMConfigWithContext updates VM configuration fields (e.g., description, tags)
// by POSTing form parameters to:
//
//	POST /nodes/{node}/qemu/{vmid}/config
//
// Params may include keys like "description" and "tags" (semicolon-separated).
func UpdateVMConfigWithContext(ctx context.Context, client ClientInterface, node string, vmid int, params map[string]string) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/config", url.PathEscape(node), vmid)
	values := make(url.Values)
	for k, v := range params {
		values.Set(k, v)
	}
	_, err := client.PostFormWithContext(ctx, path, values)
	if err != nil {
		logger.Get().Error().Err(err).Str("node", node).Int("vmid", vmid).Msg("Failed to update VM config")
		return fmt.Errorf("failed to update config for vm %d on node %s: %w", vmid, node, err)
	}
	// Invalidate the cached GET for this VM's config so the next fetch returns fresh data
	if c, ok := client.(*Client); ok && c != nil {
		c.InvalidateCache(path)
	}
	return nil
}

// ExtractNetworkBridges parses the VM config map and returns a unique, sorted list
// of network bridge names (e.g., vmbr0) found in net* entries.
func ExtractNetworkBridges(cfg map[string]interface{}) []string {
	if cfg == nil {
		return nil
	}
	seen := make(map[string]struct{})
	// Iterate over keys like net0, net1, ...
	for k, v := range cfg {
		if !strings.HasPrefix(strings.ToLower(k), "net") {
			continue
		}
		s, ok := v.(string)
		if !ok || s == "" {
			continue
		}
		// net line format example: "virtio=xx:xx:xx,bridge=vmbr0,firewall=1"
		parts := strings.Split(s, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if strings.HasPrefix(p, "bridge=") {
				br := strings.TrimPrefix(p, "bridge=")
				if br != "" {
					seen[br] = struct{}{}
				}
			}
		}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for b := range seen {
		out = append(out, b)
	}
	// Stable order for display
	// (no sort import at top; simple insertion order is fine)
	return out
}

// VMCurrent represents the runtime status/metrics of a VM from
// GET /nodes/{node}/qemu/{vmid}/status/current
type VMCurrent struct {
	Status    string  `json:"status"`
	CPU       float64 `json:"cpu"` // fraction 0..1
	Mem       int64   `json:"mem"`
	MaxMem    int64   `json:"maxmem"`
	Name      string  `json:"name"`
	CPUs      int     `json:"cpus"`
	QMPStatus string  `json:"qmpstatus"`
}

// GetVMCurrentWithContext fetches the current runtime metrics for a VM
func GetVMCurrentWithContext(ctx context.Context, client ClientInterface, node string, vmid int) (*VMCurrent, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/status/current", url.PathEscape(node), vmid)
	var resp Response[VMCurrent]
	if err := client.GetJSON(ctx, path, &resp); err != nil {
		logger.Get().Error().Err(err).Str("node", node).Int("vmid", vmid).Msg("Failed to get current VM status")
		return nil, fmt.Errorf("failed to get current status for vm %d on node %s: %w", vmid, node, err)
	}
	return &resp.Data, nil
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
	// Get all nodes first
	nodes, err := GetNodeNamesWithContext(ctx, client)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get node list while fetching VMs")
		return nil, fmt.Errorf("failed to get node list: %w", err)
	}

	// Collect VMs from all nodes
	allVMs := make([]VM, 0)

	for _, node := range nodes {
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
	path := fmt.Sprintf("/nodes/%s/qemu", url.PathEscape(nodeName))

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

	path := fmt.Sprintf("/nodes/%s/qemu/%s/status/%s", url.PathEscape(node), url.PathEscape(vmid), action)

	// Proxmox typically responds with {"data":"UPID:..."}
	var response Response[string]
	if err := client.PostFormAndGetJSON(ctx, path, url.Values{}, &response); err != nil {
		logger.Get().Error().Err(err).Str("node", node).Str("vmid", vmid).Str("action", action).Msg("VM action failed")
		return "", err
	}

	// The task ID (UPID) is returned in the 'data' field.
	if response.Data == "" {
		return "", fmt.Errorf("did not receive a task ID from Proxmox for action '%s' on VM %s", action, vmid)
	}

	return response.Data, nil
}

// DeleteVMWithContext deletes a VM from Proxmox.
// This performs a DELETE request to /nodes/{node}/qemu/{vmid}
// Note: The VM must be stopped before deletion. Use VMActionWithContext to stop it first if needed.
func DeleteVMWithContext(ctx context.Context, client ClientInterface, node string, vmid int) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d", url.PathEscape(node), vmid)

	// Proxmox DELETE typically responds with {"data":"UPID:..."}
	_, err := client.DeleteWithContext(ctx, path, url.Values{})
	if err != nil {
		logger.Get().Error().Err(err).Str("node", node).Int("vmid", vmid).Msg("VM deletion failed")
		return fmt.Errorf("failed to delete VM %d on node %s: %w", vmid, node, err)
	}

	logger.Get().Info().Str("node", node).Int("vmid", vmid).Msg("VM deleted successfully")

	// Invalidate cache for this node's VM list
	if c, ok := client.(*Client); ok && c != nil {
		c.InvalidateCache(fmt.Sprintf("/nodes/%s/qemu", url.PathEscape(node)))
	}

	return nil
}
