package apiv1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// AdminMutationsHandler handles admin write operations.
type AdminMutationsHandler struct {
	state state.StateManager
}

// MakeAdminMutationsHandler creates a new AdminMutationsHandler.
func MakeAdminMutationsHandler(s state.StateManager) *AdminMutationsHandler {
	return &AdminMutationsHandler{state: s}
}

// --- User Pool ---

// poolMember is a single entry in a Proxmox pool's members list.
type poolMember struct {
	ID   string `json:"id"`
	Type string `json:"type"`
}

// ListPools handles GET /api/v1/admin/userpool.
// The Proxmox GET /pools list endpoint does NOT return members; we must call
// GET /pools/{poolid} per pool to get accurate member counts.
func (h *AdminMutationsHandler) ListPools(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		writeJSON(w, []AdminPoolResponse{})
		return
	}
	restyClient, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
	if err != nil {
		errInternal(w)
		return
	}

	// Step 1: list all pools (no members here)
	var listResp struct {
		Data []struct {
			PoolID  string `json:"poolid"`
			Comment string `json:"comment"`
		} `json:"data"`
	}
	if err := restyClient.Get(r.Context(), "/pools", &listResp); err != nil {
		errInternal(w)
		return
	}

	// Step 2: for each pvmss-managed pool, fetch detail to get members
	type detailResp struct {
		Data struct {
			PoolID  string       `json:"poolid"`
			Comment string       `json:"comment"`
			Members []poolMember `json:"members"`
		} `json:"data"`
	}

	result := make([]AdminPoolResponse, 0, len(listResp.Data))
	for _, p := range listResp.Data {
		if !strings.HasPrefix(p.PoolID, "pvmss_") {
			continue
		}
		var detail detailResp
		if err := restyClient.Get(r.Context(), "/pools/"+url.PathEscape(p.PoolID), &detail); err != nil {
			logger.Get().Warn().Err(err).Str("pool", p.PoolID).Msg("failed to fetch pool detail")
			// Still include the pool but with zero count
			result = append(result, AdminPoolResponse{
				PoolID:  p.PoolID,
				Comment: p.Comment,
				Members: []string{},
				VMCount: 0,
			})
			continue
		}
		members := make([]string, 0, len(detail.Data.Members))
		vmCount := 0
		for _, m := range detail.Data.Members {
			members = append(members, m.ID)
			t := strings.ToLower(m.Type)
			if t == "qemu" || t == "lxc" {
				vmCount++
			}
		}
		result = append(result, AdminPoolResponse{
			PoolID:  detail.Data.PoolID,
			Comment: detail.Data.Comment,
			Members: members,
			VMCount: vmCount,
		})
	}
	writeJSON(w, result)
}

// CreatePool handles POST /api/v1/admin/userpool.
func (h *AdminMutationsHandler) CreatePool(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		errOffline(w)
		return
	}
	var req CreatePoolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid JSON body")
		return
	}
	if req.Pool == "" || req.Password == "" {
		errBadRequest(w, "pool and password are required")
		return
	}

	restyClient, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
	if err != nil {
		errInternal(w)
		return
	}
	ctx := r.Context()

	// Ensure role exists
	if err := proxmox.EnsureRoleResty(ctx, restyClient, "PVMSSUser", []string{
		"VM.Allocate", "VM.Audit", "VM.Console", "VM.Config.Disk",
		"VM.Config.Network", "VM.Config.CPU", "VM.Config.Memory",
		"VM.Config.Options", "VM.Config.Cloudinit", "VM.Config.CDROM",
		"VM.PowerMgmt", "VM.Snapshot", "VM.Snapshot.Rollback",
		"Datastore.AllocateSpace", "Datastore.Audit",
		"SDN.Use",
	}); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to ensure PVMSSUser role")
		errInternal(w)
		return
	}

	// Derive username from pool name (pool_name@pve)
	username := req.Pool + "@pve"
	if err := proxmox.EnsureUserResty(ctx, restyClient, username, req.Password, "", fmt.Sprintf("PVMSS user for pool %s", req.Pool), "pve", true); err != nil {
		writeError(w, http.StatusInternalServerError, "user_creation_failed", err.Error())
		return
	}

	// Create pool
	poolID := "pvmss_" + req.Pool
	if err := proxmox.EnsurePoolResty(ctx, restyClient, poolID, fmt.Sprintf("PVMSS managed pool for %s", req.Pool)); err != nil {
		writeError(w, http.StatusInternalServerError, "pool_creation_failed", err.Error())
		return
	}

	// Set ACL
	if err := proxmox.EnsurePoolACLResty(ctx, restyClient, username, poolID, "PVMSSUser", true); err != nil {
		writeError(w, http.StatusInternalServerError, "acl_creation_failed", err.Error())
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, map[string]string{"poolid": poolID, "username": username})
}

