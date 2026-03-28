package apiv1

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/logger"
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
	VMID        int                       `json:"vmid"`
	Name        string                    `json:"name"`
	Node        string                    `json:"node"`
	Status      string                    `json:"status"`
	CPU         float64                   `json:"cpu"`
	CPUs        int                       `json:"cpus"`
	MemMB       int64                     `json:"mem_mb"`
	MaxMemMB    int64                     `json:"max_mem_mb"`
	DiskMB      int64                     `json:"disk_mb"`
	Uptime      int64                     `json:"uptime"`
	Tags        string                    `json:"tags"`
	Description string                    `json:"description"`
	Networks    []proxmox.NetworkInterface `json:"networks"`
	Disks       []DiskInfo                `json:"disks"`
	HasCDROM    bool                      `json:"has_cdrom"`
	CurrentISO  string                    `json:"current_iso"`
	EFIEnabled  bool                      `json:"efi_enabled"`
	TPMEnabled  bool                      `json:"tpm_enabled"`
	CloudInit   *CloudInitInfo            `json:"cloud_init,omitempty"`
}

type DiskInfo struct {
	Index   string `json:"index"`
	Bus     string `json:"bus"`
	Storage string `json:"storage"`
	SizeGB  int    `json:"size_gb"`
	Raw     string `json:"raw"`
}

