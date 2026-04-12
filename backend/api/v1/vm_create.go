package apiv1

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
	"pvmss/utils"
)

// VMCreateHandler handles VM creation API endpoints.
type VMCreateHandler struct {
	state state.StateManager
}

// MakeVMCreateHandler creates a new VMCreateHandler.
func MakeVMCreateHandler(s state.StateManager) *VMCreateHandler {
	return &VMCreateHandler{state: s}
}

// ── Response types ────────────────────────────────────────────────────────────

// VMCreateSettingsResponse is the response for GET /api/v1/vm-create/settings.
type VMCreateSettingsResponse struct {
	Nodes             []VMCreateNodeOption    `json:"nodes"`
	Storages          []VMCreateStorageOption `json:"storages"`
	Bridges           []VMCreateBridgeOption  `json:"bridges"`
	ISOs              []VMCreateISOOption     `json:"isos"`
	Tags              []string                `json:"tags"`
	CloudInitTemplate []VMCreateCITemplate    `json:"cloudinit_templates"`
	Limits            VMCreateLimits          `json:"limits"`
	MaxNetworkCards   int                     `json:"max_network_cards"`
	MaxDiskPerVM      int                     `json:"max_disk_per_vm"`
	MaxVMPerUser      int                     `json:"max_vm_per_user"`
	RemainingVMs      int                     `json:"remaining_vms"`
	ProxmoxConnected  bool                    `json:"proxmox_connected"`
	AllowCustomYAML   bool                    `json:"allow_custom_yaml"`
}

// VMCreateNodeOption represents a node option for VM creation.
type VMCreateNodeOption struct {
	Name     string `json:"name"`
	Disabled bool   `json:"disabled"`
	Reason   string `json:"reason,omitempty"`
}

// VMCreateStorageOption represents a storage option for VM creation.
type VMCreateStorageOption struct {
	Name string `json:"name"`
	Node string `json:"node"`
}

// VMCreateBridgeOption represents a bridge option for VM creation.
type VMCreateBridgeOption struct {
	Name        string `json:"name"`
	Node        string `json:"node"`
	Description string `json:"description,omitempty"`
}

// VMCreateISOOption represents an ISO option for VM creation.
type VMCreateISOOption struct {
	VolID string `json:"volid"`
	Name  string `json:"name"`
}

// VMCreateCITemplate represents a cloud-init template for VM creation.
type VMCreateCITemplate struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// VMCreateLimits defines resource limits for the VM creation form.
type VMCreateLimits struct {
	Sockets VMCreateRange `json:"sockets"`
	Cores   VMCreateRange `json:"cores"`
	RAM     VMCreateRange `json:"ram"`
	Disk    VMCreateRange `json:"disk"`
}

// VMCreateRange defines min/max for a resource.
type VMCreateRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// VMCreateRequest is the body for POST /api/v1/vms.
type VMCreateRequest struct {
	Name        string             `json:"name"`
	Node        string             `json:"node"`
	Storage     string             `json:"storage"`
	Description string             `json:"description,omitempty"`
	ISO         string             `json:"iso,omitempty"`
	Tags        []string           `json:"tags,omitempty"`
	Sockets     int                `json:"sockets"`
	Cores       int                `json:"cores"`
	MemoryMB    int                `json:"memory_mb"`
	Disks       []VMCreateDisk     `json:"disks"`
	Networks    []VMCreateNetwork  `json:"networks"`
	EnableEFI   bool               `json:"enable_efi"`
	EnableTPM   bool               `json:"enable_tpm"`
	DiskBus     string             `json:"disk_bus"`
	StartVM     bool               `json:"start_vm"`
	CloudInit   *VMCreateCloudInit `json:"cloud_init,omitempty"`
}

// VMCreateDisk represents a disk in the VM creation request.
type VMCreateDisk struct {
	SizeGB int `json:"size_gb"`
}

// VMCreateNetwork represents a network card in the VM creation request.
type VMCreateNetwork struct {
	Bridge    string `json:"bridge"`
	Model     string `json:"model"`
	MAC       string `json:"mac,omitempty"`
	VLAN      int    `json:"vlan,omitempty"`
	RateLimit string `json:"rate_limit,omitempty"`
	MTU       int    `json:"mtu,omitempty"`
	Enabled   bool   `json:"enabled"`
}

// VMCreateCloudInit represents cloud-init configuration in the VM creation request.
type VMCreateCloudInit struct {
	User       string `json:"user,omitempty"`
	Password   string `json:"password,omitempty"`
	SSHKeys    string `json:"ssh_keys,omitempty"`
	IPConfig   string `json:"ip_config"`
	IP         string `json:"ip,omitempty"`
	Gateway    string `json:"gateway,omitempty"`
	DNS        string `json:"dns,omitempty"`
	TemplateID string `json:"template_id,omitempty"`
}

// VMCreateResponse is returned after a successful VM creation.
type VMCreateResponse struct {
	VMID          int    `json:"vmid"`
	Name          string `json:"name"`
	Node          string `json:"node"`
	CloudInitWarn string `json:"cloud_init_warning,omitempty"`
}

