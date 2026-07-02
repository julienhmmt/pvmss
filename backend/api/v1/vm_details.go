package apiv1

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"pvmss/constants"
	"pvmss/proxmox"
	"pvmss/state"
)

// VMDetailsHandler handles VM detail, config, snapshot and update endpoints.
type VMDetailsHandler struct {
	state state.StateManager
}

// MakeVMDetailsHandler creates a new VMDetailsHandler.
func MakeVMDetailsHandler(s state.StateManager) *VMDetailsHandler {
	return &VMDetailsHandler{state: s}
}

// ── Response types ────────────────────────────────────────────────────────────

type VMConfigResponse struct {
	VMID        int                        `json:"vmid"`
	Name        string                     `json:"name"`
	Node        string                     `json:"node"`
	Status      string                     `json:"status"`
	CPU         float64                    `json:"cpu"`
	CPUs        int                        `json:"cpus"`
	Sockets     int                        `json:"sockets"`
	Cores       int                        `json:"cores"`
	MemMB       int64                      `json:"mem_mb"`
	MaxMemMB    int64                      `json:"max_mem_mb"`
	DiskMB      int64                      `json:"disk_mb"`
	Uptime      int64                      `json:"uptime"`
	Tags        string                     `json:"tags"`
	Description string                     `json:"description"`
	Networks    []proxmox.NetworkInterface `json:"networks"`
	Disks       []DiskInfo                 `json:"disks"`
	HasCDROM    bool                       `json:"has_cdrom"`
	CurrentISO  string                     `json:"current_iso"`
	EFIEnabled  bool                       `json:"efi_enabled"`
	TPMEnabled  bool                       `json:"tpm_enabled"`
	CloudInit   *CloudInitInfo             `json:"cloud_init,omitempty"`
	// CloudInitSFTPEnabled reports whether SFTP snippet upload is configured.
	// The frontend uses this to enable/disable the custom cloud-config YAML
	// editor in the cloud-init tab (the Proxmox HTTP API cannot reliably read
	// or write snippets, so SFTP is required).
	CloudInitSFTPEnabled bool `json:"cloud_init_sftp_enabled"`
}

type DiskInfo struct {
	Index   string `json:"index"`
	Bus     string `json:"bus"`
	Storage string `json:"storage"`
	SizeGB  int    `json:"size_gb"`
	Raw     string `json:"raw"`
	IsBoot  bool   `json:"is_boot"`
}

type CloudInitInfo struct {
	User         string `json:"user,omitempty"`
	SSHKeys      string `json:"ssh_keys,omitempty"`
	IPConfig     string `json:"ip_config,omitempty"`
	Nameserver   string `json:"nameserver,omitempty"`
	Searchdomain string `json:"searchdomain,omitempty"`
	CICustom     string `json:"cicustom,omitempty"` // e.g. "user=local:snippets/pvmss-100.yml"
}

type VMMetricsResponse struct {
	Status string  `json:"status"`
	CPU    float64 `json:"cpu"`
	MemMB  int64   `json:"mem_mb"`
	MaxMem int64   `json:"max_mem_mb"`
}

type SnapshotResponse struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Snaptime    int64  `json:"snaptime"`
	Vmstate     int    `json:"vmstate"`
	Parent      string `json:"parent"`
	Current     bool   `json:"current"`
}

type SnapshotListResponse struct {
	Snapshots  []SnapshotResponse `json:"snapshots"`
	MaxAllowed int                `json:"max_allowed"`
}

type VMSettingsResponse struct {
	AvailableISOs     []ISOOption     `json:"available_isos"`
	AvailableVMBRs    []VMBROption    `json:"available_vmbrs"`
	AvailableTags     []string        `json:"available_tags"`
	AvailableStorages []StorageOption `json:"available_storages"`
	Limits            LimitsInfo      `json:"limits"`
}

type ISOOption struct {
	VolID   string `json:"volid"`
	Name    string `json:"name"`
	Node    string `json:"node"`
	Enabled bool   `json:"enabled"`
}

type VMBROption struct {
	Iface   string `json:"iface"`
	Node    string `json:"node"`
	Enabled bool   `json:"enabled"`
}