type CloudInitInfo struct {
	User       string `json:"user,omitempty"`
	SSHKeys    string `json:"ssh_keys,omitempty"`
	IPConfig   string `json:"ip_config,omitempty"`
	Nameserver string `json:"nameserver,omitempty"`
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
	AvailableISOs    []ISOOption    `json:"available_isos"`
	AvailableVMBRs   []VMBROption   `json:"available_vmbrs"`
	AvailableTags    []string       `json:"available_tags"`
	Limits           LimitsInfo     `json:"limits"`
	MaxSnapshots     int            `json:"max_snapshots"`
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

type LimitsInfo struct {
	MinSockets int `json:"min_sockets"`
	MaxSockets int `json:"max_sockets"`
	MinCores   int `json:"min_cores"`
	MaxCores   int `json:"max_cores"`
	MinRAMGB   int `json:"min_ram_gb"`
	MaxRAMGB   int `json:"max_ram_gb"`
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
	pool := fetchPoolVMIDs(ctx, client, "pvmss_"+username)
	return pool[vmid]
}

// resolveNode finds the node a VM lives on using the list already in hand, or by scanning.
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

func parseDisks(cfg map[string]interface{}) ([]DiskInfo, bool, string, int64) {
	buses := []string{"virtio", "scsi", "sata", "ide"}
	var disks []DiskInfo
	var hasCDROM bool
	var currentISO string
	var totalMB int64

	for _, bus := range buses {
		for i := 0; i < 16; i++ {
			key := bus + strconv.Itoa(i)
			val, ok := cfg[key].(string)
			if !ok || val == "" {
				continue
			}
			// CD-ROM detection
			if strings.Contains(val, "media=cdrom") || bus == "ide" && strings.Contains(val, "iso") {
				hasCDROM = true
				parts := strings.SplitN(val, ",", 2)
				if parts[0] != "none" && !strings.HasPrefix(parts[0], "local") {
					currentISO = parts[0]
				} else if strings.Contains(parts[0], ".iso") {
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
					}
				}
			}
			storage := ""
			parts := strings.SplitN(val, ":", 2)
			if len(parts) >= 1 {
				storage = parts[0]
			}
			disks = append(disks, DiskInfo{
				Index:   key,
				Bus:     bus,
				Storage: storage,
				SizeGB:  sizeGB,
				Raw:     val,
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
		decoded, _ := strconv.Unquote(`"` + strings.ReplaceAll(v, `%0A`, "\n") + `"`)
		if decoded == "" {
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
	if !has {
		return nil
	}
	return ci
}

// ── Handlers ──────────────────────────────────────────────────────────────────

// GetVMConfig handles GET /api/v1/vms/:id/config
// Returns full VM configuration including disks, networks, cloud-init.
func (h *VMDetailsHandler) GetVMConfig(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	vmid, err := strconv.Atoi(ps.ByName("id"))
	if err != nil || vmid <= 0 {
		errBadRequest(w, "invalid vm id")
		return
	}
	if h.isOffline() {
		errOffline(w)
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	client, err := restyClient()
	if err != nil {
		errInternal(w)
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

	// Get the VM summary (for fields not in VMCurrent: MaxDisk, Uptime, Tags)
	var vmSummary *proxmox.VM
	{
		allVMs, err2 := proxmox.GetVMsResty(ctx, client)
		if err2 == nil {
			for i := range allVMs {
				if allVMs[i].VMID == vmid {
					vmSummary = &allVMs[i]
					break
				}
			}
		}
	}

	// Get runtime status
	current, err := proxmox.GetVMCurrentResty(ctx, client, node, vmid)
	if err != nil {
		errInternal(w)
		return
	}

	// Get full config
	cfg, err := proxmox.GetVMConfigResty(ctx, client, node, vmid)
	if err != nil {
		errInternal(w)
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

	const mb = int64(1024 * 1024)
	var diskMB, uptime int64
	var tags string
	if vmSummary != nil {
		diskMB = vmSummary.MaxDisk / mb
		uptime = vmSummary.Uptime
		tags = vmSummary.Tags
	}
	resp := VMConfigResponse{
		VMID:        vmid,
		Name:        current.Name,
		Node:        node,
		Status:      current.Status,
		CPU:         current.CPU,
		CPUs:        current.CPUs,
		MemMB:       current.Mem / mb,
		MaxMemMB:    current.MaxMem / mb,
		DiskMB:      diskMB,
		Uptime:      uptime,
		Tags:        tags,
		Description: description,
		Networks:    networks,
		Disks:       disks,
		HasCDROM:    hasCDROM,
		CurrentISO:  currentISO,
		EFIEnabled:  efiEnabled,
		TPMEnabled:  tpmEnabled,
		CloudInit:   cloudInit,
	}

	writeJSON(w, resp)
}

// GetVMMetrics handles GET /api/v1/vms/:id/metrics — lightweight live poll.
func (h *VMDetailsHandler) GetVMMetrics(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	vmid, err := strconv.Atoi(ps.ByName("id"))
	if err != nil || vmid <= 0 {
		errBadRequest(w, "invalid vm id")
		return
	}
	if h.isOffline() {
		errOffline(w)
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	client, err := restyClient()
	if err != nil {
		errInternal(w)
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
		errInternal(w)
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

// GetVMSnapshots handles GET /api/v1/vms/:id/snapshots
func (h *VMDetailsHandler) GetVMSnapshots(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	vmid, err := strconv.Atoi(ps.ByName("id"))
	if err != nil || vmid <= 0 {
		errBadRequest(w, "invalid vm id")
		return
	}
	if h.isOffline() {
		errOffline(w)
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	client, err := restyClient()
	if err != nil {
		errInternal(w)
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

	vmidStr := strconv.Itoa(vmid)
	snaps, err := proxmox.GetVMSnapshotsResty(ctx, client, node, vmidStr)
	if err != nil {
		errInternal(w)
		return
	}

	maxSnapshots := 5
	if settings, _, err := state.LoadSettings(); err == nil {
		maxSnapshots = settings.Limits.MaxSnapshots
	}

	result := make([]SnapshotResponse, 0, len(snaps))
	for _, s := range snaps {
		result = append(result, SnapshotResponse{
			Name:        s.Name,
			Description: s.Description,
			Snaptime:    s.Snaptime,
			Vmstate:     s.Vmstate,
			Parent:      s.Parent,
			Current:     s.Name == "current",
		})
	}

	writeJSON(w, SnapshotListResponse{Snapshots: result, MaxAllowed: maxSnapshots})
}

// CreateSnapshot handles POST /api/v1/vms/:id/snapshots
func (h *VMDetailsHandler) CreateSnapshot(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	vmid, err := strconv.Atoi(ps.ByName("id"))
	if err != nil || vmid <= 0 {
		errBadRequest(w, "invalid vm id")
		return
	}

	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Vmstate     bool   `json:"vmstate"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid JSON")
		return
	}
	if !proxmox.IsValidSnapshotName(req.Name) {
		errBadRequest(w, "invalid snapshot name: use only letters, numbers, hyphens and underscores (max 40 chars)")
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	client, err := restyClient()
	if err != nil {
		errInternal(w)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
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

	vmidStr := strconv.Itoa(vmid)

	// Enforce snapshot limit
	snaps, err := proxmox.GetVMSnapshotsResty(ctx, client, node, vmidStr)
	if err != nil {
		errInternal(w)
		return
	}
	maxSnapshots := 5
	if settings, _, sErr := state.LoadSettings(); sErr == nil {
		maxSnapshots = settings.Limits.MaxSnapshots
	}
	actual := 0
	for _, s := range snaps {
		if s.Name != "current" {
			actual++
		}
	}
	if actual >= maxSnapshots {
		writeError(w, http.StatusConflict, "limit_reached", "maximum snapshot limit reached")
		return
	}

	if err := proxmox.CreateVMSnapshotResty(ctx, client, node, vmidStr, proxmox.VMSnapshotConfig{
		Name:        req.Name,
		Description: req.Description,
		Vmstate:     req.Vmstate,
	}); err != nil {
		logger.Get().Error().Err(err).Int("vmid", vmid).Msg("api/v1: CreateSnapshot failed")
		errInternal(w)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]string{"status": "ok"})
}

// DeleteSnapshot handles DELETE /api/v1/vms/:id/snapshots/:name
func (h *VMDetailsHandler) DeleteSnapshot(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	vmid, err := strconv.Atoi(ps.ByName("id"))
	if err != nil || vmid <= 0 {
		errBadRequest(w, "invalid vm id")
		return
	}
	snapName := ps.ByName("name")
	if snapName == "" || snapName == "current" {
		errBadRequest(w, "invalid snapshot name")
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	client, err := restyClient()
	if err != nil {
		errInternal(w)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
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

	if err := proxmox.DeleteVMSnapshotResty(ctx, client, node, strconv.Itoa(vmid), snapName); err != nil {
		errInternal(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// RollbackSnapshot handles POST /api/v1/vms/:id/snapshots/:name/rollback
func (h *VMDetailsHandler) RollbackSnapshot(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	vmid, err := strconv.Atoi(ps.ByName("id"))
	if err != nil || vmid <= 0 {
		errBadRequest(w, "invalid vm id")
		return
	}
	snapName := ps.ByName("name")
	if snapName == "" || snapName == "current" {
		errBadRequest(w, "invalid snapshot name")
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	client, err := restyClient()
	if err != nil {
		errInternal(w)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
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

	if err := proxmox.RollbackVMSnapshotResty(ctx, client, node, strconv.Itoa(vmid), snapName); err != nil {
		errInternal(w)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

// UpdateVMConfig handles PATCH /api/v1/vms/:id/config
// Supports updating description and tags.
func (h *VMDetailsHandler) UpdateVMConfig(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	vmid, err := strconv.Atoi(ps.ByName("id"))
	if err != nil || vmid <= 0 {
		errBadRequest(w, "invalid vm id")
		return
	}

	var req struct {
		Description *string `json:"description"`
		Tags        *string `json:"tags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid JSON")
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	client, err := restyClient()
	if err != nil {
		errInternal(w)
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
	if len(params) == 0 {
		errBadRequest(w, "nothing to update")
		return
	}

	if err := proxmox.UpdateVMConfigResty(ctx, client, node, vmid, params); err != nil {
		errInternal(w)
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

// GetVMSettings handles GET /api/v1/vms/:id/settings
// Returns allowed ISOs, VМBRs, tags and resource limits for the VM edit form.
func (h *VMDetailsHandler) GetVMSettings(w http.ResponseWriter, r *http.Request) {
	settings, _, err := state.LoadSettings()
	if err != nil {
		errInternal(w)
		return
	}

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
	tags := make([]string, 0, len(settings.Tags))
	for _, t := range settings.Tags {
		tags = append(tags, t)
	}

	resp := VMSettingsResponse{
		AvailableISOs:  isos,
		AvailableVMBRs: vmbrs,
		AvailableTags:  tags,
		MaxSnapshots:   settings.Limits.MaxSnapshots,
		Limits: LimitsInfo{
			MinSockets: settings.Limits.VM.Sockets.Min,
			MaxSockets: settings.Limits.VM.Sockets.Max,
			MinCores:   settings.Limits.VM.Cores.Min,
			MaxCores:   settings.Limits.VM.Cores.Max,
			MinRAMGB:   settings.Limits.VM.RAM.Min,
			MaxRAMGB:   settings.Limits.VM.RAM.Max,
		},
	}
	writeJSON(w, resp)
}

// snapshotNameRe validates snapshot names.
var snapshotNameRe = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,40}$`)