// ── GET /api/v1/vm-create/settings ────────────────────────────────────────────

// GetSettings handles GET /api/v1/vm-create/settings.
// Returns all data needed to render the VM creation form.
func (h *VMCreateHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	settings := h.state.GetSettings()
	if settings == nil {
		writeError(w, http.StatusInternalServerError, "settings_unavailable", "Settings not available")
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)
	connected := !h.state.IsOfflineMode()

	resp := VMCreateSettingsResponse{
		ProxmoxConnected: connected,
		AllowCustomYAML:  settings.AllowCustomYAML,
		MaxNetworkCards:  settings.MaxNetworkCards,
		MaxDiskPerVM:     settings.MaxDiskPerVM,
		MaxVMPerUser:     settings.MaxVMPerUser,
		Tags:             settings.Tags,
		Limits: VMCreateLimits{
			Sockets: VMCreateRange{Min: settings.Limits.VM.Sockets.Min, Max: settings.Limits.VM.Sockets.Max},
			Cores:   VMCreateRange{Min: settings.Limits.VM.Cores.Min, Max: settings.Limits.VM.Cores.Max},
			RAM:     VMCreateRange{Min: settings.Limits.VM.RAM.Min, Max: settings.Limits.VM.RAM.Max},
			Disk:    VMCreateRange{Min: settings.Limits.VM.Disk.Min, Max: settings.Limits.VM.Disk.Max},
		},
	}

	if resp.MaxNetworkCards <= 0 {
		resp.MaxNetworkCards = 1
	}
	if resp.MaxDiskPerVM <= 0 {
		resp.MaxDiskPerVM = 1
	}
	if resp.Tags == nil {
		resp.Tags = []string{}
	}

	// ISOs from settings
	isos := make([]VMCreateISOOption, 0, len(settings.ISOs))
	for _, volid := range settings.ISOs {
		name := volid
		if idx := strings.LastIndex(volid, "/"); idx >= 0 {
			name = volid[idx+1:]
		}
		isos = append(isos, VMCreateISOOption{VolID: volid, Name: name})
	}
	resp.ISOs = isos

	// Cloud-init templates (enabled only)
	ciTemplates := make([]VMCreateCITemplate, 0)
	for _, t := range settings.CloudInitTemplates {
		if t.Enabled {
			ciTemplates = append(ciTemplates, VMCreateCITemplate{
				ID:          t.ID,
				Name:        t.Name,
				Description: t.Description,
			})
		}
	}
	resp.CloudInitTemplate = ciTemplates

	// Compute remaining VMs for the user
	resp.RemainingVMs = -1 // -1 = unlimited
	if settings.MaxVMPerUser > 0 && !isAdmin {
		poolName := "pvmss_" + username
		remaining := settings.MaxVMPerUser
		if connected {
			client, err := restyClient()
			if err == nil {
				ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
				poolVMIDs := fetchPoolVMIDs(ctx, client, poolName)
				cancel()
				remaining = settings.MaxVMPerUser - len(poolVMIDs)
				if remaining < 0 {
					remaining = 0
				}
			}
		}
		resp.RemainingVMs = remaining
	}

	// Fetch nodes, storages, bridges from Proxmox snapshot or live
	if connected {
		snapshot := h.state.GetProxmoxSnapshot()
		nodes, disabledNodes := h.resolveNodes(r.Context(), snapshot, settings)
		resp.Nodes = nodes
		resp.Storages = h.resolveStorages(r.Context(), snapshot, settings, disabledNodes)
		resp.Bridges = h.resolveBridges(r.Context(), snapshot, settings, disabledNodes)
	} else {
		resp.Nodes = h.nodesFromSettings(settings)
		resp.Storages = h.storagesFromSettings(settings)
		resp.Bridges = h.bridgesFromSettings(settings)
	}

	if resp.Nodes == nil {
		resp.Nodes = []VMCreateNodeOption{}
	}
	if resp.Storages == nil {
		resp.Storages = []VMCreateStorageOption{}
	}
	if resp.Bridges == nil {
		resp.Bridges = []VMCreateBridgeOption{}
	}

	logger.Get().Info().
		Str("username", username).
		Bool("is_admin", isAdmin).
		Int("nodes", len(resp.Nodes)).
		Int("storages", len(resp.Storages)).
		Int("bridges", len(resp.Bridges)).
		Int("isos", len(resp.ISOs)).
		Msg("api/v1: VM create settings served")

	writeJSON(w, resp)
}

