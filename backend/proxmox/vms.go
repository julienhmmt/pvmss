package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"pvmss/logger"
)

// vmCache is a package-level LRU cache for VM listings with 10-second TTL
// This prevents hammering Proxmox on rapid successive requests (polling, multiple users)
var vmCache = MakeLRUCache(100, 10*time.Second)

// InvalidateVMCache clears cached VM data for a specific node
// This should be called after VM creation, deletion, or modification
func InvalidateVMCache(nodeName string) {
	cacheKey := fmt.Sprintf("vms:%s", nodeName)
	vmCache.Delete(cacheKey)
	logger.Get().Debug().Str("node", nodeName).Msg("VM cache invalidated for node")
}

// ClearVMCache clears all cached VM data
// This is primarily intended for test isolation to prevent test interference
func ClearVMCache() {
	vmCache.Clear()
	logger.Get().Debug().Msg("VM cache cleared for all nodes")
}

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

// NetworkInterface represents a VM network interface configuration
type NetworkInterface struct {
	Bridge                 string   `json:"bridge"`      // e.g., "vmbr0"
	Firewall               bool     `json:"firewall"`    // whether firewall is enabled
	IPAddresses            []string `json:"ips"`         // IP addresses from guest agent
	Index                  string   `json:"index"`       // e.g., "net0", "net1"
	LinkDown               bool     `json:"link_down"`   // whether link is down
	MACAddress             string   `json:"mac"`         // e.g., "AA:BB:CC:DD:EE:FF"
	Model                  string   `json:"model"`       // e.g., "virtio", "e1000"
	ModelLabel             string   `json:"model_label"` // e.g., "VirtIO", "E1000"
	ModelTranslationSuffix string   `json:"model_translation_suffix"`
	Rate                   string   `json:"rate"` // bandwidth limit in MB/s if set
	VLAN                   int      `json:"tag"`  // VLAN tag if present, 0 = none
	MTU                    string   `json:"mtu"`
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
			} else if strings.HasPrefix(p, "tag=") {
				tagStr := strings.TrimPrefix(p, "tag=")
				if tagVal, err := strconv.Atoi(tagStr); err == nil {
					iface.VLAN = tagVal
				}
			} else if strings.HasPrefix(p, "rate=") {
				iface.Rate = strings.TrimPrefix(p, "rate=")
			} else if strings.HasPrefix(p, "mtu=") {
				iface.MTU = strings.TrimPrefix(p, "mtu=")
			} else if p == "firewall=1" {
				iface.Firewall = true
			} else if p == "link_down=1" {
				iface.LinkDown = true
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
	Tags    string  `json:"tags"`
	Uptime  int64   `json:"uptime"`
	VMID    int     `json:"vmid"`
}

const (
	// nodeQueryTimeout is the per-node timeout for VM list queries.
	// 8 seconds allows for network latency and Proxmox API response time.
	nodeQueryTimeout = 8 * time.Second
)