// DeletePool handles DELETE /api/v1/admin/userpool/:name.
func (h *AdminMutationsHandler) DeletePool(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		errOffline(w)
		return
	}
	ps := r.Context().Value(httprouter.ParamsKey).(httprouter.Params)
	name := ps.ByName("name")
	if name == "" {
		errBadRequest(w, "missing pool name")
		return
	}
	restyClient, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
	if err != nil {
		errInternal(w)
		return
	}
	ctx := r.Context()
	poolID := name
	if !strings.HasPrefix(poolID, "pvmss_") {
		poolID = "pvmss_" + poolID
	}

	// Step 1: get pool members
	var detailResp struct {
		Data struct {
			Members []struct {
				Type string `json:"type"`
				VMID int    `json:"vmid"`
				Node string `json:"node"`
			} `json:"members"`
		} `json:"data"`
	}
	if err := restyClient.Get(ctx, "/pools/"+url.PathEscape(poolID), &detailResp); err != nil {
		writeError(w, http.StatusInternalServerError, "pool_members_failed", err.Error())
		return
	}

	// Step 2: stop all QEMU VMs concurrently
	{
		var wg sync.WaitGroup
		for _, m := range detailResp.Data.Members {
			if m.VMID <= 0 || m.Node == "" || strings.ToLower(m.Type) != "qemu" {
				continue
			}
			m := m
			wg.Add(1)
			go func() {
				defer wg.Done()
				c, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
				if err != nil {
					return
				}
				if _, err := proxmox.VMActionResty(ctx, c, m.Node, strconv.Itoa(m.VMID), "stop"); err != nil {
					logger.Get().Warn().Err(err).Int("vmid", m.VMID).Msg("stop VM before pool delete failed")
				}
			}()
		}
		wg.Wait()
		time.Sleep(3 * time.Second)
	}

	// Step 3: delete all VMs (purge)
	for _, m := range detailResp.Data.Members {
		if m.VMID <= 0 || m.Node == "" {
			continue
		}
		var path string
		switch strings.ToLower(m.Type) {
		case "qemu":
			path = "/nodes/" + url.PathEscape(m.Node) + "/qemu/" + url.PathEscape(strconv.Itoa(m.VMID)) + "?purge=1"
		case "lxc":
			path = "/nodes/" + url.PathEscape(m.Node) + "/lxc/" + url.PathEscape(strconv.Itoa(m.VMID)) + "?purge=1"
		default:
			continue
		}
		if err := restyClient.Delete(ctx, path, nil); err != nil {
			logger.Get().Error().Err(err).Str("path", path).Msg("failed to delete VM during pool purge")
			writeError(w, http.StatusInternalServerError, "vm_delete_failed", err.Error())
			return
		}
	}

	// Step 4: wait until pool is empty (up to 15s)
	deadline := time.Now().Add(15 * time.Second)
	for {
		var check struct {
			Data struct {
				Members []any `json:"members"`
			} `json:"data"`
		}
		if err := restyClient.Get(ctx, "/pools/"+url.PathEscape(poolID), &check); err == nil {
			if len(check.Data.Members) == 0 {
				break
			}
		}
		if time.Now().After(deadline) {
			logger.Get().Warn().Str("pool", poolID).Msg("pool still not empty after deletions; proceeding anyway")
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Step 5: delete pool
	if err := restyClient.Delete(ctx, "/pools/"+url.PathEscape(poolID), nil); err != nil {
		writeError(w, http.StatusInternalServerError, "pool_delete_failed", err.Error())
		return
	}

	// Step 6: delete user (best-effort)
	username := strings.TrimPrefix(poolID, "pvmss_")
	if !strings.Contains(username, "@") {
		username = username + "@pve"
	}
	_ = restyClient.Delete(ctx, fmt.Sprintf("/access/users/%s", username), nil)

	w.WriteHeader(http.StatusNoContent)
}

// --- Tags ---

// ListTags handles GET /api/v1/admin/tags.
func (h *AdminMutationsHandler) ListTags(w http.ResponseWriter, r *http.Request) {
	tags := h.state.GetTags()
	if tags == nil {
		tags = []string{}
	}

	// Count VMs per tag if online
	tagCounts := make(map[string]int, len(tags))
	if !h.state.IsOfflineMode() {
		snap := h.state.GetProxmoxSnapshot()
		if snap != nil {
			for _, vm := range snap.VMs {
				for _, tag := range strings.Split(vm.Tags, ";") {
					tag = strings.TrimSpace(tag)
					if tag != "" {
						tagCounts[tag]++
					}
				}
			}
		}
	}

	result := make([]AdminTagResponse, 0, len(tags))
	for _, tag := range tags {
		result = append(result, AdminTagResponse{
			Name:    tag,
			VMCount: tagCounts[tag],
		})
	}
	writeJSON(w, result)
}

// CreateTag handles POST /api/v1/admin/tags.
func (h *AdminMutationsHandler) CreateTag(w http.ResponseWriter, r *http.Request) {
	var req CreateTagRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid JSON body")
		return
	}
	if req.Name == "" {
		errBadRequest(w, "name is required")
		return
	}

	settings := h.state.GetSettings()
	// Check for duplicate
	for _, t := range settings.Tags {
		if t == req.Name {
			errBadRequest(w, "tag already exists")
			return
		}
	}

	newSettings := *settings
	newTags := make([]string, len(settings.Tags), len(settings.Tags)+1)
	copy(newTags, settings.Tags)
	newTags = append(newTags, req.Name)
	newSettings.Tags = newTags
	// Deep copy maps
	newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
	if err := h.state.SetSettings(&newSettings); err != nil {
		errInternal(w)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, AdminTagResponse{Name: req.Name})
}