// resolveNodes returns node options from snapshot or live data.
// A node is disabled when its current pvmss aggregate usage leaves no room
// for even a minimum-sized VM, according to the limits defined in settings.
func (h *VMCreateHandler) resolveNodes(ctx context.Context, snapshot *state.ProxmoxClusterSnapshot, settings *state.AppSettings) ([]VMCreateNodeOption, map[string]bool) {
	disabledNodes := make(map[string]bool)
	var nodeNames []string

	if snapshot != nil && len(snapshot.OnlineNodes) > 0 {
		nodeNames = append(nodeNames, snapshot.OnlineNodes...)
	}

	if len(nodeNames) == 0 {
		client, err := restyClient()
		if err == nil {
			names, _ := proxmox.GetNodeNamesResty(ctx, client)
			nodeNames = names
		}
	}

	// Compute per-node aggregate usage from pvmss-tagged VMs in the snapshot,
	// then disable any node whose remaining capacity (per settings limits) cannot
	// accommodate the smallest possible VM.
	if snapshot != nil && settings != nil && len(settings.Limits.Nodes) > 0 {
		// Validate VM limits are properly initialized before capacity check
		if settings.Limits.VM.Sockets.Min > 0 && settings.Limits.VM.Cores.Min > 0 && settings.Limits.VM.RAM.Min > 0 {
			nodeUsage := computeNodeUsageFromSnapshot(snapshot)
			minCores := settings.Limits.VM.Sockets.Min * settings.Limits.VM.Cores.Min
			minRAMGB := settings.Limits.VM.RAM.Min
			// Ensure minimum values are at least 1 to prevent zero-value issues
			if minCores <= 0 {
				minCores = 1
			}
			if minRAMGB <= 0 {
				minRAMGB = 1
			}
			for nodeName, nodeLimits := range settings.Limits.Nodes {
				usage := nodeUsage[nodeName]
				if nodeLimits.Cores.Max > 0 && usage.cores+minCores > nodeLimits.Cores.Max {
					disabledNodes[nodeName] = true
				}
				if nodeLimits.RAM.Max > 0 && usage.ramGB+minRAMGB > nodeLimits.RAM.Max {
					disabledNodes[nodeName] = true
				}
			}
		}
	}

	sort.Strings(nodeNames)
	options := make([]VMCreateNodeOption, 0, len(nodeNames))
	for _, name := range nodeNames {
		opt := VMCreateNodeOption{Name: name, Disabled: disabledNodes[name]}
		if disabledNodes[name] {
			opt.Reason = "Node limit reached"
		}
		options = append(options, opt)
	}
	return options, disabledNodes
}

// nodeAggregateUsage holds the aggregate cores and RAM (in GB) used by pvmss VMs on a node.
type nodeAggregateUsage struct {
	cores int
	ramGB int
}

// computeNodeUsageFromSnapshot sums cores and RAM for pvmss-tagged VMs per node.
func computeNodeUsageFromSnapshot(snapshot *state.ProxmoxClusterSnapshot) map[string]nodeAggregateUsage {
	usage := make(map[string]nodeAggregateUsage)
	for _, vm := range snapshot.VMs {
		if vm.Node == "" || vm.Tags == "" {
			continue
		}
		hasPvmss := false
		// Try semicolon delimiter first (Proxmox standard)
		tagParts := strings.Split(vm.Tags, ";")
		// If only one part, try space delimiter (alternative format)
		if len(tagParts) == 1 {
			tagParts = strings.Fields(vm.Tags)
		}
		for _, tag := range tagParts {
			if strings.EqualFold(strings.TrimSpace(tag), "pvmss") {
				hasPvmss = true
				break
			}
		}
		if !hasPvmss {
			continue
		}
		sockets := vm.Sockets
		if sockets <= 0 {
			sockets = 1
		}
		cores := vm.Cores
		if cores <= 0 {
			cores = 1
		}
		u := usage[vm.Node]
		u.cores += sockets * cores
		// Round to nearest GB to avoid truncation errors
		u.ramGB += int((vm.MemoryMB + 512) / 1024) //nolint:gosec
		usage[vm.Node] = u
	}
	return usage
}

// vmDiskCompatibleStorageTypes defines storage types that support VM disk images.
var vmDiskCompatibleStorageTypes = map[string]bool{
	"cifs":    true,
	"dir":     true,
	"iscsi":   true,
	"lvm":     true,
	"lvmthin": true,
	"nfs":     true,
	"rbd":     true,
	"zfs":     true,
}