type StorageOption struct {
	Storage string `json:"storage"`
	Node    string `json:"node,omitempty"`
	Enabled bool   `json:"enabled"`
}

type LimitsInfo struct {
	MinSockets    int `json:"min_sockets"`
	MaxSockets    int `json:"max_sockets"`
	MinCores      int `json:"min_cores"`
	MaxCores      int `json:"max_cores"`
	MinRAMGB      int `json:"min_ram_gb"`
	MaxRAMGB      int `json:"max_ram_gb"`
	MinDiskGB     int `json:"min_disk_gb"`
	MaxDiskGB     int `json:"max_disk_gb"`
	MaxDisksPerVM int `json:"max_disks_per_vm"`
	MaxSnapshots  int `json:"max_snapshots"`
}

// ── Helpers ───────────────────────────────────────────────────────────────────

func (h *VMDetailsHandler) isOffline() bool {
	return h.state != nil && h.state.IsOfflineMode()
}

// ownsVM checks pool membership for non-admin users.
func ownsVM(ctx context.Context, client *proxmox.RestyClient, username string, isAdmin bool, vmid int) bool {
	if isAdmin {
		return true
	}
	pool := fetchPoolVMIDs(ctx, client, constants.PoolPrefix+username)
	return pool[vmid]
}

// resolveNode finds the node a VM lives on by calling GetVMsResty.
func resolveNode(ctx context.Context, client *proxmox.RestyClient, vmid int) (string, error) {
	vms, err := proxmox.GetVMsResty(ctx, client)
	if err != nil {
		return "", err
	}
	for _, vm := range vms {
		if vm.VMID == vmid {
			return vm.Node, nil
		}
	}
	return "", nil
}

// resolveNodeFromList finds the node a VM lives on using the list already in hand.
func resolveNodeFromList(allVMs []proxmox.VM, vmid int) (string, error) {
	for _, vm := range allVMs {
		if vm.VMID == vmid {
			return vm.Node, nil
		}
	}
	return "", nil
}

func parseDisks(cfg map[string]interface{}) ([]DiskInfo, bool, string, int64) {
	buses := []string{"virtio", "scsi", "sata", "ide"}
	var disks []DiskInfo
	var hasCDROM bool
	var currentISO string
	var totalMB int64

	for _, bus := range buses {
		for i := 0; i < state.GetMaxDisksForBus(bus); i++ {
			key := bus + strconv.Itoa(i)
			val, ok := cfg[key].(string)
			if !ok || val == "" {
				continue
			}
			// CD-ROM detection
			if strings.Contains(val, "media=cdrom") || bus == "ide" && strings.HasSuffix(strings.SplitN(val, ",", 2)[0], ".iso") {
				hasCDROM = true
				parts := strings.SplitN(val, ",", 2)
				if parts[0] != "none" && !strings.HasPrefix(parts[0], "local") {
					currentISO = parts[0]
				} else if strings.HasSuffix(parts[0], ".iso") {
					currentISO = parts[0]
				}
				continue
			}
			// Parse size
			sizeGB := 0
			for _, part := range strings.Split(val, ",") {
				if strings.HasPrefix(part, "size=") {
					s := strings.TrimPrefix(part, "size=")
					if strings.HasSuffix(s, "G") {
						if n, err := strconv.Atoi(strings.TrimSuffix(s, "G")); err == nil {
							sizeGB = n
						}
					} else if strings.HasSuffix(s, "T") {
						if n, err := strconv.Atoi(strings.TrimSuffix(s, "T")); err == nil {
							sizeGB = n * 1024
						}
					} else if strings.HasSuffix(s, "M") {
						if n, err := strconv.Atoi(strings.TrimSuffix(s, "M")); err == nil {
							sizeGB = n / 1024
							if sizeGB == 0 {
								sizeGB = 1 // Round up sub-GB sizes to 1 GB
							}
						}
					}
				}
			}
			storage := ""
			parts := strings.SplitN(val, ":", 2)
			if len(parts) == 2 && parts[0] != "" {
				storage = parts[0]
			}
			disks = append(disks, DiskInfo{
				Index:   key,
				Bus:     bus,
				Storage: storage,
				SizeGB:  sizeGB,
				Raw:     val,
				IsBoot:  isBootDisk(cfg, key),
			})
			totalMB += int64(sizeGB) * 1024
		}
	}
	return disks, hasCDROM, currentISO, totalMB
}

