package apiv1

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/cloudinit"
	"pvmss/database"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

var tagNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,50}$`)

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
		writeAppError(w, err)
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
		writeAppError(w, err)
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
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Pool == "" || req.Password == "" {
		errBadRequest(w, "pool and password are required")
		return
	}

	restyClient, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
	if err != nil {
		writeAppError(w, err)
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
		writeAppError(w, err)
		return
	}

	// Derive username from pool name (pool_name@pve)
	username := req.Pool + "@pve"
	if err := proxmox.EnsureUserResty(ctx, restyClient, username, req.Password, "", fmt.Sprintf("PVMSS user for pool %s", req.Pool), "pve", true); err != nil {
		logger.Get().Error().Err(err).Str("username", username).Str("pool", req.Pool).Msg("api/v1: failed to ensure user")
		writeError(w, http.StatusInternalServerError, "user_creation_failed", "Failed to create user")
		return
	}

	// Create pool
	poolID := "pvmss_" + req.Pool
	if err := proxmox.EnsurePoolResty(ctx, restyClient, poolID, fmt.Sprintf("PVMSS managed pool for %s", req.Pool)); err != nil {
		logger.Get().Error().Err(err).Str("poolid", poolID).Msg("api/v1: failed to ensure pool")
		writeError(w, http.StatusInternalServerError, "pool_creation_failed", "Failed to create pool")
		return
	}

	// Set ACL
	if err := proxmox.EnsurePoolACLResty(ctx, restyClient, username, poolID, "PVMSSUser", true); err != nil {
		logger.Get().Error().Err(err).Str("username", username).Str("poolid", poolID).Msg("api/v1: failed to ensure pool ACL")
		writeError(w, http.StatusInternalServerError, "acl_creation_failed", "Failed to set ACL")
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
	ps := httprouter.ParamsFromContext(r.Context())
	name := ps.ByName("name")
	if name == "" {
		errBadRequest(w, "missing pool name")
		return
	}
	restyClient, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
	if err != nil {
		writeAppError(w, err)
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
		logger.Get().Error().Err(err).Str("poolid", poolID).Msg("api/v1: failed to get pool members")
		writeError(w, http.StatusInternalServerError, "pool_members_failed", "Failed to get pool members")
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
			logger.Get().Error().Err(err).Str("path", path).Msg("api/v1: failed to delete VM during pool purge")
			writeError(w, http.StatusInternalServerError, "vm_delete_failed", "Failed to delete VM")
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
		logger.Get().Error().Err(err).Str("poolid", poolID).Msg("api/v1: failed to delete pool")
		writeError(w, http.StatusInternalServerError, "pool_delete_failed", "Failed to delete pool")
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
	tagColors := map[string]proxmox.TagColor{}
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
		if restyClient, err := proxmox.MakeRestyClientFromEnv(5 * time.Second); err == nil {
			if colors, cErr := proxmox.GetTagColorsResty(r.Context(), restyClient); cErr == nil {
				tagColors = colors
			}
		}
	}

	result := make([]AdminTagResponse, 0, len(tags))
	for _, tag := range tags {
		entry := AdminTagResponse{
			Name:    tag,
			VMCount: tagCounts[tag],
		}
		if color, ok := tagColors[tag]; ok {
			entry.Color = color.Background
			entry.TextColor = color.Text
			entry.FromProxmox = true
		}
		result = append(result, entry)
	}
	writeJSON(w, result)
}

// CreateTag handles POST /api/v1/admin/tags.
func (h *AdminMutationsHandler) CreateTag(w http.ResponseWriter, r *http.Request) {
	var req CreateTagRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Name == "" {
		errBadRequest(w, "name is required")
		return
	}
	if !tagNameRegex.MatchString(req.Name) {
		errBadRequest(w, "invalid tag name: use only letters, digits, hyphens, underscores (max 50 chars)")
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

	newTags := make([]string, len(settings.Tags), len(settings.Tags)+1)
	copy(newTags, settings.Tags)
	newTags = append(newTags, req.Name)
	if h.state.HasDB() {
		if err := h.state.SetTags(newTags, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newSettings.Tags = newTags
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, AdminTagResponse{Name: req.Name})
}

// hexColorRegex validates 6-digit hex colors (with optional leading #).
var hexColorRegex = regexp.MustCompile(`^#?[0-9a-fA-F]{6}$`)