// resolveStorages returns storage options from snapshot or live data.
func (h *VMCreateHandler) resolveStorages(_ context.Context, snapshot *state.ProxmoxClusterSnapshot, settings *state.AppSettings, disabledNodes map[string]bool) []VMCreateStorageOption {
	enabledSet := make(map[string]bool, len(settings.EnabledStorages))
	for _, s := range settings.EnabledStorages {
		enabledSet[s] = true
	}
	allowAll := len(settings.EnabledStorages) == 0

	// Log what storages are configured in settings
	for i, s := range settings.EnabledStorages {
		logger.Get().Debug().Int("index", i).Str("enabled_storage", s).Msg("resolveStorages: configured in settings")
	}

	nodeStoragesCount := 0
	globalStoragesCount := 0
	if snapshot != nil {
		nodeStoragesCount = len(snapshot.NodeStorages)
		globalStoragesCount = len(snapshot.GlobalStorages)
	}
	logger.Get().Debug().
		Int("enabled_storages_count", len(settings.EnabledStorages)).
		Bool("allow_all", allowAll).
		Int("node_storages_count", nodeStoragesCount).
		Int("global_storages_count", globalStoragesCount).
		Msg("resolveStorages: starting storage resolution")

	// Build global storage info map for content/type enrichment
	globalInfo := make(map[string]proxmox.Storage)
	if snapshot != nil {
		for _, st := range snapshot.GlobalStorages {
			globalInfo[st.Storage] = st
		}
	}

	storageMap := make(map[string]string) // storage name -> node

	if snapshot != nil && len(snapshot.NodeStorages) > 0 {
		for nodeName, nodeStorages := range snapshot.NodeStorages {
			if disabledNodes[nodeName] {
				logger.Get().Debug().Str("node", nodeName).Msg("resolveStorages: skipping disabled node")
				continue
			}
			for _, storage := range nodeStorages {
				// Enrich with global info if available
				info := storage
				if global, exists := globalInfo[storage.Storage]; exists {
					if info.Content == "" && global.Content != "" {
						info.Content = global.Content
					}
					if info.Type == "" && global.Type != "" {
						info.Type = global.Type
					}
				}

				// Check if storage is enabled (node:storage format)
				uniqueID := nodeName + ":" + storage.Storage
				isEnabled := allowAll || enabledSet[uniqueID]

				logger.Get().Debug().
					Str("node", nodeName).
					Str("storage", storage.Storage).
					Str("unique_id", uniqueID).
					Bool("is_enabled", isEnabled).
					Int("storage_enabled", storage.Enabled).
					Str("content", info.Content).
					Str("type", info.Type).
					Msg("resolveStorages: evaluating storage")

				if !isEnabled {
					continue
				}
				if storage.Enabled != 1 {
					continue
				}

				// Check if storage supports VM disk images
				storageType := strings.ToLower(info.Type)
				storageContent := strings.ToLower(info.Content)
				supportsVMDisk := strings.Contains(storageContent, "images")
				if !supportsVMDisk {
					_, supportsVMDisk = vmDiskCompatibleStorageTypes[storageType]
				}
				if !supportsVMDisk {
					logger.Get().Debug().
						Str("storage", storage.Storage).
						Str("content", storageContent).
						Str("type", storageType).
						Msg("resolveStorages: storage rejected - does not support VM disk")
					continue
				}

				if _, exists := storageMap[storage.Storage]; !exists {
					if storageType == "rbd" || info.Shared == 1 {
						storageMap[storage.Storage] = ""
					} else {
						storageMap[storage.Storage] = nodeName
					}
					logger.Get().Debug().
						Str("storage", storage.Storage).
						Str("node", storageMap[storage.Storage]).
						Msg("resolveStorages: storage added")
				}
			}
		}
	} else {
		// Fallback: use enabled_storages from settings
		logger.Get().Debug().Msg("resolveStorages: using fallback from settings.EnabledStorages")
		for _, s := range settings.EnabledStorages {
			parts := strings.SplitN(s, ":", 2)
			if len(parts) == 2 {
				storageMap[parts[1]] = parts[0]
				logger.Get().Debug().Str("storage", parts[1]).Str("node", parts[0]).Msg("resolveStorages: added from settings")
			} else {
				storageMap[s] = ""
				logger.Get().Debug().Str("storage", s).Msg("resolveStorages: added shared from settings")
			}
		}
	}

	result := make([]VMCreateStorageOption, 0, len(storageMap))
	for name, node := range storageMap {
		result = append(result, VMCreateStorageOption{Name: name, Node: node})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })

	logger.Get().Info().
		Int("result_count", len(result)).
		Int("source_enabled_count", len(settings.EnabledStorages)).
		Int("source_node_storage_count", len(snapshot.NodeStorages)).
		Msg("resolveStorages: completed storage resolution")

	return result
}

// resolveBridges returns bridge options from snapshot or settings.
func (h *VMCreateHandler) resolveBridges(_ context.Context, snapshot *state.ProxmoxClusterSnapshot, settings *state.AppSettings, disabledNodes map[string]bool) []VMCreateBridgeOption {
	bridgeNodes := make(map[string]string)
	bridgeDescs := make(map[string]string)

	if snapshot != nil && len(snapshot.NetworkBridges) > 0 {
		for nodeName, vmbrs := range snapshot.NetworkBridges {
			if disabledNodes[nodeName] {
				continue
			}
			for _, vmbr := range vmbrs {
				name := extractVMBRIface(vmbr)
				if name == "" {
					continue
				}
				if _, exists := bridgeNodes[name]; !exists {
					bridgeNodes[name] = nodeName
				}
				if bridgeDescs[name] == "" {
					bridgeDescs[name] = strings.TrimSpace(vmbr.Comments)
				}
			}
		}
	}

	result := make([]VMCreateBridgeOption, 0, len(settings.VMBRs))
	for _, bridgeID := range settings.VMBRs {
		bridgeName := bridgeID
		if idx := strings.Index(bridgeID, ":"); idx != -1 {
			bridgeName = bridgeID[idx+1:]
		}
		result = append(result, VMCreateBridgeOption{
			Name:        bridgeName,
			Node:        bridgeNodes[bridgeName],
			Description: bridgeDescs[bridgeName],
		})
	}
	return result
}

