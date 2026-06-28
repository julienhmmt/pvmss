package apiv1

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"pvmss/constants"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
	"pvmss/utils"
)

// VMCreateHandler handles VM creation API endpoints. The per-concern
// implementations live in sibling files:
//
//   - vm_create_resolve.go    — node / storage / bridge resolution
//   - vm_create_cloudinit.go  — cloud-init wiring + TPM compat check
type VMCreateHandler struct {
	state state.StateManager

	// uploadSnippetSFTP and uploadSnippetAPI are the snippet-upload backends.
	// They default to the real proxmox package functions but are overridable
	// in tests to exercise the SFTP-vs-API decision logic and warning codes
	// without a live Proxmox connection.
	uploadSnippetSFTP func(ctx context.Context, config proxmox.CloudInitSFTPConfig, filename, content string) error
	uploadSnippetAPI  func(ctx context.Context, client *proxmox.RestyClient, node, storage, filename, content string) error
}

// MakeVMCreateHandler creates a new VMCreateHandler.
func MakeVMCreateHandler(s state.StateManager) *VMCreateHandler {
	return &VMCreateHandler{
		state:             s,
		uploadSnippetSFTP: proxmox.UploadSnippetFileSFTP,
		uploadSnippetAPI:  proxmox.UploadSnippetFileResty,
	}
}

// ── Response types ────────────────────────────────────────────────────────────

