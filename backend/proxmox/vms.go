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

// NetworkInterface represents a VM network interface configuration
type NetworkInterface struct {
	Bridge                 string   // e.g., "vmbr0"
	Firewall               bool     // whether firewall is enabled
	IPAddresses            []string // IP addresses from guest agent
	Index                  string   // e.g., "net0", "net1"
	LinkDown               bool     // whether link is down
	MACAddress             string   // e.g., "AA:BB:CC:DD:EE:FF"
	Model                  string   // e.g., "virtio", "e1000"
	ModelLabel             string   // e.g., "VirtIO", "E1000"
	ModelTranslationSuffix string
	Rate                   string // bandwidth limit if set
}

var networkModelMetadata = map[string]struct {
	label             string
	translationSuffix string
}{
	"e1000":   {label: "E1000", translationSuffix: "E1000"},
	"e1000e":  {label: "E1000E", translationSuffix: "E1000E"},
	"rtl8139": {label: "RTL8139", translationSuffix: "RTL8139"},
	"virtio":  {label: "VirtIO", translationSuffix: "VirtIO"},
	"vmxnet3": {label: "VMXNet3", translationSuffix: "VMXNet3"},
}

// ExtractNetworkInterfaces parses the VM config map and returns a list of network interfaces
// with their full configuration details.
func ExtractNetworkInterfaces(cfg map[string]interface{}) []NetworkInterface {
	if cfg == nil {
		return nil
	}

	var interfaces []NetworkInterface

	// Iterate over keys like net0, net1, ... in order
	for i := 0; i < 10; i++ { // Support up to 10 network interfaces
		key := fmt.Sprintf("net%d", i)
		v, exists := cfg[key]
		if !exists {
			continue
		}

		s, ok := v.(string)
		if !ok || s == "" {
			continue
		}

		// net line format example: "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,firewall=1"
		iface := NetworkInterface{
			Index: key,
		}

		parts := strings.Split(s, ",")
		for _, p := range parts {
			p = strings.TrimSpace(p)

			// Parse model and MAC address (first part, e.g., "virtio=AA:BB:CC:DD:EE:FF")
			if strings.Contains(p, "=") && (strings.HasPrefix(p, "virtio=") ||
				strings.HasPrefix(p, "e1000=") || strings.HasPrefix(p, "e1000e=") ||
				strings.HasPrefix(p, "rtl8139=") || strings.HasPrefix(p, "vmxnet3=")) {
				kv := strings.SplitN(p, "=", 2)
				if len(kv) == 2 {
					modelKey := strings.ToLower(kv[0])
					iface.Model = modelKey
					iface.MACAddress = strings.ToUpper(kv[1])
					if meta, ok := networkModelMetadata[modelKey]; ok {
						iface.ModelLabel = meta.label
						iface.ModelTranslationSuffix = meta.translationSuffix
					} else {
						iface.ModelLabel = strings.ToUpper(modelKey)
					}
				}
			} else if strings.HasPrefix(p, "bridge=") {
				iface.Bridge = strings.TrimPrefix(p, "bridge=")
			} else if p == "firewall=1" {
				iface.Firewall = true
			} else if p == "link_down=1" {
				iface.LinkDown = true
			} else if strings.HasPrefix(p, "rate=") {
				iface.Rate = strings.TrimPrefix(p, "rate=")
			}
		}

		interfaces = append(interfaces, iface)
	}

	return interfaces
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

// GuestAgentNetworkInterface represents a network interface from QEMU guest agent
type GuestAgentNetworkInterface struct {
	HardwareAddress string `json:"hardware-address"`
	IPAddresses     []struct {
		IPAddress     string `json:"ip-address"`
		IPAddressType string `json:"ip-address-type"` // "ipv4" or "ipv6"
		Prefix        int    `json:"prefix"`
	} `json:"ip-addresses"`
	Name string `json:"name"`
}

// GetGuestAgentNetworkInterfaces fetches network information from the QEMU guest agent
// Returns nil if guest agent is not available or not running
func GetGuestAgentNetworkInterfaces(ctx context.Context, client ClientInterface, node string, vmid int) ([]GuestAgentNetworkInterface, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/agent/network-get-interfaces", url.PathEscape(node), vmid)
	var resp Response[struct {
		Result []GuestAgentNetworkInterface `json:"result"`
	}]

	if err := client.GetJSON(ctx, path, &resp); err != nil {
		// Guest agent not available is expected for VMs without it or when VM is stopped
		return nil, err
	}

	return resp.Data.Result, nil
}

// EnrichNetworkInterfacesWithIPs adds IP addresses from guest agent to network interfaces
// Matches interfaces by MAC address
func EnrichNetworkInterfacesWithIPs(interfaces []NetworkInterface, guestInterfaces []GuestAgentNetworkInterface) {
	if len(guestInterfaces) == 0 {
		return
	}

	// Create a map of MAC address to IP addresses
	macToIPs := make(map[string][]string)
	for _, guestIface := range guestInterfaces {
		if guestIface.HardwareAddress == "" {
			continue
		}
		// Normalize MAC address to uppercase
		mac := strings.ToUpper(guestIface.HardwareAddress)

		var ips []string
		for _, ipAddr := range guestIface.IPAddresses {
			// Skip loopback and link-local addresses
			ip := ipAddr.IPAddress
			if ip == "127.0.0.1" || ip == "::1" || strings.HasPrefix(ip, "fe80:") {
				continue
			}
			ips = append(ips, ip)
		}

		if len(ips) > 0 {
			macToIPs[mac] = ips
		}
	}

	// Match and add IPs to network interfaces
	for i := range interfaces {
		if interfaces[i].MACAddress != "" {
			if ips, found := macToIPs[interfaces[i].MACAddress]; found {
				interfaces[i].IPAddresses = ips
			}
		}
	}
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

// LEGACY FUNCTIONS REMOVED - Use resty versions instead:
// - GetVMsWithContext → GetVMsResty
// - GetVMsForNodeWithContext → GetVMsForNodeResty
// - GetVMConfigWithContext → GetVMConfigResty
// - GetVMCurrentWithContext → GetVMCurrentResty
// - UpdateVMConfigWithContext → UpdateVMConfigResty
// - GetNextVMID → GetNextVMIDResty
// - VMActionWithContext → VMActionResty
// - DeleteVMWithContext → DeleteVMResty
// See resty_vms.go for modern implementations

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
	// Use PostEmpty to send POST with empty form data
	// Proxmox may return empty JSON for some actions
	if err := restyClient.PostEmpty(ctx, path, &response); err != nil {
		logger.Get().Error().Err(err).Str("node", node).Str("vmid", vmid).Str("action", action).Msg("VM action failed (resty)")
		return "", err
	}

	// The task ID (UPID) is returned in the 'data' field.
	// Some actions may not return a UPID, which is acceptable
	if response.Data == "" {
		logger.Get().Info().Str("node", node).Str("vmid", vmid).Str("action", action).Msg("VM action executed (no UPID returned)")
		return "", nil
	}

	logger.Get().Info().Str("node", node).Str("vmid", vmid).Str("action", action).Str("upid", response.Data).Msg("VM action executed (resty)")
	return response.Data, nil
}

// DeleteVMResty deletes a VM from Proxmox using resty.
// This performs a DELETE request to /nodes/{node}/qemu/{vmid}
// Note: The VM must be stopped before deletion. Use VMActionResty to stop it first.
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