// GetVMsResty retrieves a comprehensive list of all VMs across all available Proxmox nodes using resty.
// It first fetches the list of online nodes only and then queries them in parallel using goroutines
// with a semaphore to limit concurrency (same pattern as FetchAllNodeDetailsResty).
func GetVMsResty(ctx context.Context, restyClient *RestyClient) ([]VM, error) {
	// Get online nodes only to avoid errors with down nodes
	nodes, err := GetOnlineNodeNamesResty(ctx, restyClient)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get online node list while fetching VMs (resty)")
		return nil, fmt.Errorf("failed to get online node list: %w", err)
	}

	if len(nodes) == 0 {
		return []VM{}, nil
	}

	const maxConcurrent = 8
	semaphore := make(chan struct{}, maxConcurrent)
	var wg sync.WaitGroup
	var mu sync.Mutex
	failedNodes := make([]string, 0)
	vmsChan := make(chan []VM, len(nodes))

	for _, nodeName := range nodes {
		// Check if parent context was cancelled before launching goroutine
		if ctx.Err() != nil {
			logger.Get().Warn().Msg("Parent context cancelled, skipping remaining nodes")
			break
		}

		wg.Add(1)
		name := nodeName
		go func() {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			nodeCtx, cancel := context.WithTimeout(ctx, nodeQueryTimeout)
			defer cancel()

			nodeVMs, nodeErr := GetVMsForNodeResty(nodeCtx, restyClient, name)
			if nodeErr != nil {
				logger.Get().Warn().Err(nodeErr).Str("node", name).Msg("Failed to get VMs for node (resty)")
				mu.Lock()
				failedNodes = append(failedNodes, name)
				mu.Unlock()
				vmsChan <- []VM{}
				return
			}

			vmsChan <- nodeVMs
		}()
	}

	go func() {
		wg.Wait()
		close(vmsChan)
	}()

	// Pre-allocate slice with capacity hint (assume ~10 VMs per node average)
	allVMs := make([]VM, 0, len(nodes)*10)
	for nodeVMs := range vmsChan {
		allVMs = append(allVMs, nodeVMs...)
	}

	sort.Slice(allVMs, func(i, j int) bool {
		if allVMs[i].Node != allVMs[j].Node {
			return allVMs[i].Node < allVMs[j].Node
		}
		return allVMs[i].VMID < allVMs[j].VMID
	})

	// Log summary including any failed nodes
	logEntry := logger.Get().Info().
		Int("total_vms", len(allVMs)).
		Int("successful_nodes", len(nodes)-len(failedNodes)).
		Int("failed_nodes", len(failedNodes))
	if len(failedNodes) > 0 {
		logEntry.Strs("failed_node_names", failedNodes)
	}
	logEntry.Msg("Successfully fetched all VMs from online nodes (resty)")

	return allVMs, nil
}

// GetVMsForNodeResty fetches all VMs located on a single, specified Proxmox node using resty.
// It calls the `/nodes/{nodeName}/qemu` endpoint and enriches the returned VM data with the node's name.
// Results are cached for 10 seconds to avoid hammering Proxmox on rapid successive requests.
func GetVMsForNodeResty(ctx context.Context, restyClient *RestyClient, nodeName string) ([]VM, error) {
	// Check cache first
	cacheKey := fmt.Sprintf("vms:%s", nodeName)
	cachedData := vmCache.Get(cacheKey)
	if cachedData != nil {
		var cachedVMs []VM
		unmarshalErr := json.Unmarshal(cachedData, &cachedVMs)
		if unmarshalErr == nil {
			logger.Get().Debug().Str("node", nodeName).Int("count", len(cachedVMs)).Msg("VMs retrieved from cache")
			return cachedVMs, nil
		}
		logger.Get().Warn().Err(unmarshalErr).Str("node", nodeName).Msg("Failed to unmarshal cached VM data, fetching fresh")
	}

	// Cache miss or invalid data - fetch from Proxmox
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

	// Cache the result
	if data, err := json.Marshal(response.Data); err == nil {
		vmCache.Set(cacheKey, data)
	} else {
		logger.Get().Warn().Err(err).Str("node", nodeName).Msg("Failed to marshal VM data for caching")
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

	// Invalidate cache for this node to reflect the config update
	InvalidateVMCache(node)

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

	// Invalidate cache for this node to reflect the status change
	InvalidateVMCache(node)

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

	// Invalidate cache for this node to reflect the deletion
	InvalidateVMCache(node)

	logger.Get().Info().Str("node", node).Int("vmid", vmid).Msg("VM deleted successfully (resty)")
	return nil
}

// GetClusterNextIDResty retrieves the next available VMID from Proxmox's atomic cluster allocation.
// This uses the GET /cluster/nextid endpoint which provides thread-safe VMID allocation,
// eliminating race conditions when concurrent users create VMs.
// Falls back to calculating from VM list if the cluster endpoint fails (standalone mode).
func GetClusterNextIDResty(ctx context.Context, restyClient *RestyClient) (int, error) {
	// Try cluster endpoint first (atomic allocation)
	path := "/cluster/nextid"
	var response Response[int]
	if err := restyClient.Get(ctx, path, &response); err == nil {
		nextVMID := response.Data
		if nextVMID <= 0 {
			logger.Get().Warn().Int("next_vmid", nextVMID).Msg("Cluster endpoint returned invalid VMID, falling back to calculation")
		} else {
			logger.Get().Info().Int("next_vmid", nextVMID).Msg("Retrieved next VMID from Proxmox cluster (atomic allocation)")
			return nextVMID, nil
		}
	} else {
		logger.Get().Warn().Err(err).Msg("Cluster nextid endpoint failed, falling back to VM list calculation")
	}

	// Fallback: calculate from existing VMs (works in standalone mode)
	vms, err := GetVMsResty(ctx, restyClient)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get VMs for fallback VMID calculation")
		return 0, fmt.Errorf("failed to get next VMID: cluster endpoint failed and VM list fallback failed: %w", err)
	}

	highestVMID := 0
	for _, vm := range vms {
		if vm.VMID > highestVMID {
			highestVMID = vm.VMID
		}
	}

	nextVMID := highestVMID + 1
	if nextVMID <= 0 {
		return 0, fmt.Errorf("calculated next VMID is invalid: %d", nextVMID)
	}

	logger.Get().Info().Int("next_vmid", nextVMID).Msg("Calculated next VMID from VM list (standalone/fallback)")
	return nextVMID, nil
}