// VMCreateSettingsResponse is the response for GET /api/v1/vm-create/settings.
type VMCreateSettingsResponse struct {
	Nodes              []VMCreateNodeOption    `json:"nodes"`
	Storages           []VMCreateStorageOption `json:"storages"`
	Bridges            []VMCreateBridgeOption  `json:"bridges"`
	ISOs               []VMCreateISOOption     `json:"isos"`
	Tags               []string                `json:"tags"`
	CloudInitTemplate  []VMCreateCITemplate    `json:"cloudinit_templates"`
	CloudInitAvailable bool                    `json:"cloud_init_available"`
	// CloudInitSFTPEnabled reports whether SFTP snippet upload is configured.
	// Custom-YAML templates can only be attached to a VM when SFTP is enabled
	// (the Proxmox HTTP API cannot reliably write snippet files), so the UI
	// gates the template picker on this flag.
	CloudInitSFTPEnabled bool `json:"cloud_init_sftp_enabled"`
	VMProfiles         []state.VMProfileConfig `json:"vm_profiles"`
	Limits             VMCreateLimits          `json:"limits"`
	MaxNetworkCards    int                     `json:"max_network_cards"`
	MaxDiskPerVM       int                     `json:"max_disk_per_vm"`
	MaxVMPerUser       int                     `json:"max_vm_per_user"`
	RemainingVMs       int                     `json:"remaining_vms"`
	ProxmoxConnected   bool                    `json:"proxmox_connected"`
	AllowCustomYAML    bool                    `json:"allow_custom_yaml"`
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
	Bridge  string `json:"bridge"`
	Model   string `json:"model"`
	MAC     string `json:"mac,omitempty"`
	VLAN    int    `json:"vlan,omitempty"`
	Rate    string `json:"rate,omitempty"`
	MTU     int    `json:"mtu,omitempty"`
	Enabled bool   `json:"enabled"`
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
	UPID          string `json:"upid,omitempty"`
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
		VMProfiles:       settings.GetEnabledVMProfiles(),
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

	isos := make([]VMCreateISOOption, 0, len(settings.ISOs))
	for _, volid := range settings.ISOs {
		name := volid
		if idx := strings.LastIndex(volid, "/"); idx >= 0 {
			name = volid[idx+1:]
		}
		isos = append(isos, VMCreateISOOption{VolID: volid, Name: name})
	}
	resp.ISOs = isos

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
	resp.CloudInitAvailable = len(ciTemplates) > 0
	resp.CloudInitSFTPEnabled = settings.CloudInitSFTP.Enabled

	resp.RemainingVMs = -1
	if settings.MaxVMPerUser > 0 && !isAdmin {
		poolName := constants.PoolPrefix + username
		remaining := settings.MaxVMPerUser
		if connected {
			cfg := h.state.GetEnvConfig()
			client, err := proxmox.MakeRestyClientFromEnvConfig(cfg, 30*time.Second)
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
	if !decodeBody(w, r, &req) {
		return
	}

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

	if len(settings.EnabledStorages) > 0 {
		storageAllowed := false
		for _, enabledStorage := range settings.EnabledStorages {
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

	if len(settings.VMBRs) > 0 {
		for i, net := range req.Networks {
			if net.Bridge == "" {
				continue
			}
			bridgeAllowed := false
			for _, allowedVMBR := range settings.VMBRs {
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
	ramGB := (req.MemoryMB + 512) / 1024
	if ramGB < limits.RAM.Min || ramGB > limits.RAM.Max {
		errBadRequest(w, fmt.Sprintf("RAM must be between %d and %d GB", limits.RAM.Min, limits.RAM.Max))
		return
	}
	for i, disk := range req.Disks {
		if disk.SizeGB < limits.Disk.Min || disk.SizeGB > limits.Disk.Max {
			errBadRequest(w, fmt.Sprintf("Disk %d must be between %d and %d GB", i+1, limits.Disk.Min, limits.Disk.Max))
			return
		}
	}

	if err := validateNodeAggregateLimits(h.state, req.Node, req.Sockets, req.Cores, req.MemoryMB); err != nil {
		writeError(w, http.StatusConflict, nodeLimitCode(err), err.Error())
		return
	}
	nodeLimitReservationCommitted := false
	defer func() {
		if !nodeLimitReservationCommitted {
			releaseNodeAggregateReservation(req.Node, req.Sockets, req.Cores, req.MemoryMB)
		}
	}()

	if settings.MaxVMPerUser > 0 && !isAdmin {
		poolName := constants.PoolPrefix + username
		cfg := h.state.GetEnvConfig()
		client, err := proxmox.MakeRestyClientFromEnvConfig(cfg, 30*time.Second)
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
		if net.Rate != "" {
			rate := strings.TrimSpace(net.Rate)
			rateLimit, err := strconv.ParseFloat(rate, 64)
			if err != nil || rateLimit < 0 {
				errBadRequest(w, fmt.Sprintf("network[%d]: rate limit must be a non-negative number", i))
				return
			}
		}
	}

	cfg := h.state.GetEnvConfig()
	client, err := proxmox.MakeRestyClientFromEnvConfig(cfg, 30*time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}

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
		nextID, err := proxmox.GetClusterNextIDResty(ctx, client)
		if err != nil {
			logger.Get().Error().Err(err).Msg("api/v1: failed to get next VMID")
			writeError(w, http.StatusInternalServerError, "vmid_error", "Failed to get next VMID")
			return
		}
		vmid = nextID
	}

	diskBus := req.DiskBus
	if diskBus == "" {
		diskBus = "virtio"
	}
	validBuses := map[string]bool{"virtio": true, "scsi": true, "sata": true, "ide": true}
	if !validBuses[diskBus] {
		diskBus = "virtio"
	}

	var maxDisksForBus int
	switch diskBus {
	case state.DiskBusIDE:
		maxDisksForBus = state.MaxDisksIDE
	case state.DiskBusSATA:
		maxDisksForBus = state.MaxDisksSATA
	case state.DiskBusVirtIO:
		maxDisksForBus = state.MaxDisksVirtIO
	case state.DiskBusSCSI:
		maxDisksForBus = state.MaxDisksSCSI
	default:
		maxDisksForBus = state.MaxDisksVirtIO
	}
	if len(req.Disks) > maxDisksForBus {
		errBadRequest(w, fmt.Sprintf("Too many disks for %s bus: maximum %d disks allowed", diskBus, maxDisksForBus))
		return
	}

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

	pool := ""
	if !isAdmin && username != "" {
		pool = constants.PoolPrefix + username
		params.Set("pool", pool)
	}

	cleanedTags := []string{constants.RequiredTag}
	for _, tag := range req.Tags {
		if cleaned := strings.TrimSpace(tag); cleaned != "" && !strings.EqualFold(cleaned, constants.RequiredTag) {
			cleanedTags = append(cleanedTags, cleaned)
		}
	}
	params.Set("tags", strings.Join(cleanedTags, ";"))

	if req.ISO != "" {
		if !strings.Contains(req.ISO, ":") {
			errBadRequest(w, "Invalid ISO volume ID: expected format storage:path/to/file.iso")
			return
		}
		params.Set("ide2", req.ISO+",media=cdrom")
		params.Set("boot", "order=ide2;"+diskBus+"0")
	} else {
		params.Set("boot", "order="+diskBus+"0")
	}

	if req.EnableEFI {
		params.Set("bios", "ovmf")
		params.Set("efidisk0", req.Storage+":1,format=raw,efitype=4m")
	}

	params.Set(diskBus+"0", fmt.Sprintf("%s:%d", req.Storage, req.Disks[0].SizeGB))

	maxDisks := settings.MaxDiskPerVM
	if maxDisks <= 0 {
		maxDisks = 1
	}
	isoReservesIDE2 := diskBus == state.DiskBusIDE && req.ISO != ""
	slotOffset := 0
	for i := 1; i < len(req.Disks) && i < maxDisks; i++ {
		if req.Disks[i].SizeGB <= 0 {
			continue
		}
		slot := i + slotOffset
		if isoReservesIDE2 && slot == 2 {
			slotOffset++
			slot++
		}
		params.Set(fmt.Sprintf("%s%d", diskBus, slot), fmt.Sprintf("%s:%d", req.Storage, req.Disks[i].SizeGB))
	}

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
		if net.Rate != "" {
			rate := strings.TrimSpace(net.Rate)
			if rate != "" && rate != "0" {
				netConfig += ",rate=" + rate
			}
		}
		if net.MTU > 0 {
			netConfig += ",mtu=" + strconv.Itoa(net.MTU)
		}
		if !net.Enabled {
			netConfig += ",link_down=1"
		}
		params.Set(fmt.Sprintf("net%d", i), netConfig)
	}

	path := "/nodes/" + url.PathEscape(req.Node) + "/qemu"
	logger.Get().Info().Str("path", path).Int("vmid", vmid).Str("name", req.Name).Msg("api/v1: sending VM creation request")

	var createResp proxmox.Response[string]
	if err := client.Post(ctx, path, params, &createResp); err != nil {
		writeAppError(w, err)
		return
	}
	nodeLimitReservationCommitted = true
	upid := createResp.Data

	logger.Get().Info().
		Int("vmid", vmid).
		Str("name", req.Name).
		Str("node", req.Node).
		Str("upid", upid).
		Str("username", username).
		Bool("is_admin", isAdmin).
		Int("sockets", req.Sockets).
		Int("cores", req.Cores).
		Int("memory_mb", req.MemoryMB).
		Int("disk_gb", req.Disks[0].SizeGB).
		Str("storage", req.Storage).
		Msg("api/v1: VM creation task dispatched")

	proxmox.InvalidateVMCache(req.Node)
	h.state.RequestSnapshotRefresh()

	if upid != "" {
		// Mark cloud-init as pending so the task-status endpoint knows to wait for
		// the asynchronous cloud-init result (see finalizeAfterTask). Only set
		// when cloud-init was actually requested.
		if req.CloudInit != nil {
			h.state.SetCloudInitWarning(upid, state.CloudInitPending)
		}
		go h.finalizeAfterTask(client, req.Node, vmid, upid, req.StartVM, req.CloudInit, req.Storage, settings, req.Sockets, req.Cores, req.MemoryMB)

		w.WriteHeader(http.StatusAccepted)
		writeJSON(w, VMCreateResponse{
			VMID:          vmid,
			Name:          req.Name,
			Node:          req.Node,
			UPID:          upid,
			CloudInitWarn: "",
		})
		return
	}

	cloudInitWarning := ""
	if req.CloudInit != nil {
		cloudInitWarning = h.applyCloudInit(ctx, client, req.Node, vmid, req.Storage, req.CloudInit, settings)
	}
	if req.StartVM {
		startCtx, startCancel := context.WithTimeout(ctx, 15*time.Second)
		defer startCancel()
		if _, err := proxmox.VMActionResty(startCtx, client, req.Node, strconv.Itoa(vmid), "start"); err != nil {
			logger.Get().Warn().Err(err).Int("vmid", vmid).Msg("api/v1: failed to start VM after creation")
		}
	}
	h.state.RequestSnapshotRefresh()
	releaseNodeAggregateReservation(req.Node, req.Sockets, req.Cores, req.MemoryMB)

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, VMCreateResponse{
		VMID:          vmid,
		Name:          req.Name,
		Node:          req.Node,
		UPID:          "",
		CloudInitWarn: cloudInitWarning,
	})
}

// finalizeAfterTask polls a Proxmox creation task UPID until it completes,
// then applies cloud-init config, performs a final cache refresh, and
// optionally starts the VM.
//
// It runs in a goroutine and does not affect the HTTP response.
func (h *VMCreateHandler) finalizeAfterTask(client *proxmox.RestyClient, node string, vmid int, upid string, startVM bool, ci *VMCreateCloudInit, storage string, settings *state.AppSettings, sockets, cores, memoryMB int) {
	defer releaseNodeAggregateReservation(node, sockets, cores, memoryMB)

	if client == nil {
		logger.Get().Error().Int("vmid", vmid).Msg("api/v1: finalizeAfterTask: resty client is nil")
		if ci != nil {
			h.state.SetCloudInitWarning(upid, "")
		}
		return
	}
	if settings == nil {
		logger.Get().Error().Int("vmid", vmid).Msg("api/v1: finalizeAfterTask: settings is nil")
		if ci != nil {
			h.state.SetCloudInitWarning(upid, "")
		}
		return
	}

	const (
		pollInterval = 3 * time.Second
		maxWait      = 10 * time.Minute
	)

	ctx, cancelAll := context.WithTimeout(context.Background(), maxWait)
	defer cancelAll()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			logger.Get().Warn().Int("vmid", vmid).Str("upid", upid).Msg("api/v1: finalizeAfterTask: timed out waiting for creation task")
			if ci != nil {
				h.state.SetCloudInitWarning(upid, "")
			}
			return
		case <-ticker.C:
		}

		pollCtx, pollCancel := context.WithTimeout(ctx, 10*time.Second)
		status, pollErr := proxmox.GetTaskStatusResty(pollCtx, client, node, upid)
		pollCancel()

		if pollErr != nil {
			logger.Get().Warn().Err(pollErr).Int("vmid", vmid).Str("upid", upid).Msg("api/v1: finalizeAfterTask: polling error, retrying")
			continue
		}

		if status.Status != "stopped" {
			continue
		}

		proxmox.InvalidateVMCache(node)
		h.state.RequestSnapshotRefresh()

		if status.ExitStatus != "OK" {
			logger.Get().Warn().Str("exit_status", status.ExitStatus).Int("vmid", vmid).Msg("api/v1: finalizeAfterTask: creation task did not succeed")
			if ci != nil {
				h.state.SetCloudInitWarning(upid, "")
			}
			return
		}

		if ci != nil {
			ciCtx, ciCancel := context.WithTimeout(ctx, 30*time.Second)
			warn := h.applyCloudInit(ciCtx, client, node, vmid, storage, ci, settings)
			ciCancel()
			// Replace the pending sentinel with the real result (empty = OK).
			h.state.SetCloudInitWarning(upid, warn)
			if warn != "" {
				logger.Get().Warn().Str("warning", warn).Int("vmid", vmid).Msg("api/v1: finalizeAfterTask: cloud-init issue")
			}
		}

		if startVM {
			startCtx, startCancel := context.WithTimeout(ctx, 15*time.Second)
			_, startErr := proxmox.VMActionResty(startCtx, client, node, strconv.Itoa(vmid), "start")
			startCancel()
			if startErr != nil {
				logger.Get().Warn().Err(startErr).Int("vmid", vmid).Msg("api/v1: finalizeAfterTask: failed to start VM")
			} else {
				logger.Get().Info().Int("vmid", vmid).Str("node", node).Msg("api/v1: finalizeAfterTask: VM started after creation")
			}
		}
		return
	}
}