// extractVMBRIface extracts the interface name from a VMBR struct.
func extractVMBRIface(vmbr proxmox.VMBR) string {
	return vmbr.Iface
}

// nodesFromSettings derives node list from settings when offline.
func (h *VMCreateHandler) nodesFromSettings(settings *state.AppSettings) []VMCreateNodeOption {
	nodeSet := make(map[string]bool)
	for _, s := range settings.EnabledStorages {
		parts := strings.SplitN(s, ":", 2)
		if len(parts) == 2 {
			nodeSet[parts[0]] = true
		}
	}
	for _, v := range settings.VMBRs {
		parts := strings.SplitN(v, ":", 2)
		if len(parts) == 2 {
			nodeSet[parts[0]] = true
		}
	}
	result := make([]VMCreateNodeOption, 0, len(nodeSet))
	for name := range nodeSet {
		result = append(result, VMCreateNodeOption{Name: name})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// storagesFromSettings derives storages from settings when offline.
func (h *VMCreateHandler) storagesFromSettings(settings *state.AppSettings) []VMCreateStorageOption {
	result := make([]VMCreateStorageOption, 0, len(settings.EnabledStorages))
	for _, s := range settings.EnabledStorages {
		parts := strings.SplitN(s, ":", 2)
		if len(parts) == 2 {
			result = append(result, VMCreateStorageOption{Name: parts[1], Node: parts[0]})
		} else {
			result = append(result, VMCreateStorageOption{Name: s})
		}
	}
	return result
}

// bridgesFromSettings derives bridges from settings when offline.
func (h *VMCreateHandler) bridgesFromSettings(settings *state.AppSettings) []VMCreateBridgeOption {
	result := make([]VMCreateBridgeOption, 0, len(settings.VMBRs))
	for _, v := range settings.VMBRs {
		bridgeName := v
		node := ""
		if idx := strings.Index(v, ":"); idx != -1 {
			node = v[:idx]
			bridgeName = v[idx+1:]
		}
		result = append(result, VMCreateBridgeOption{Name: bridgeName, Node: node})
	}
	return result
}

// ── POST /api/v1/vms ─────────────────────────────────────────────────────────

// CreateVM handles POST /api/v1/vms.
func (h *VMCreateHandler) CreateVM(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if h.state.IsOfflineMode() {
		errOffline(w)
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	var req VMCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid JSON body")
		return
	}

	// Validate required fields
	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		errBadRequest(w, "VM name is required")
		return
	}
	if req.Node == "" {
		errBadRequest(w, "Node is required")
		return
	}
	if req.Storage == "" {
		errBadRequest(w, "Storage is required")
		return
	}
	if len(req.Disks) == 0 {
		errBadRequest(w, "At least one disk is required")
		return
	}

	settings := h.state.GetSettings()
	if settings == nil {
		writeError(w, http.StatusInternalServerError, "settings_unavailable", "Settings not available")
		return
	}

	// Security: Validate inputs against settings allowlists
	// This prevents users from creating VMs with unauthorized nodes, storages, ISOs, or bridges.
	// Validation applies in both online and offline modes.

	// Validate node is in enabled nodes list (if configured).
	// Note: Node names are simple strings (no "node:resource" format like storage/bridges).
	// Empty EnabledNodes list means any node is allowed (no restriction).
	if len(settings.EnabledNodes) > 0 {
		nodeAllowed := false
		for _, enabledNode := range settings.EnabledNodes {
			if enabledNode == req.Node {
				nodeAllowed = true
				break
			}
		}
		if !nodeAllowed {
			logger.Get().Warn().Str("requested_node", req.Node).Strs("enabled_nodes", settings.EnabledNodes).Msg("api/v1: VM creation rejected - node not in allowlist")
			errBadRequest(w, "Invalid selection")
			return
		}
	}

	// Validate storage is in enabled storages list (if configured).
	// Handles both "node:storage" and "storage" formats.
	// Empty EnabledStorages list means any storage is allowed (no restriction).
	if len(settings.EnabledStorages) > 0 {
		storageAllowed := false
		for _, enabledStorage := range settings.EnabledStorages {
			// Handle both "node:storage" and "storage" formats
			parts := strings.SplitN(enabledStorage, ":", 2)
			storageName := enabledStorage
			if len(parts) == 2 {
				storageName = parts[1]
			}
			if storageName == req.Storage {
				storageAllowed = true
				break
			}
		}
		if !storageAllowed {
			logger.Get().Warn().Str("requested_storage", req.Storage).Strs("enabled_storages", settings.EnabledStorages).Msg("api/v1: VM creation rejected - storage not in allowlist")
			errBadRequest(w, "Invalid selection")
			return
		}
	}

	// Validate ISO is in allowed ISOs list (if ISO is provided and ISOs are configured).
	// Empty ISOs list means any ISO is allowed (no restriction).
	if req.ISO != "" && len(settings.ISOs) > 0 {
		isoAllowed := false
		for _, allowedISO := range settings.ISOs {
			if allowedISO == req.ISO {
				isoAllowed = true
				break
			}
		}
		if !isoAllowed {
			logger.Get().Warn().Str("requested_iso", req.ISO).Strs("allowed_isos", settings.ISOs).Msg("api/v1: VM creation rejected - ISO not in allowlist")
			errBadRequest(w, "Invalid selection")
			return
		}
	}

	// Validate network bridges are in allowed VMBRs list (if configured).
	// Handles both "node:bridge" and "bridge" formats.
	// Empty VMBRs list means any bridge is allowed (no restriction).
	if len(settings.VMBRs) > 0 {
		for i, net := range req.Networks {
			if net.Bridge == "" {
				continue // Skip validation if bridge is empty (will be caught by required field check)
			}
			bridgeAllowed := false
			for _, allowedVMBR := range settings.VMBRs {
				// Handle both "node:bridge" and "bridge" formats
				parts := strings.SplitN(allowedVMBR, ":", 2)
				bridgeName := allowedVMBR
				if len(parts) == 2 {
					bridgeName = parts[1]
				}
				if bridgeName == net.Bridge {
					bridgeAllowed = true
					break
				}
			}
			if !bridgeAllowed {
				logger.Get().Warn().Str("requested_bridge", net.Bridge).Int("network_index", i).Strs("allowed_vmbrs", settings.VMBRs).Msg("api/v1: VM creation rejected - bridge not in allowlist")
				errBadRequest(w, "Invalid selection")
				return
			}
		}
	}

	// Validate resource limits
	limits := settings.Limits.VM
	if limits.Sockets.Min == 0 {
		writeError(w, http.StatusInternalServerError, "limits_unavailable", "Resource limits not configured")
		return
	}

	if req.Sockets < limits.Sockets.Min || req.Sockets > limits.Sockets.Max {
		errBadRequest(w, fmt.Sprintf("Sockets must be between %d and %d", limits.Sockets.Min, limits.Sockets.Max))
		return
	}
	if req.Cores < limits.Cores.Min || req.Cores > limits.Cores.Max {
		errBadRequest(w, fmt.Sprintf("Cores must be between %d and %d", limits.Cores.Min, limits.Cores.Max))
		return
	}
	// Round to nearest GB to avoid truncation errors
	ramGB := (req.MemoryMB + 512) / 1024
	if ramGB < limits.RAM.Min || ramGB > limits.RAM.Max {
		errBadRequest(w, fmt.Sprintf("RAM must be between %d and %d GB", limits.RAM.Min, limits.RAM.Max))
		return
	}
	primaryDiskGB := req.Disks[0].SizeGB
	if primaryDiskGB < limits.Disk.Min || primaryDiskGB > limits.Disk.Max {
		errBadRequest(w, fmt.Sprintf("Disk must be between %d and %d GB", limits.Disk.Min, limits.Disk.Max))
		return
	}

	// Check VM quota
	if settings.MaxVMPerUser > 0 && !isAdmin {
		poolName := "pvmss_" + username
		client, err := restyClient()
		if err == nil {
			poolCtx, poolCancel := context.WithTimeout(ctx, 10*time.Second)
			poolVMIDs := fetchPoolVMIDs(poolCtx, client, poolName)
			poolCancel()
			if len(poolVMIDs) >= settings.MaxVMPerUser {
				writeError(w, http.StatusConflict, "quota_exceeded",
					fmt.Sprintf("Maximum VM limit reached (%d/%d)", len(poolVMIDs), settings.MaxVMPerUser))
				return
			}
		}
	}

	// Validate network cards
	for i, net := range req.Networks {
		if net.Bridge == "" {
			errBadRequest(w, fmt.Sprintf("Bridge is required for network card %d", i))
			return
		}
		if net.MAC != "" && !utils.ValidateMACAddress(net.MAC) {
			errBadRequest(w, fmt.Sprintf("Invalid MAC address for network card %d", i))
			return
		}
		if net.VLAN != 0 && (net.VLAN < 1 || net.VLAN > 4096) {
			errBadRequest(w, "VLAN tag must be between 1 and 4096")
			return
		}
		if net.MTU != 0 && (net.MTU < 576 || net.MTU > 9000) {
			errBadRequest(w, "MTU must be between 576 and 9000")
			return
		}
	}

	// Get Proxmox client
	client, err := restyClient()
	if err != nil {
		logger.Get().Error().Err(err).Msg("api/v1: failed to create resty client for VM creation")
		errInternal(w)
		return
	}

	// Get next VMID
	vmid := 0
	if snapshot := h.state.GetProxmoxSnapshot(); snapshot != nil && len(snapshot.VMs) > 0 {
		highest := 0
		for _, svm := range snapshot.VMs {
			if svm.VMID > highest {
				highest = svm.VMID
			}
		}
		if highest > 0 {
			vmid = highest + 1
		}
	}
	if vmid == 0 {
		nextID, err := proxmox.GetNextVMIDResty(ctx, client)
		if err != nil {
			logger.Get().Error().Err(err).Msg("api/v1: failed to get next VMID")
			writeError(w, http.StatusInternalServerError, "vmid_error", "Failed to get next VMID")
			return
		}
		vmid = nextID
	}

	// Disk bus type
	diskBus := req.DiskBus
	if diskBus == "" {
		diskBus = "virtio"
	}
	validBuses := map[string]bool{"virtio": true, "scsi": true, "sata": true, "ide": true}
	if !validBuses[diskBus] {
		diskBus = "virtio"
	}

	// Build Proxmox API params
	params := url.Values{}
	params.Set("vmid", strconv.Itoa(vmid))
	params.Set("name", req.Name)
	params.Set("memory", strconv.Itoa(req.MemoryMB))
	params.Set("sockets", strconv.Itoa(req.Sockets))
	params.Set("cores", strconv.Itoa(req.Cores))
	params.Set("cpu", "host")
	params.Set("agent", "1")

	if req.Description != "" {
		params.Set("description", req.Description)
	}

	// Pool assignment for non-admin users
	pool := ""
	if !isAdmin && username != "" {
		pool = "pvmss_" + username
		params.Set("pool", pool)
	}

	// Tags — always include the "pvmss" tag so ListVMs/GetVM can find the VM.
	cleanedTags := []string{"pvmss"}
	for _, tag := range req.Tags {
		if cleaned := strings.TrimSpace(tag); cleaned != "" && !strings.EqualFold(cleaned, "pvmss") {
			cleanedTags = append(cleanedTags, cleaned)
		}
	}
	params.Set("tags", strings.Join(cleanedTags, ";"))

	// ISO
	if req.ISO != "" {
		params.Set("ide2", req.ISO+",media=cdrom")
		params.Set("boot", "order=ide2;"+diskBus+"0")
	} else {
		params.Set("boot", "order="+diskBus+"0")
	}

	// EFI
	if req.EnableEFI {
		params.Set("bios", "ovmf")
		params.Set("efidisk0", req.Storage+":1,format=raw,efitype=4m")
	}

	// Primary disk
	params.Set(diskBus+"0", fmt.Sprintf("%s:%d", req.Storage, req.Disks[0].SizeGB))

	// Additional disks
	maxDisks := settings.MaxDiskPerVM
	if maxDisks <= 0 {
		maxDisks = 1
	}
	for i := 1; i < len(req.Disks) && i < maxDisks; i++ {
		if req.Disks[i].SizeGB <= 0 {
			continue
		}
		params.Set(fmt.Sprintf("%s%d", diskBus, i), fmt.Sprintf("%s:%d", req.Storage, req.Disks[i].SizeGB))
	}

	// TPM
	if req.EnableTPM {
		tpmCompatible := false
		if snapshot := h.state.GetProxmoxSnapshot(); snapshot != nil {
			for _, st := range snapshot.GlobalStorages {
				if st.Storage == req.Storage {
					tpmCompatible = isTPMCompatibleStorage(st.Type)
					break
				}
			}
		}
		if tpmCompatible {
			params.Set("tpmstate0", fmt.Sprintf("%s:4,version=v2.0", req.Storage))
		}
	}

	// Network cards
	maxNetCards := settings.MaxNetworkCards
	if maxNetCards <= 0 {
		maxNetCards = 1
	}
	for i, net := range req.Networks {
		if i >= maxNetCards {
			break
		}
		if net.Bridge == "" {
			continue
		}
		model := net.Model
		if model == "" {
			model = "virtio"
		}
		validModels := map[string]bool{"virtio": true, "e1000": true, "e1000e": true, "rtl8139": true, "vmxnet3": true}
		if !validModels[model] {
			model = "virtio"
		}

		var netConfig string
		mac := strings.TrimSpace(net.MAC)
		if mac != "" {
			mac = utils.NormalizeMACAddress(mac)
			netConfig = model + "=" + mac + ",bridge=" + net.Bridge
		} else {
			netConfig = model + ",bridge=" + net.Bridge
		}
		if net.VLAN > 0 {
			netConfig += ",tag=" + strconv.Itoa(net.VLAN)
		}
		if net.RateLimit != "" {
			netConfig += ",rate=" + net.RateLimit
		}
		if net.MTU > 0 {
			netConfig += ",mtu=" + strconv.Itoa(net.MTU)
		}
		if !net.Enabled {
			netConfig += ",link_down=1"
		}
		params.Set(fmt.Sprintf("net%d", i), netConfig)
	}

	// Send creation request to Proxmox
	path := "/nodes/" + url.PathEscape(req.Node) + "/qemu"
	logger.Get().Info().Str("path", path).Int("vmid", vmid).Str("name", req.Name).Msg("api/v1: sending VM creation request")

	var createResp interface{}
	if err := client.Post(ctx, path, params, &createResp); err != nil {
		logger.Get().Error().Err(err).Int("vmid", vmid).Str("node", req.Node).Msg("api/v1: VM creation failed")
		writeError(w, http.StatusInternalServerError, "creation_failed", "Failed to create VM")
		return
	}

	// Apply cloud-init if requested
	cloudInitWarning := ""
	if req.CloudInit != nil {
		cloudInitWarning = h.applyCloudInit(ctx, client, req.Node, vmid, req.Storage, req.CloudInit, settings)
	}

	// Start VM if requested
	if req.StartVM {
		startCtx, startCancel := context.WithTimeout(ctx, 15*time.Second)
		defer startCancel()
		if _, err := proxmox.VMActionResty(startCtx, client, req.Node, strconv.Itoa(vmid), "start"); err != nil {
			logger.Get().Warn().Err(err).Int("vmid", vmid).Msg("api/v1: failed to start VM after creation")
		}
	}

	logger.Get().Info().
		Int("vmid", vmid).
		Str("name", req.Name).
		Str("node", req.Node).
		Str("username", username).
		Bool("is_admin", isAdmin).
		Int("sockets", req.Sockets).
		Int("cores", req.Cores).
		Int("memory_mb", req.MemoryMB).
		Int("disk_gb", req.Disks[0].SizeGB).
		Str("storage", req.Storage).
		Msg("api/v1: VM created successfully")

	// Request snapshot refresh so the new VM appears
	h.state.RequestSnapshotRefresh()

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, VMCreateResponse{
		VMID:          vmid,
		Name:          req.Name,
		Node:          req.Node,
		CloudInitWarn: cloudInitWarning,
	})
}