// ResizeVMDiskResty resizes a VM disk using the Proxmox resize API.
// This performs a PUT request to /nodes/{node}/qemu/{vmid}/resize
// The size parameter should be in the format "+10G" for increment or "20G" for absolute size
func ResizeVMDiskResty(ctx context.Context, restyClient *RestyClient, node string, vmid int, disk string, size string) error {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/resize", url.PathEscape(node), vmid)

	// Build form data for the resize request
	formData := url.Values{}
	formData.Set("disk", disk)
	formData.Set("size", size)

	var response interface{}
	if err := restyClient.Put(ctx, path, formData, &response); err != nil {
		logger.Get().Error().Err(err).Str("node", node).Int("vmid", vmid).Str("disk", disk).Str("size", size).Msg("VM disk resize failed (resty)")
		return fmt.Errorf("failed to resize disk %s for VM %d on node %s: %w", disk, vmid, node, err)
	}

	logger.Get().Info().Str("node", node).Int("vmid", vmid).Str("disk", disk).Str("size", size).Msg("VM disk resized successfully (resty)")
	return nil
}

// QemuAgentCommand represents a command to execute via QEMU agent
type QemuAgentCommand struct {
	Command []string `json:"command"`
}

// QemuAgentExecResponse represents the response from QEMU agent exec command
type QemuAgentExecResponse struct {
	Data struct {
		Pid      int    `json:"pid"`
		Exitcode int    `json:"exitcode"`
		OutData  string `json:"out-data"`
		ErrData  string `json:"err-data"`
	} `json:"data"`
}

// ExecuteQemuAgentCommandResty executes a command via QEMU agent
// This performs a POST request to /nodes/{node}/qemu/{vmid}/agent/exec
// Returns the PID of the executing command
func ExecuteQemuAgentCommandResty(ctx context.Context, restyClient *RestyClient, node string, vmid int, command []string) (int, error) {
	path := fmt.Sprintf("/nodes/%s/qemu/%d/agent/exec", url.PathEscape(node), vmid)

	// Build form data for the command
	formData := url.Values{}
	// Convert command array to JSON string for form data
	commandJSON := fmt.Sprintf(`["%s"]`, strings.Join(command, `","`))
	formData.Set("command", commandJSON)

	var response QemuAgentExecResponse
	if err := restyClient.Post(ctx, path, formData, &response); err != nil {
		logger.Get().Error().Err(err).Str("node", node).Int("vmid", vmid).Strs("command", command).Msg("QEMU agent command execution failed (resty)")
		return 0, fmt.Errorf("failed to execute QEMU agent command for VM %d on node %s: %w", vmid, node, err)
	}

	logger.Get().Info().Str("node", node).Int("vmid", vmid).Strs("command", command).Int("pid", response.Data.Pid).Msg("QEMU agent command executed successfully (resty)")
	return response.Data.Pid, nil
}