// SetTagColor handles PUT /api/v1/admin/tags/:name/color.
// Updates the cluster-wide `tag-style` color-map in Proxmox datacenter options.
// An empty color in the body removes the entry.
func (h *AdminMutationsHandler) SetTagColor(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		errBadRequest(w, "tag colors require an online Proxmox connection")
		return
	}
	ps := httprouter.ParamsFromContext(r.Context())
	name := ps.ByName("name")
	if name == "" {
		errBadRequest(w, "missing tag name")
		return
	}
	// Ensure the tag is actually known to PVMSS to avoid polluting tag-style
	// with foreign names.
	settings := h.state.GetSettings()
	known := false
	for _, t := range settings.Tags {
		if t == name {
			known = true
			break
		}
	}
	if !known {
		errBadRequest(w, "tag not found")
		return
	}

	var req SetTagColorRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Color != "" && !hexColorRegex.MatchString(req.Color) {
		errBadRequest(w, "color must be a 6-digit hex value")
		return
	}
	if req.TextColor != "" && !hexColorRegex.MatchString(req.TextColor) {
		errBadRequest(w, "text_color must be a 6-digit hex value")
		return
	}
	// Text color without background is meaningless.
	if req.Color == "" && req.TextColor != "" {
		errBadRequest(w, "text_color requires color")
		return
	}

	restyClient, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}
	if err := proxmox.SetTagColorResty(r.Context(), restyClient, name, req.Color, req.TextColor); err != nil {
		writeAppError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteTag handles DELETE /api/v1/admin/tags/:name.
func (h *AdminMutationsHandler) DeleteTag(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	name := ps.ByName("name")
	if name == "" {
		errBadRequest(w, "missing tag name")
		return
	}
	if strings.EqualFold(name, "pvmss") {
		errBadRequest(w, "cannot delete the default 'pvmss' tag")
		return
	}

	settings := h.state.GetSettings()
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
	if h.state.HasDB() {
		if err := h.state.SetTags(newTags, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newSettings.Tags = newTags
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
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
			MaxVMs:  v.MaxVMs,
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
	if !decodeBody(w, r, &req) {
		return
	}

	changedBy := usernameFromCtx(r)

	if h.state.HasDB() {
		// Persist VM limits (MaxVMs, MaxVMPerUser, MaxNetworkCards, MaxDiskPerVM,
		// AllowCustomYAML, MaxSnapshots) via the fine-grained DB setter.
		current := h.state.GetSettings()
		limits := &database.VMLimits{
			MaxVMs:          0, // no global VM cap
			MaxVMPerUser:    req.MaxVMPerUser,
			MaxNetworkCards: req.MaxNetworkCards,
			MaxDiskPerVM:    req.MaxDiskPerVM,
			AllowCustomYAML: current.AllowCustomYAML,
			MaxSnapshots:    req.MaxSnapshots,
		}
		if err := h.state.SetVMLimits(limits, changedBy); err != nil {
			writeAppError(w, err)
			return
		}
		// Upsert per-node limits from the request's Nodes map.
		// Preserve capacity limits (max_vcpus, max_ram_gb, max_disk_gb) already
		// stored in the DB; only update max_vms from the Limits admin page.
		for nodeName, nodeReq := range req.Nodes {
			if nodeReq.MaxVMs > 0 {
				// Read existing limits directly from DB to avoid cache staleness.
				existing, found, err := h.state.GetNodeLimitFromDB(nodeName)
				if err != nil {
					writeAppError(w, err)
					return
				}
				if !found {
					existing = database.NodeLimit{NodeName: nodeName}
				}
				if err := h.state.SetNodeLimit(database.NodeLimit{
					NodeName:  nodeName,
					MaxVMs:    nodeReq.MaxVMs,
					MaxVCPUs:  existing.MaxVCPUs,
					MaxRAMGB:  existing.MaxRAMGB,
					MaxDiskGB: existing.MaxDiskGB,
				}, changedBy); err != nil {
					writeAppError(w, err)
					return
				}
			} else {
				// Zero means remove the VM count limit, but preserve capacity limits.
				// Read existing to preserve capacity limits that may have been set via Settings Overview.
				existing, found, err := h.state.GetNodeLimitFromDB(nodeName)
				if err != nil {
					writeAppError(w, err)
					return
				}
				if found && (existing.MaxVCPUs > 0 || existing.MaxRAMGB > 0 || existing.MaxDiskGB > 0) {
					// Preserve capacity limits by updating with zero MaxVMs instead of deleting.
					if err := h.state.SetNodeLimit(database.NodeLimit{
						NodeName:  nodeName,
						MaxVMs:    0,
						MaxVCPUs:  existing.MaxVCPUs,
						MaxRAMGB:  existing.MaxRAMGB,
						MaxDiskGB: existing.MaxDiskGB,
					}, changedBy); err != nil {
						writeAppError(w, err)
						return
					}
				} else {
					// No capacity limits exist, safe to delete the row entirely.
					_ = h.state.DeleteNodeLimit(nodeName, changedBy)
				}
			}
		}
	} else {
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
				MaxVMs:  v.MaxVMs,
			}
		}
		newSettings.MaxNetworkCards = req.MaxNetworkCards
		newSettings.MaxDiskPerVM = req.MaxDiskPerVM
		newSettings.MaxVMPerUser = req.MaxVMPerUser
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
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
			YAMLContent: t.YAMLContent,
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
			StatusText: "settings-unavailable",
			StatusType: "danger",
		}
	}
	cfg := settings.CloudInitSFTP
	isConfigured := cfg.Host != "" && cfg.Username != "" && cfg.PrivateKeyPath != ""

	keyExists := false
	if cfg.PrivateKeyPath != "" {
		if _, err := os.Stat(cfg.PrivateKeyPath); err == nil {
			keyExists = true
		}
	}

	if !cfg.Enabled {
		return &AdminSFTPStatusResponse{
			Enabled:      false,
			Host:         cfg.Host,
			Username:     cfg.Username,
			KeyExists:    keyExists,
			IsConfigured: isConfigured,
			StatusText:   "disabled",
			StatusType:   "warning",
		}
	}
	status := &AdminSFTPStatusResponse{
		Enabled:      true,
		Host:         cfg.Host,
		Username:     cfg.Username,
		KeyExists:    keyExists,
		IsConfigured: isConfigured,
	}
	if !keyExists {
		status.StatusText = "private-key-not-found"
		status.StatusType = "danger"
	} else if cfg.Host == "" {
		status.StatusText = "host-not-configured"
		status.StatusType = "danger"
	} else {
		status.StatusText = "configured"
		status.StatusType = "success"
	}
	return status
}