// applyCloudInit applies cloud-init configuration to a newly created VM.
func (h *VMCreateHandler) applyCloudInit(ctx context.Context, client *proxmox.RestyClient, node string, vmid int, storage string, ci *VMCreateCloudInit, settings *state.AppSettings) string {
	warning := ""

	ciParams := proxmox.CloudInitParams{CIUser: ci.User}
	if ci.Password != "" {
		ciParams.CIPassword = ci.Password
	}
	if ci.SSHKeys != "" {
		ciParams.SSHKeys = ci.SSHKeys
	}
	if ci.IPConfig == "static" && ci.IP != "" {
		ipConfig := "ip=" + ci.IP
		if ci.Gateway != "" {
			ipConfig += ",gw=" + ci.Gateway
		}
		ciParams.IPConfig0 = ipConfig
	} else {
		ciParams.IPConfig0 = "ip=dhcp"
	}
	if ci.DNS != "" {
		ciParams.Nameserver = ci.DNS
	}

	// Apply template if specified
	if ci.TemplateID != "" {
		template := settings.GetCloudInitTemplateByID(ci.TemplateID)
		if template != nil && strings.TrimSpace(template.YAMLContent) != "" {
			snippetStorage, err := h.selectSnippetStorage(ctx, client, node, template.Storage)
			if err == nil {
				filename := fmt.Sprintf("%s%d.yml", state.CloudInitTemplatePrefix, vmid)
				uploaded := false
				if settings.CloudInitSFTP.Enabled {
					if err := proxmox.UploadSnippetFileSFTP(ctx, settings.CloudInitSFTP, filename, template.YAMLContent); err == nil {
						uploaded = true
					}
				}
				if !uploaded {
					if err := proxmox.UploadSnippetFileResty(ctx, client, node, snippetStorage, filename, template.YAMLContent); err == nil {
						uploaded = true
					}
				}
				if uploaded {
					ciParams.CICustom = fmt.Sprintf("user=%s:snippets/%s", snippetStorage, filename)
				} else {
					warning = "upload-failed"
				}
			} else {
				warning = "no-snippets-storage"
			}
		}
	}

	// Ensure cloud-init drive
	if err := proxmox.EnsureCloudInitDriveResty(ctx, client, node, vmid, storage); err != nil {
		logger.Get().Error().Err(err).Int("vmid", vmid).Msg("api/v1: failed to ensure cloud-init drive")
	}

	// Apply cloud-init config
	if err := proxmox.UpdateVMCloudInitConfigResty(ctx, client, node, vmid, ciParams); err != nil {
		logger.Get().Error().Err(err).Int("vmid", vmid).Msg("api/v1: failed to apply cloud-init config")
		return "cloud-init-config-failed"
	}

	return warning
}

// selectSnippetStorage picks a snippets storage for the given node.
func (h *VMCreateHandler) selectSnippetStorage(ctx context.Context, client *proxmox.RestyClient, node string, preferred string) (string, error) {
	storages, err := proxmox.GetSnippetsStoragesResty(ctx, client)
	if err != nil {
		return "", err
	}
	var fallback string
	for _, s := range storages {
		if s.Nodes != "" {
			found := false
			for _, n := range strings.Split(s.Nodes, ",") {
				if strings.TrimSpace(n) == node {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if preferred != "" && s.Storage == preferred {
			return s.Storage, nil
		}
		if fallback == "" {
			fallback = s.Storage
		}
	}
	if fallback == "" {
		return "", fmt.Errorf("no snippets storage available for node %s", node)
	}
	return fallback, nil
}

// isTPMCompatibleStorage checks if the storage type supports TPM.
func isTPMCompatibleStorage(storageType string) bool {
	compatible := map[string]bool{
		"iscsi": true, "lvm": true, "lvmthin": true, "rbd": true, "zfs": true,
		"cephfs": true, "cifs": true, "dir": true, "nfs": true,
	}
	return compatible[storageType]
}