// DeleteTag handles DELETE /api/v1/admin/tags/:name.
func (h *AdminMutationsHandler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	ps := r.Context().Value(httprouter.ParamsKey).(httprouter.Params)
	name := ps.ByName("name")
	if name == "" {
		errBadRequest(w, "missing tag name")
		return
	}

	settings := h.state.GetSettings()
	newSettings := *settings
	newTags := make([]string, 0, len(settings.Tags))
	found := false
	for _, t := range settings.Tags {
		if t == name {
			found = true
			continue
		}
		newTags = append(newTags, t)
	}
	if !found {
		errBadRequest(w, "tag not found")
		return
	}
	newSettings.Tags = newTags
	newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
	if err := h.state.SetSettings(&newSettings); err != nil {
		errInternal(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Limits ---

// GetLimits handles GET /api/v1/admin/limits.
func (h *AdminMutationsHandler) GetLimits(w http.ResponseWriter, _ *http.Request) {
	settings := h.state.GetSettings()
	nodes := make(map[string]NodeResourceLimitsResponse, len(settings.Limits.Nodes))
	for k, v := range settings.Limits.Nodes {
		nodes[k] = NodeResourceLimitsResponse{
			Sockets: ResourceRangeResponse{Min: v.Sockets.Min, Max: v.Sockets.Max},
			Cores:   ResourceRangeResponse{Min: v.Cores.Min, Max: v.Cores.Max},
			RAM:     ResourceRangeResponse{Min: v.RAM.Min, Max: v.RAM.Max},
			Disk:    ResourceRangeResponse{Min: v.Disk.Min, Max: v.Disk.Max},
		}
	}
	writeJSON(w, AdminLimitsResponse{
		VM: VMResourceLimitsResponse{
			Sockets: ResourceRangeResponse{Min: settings.Limits.VM.Sockets.Min, Max: settings.Limits.VM.Sockets.Max},
			Cores:   ResourceRangeResponse{Min: settings.Limits.VM.Cores.Min, Max: settings.Limits.VM.Cores.Max},
			RAM:     ResourceRangeResponse{Min: settings.Limits.VM.RAM.Min, Max: settings.Limits.VM.RAM.Max},
			Disk:    ResourceRangeResponse{Min: settings.Limits.VM.Disk.Min, Max: settings.Limits.VM.Disk.Max},
		},
		Nodes:           nodes,
		MaxSnapshots:    settings.Limits.MaxSnapshots,
		MaxNetworkCards: settings.MaxNetworkCards,
		MaxDiskPerVM:    settings.MaxDiskPerVM,
		MaxVMPerUser:    settings.MaxVMPerUser,
	})
}

// UpdateLimits handles PUT /api/v1/admin/limits.
func (h *AdminMutationsHandler) UpdateLimits(w http.ResponseWriter, r *http.Request) {
	var req AdminLimitsResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid JSON body")
		return
	}

	settings := h.state.GetSettings()
	newSettings := *settings
	newSettings.Limits = state.LimitsConfig{
		VM: state.VMResourceLimits{
			Sockets: state.ResourceRange{Min: req.VM.Sockets.Min, Max: req.VM.Sockets.Max},
			Cores:   state.ResourceRange{Min: req.VM.Cores.Min, Max: req.VM.Cores.Max},
			RAM:     state.ResourceRange{Min: req.VM.RAM.Min, Max: req.VM.RAM.Max},
			Disk:    state.ResourceRange{Min: req.VM.Disk.Min, Max: req.VM.Disk.Max},
		},
		Nodes:        make(map[string]state.NodeResourceLimits, len(req.Nodes)),
		MaxSnapshots: req.MaxSnapshots,
	}
	for k, v := range req.Nodes {
		newSettings.Limits.Nodes[k] = state.NodeResourceLimits{
			Sockets: state.ResourceRange{Min: v.Sockets.Min, Max: v.Sockets.Max},
			Cores:   state.ResourceRange{Min: v.Cores.Min, Max: v.Cores.Max},
			RAM:     state.ResourceRange{Min: v.RAM.Min, Max: v.RAM.Max},
			Disk:    state.ResourceRange{Min: v.Disk.Min, Max: v.Disk.Max},
		}
	}
	newSettings.MaxNetworkCards = req.MaxNetworkCards
	newSettings.MaxDiskPerVM = req.MaxDiskPerVM
	newSettings.MaxVMPerUser = req.MaxVMPerUser
	if err := h.state.SetSettings(&newSettings); err != nil {
		errInternal(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Cloud-Init ---

// ListCloudInit handles GET /api/v1/admin/cloudinit.
func (h *AdminMutationsHandler) ListCloudInit(w http.ResponseWriter, _ *http.Request) {
	settings := h.state.GetSettings()
	templates := make([]AdminCloudInitResponse, 0, len(settings.CloudInitTemplates))
	for _, t := range settings.CloudInitTemplates {
		templates = append(templates, AdminCloudInitResponse{
			ID:          t.ID,
			Name:        t.Name,
			Description: t.Description,
			Storage:     t.Storage,
			Filename:    t.Filename,
			Enabled:     t.Enabled,
		})
	}
	writeJSON(w, AdminCloudInitListResponse{
		Templates:  templates,
		SFTPStatus: buildSFTPStatus(settings),
	})
}

// buildSFTPStatus constructs SFTP status from settings without i18n (API returns machine-readable status).
func buildSFTPStatus(settings *state.AppSettings) *AdminSFTPStatusResponse {
	if settings == nil {
		return &AdminSFTPStatusResponse{
			Enabled:    false,
			StatusText: "settings unavailable",
			StatusType: "danger",
		}
	}
	cfg := settings.CloudInitSFTP
	if !cfg.Enabled {
		return &AdminSFTPStatusResponse{
			Enabled:    false,
			StatusText: "disabled",
			StatusType: "warning",
		}
	}
	keyExists := false
	if cfg.PrivateKeyPath != "" {
		if _, err := os.Stat(cfg.PrivateKeyPath); err == nil {
			keyExists = true
		}
	}
	status := &AdminSFTPStatusResponse{
		Enabled:   true,
		Host:      cfg.Host,
		Username:  cfg.Username,
		KeyExists: keyExists,
	}
	if !keyExists {
		status.StatusText = "private key not found"
		status.StatusType = "danger"
	} else if cfg.Host == "" {
		status.StatusText = "host not configured"
		status.StatusType = "danger"
	} else {
		status.StatusText = "configured"
		status.StatusType = "success"
	}
	return status
}

// CreateCloudInit handles POST /api/v1/admin/cloudinit.
func (h *AdminMutationsHandler) CreateCloudInit(w http.ResponseWriter, r *http.Request) {
	var req CreateCloudInitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid JSON body")
		return
	}
	if req.Name == "" {
		errBadRequest(w, "name is required")
		return
	}

	id := strings.ToLower(strings.ReplaceAll(req.Name, " ", "-"))
	filename := "pvmss-" + id + ".yml"

	template := state.CloudInitTemplate{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Storage:     req.Storage,
		Filename:    filename,
		YAMLContent: req.YAMLContent,
		Enabled:     true,
	}

	settings := h.state.GetSettings()
	newSettings := *settings
	newTemplates := make([]state.CloudInitTemplate, len(settings.CloudInitTemplates), len(settings.CloudInitTemplates)+1)
	copy(newTemplates, settings.CloudInitTemplates)
	newTemplates = append(newTemplates, template)
	newSettings.CloudInitTemplates = newTemplates
	newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
	if err := h.state.SetSettings(&newSettings); err != nil {
		errInternal(w)
		return
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, AdminCloudInitResponse{
		ID: id, Name: req.Name, Description: req.Description,
		Storage: req.Storage, Filename: filename, Enabled: true,
	})
}

// UpdateCloudInit handles PUT /api/v1/admin/cloudinit/:id.
func (h *AdminMutationsHandler) UpdateCloudInit(w http.ResponseWriter, r *http.Request) {
	ps := r.Context().Value(httprouter.ParamsKey).(httprouter.Params)
	id := ps.ByName("id")
	if id == "" {
		errBadRequest(w, "missing cloud-init ID")
		return
	}
	var req UpdateCloudInitRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid JSON body")
		return
	}

	settings := h.state.GetSettings()
	newSettings := *settings
	newTemplates := make([]state.CloudInitTemplate, len(settings.CloudInitTemplates))
	copy(newTemplates, settings.CloudInitTemplates)
	found := false
	for i, t := range newTemplates {
		if t.ID == id {
			newTemplates[i].Name = req.Name
			newTemplates[i].Description = req.Description
			newTemplates[i].Storage = req.Storage
			newTemplates[i].YAMLContent = req.YAMLContent
			found = true
			break
		}
	}
	if !found {
		errBadRequest(w, "cloud-init template not found")
		return
	}
	newSettings.CloudInitTemplates = newTemplates
	newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
	if err := h.state.SetSettings(&newSettings); err != nil {
		errInternal(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteCloudInit handles DELETE /api/v1/admin/cloudinit/:id.
func (h *AdminMutationsHandler) DeleteCloudInit(w http.ResponseWriter, r *http.Request) {
	ps := r.Context().Value(httprouter.ParamsKey).(httprouter.Params)
	id := ps.ByName("id")
	if id == "" {
		errBadRequest(w, "missing cloud-init ID")
		return
	}

	settings := h.state.GetSettings()
	newSettings := *settings
	newTemplates := make([]state.CloudInitTemplate, 0, len(settings.CloudInitTemplates))
	found := false
	for _, t := range settings.CloudInitTemplates {
		if t.ID == id {
			found = true
			continue
		}
		newTemplates = append(newTemplates, t)
	}
	if !found {
		errBadRequest(w, "cloud-init template not found")
		return
	}
	newSettings.CloudInitTemplates = newTemplates
	newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
	if err := h.state.SetSettings(&newSettings); err != nil {
		errInternal(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ToggleCloudInit handles POST /api/v1/admin/cloudinit/:id/toggle.
func (h *AdminMutationsHandler) ToggleCloudInit(w http.ResponseWriter, r *http.Request) {
	ps := r.Context().Value(httprouter.ParamsKey).(httprouter.Params)
	id := ps.ByName("id")
	if id == "" {
		errBadRequest(w, "missing cloud-init ID")
		return
	}

	settings := h.state.GetSettings()
	newSettings := *settings
	newTemplates := make([]state.CloudInitTemplate, len(settings.CloudInitTemplates))
	copy(newTemplates, settings.CloudInitTemplates)
	found := false
	for i, t := range newTemplates {
		if t.ID == id {
			newTemplates[i].Enabled = !t.Enabled
			found = true
			break
		}
	}
	if !found {
		errBadRequest(w, "cloud-init template not found")
		return
	}
	newSettings.CloudInitTemplates = newTemplates
	newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
	if err := h.state.SetSettings(&newSettings); err != nil {
		errInternal(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Storage Toggle ---

// ToggleStorage handles POST /api/v1/admin/storage/toggle.
func (h *AdminMutationsHandler) ToggleStorage(w http.ResponseWriter, r *http.Request) {
	var req ToggleStorageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid JSON body")
		return
	}
	if req.Storage == "" || req.Node == "" {
		errBadRequest(w, "storage and node are required")
		return
	}

	uniqueID := req.Node + ":" + req.Storage

	settings := h.state.GetSettings()
	newSettings := *settings
	newStorages := make([]string, len(settings.EnabledStorages))
	copy(newStorages, settings.EnabledStorages)

	// Check if currently enabled
	found := false
	for _, s := range newStorages {
		if s == uniqueID {
			found = true
			break
		}
	}

	if found {
		// Remove (disable)
		filtered := make([]string, 0, len(newStorages))
		for _, s := range newStorages {
			if s != uniqueID {
				filtered = append(filtered, s)
			}
		}
		newStorages = filtered
	} else {
		// Add (enable)
		newStorages = append(newStorages, uniqueID)
	}

	newSettings.EnabledStorages = newStorages
	newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
	if err := h.state.SetSettings(&newSettings); err != nil {
		errInternal(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- VMBR Toggle ---

// ToggleVMBR handles POST /api/v1/admin/vmbr/toggle.
func (h *AdminMutationsHandler) ToggleVMBR(w http.ResponseWriter, r *http.Request) {
	var req ToggleVMBRRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid JSON body")
		return
	}
	if req.VMBR == "" || req.Node == "" {
		errBadRequest(w, "vmbr and node are required")
		return
	}

	uniqueID := req.Node + ":" + req.VMBR

	settings := h.state.GetSettings()
	newSettings := *settings
	newVMBRs := make([]string, len(settings.VMBRs))
	copy(newVMBRs, settings.VMBRs)

	found := false
	for _, v := range newVMBRs {
		if v == uniqueID {
			found = true
			break
		}
	}

	if found {
		filtered := make([]string, 0, len(newVMBRs))
		for _, v := range newVMBRs {
			if v != uniqueID {
				filtered = append(filtered, v)
			}
		}
		newVMBRs = filtered
	} else {
		newVMBRs = append(newVMBRs, uniqueID)
	}

	newSettings.VMBRs = newVMBRs
	newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
	if err := h.state.SetSettings(&newSettings); err != nil {
		errInternal(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- ISO Toggle ---

// ToggleISO handles POST /api/v1/admin/iso/toggle.
func (h *AdminMutationsHandler) ToggleISO(w http.ResponseWriter, r *http.Request) {
	var req ToggleISORequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid JSON body")
		return
	}
	if req.VolID == "" {
		errBadRequest(w, "volid is required")
		return
	}

	settings := h.state.GetSettings()
	newSettings := *settings
	newISOs := make([]string, len(settings.ISOs))
	copy(newISOs, settings.ISOs)

	found := false
	for _, iso := range newISOs {
		if iso == req.VolID {
			found = true
			break
		}
	}

	if found {
		filtered := make([]string, 0, len(newISOs))
		for _, iso := range newISOs {
			if iso != req.VolID {
				filtered = append(filtered, iso)
			}
		}
		newISOs = filtered
	} else {
		newISOs = append(newISOs, req.VolID)
	}

	newSettings.ISOs = newISOs
	newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
	if err := h.state.SetSettings(&newSettings); err != nil {
		errInternal(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// copyNodeLimits deep-copies the node limits map to avoid shared references.
func copyNodeLimits(src map[string]state.NodeResourceLimits) map[string]state.NodeResourceLimits {
	if src == nil {
		return make(map[string]state.NodeResourceLimits)
	}
	dst := make(map[string]state.NodeResourceLimits, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