// ToggleSFTP handles POST /api/v1/admin/cloudinit/sftp/toggle.
func (h *AdminMutationsHandler) ToggleSFTP(w http.ResponseWriter, r *http.Request) {
	settings := h.state.GetSettings()
	cfg := settings.CloudInitSFTP
	newEnabled := !cfg.Enabled

	if h.state.HasDB() {
		dbCfg := &database.SFTPConfig{
			Enabled:        newEnabled,
			Host:           cfg.Host,
			Port:           cfg.Port,
			Username:       cfg.Username,
			PrivateKeyPath: cfg.PrivateKeyPath,
			RemotePath:     cfg.SnippetBaseDir,
		}
		if err := h.state.SetSFTPConfig(dbCfg, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newSettings.CloudInitSFTP = cfg
		newSettings.CloudInitSFTP.Enabled = newEnabled
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// generateCloudInitID generates a safe ID from a template name (lowercase, alphanumeric, hyphens).
func generateCloudInitID(name string) string {
	id := strings.ToLower(name)
	id = strings.ReplaceAll(id, " ", "-")
	id = strings.ReplaceAll(id, "_", "-")
	safeRe := regexp.MustCompile(`[^a-z0-9-]`)
	id = safeRe.ReplaceAllString(id, "")
	for strings.Contains(id, "--") {
		id = strings.ReplaceAll(id, "--", "-")
	}
	id = strings.Trim(id, "-")
	if len(id) < 2 {
		id = "template-" + id
	}
	return id
}

// CreateCloudInit handles POST /api/v1/admin/cloudinit.
func (h *AdminMutationsHandler) CreateCloudInit(w http.ResponseWriter, r *http.Request) {
	var req CreateCloudInitRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Name == "" || req.YAMLContent == "" {
		errBadRequest(w, "name and yaml_content are required")
		return
	}
	if err := cloudinit.ValidateCloudInitYAMLStrict(req.YAMLContent); err != nil {
		errBadRequest(w, "invalid YAML: "+err.Error())
		return
	}

	id := generateCloudInitID(req.Name)
	filename := state.CloudInitTemplatePrefix + id + ".yml"

	settings := h.state.GetSettings()
	for _, t := range settings.CloudInitTemplates {
		if t.ID == id {
			errBadRequest(w, "a template with this name already exists")
			return
		}
	}

	template := state.CloudInitTemplate{
		ID:          id,
		Name:        req.Name,
		Description: req.Description,
		Storage:     req.Storage,
		Filename:    filename,
		YAMLContent: req.YAMLContent,
		Enabled:     true,
	}

	if h.state.HasDB() {
		dbTemplate := &database.CloudInitTemplate{
			ID: template.ID, Name: template.Name, Description: template.Description,
			Storage: template.Storage, Filename: template.Filename,
			YAMLContent: template.YAMLContent, Enabled: template.Enabled,
		}
		if err := h.state.CreateCloudInitTemplate(dbTemplate, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newTemplates := make([]state.CloudInitTemplate, len(settings.CloudInitTemplates), len(settings.CloudInitTemplates)+1)
		copy(newTemplates, settings.CloudInitTemplates)
		newTemplates = append(newTemplates, template)
		newSettings.CloudInitTemplates = newTemplates
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, AdminCloudInitResponse{
		ID: id, Name: req.Name, Description: req.Description,
		Storage: req.Storage, Filename: filename, YAMLContent: req.YAMLContent, Enabled: true,
	})
}

// UpdateCloudInit handles PUT /api/v1/admin/cloudinit/:id.
func (h *AdminMutationsHandler) UpdateCloudInit(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	id := ps.ByName("id")
	if id == "" {
		errBadRequest(w, "missing cloud-init ID")
		return
	}
	var req UpdateCloudInitRequest
	if !decodeBody(w, r, &req) {
		return
	}

	if req.YAMLContent != "" {
		if err := cloudinit.ValidateCloudInitYAMLStrict(req.YAMLContent); err != nil {
			errBadRequest(w, "invalid YAML: "+err.Error())
			return
		}
	}

	settings := h.state.GetSettings()
	found := false
	var existing state.CloudInitTemplate
	for _, t := range settings.CloudInitTemplates {
		if t.ID == id {
			existing = t
			found = true
			break
		}
	}
	if !found {
		errBadRequest(w, "cloud-init template not found")
		return
	}
	if req.Name != "" {
		existing.Name = req.Name
	}
	existing.Description = req.Description
	existing.Storage = req.Storage
	if req.YAMLContent != "" {
		existing.YAMLContent = req.YAMLContent
	}
	if h.state.HasDB() {
		dbTemplate := &database.CloudInitTemplate{
			ID: existing.ID, Name: existing.Name, Description: existing.Description,
			Storage: existing.Storage, Filename: existing.Filename,
			YAMLContent: existing.YAMLContent, Enabled: existing.Enabled,
		}
		if err := h.state.UpdateCloudInitTemplate(dbTemplate, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newTemplates := make([]state.CloudInitTemplate, len(settings.CloudInitTemplates))
		copy(newTemplates, settings.CloudInitTemplates)
		for i, t := range newTemplates {
			if t.ID == id {
				newTemplates[i] = existing
				break
			}
		}
		newSettings.CloudInitTemplates = newTemplates
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// DeleteCloudInit handles DELETE /api/v1/admin/cloudinit/:id.
func (h *AdminMutationsHandler) DeleteCloudInit(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	id := ps.ByName("id")
	if id == "" {
		errBadRequest(w, "missing cloud-init ID")
		return
	}

	settings := h.state.GetSettings()
	found := false
	for _, t := range settings.CloudInitTemplates {
		if t.ID == id {
			found = true
			break
		}
	}
	if !found {
		errBadRequest(w, "cloud-init template not found")
		return
	}
	if h.state.HasDB() {
		if err := h.state.DeleteCloudInitTemplate(id, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newTemplates := make([]state.CloudInitTemplate, 0, len(settings.CloudInitTemplates))
		for _, t := range settings.CloudInitTemplates {
			if t.ID == id {
				continue
			}
			newTemplates = append(newTemplates, t)
		}
		newSettings.CloudInitTemplates = newTemplates
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// ToggleCloudInit handles POST /api/v1/admin/cloudinit/:id/toggle.
func (h *AdminMutationsHandler) ToggleCloudInit(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	id := ps.ByName("id")
	if id == "" {
		errBadRequest(w, "missing cloud-init ID")
		return
	}

	settings := h.state.GetSettings()
	found := false
	var toggled state.CloudInitTemplate
	for _, t := range settings.CloudInitTemplates {
		if t.ID == id {
			t.Enabled = !t.Enabled
			toggled = t
			found = true
			break
		}
	}
	if !found {
		errBadRequest(w, "cloud-init template not found")
		return
	}
	if h.state.HasDB() {
		dbTemplate := &database.CloudInitTemplate{
			ID: toggled.ID, Name: toggled.Name, Description: toggled.Description,
			Storage: toggled.Storage, Filename: toggled.Filename,
			YAMLContent: toggled.YAMLContent, Enabled: toggled.Enabled,
		}
		if err := h.state.UpdateCloudInitTemplate(dbTemplate, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newTemplates := make([]state.CloudInitTemplate, len(settings.CloudInitTemplates))
		copy(newTemplates, settings.CloudInitTemplates)
		for i, t := range newTemplates {
			if t.ID == id {
				newTemplates[i].Enabled = toggled.Enabled
				break
			}
		}
		newSettings.CloudInitTemplates = newTemplates
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- Storage Toggle ---

// ToggleStorage handles POST /api/v1/admin/storage/toggle.
func (h *AdminMutationsHandler) ToggleStorage(w http.ResponseWriter, r *http.Request) {
	var req ToggleStorageRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.Storage == "" || req.Node == "" {
		errBadRequest(w, "storage and node are required")
		return
	}

	uniqueID := req.Node + ":" + req.Storage

	settings := h.state.GetSettings()
	newStorages := make([]string, len(settings.EnabledStorages))
	copy(newStorages, settings.EnabledStorages)

	found := false
	for _, s := range newStorages {
		if s == uniqueID {
			found = true
			break
		}
	}

	if found {
		filtered := make([]string, 0, len(newStorages))
		for _, s := range newStorages {
			if s != uniqueID {
				filtered = append(filtered, s)
			}
		}
		newStorages = filtered
	} else {
		newStorages = append(newStorages, uniqueID)
	}

	if h.state.HasDB() {
		if err := h.state.SetEnabledStorages(newStorages, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newSettings.EnabledStorages = newStorages
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- VMBR Toggle ---

// ToggleVMBR handles POST /api/v1/admin/vmbr/toggle.
func (h *AdminMutationsHandler) ToggleVMBR(w http.ResponseWriter, r *http.Request) {
	var req ToggleVMBRRequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.VMBR == "" || req.Node == "" {
		errBadRequest(w, "vmbr and node are required")
		return
	}

	uniqueID := req.Node + ":" + req.VMBR

	settings := h.state.GetSettings()
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

	if h.state.HasDB() {
		if err := h.state.SetEnabledVMBRs(newVMBRs, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newSettings.VMBRs = newVMBRs
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- ISO Toggle ---

// ToggleISO handles POST /api/v1/admin/iso/toggle.
func (h *AdminMutationsHandler) ToggleISO(w http.ResponseWriter, r *http.Request) {
	var req ToggleISORequest
	if !decodeBody(w, r, &req) {
		return
	}
	if req.VolID == "" {
		errBadRequest(w, "volid is required")
		return
	}

	settings := h.state.GetSettings()
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

	if h.state.HasDB() {
		if err := h.state.SetEnabledISOs(newISOs, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		newSettings.ISOs = newISOs
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// --- VM Profiles ---

var profileIDRegex = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,49}$`)
var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

var validDiskBuses = map[string]bool{"virtio": true, "scsi": true, "sata": true, "ide": true}
var validProfileIcons = map[string]bool{"Globe": true, "Code": true, "Cube": true, "Database": true, "Flask": true, "Monitor": true, "Cpu": true, "HardDrive": true, "Cloud": true, "Info": true}
var validProfileColors = map[string]bool{"blue": true, "violet": true, "emerald": true, "teal": true, "amber": true, "rose": true, "indigo": true, "sky": true, "orange": true, "gray": true}

func validateVMProfile(p *state.VMProfileConfig) error {
	if !profileIDRegex.MatchString(p.ID) {
		return fmt.Errorf("id must be lowercase alphanumeric with hyphens (max 50 chars, start with alphanumeric)")
	}
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if p.Sockets < 1 || p.Sockets > 8 {
		return fmt.Errorf("sockets must be between 1 and 8")
	}
	if p.Cores < 1 || p.Cores > 64 {
		return fmt.Errorf("cores must be between 1 and 64")
	}
	if p.RAMGB < 1 || p.RAMGB > 512 {
		return fmt.Errorf("ram_gb must be between 1 and 512")
	}
	if p.DiskGB < 1 || p.DiskGB > 2000 {
		return fmt.Errorf("disk_gb must be between 1 and 2000")
	}
	if !validDiskBuses[p.DiskBus] {
		return fmt.Errorf("disk_bus must be one of: virtio, scsi, sata, ide")
	}
	if !validProfileIcons[p.Icon] {
		return fmt.Errorf("icon must be one of: Globe, Code, Cube, Database, Flask, Monitor, Cpu, HardDrive, Cloud, Info")
	}
	if !validProfileColors[p.Color] {
		return fmt.Errorf("color must be one of: blue, violet, emerald, teal, amber, rose, indigo, sky, orange, gray")
	}
	return nil
}

// slugifyProfile converts a name to a safe profile ID.
func slugifyProfile(name string) string {
	slug := strings.ToLower(strings.TrimSpace(name))
	slug = slugRe.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 50 {
		slug = slug[:50]
	}
	if slug == "" {
		return "profile"
	}
	return slug
}

// VMProfileListResponse is the response for GET /api/v1/admin/vm-profiles.
type VMProfileListResponse struct {
	Profiles      []state.VMProfileConfig `json:"profiles"`
	UsingDefaults bool                    `json:"using_defaults"`
}

// ListVMProfiles handles GET /api/v1/admin/vm-profiles.
func (h *AdminMutationsHandler) ListVMProfiles(w http.ResponseWriter, _ *http.Request) {
	settings := h.state.GetSettings()
	usingDefaults := len(settings.VMProfiles) == 0
	writeJSON(w, VMProfileListResponse{
		Profiles:      settings.GetVMProfiles(),
		UsingDefaults: usingDefaults,
	})
}

// vmProfileConfigToDB converts a state.VMProfileConfig to a database.VMProfile
// by marshalling the config fields into a JSON blob.
func vmProfileConfigToDB(p state.VMProfileConfig) (*database.VMProfile, error) {
	blob := database.VMProfileConfigBlob{
		Sockets: p.Sockets, Cores: p.Cores, RAMGB: p.RAMGB,
		DiskGB: p.DiskGB, DiskBus: p.DiskBus, Node: p.Node,
		Storage: p.Storage, Icon: p.Icon, Color: p.Color,
		EnableEFI: p.EnableEFI,
	}
	configBytes, err := json.Marshal(blob)
	if err != nil {
		return nil, fmt.Errorf("marshal profile config: %w", err)
	}
	return &database.VMProfile{
		ID: p.ID, Name: p.Name, Description: p.Description,
		Config: string(configBytes), Enabled: p.Enabled,
	}, nil
}

// CreateVMProfile handles POST /api/v1/admin/vm-profiles.
func (h *AdminMutationsHandler) CreateVMProfile(w http.ResponseWriter, r *http.Request) {
	var req state.VMProfileConfig
	if !decodeBody(w, r, &req) {
		return
	}
	req.ID = strings.TrimSpace(req.ID)
	req.Name = strings.TrimSpace(req.Name)
	if req.ID == "" {
		req.ID = slugifyProfile(req.Name)
	}
	if err := validateVMProfile(&req); err != nil {
		errBadRequest(w, err.Error())
		return
	}
	settings := h.state.GetSettings()
	profiles := settings.GetVMProfiles()
	for _, p := range profiles {
		if p.ID == req.ID {
			errBadRequest(w, "a profile with this ID already exists")
			return
		}
	}
	if h.state.HasDB() {
		dbProfile, err := vmProfileConfigToDB(req)
		if err != nil {
			writeAppError(w, err)
			return
		}
		if err := h.state.CreateVMProfile(dbProfile, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		if len(newSettings.VMProfiles) == 0 {
			newSettings.VMProfiles = state.DefaultVMProfiles()
		}
		newSettings.VMProfiles = copyVMProfiles(newSettings.VMProfiles)
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		newSettings.VMProfiles = append(newSettings.VMProfiles, req)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusCreated)
	writeJSON(w, req)
}

// UpdateVMProfile handles PUT /api/v1/admin/vm-profiles/:id.
func (h *AdminMutationsHandler) UpdateVMProfile(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	id := ps.ByName("id")
	if id == "" {
		errBadRequest(w, "missing profile id")
		return
	}
	var req state.VMProfileConfig
	if !decodeBody(w, r, &req) {
		return
	}
	req.ID = strings.TrimSpace(id)
	req.Name = strings.TrimSpace(req.Name)
	if err := validateVMProfile(&req); err != nil {
		errBadRequest(w, err.Error())
		return
	}
	settings := h.state.GetSettings()
	profiles := settings.GetVMProfiles()
	found := false
	for _, p := range profiles {
		if p.ID == req.ID {
			found = true
			break
		}
	}
	if !found {
		errNotFound(w, "profile not found")
		return
	}
	if h.state.HasDB() {
		dbProfile, err := vmProfileConfigToDB(req)
		if err != nil {
			writeAppError(w, err)
			return
		}
		if err := h.state.UpdateVMProfile(dbProfile, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		if len(newSettings.VMProfiles) == 0 {
			newSettings.VMProfiles = state.DefaultVMProfiles()
		}
		newSettings.VMProfiles = copyVMProfiles(newSettings.VMProfiles)
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		newSettings.AddOrUpdateVMProfile(req)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	writeJSON(w, req)
}

// DeleteVMProfile handles DELETE /api/v1/admin/vm-profiles/:id.
func (h *AdminMutationsHandler) DeleteVMProfile(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	id := ps.ByName("id")
	if id == "" {
		errBadRequest(w, "missing profile id")
		return
	}
	settings := h.state.GetSettings()
	found := false
	for _, p := range settings.GetVMProfiles() {
		if p.ID == id {
			found = true
			break
		}
	}
	if !found {
		errNotFound(w, "profile not found")
		return
	}
	if h.state.HasDB() {
		if err := h.state.DeleteVMProfile(id, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		if len(newSettings.VMProfiles) == 0 {
			newSettings.VMProfiles = state.DefaultVMProfiles()
		}
		newSettings.VMProfiles = copyVMProfiles(newSettings.VMProfiles)
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		newSettings.RemoveVMProfile(id)
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// ToggleVMProfile handles POST /api/v1/admin/vm-profiles/:id/toggle.
func (h *AdminMutationsHandler) ToggleVMProfile(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	id := ps.ByName("id")
	if id == "" {
		errBadRequest(w, "missing profile id")
		return
	}
	settings := h.state.GetSettings()
	found := false
	var toggled state.VMProfileConfig
	for _, p := range settings.GetVMProfiles() {
		if p.ID == id {
			p.Enabled = !p.Enabled
			toggled = p
			found = true
			break
		}
	}
	if !found {
		errNotFound(w, "profile not found")
		return
	}
	if h.state.HasDB() {
		dbProfile, err := vmProfileConfigToDB(toggled)
		if err != nil {
			writeAppError(w, err)
			return
		}
		if err := h.state.UpdateVMProfile(dbProfile, usernameFromCtx(r)); err != nil {
			writeAppError(w, err)
			return
		}
	} else {
		newSettings := *settings
		if len(newSettings.VMProfiles) == 0 {
			newSettings.VMProfiles = state.DefaultVMProfiles()
		}
		newSettings.VMProfiles = copyVMProfiles(newSettings.VMProfiles)
		newSettings.Limits.Nodes = copyNodeLimits(settings.Limits.Nodes)
		for i, p := range newSettings.VMProfiles {
			if p.ID == id {
				newSettings.VMProfiles[i].Enabled = toggled.Enabled
				break
			}
		}
		if err := h.state.SetSettings(&newSettings); err != nil {
			writeAppError(w, err)
			return
		}
	}
	w.WriteHeader(http.StatusNoContent)
}

// copyVMProfiles deep-copies a profile slice to avoid shared backing array.
func copyVMProfiles(src []state.VMProfileConfig) []state.VMProfileConfig {
	dst := make([]state.VMProfileConfig, len(src))
	copy(dst, src)
	return dst
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