func parseCloudInit(cfg map[string]interface{}) *CloudInitInfo {
	has := false
	ci := &CloudInitInfo{}
	if v, ok := cfg["ciuser"].(string); ok && v != "" {
		ci.User = v
		has = true
	}
	if v, ok := cfg["sshkeys"].(string); ok && v != "" {
		decoded, err := url.QueryUnescape(v)
		if err != nil || decoded == "" {
			decoded = v
		}
		ci.SSHKeys = decoded
		has = true
	}
	if v, ok := cfg["ipconfig0"].(string); ok && v != "" {
		ci.IPConfig = v
		has = true
	}
	if v, ok := cfg["nameserver"].(string); ok && v != "" {
		ci.Nameserver = v
		has = true
	}
	if v, ok := cfg["searchdomain"].(string); ok && v != "" {
		ci.Searchdomain = v
		has = true
	}
	if v, ok := cfg["cicustom"].(string); ok && v != "" {
		ci.CICustom = v
		has = true
	}
	if !has {
		return nil
	}
	return ci
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// GetVMConfig handles GET /api/v1/vms/:id/config
// Returns full VM configuration including disks, networks, cloud-init.
func (h *VMDetailsHandler) GetVMConfig(w http.ResponseWriter, r *http.Request) {
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}
	if h.isOffline() {
		errOffline(w)
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	envCfg := h.state.GetEnvConfig()
	client, err := proxmox.MakeRestyClientFromEnvConfig(envCfg, 30*time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	if !ownsVM(ctx, client, username, isAdmin, vmid) {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	// Get all VMs once - reuse for both node resolution and summary lookup
	allVMs, err := proxmox.GetVMsResty(ctx, client)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal_error", "Failed to retrieve VM list")
		return
	}

	node, err := resolveNodeFromList(allVMs, vmid)
	if err != nil || node == "" {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	// Get the VM summary (for fields not in VMCurrent: MaxDisk, Uptime, Tags)
	var vmSummary *proxmox.VM
	for i := range allVMs {
		if allVMs[i].VMID == vmid {
			vmSummary = &allVMs[i]
			break
		}
	}

	// Runtime status and full config are independent GETs against the same
	// node — fetch them concurrently. errgroup cancels the sibling call on the
	// first error (both must succeed, matching the previous sequential
	// fail-fast behavior). g.Wait() provides the happens-before that makes the
	// distinct current/cfg writes safe to read below.
	var (
		current *proxmox.VMCurrent
		cfg     map[string]any
	)
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() (err error) {
		current, err = proxmox.GetVMCurrentResty(gctx, client, node, vmid)
		return err
	})
	g.Go(func() (err error) {
		cfg, err = proxmox.GetVMConfigResty(gctx, client, node, vmid)
		return err
	})
	if err := g.Wait(); err != nil {
		writeAppError(w, err)
		return
	}

	// Parse networks
	networks := proxmox.ExtractNetworkInterfaces(cfg)

	// Try to enrich with guest agent IPs (best-effort, short timeout)
	if current.Status == "running" {
		agentCtx, agentCancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer agentCancel()
		if guestIfaces, err := proxmox.GetGuestAgentNetworkInterfacesResty(agentCtx, client, node, vmid); err == nil {
			proxmox.EnrichNetworkInterfacesWithIPs(networks, guestIfaces)
		}
	}

	// Parse disks
	disks, hasCDROM, currentISO, _ := parseDisks(cfg)

	// Parse cloud-init
	cloudInit := parseCloudInit(cfg)

	// EFI / TPM detection
	efiEnabled := false
	if _, ok := cfg["efidisk0"]; ok {
		efiEnabled = true
	}
	tpmEnabled := false
	if _, ok := cfg["tpmstate0"]; ok {
		tpmEnabled = true
	}

	description := ""
	if v, ok := cfg["description"].(string); ok {
		description = v
	}

	sockets := 1
	if v, ok := cfg["sockets"].(float64); ok && v > 0 {
		sockets = int(v)
	}
	cores := 1
	if v, ok := cfg["cores"].(float64); ok && v > 0 {
		cores = int(v)
	}

	const mb = int64(1024 * 1024)
	var diskMB, uptime int64
	var tags string
	if vmSummary != nil {
		diskMB = vmSummary.MaxDisk / mb
		uptime = vmSummary.Uptime
		tags = vmSummary.Tags
	}
	resp := VMConfigResponse{
		VMID:                 vmid,
		Name:                 current.Name,
		Node:                 node,
		Status:               current.Status,
		CPU:                  current.CPU,
		CPUs:                 current.CPUs,
		Sockets:              sockets,
		Cores:                cores,
		MemMB:                current.Mem / mb,
		MaxMemMB:             current.MaxMem / mb,
		DiskMB:               diskMB,
		Uptime:               uptime,
		Tags:                 tags,
		Description:          description,
		Networks:             networks,
		Disks:                disks,
		HasCDROM:             hasCDROM,
		CurrentISO:           currentISO,
		EFIEnabled:           efiEnabled,
		TPMEnabled:           tpmEnabled,
		CloudInit:            cloudInit,
		CloudInitSFTPEnabled: h.state.GetSettings().CloudInitSFTP.Enabled,
	}

	writeJSON(w, resp)
}

// GetVMMetrics handles GET /api/v1/vms/:id/metrics — lightweight live poll.
func (h *VMDetailsHandler) GetVMMetrics(w http.ResponseWriter, r *http.Request) {
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}
	if h.isOffline() {
		errOffline(w)
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	envCfg := h.state.GetEnvConfig()
	client, err := proxmox.MakeRestyClientFromEnvConfig(envCfg, 30*time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	if !ownsVM(ctx, client, username, isAdmin, vmid) {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	node, err := resolveNode(ctx, client, vmid)
	if err != nil || node == "" {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	current, err := proxmox.GetVMCurrentResty(ctx, client, node, vmid)
	if err != nil {
		writeAppError(w, err)
		return
	}

	const mb = int64(1024 * 1024)
	writeJSON(w, VMMetricsResponse{
		Status: current.Status,
		CPU:    current.CPU,
		MemMB:  current.Mem / mb,
		MaxMem: current.MaxMem / mb,
	})
}

// UpdateVMConfig handles PATCH /api/v1/vms/:id/config
// Supports updating description, tags, and name.
func (h *VMDetailsHandler) UpdateVMConfig(w http.ResponseWriter, r *http.Request) {
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}

	var req struct {
		Description *string `json:"description"`
		Tags        *string `json:"tags"`
		Name        *string `json:"name"`
	}
	if !decodeBody(w, r, &req) {
		return
	}

	if req.Name != nil {
		name := *req.Name
		if len(name) == 0 || len(name) > 64 {
			errBadRequest(w, "name must be between 1 and 64 characters")
			return
		}
		for _, ch := range name {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || ch == '-' || ch == '_' || ch == '.') { //nolint:staticcheck // clearer as positive character check
				errBadRequest(w, "name may only contain letters, digits, hyphens, underscores, and dots")
				return
			}
		}
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	envCfg := h.state.GetEnvConfig()
	client, err := proxmox.MakeRestyClientFromEnvConfig(envCfg, 30*time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if !ownsVM(ctx, client, username, isAdmin, vmid) {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	node, err := resolveNode(ctx, client, vmid)
	if err != nil || node == "" {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	params := map[string]string{}
	if req.Description != nil {
		params["description"] = *req.Description
	}
	if req.Tags != nil {
		params["tags"] = *req.Tags
	}
	if req.Name != nil {
		params["name"] = *req.Name
	}
	if len(params) == 0 {
		errBadRequest(w, "nothing to update")
		return
	}

	if err := proxmox.UpdateVMConfigResty(ctx, client, node, vmid, params); err != nil {
		writeAppError(w, err)
		return
	}
	proxmox.InvalidateVMCache(node)
	writeJSON(w, map[string]string{"status": "ok"})
}

// UpdateVMCDROM handles PATCH /api/v1/vms/:id/cdrom
// Allows setting or removing the CD-ROM ISO.
func (h *VMDetailsHandler) UpdateVMCDROM(w http.ResponseWriter, r *http.Request) {
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}

	var req struct {
		ISO        *string `json:"iso"`        // ISO volid (e.g., "local:iso/debian.iso") or empty to remove
		Disconnect bool    `json:"disconnect"` // If true, disconnect CD-ROM (keep drive but no ISO)
	}
	if !decodeBody(w, r, &req) {
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	envCfg := h.state.GetEnvConfig()
	client, err := proxmox.MakeRestyClientFromEnvConfig(envCfg, 30*time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if !ownsVM(ctx, client, username, isAdmin, vmid) {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	node, err := resolveNode(ctx, client, vmid)
	if err != nil || node == "" {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	params := map[string]string{}
	if req.Disconnect {
		// Disconnect CD-ROM (remove current ISO)
		params["ide2"] = "none"
	} else if req.ISO != nil {
		if *req.ISO == "" {
			// Remove CD-ROM
			params["ide2"] = "none"
		} else {
			// Set CD-ROM ISO
			if !strings.Contains(*req.ISO, ":") {
				errBadRequest(w, "invalid ISO format: expected storage:path")
				return
			}
			params["ide2"] = *req.ISO + ",media=cdrom"
		}
	} else {
		errBadRequest(w, "iso field is required")
		return
	}

	if err := proxmox.UpdateVMConfigResty(ctx, client, node, vmid, params); err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

// cloudInitUpdateRequest is the body for PUT /api/v1/vms/:id/cloudinit.
// Pointer fields distinguish "leave unchanged" (nil) from "clear the key"
// (non-nil empty string). Password is write-only: an empty string keeps the
// current value (Proxmox does not expose the existing password).
type cloudInitUpdateRequest struct {
	User         *string `json:"user"`
	Password     string  `json:"password"`
	SSHKeys      *string `json:"ssh_keys"`
	IPConfig     *string `json:"ip_config"`
	Nameserver   *string `json:"nameserver"`
	Searchdomain *string `json:"searchdomain"`
}

// UpdateVMCloudInit handles PUT /api/v1/vms/:id/cloudinit.
// Updates Cloud-Init fields (user, password, ssh keys, ipconfig0, nameserver,
// searchdomain) on an existing VM. Pool membership is enforced for non-admin
// users (same AuthZ pattern as the other VM detail mutators). All provided
// fields are validated server-side before the Proxmox call.
func (h *VMDetailsHandler) UpdateVMCloudInit(w http.ResponseWriter, r *http.Request) {
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}

	var req cloudInitUpdateRequest
	if !decodeBody(w, r, &req) {
		return
	}

	// Validate every provided field before contacting Proxmox so bad input
	// never reaches the cluster. Empty values that mean "clear" are allowed
	// through and handled by SetVMCloudInitConfigResty via the delete param.
	if req.User != nil && strings.TrimSpace(*req.User) != "" {
		if err := validateCIUser(strings.TrimSpace(*req.User)); err != nil {
			writeAppError(w, err)
			return
		}
	}
	if req.Password != "" {
		if err := validateCIPassword(req.Password); err != nil {
			writeAppError(w, err)
			return
		}
	}
	if req.SSHKeys != nil && strings.TrimSpace(*req.SSHKeys) != "" {
		if err := validateCISSHKeys(*req.SSHKeys); err != nil {
			writeAppError(w, err)
			return
		}
	}
	if req.IPConfig != nil && strings.TrimSpace(*req.IPConfig) != "" {
		if err := validateCIIPConfig(*req.IPConfig); err != nil {
			writeAppError(w, err)
			return
		}
	}
	if req.Nameserver != nil && strings.TrimSpace(*req.Nameserver) != "" {
		if err := validateCIDNSList(*req.Nameserver); err != nil {
			writeAppError(w, err)
			return
		}
	}
	if req.Searchdomain != nil && strings.TrimSpace(*req.Searchdomain) != "" {
		if err := validateCISearchdomain(*req.Searchdomain); err != nil {
			writeAppError(w, err)
			return
		}
	}

	// Reject empty updates early so a no-op request gets a clear 400 instead
	// of progressing to the (offline/Proxmox) gates or a silent success.
	nothingToUpdate := req.User == nil && req.Password == "" && req.SSHKeys == nil &&
		req.IPConfig == nil && req.Nameserver == nil && req.Searchdomain == nil
	if nothingToUpdate {
		errBadRequest(w, "no cloud-init fields to update")
		return
	}

	if h.isOffline() {
		errOffline(w)
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	envCfg := h.state.GetEnvConfig()
	client, err := proxmox.MakeRestyClientFromEnvConfig(envCfg, 30*time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	if !ownsVM(ctx, client, username, isAdmin, vmid) {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	node, err := resolveNode(ctx, client, vmid)
	if err != nil || node == "" {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	// Trim provided values so leading/trailing whitespace never reaches Proxmox.
	upd := proxmox.CloudInitUpdate{Password: req.Password}
	if req.User != nil {
		u := strings.TrimSpace(*req.User)
		upd.User = &u
	}
	if req.SSHKeys != nil {
		k := strings.TrimSpace(*req.SSHKeys)
		upd.SSHKeys = &k
	}
	if req.IPConfig != nil {
		ip := strings.TrimSpace(*req.IPConfig)
		upd.IPConfig0 = &ip
	}
	if req.Nameserver != nil {
		ns := strings.TrimSpace(*req.Nameserver)
		upd.Nameserver = &ns
	}
	if req.Searchdomain != nil {
		sd := strings.TrimSpace(*req.Searchdomain)
		upd.Searchdomain = &sd
	}

	if err := proxmox.SetVMCloudInitConfigResty(ctx, client, node, vmid, upd); err != nil {
		writeAppError(w, err)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

// GetVMSettings handles GET /api/v1/vms/:id/settings
// Returns allowed ISOs, VMBRs, tags and resource limits for the VM edit form.
func (h *VMDetailsHandler) GetVMSettings(w http.ResponseWriter, r *http.Request) {
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}

	if h.isOffline() {
		errOffline(w)
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	envCfg := h.state.GetEnvConfig()
	client, err := proxmox.MakeRestyClientFromEnvConfig(envCfg, 30*time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	if !ownsVM(ctx, client, username, isAdmin, vmid) {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	settings := h.state.GetSettings()

	// settings.ISOs is []string (volid values like "local:iso/debian.iso")
	isos := make([]ISOOption, 0, len(settings.ISOs))
	for _, volid := range settings.ISOs {
		parts := strings.SplitN(volid, ":", 2)
		name := volid
		if len(parts) == 2 {
			name = parts[1]
		}
		isos = append(isos, ISOOption{
			VolID:   volid,
			Name:    name,
			Enabled: true,
		})
	}

	// settings.VMBRs is []string (interface names like "vmbr0")
	vmbrs := make([]VMBROption, 0, len(settings.VMBRs))
	for _, iface := range settings.VMBRs {
		vmbrs = append(vmbrs, VMBROption{
			Iface:   iface,
			Enabled: true,
		})
	}

	// settings.Tags is []string
	tags := append([]string(nil), settings.Tags...)

	// Parse EnabledStorages ("node:storage" or plain "storage" format) into structured options
	storages := make([]StorageOption, 0, len(settings.EnabledStorages))
	for _, s := range settings.EnabledStorages {
		parts := strings.SplitN(s, ":", 2)
		if len(parts) == 2 {
			storages = append(storages, StorageOption{Storage: parts[1], Node: parts[0], Enabled: true})
		} else {
			storages = append(storages, StorageOption{Storage: s, Enabled: true})
		}
	}

	resp := VMSettingsResponse{
		AvailableISOs:     isos,
		AvailableVMBRs:    vmbrs,
		AvailableTags:     tags,
		AvailableStorages: storages,
		Limits: LimitsInfo{
			MinSockets:    settings.Limits.VM.Sockets.Min,
			MaxSockets:    settings.Limits.VM.Sockets.Max,
			MinCores:      settings.Limits.VM.Cores.Min,
			MaxCores:      settings.Limits.VM.Cores.Max,
			MinRAMGB:      settings.Limits.VM.RAM.Min,
			MaxRAMGB:      settings.Limits.VM.RAM.Max,
			MinDiskGB:     settings.Limits.VM.Disk.Min,
			MaxDiskGB:     settings.Limits.VM.Disk.Max,
			MaxDisksPerVM: settings.MaxDiskPerVM,
			MaxSnapshots:  settings.Limits.MaxSnapshots,
		},
	}
	writeJSON(w, resp)
}
