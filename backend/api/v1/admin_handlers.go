package apiv1

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"pvmss/constants"
	"pvmss/proxmox"
	"pvmss/state"
)

// AdminHandler handles read-only admin API endpoints.
type AdminHandler struct {
	state state.StateManager
}

// MakeAdminHandler creates a new AdminHandler.
func MakeAdminHandler(s state.StateManager) *AdminHandler {
	return &AdminHandler{state: s}
}

// Nodes handles GET /api/v1/admin/nodes.
func (h *AdminHandler) Nodes(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		writeJSON(w, []AdminNodeResponse{})
		return
	}
	cached, _ := h.state.GetNodeCache()
	if len(cached) == 0 {
		// Cache not yet populated — trigger a synchronous refresh so the first
		// page load returns real data instead of an empty list.
		h.state.RefreshNodeCache(r.Context())
		cached, _ = h.state.GetNodeCache()
	}
	settings := h.state.GetSettings()
	enabledNodes := settings.EnabledNodes
	// Empty list means all nodes are accessible.
	allEnabled := len(enabledNodes) == 0
	enabledSet := make(map[string]bool, len(enabledNodes))
	for _, n := range enabledNodes {
		enabledSet[n] = true
	}
	result := make([]AdminNodeResponse, 0, len(cached))
	for _, details := range cached {
		userEnabled := allEnabled || enabledSet[details.Node]
		result = append(result, AdminNodeResponse{
			Name:        details.Node,
			Status:      details.Status,
			CPU:         details.CPU,
			MaxCPU:      details.MaxCPU,
			CpuSockets:  details.Sockets,
			Memory:      details.Memory,
			MaxMemory:   details.MaxMemory,
			Disk:        details.Disk,
			MaxDisk:     details.MaxDisk,
			Uptime:      details.Uptime,
			UserEnabled: userEnabled,
		})
	}
	writeJSON(w, result)
}

// ToggleNode handles POST /api/v1/admin/nodes/toggle.
// It adds or removes a node from the EnabledNodes allowlist.
// When EnabledNodes is empty, all nodes are accessible; toggling one off
// switches to allowlist mode and enables all other nodes.
func (h *AdminHandler) ToggleNode(w http.ResponseWriter, r *http.Request) {
	var req ToggleNodeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid JSON body")
		return
	}
	if req.Name == "" {
		errBadRequest(w, "name is required")
		return
	}

	settings := h.state.GetSettings()
	currentEnabled := settings.EnabledNodes

	var newEnabled []string
	if len(currentEnabled) == 0 {
		// All-allowed mode: switching to allowlist — enable all known nodes except this one.
		cached, _ := h.state.GetNodeCache()
		newEnabled = make([]string, 0, len(cached))
		for _, n := range cached {
			if n.Node != req.Name {
				newEnabled = append(newEnabled, n.Node)
			}
		}
	} else {
		// Allowlist mode: toggle the node in/out.
		found := false
		filtered := make([]string, 0, len(currentEnabled))
		for _, n := range currentEnabled {
			if n == req.Name {
				found = true
			} else {
				filtered = append(filtered, n)
			}
		}
		if found {
			// Remove (disable) the node.
			newEnabled = filtered
		} else {
			// Add (enable) the node.
			newEnabled = append(currentEnabled, req.Name)
		}
	}

	changedBy := usernameFromCtx(r)
	if err := h.state.SetEnabledNodes(newEnabled, changedBy); err != nil {
		errInternal(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Storage handles GET /api/v1/admin/storage.
// It queries each node in parallel to obtain disk usage stats (used/total/avail),
// which are only available from the per-node endpoint /nodes/{node}/storage.
func (h *AdminHandler) Storage(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		writeJSON(w, []AdminStorageResponse{})
		return
	}
	restyClient, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
	if err != nil {
		errInternal(w)
		return
	}
	nodeNames, err := proxmox.GetNodeNamesResty(r.Context(), restyClient)
	if err != nil {
		errInternal(w)
		return
	}
	enabled := h.state.GetStorages()
	enabledSet := make(map[string]bool, len(enabled))
	for _, s := range enabled {
		enabledSet[s] = true
	}

	type nodeResult struct {
		node     string
		storages []proxmox.Storage
	}
	ch := make(chan nodeResult, len(nodeNames))
	var wg sync.WaitGroup
	for _, node := range nodeNames {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			storages, err := proxmox.GetNodeStoragesResty(r.Context(), restyClient, n)
			if err != nil {
				return
			}
			ch <- nodeResult{node: n, storages: storages}
		}(node)
	}
	wg.Wait()
	close(ch)

	var result []AdminStorageResponse
	for nr := range ch {
		for _, s := range nr.storages {
			// Only include storages that can hold VM disk images.
			if !strings.Contains(s.Content, "images") {
				continue
			}
			total, _ := s.Total.Int64()
			used, _ := s.Used.Int64()
			free, _ := s.Avail.Int64()
			result = append(result, AdminStorageResponse{
				Storage: s.Storage,
				Type:    s.Type,
				Content: s.Content,
				Total:   total,
				Used:    used,
				Free:    free,
				Node:    nr.node,
				Enabled: enabledSet[nr.node+":"+s.Storage],
			})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Node != result[j].Node {
			return result[i].Node < result[j].Node
		}
		return result[i].Storage < result[j].Storage
	})
	if result == nil {
		result = []AdminStorageResponse{}
	}
	writeJSON(w, result)
}

// VMBR handles GET /api/v1/admin/vmbr.
func (h *AdminHandler) VMBR(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		writeJSON(w, []AdminVMBRResponse{})
		return
	}
	restyClient, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
	if err != nil {
		errInternal(w)
		return
	}
	nodeNames, err := proxmox.GetNodeNamesResty(r.Context(), restyClient)
	if err != nil {
		errInternal(w)
		return
	}
	enabledVMBRs := h.state.GetVMBRs()
	enabledSet := make(map[string]bool, len(enabledVMBRs))
	for _, v := range enabledVMBRs {
		enabledSet[v] = true
	}
	type nodeResult struct {
		node    string
		bridges []proxmox.VMBR
	}
	ch := make(chan nodeResult, len(nodeNames))
	var wg sync.WaitGroup
	for _, node := range nodeNames {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			bridges, err := proxmox.GetVMBRsResty(r.Context(), restyClient, n)
			if err != nil {
				return
			}
			ch <- nodeResult{node: n, bridges: bridges}
		}(node)
	}
	wg.Wait()
	close(ch)

	var result []AdminVMBRResponse
	for nr := range ch {
		for _, b := range nr.bridges {
			active := false
			if b.Active != nil {
				switch v := b.Active.(type) {
				case float64:
					active = v == 1
				case bool:
					active = v
				}
			}
			result = append(result, AdminVMBRResponse{
				Iface:       b.Iface,
				Type:        b.Type,
				Active:      active,
				BridgePorts: b.BridgePorts,
				Comments:    strings.TrimSpace(b.Comments),
				Node:        nr.node,
				Enabled:     enabledSet[nr.node+":"+b.Iface],
			})
		}
	}
	if result == nil {
		result = []AdminVMBRResponse{}
	}
	writeJSON(w, result)
}

const isoPerStorageTimeout = 15 * time.Second

// isoStorageSupportsISO checks if a storage content field includes "iso" as a proper content type.
func isoStorageSupportsISO(content string) bool {
	for _, part := range strings.Split(content, ",") {
		if strings.TrimSpace(part) == "iso" {
			return true
		}
	}
	return false
}

// isoStorageAvailableOnNode returns true if the storage is available on the given node.
// An empty Nodes field means the storage is shared across all nodes.
func isoStorageAvailableOnNode(nodesField, nodeName string) bool {
	if nodesField == "" {
		return true
	}
	for _, n := range strings.Split(nodesField, ",") {
		if strings.TrimSpace(n) == nodeName {
			return true
		}
	}
	return false
}

// ISO handles GET /api/v1/admin/iso.
func (h *AdminHandler) ISO(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		writeJSON(w, []AdminISOResponse{})
		return
	}
	restyClient, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
	if err != nil {
		errInternal(w)
		return
	}
	nodeNames, err := proxmox.GetNodeNamesResty(r.Context(), restyClient)
	if err != nil {
		errInternal(w)
		return
	}
	enabledISOs := h.state.GetISOs()
	enabledSet := make(map[string]bool, len(enabledISOs))
	for _, iso := range enabledISOs {
		enabledSet[iso] = true
	}
	storages, err := proxmox.GetStoragesResty(r.Context(), restyClient)
	if err != nil {
		errInternal(w)
		return
	}
	seen := make(map[string]bool)
	var result []AdminISOResponse
	for _, node := range nodeNames {
		for _, s := range storages {
			if !isoStorageSupportsISO(s.Content) || !isoStorageAvailableOnNode(s.Nodes, node) {
				continue
			}
			storageCtx, storageCancel := context.WithTimeout(r.Context(), isoPerStorageTimeout)
			isos, err := proxmox.GetISOListResty(storageCtx, restyClient, node, s.Storage)
			storageCancel()
			if err != nil {
				continue
			}
			for _, iso := range isos {
				if iso.VolID == "" || seen[iso.VolID] {
					continue
				}
				seen[iso.VolID] = true
				name := iso.VolID
				if idx := strings.LastIndex(name, "/"); idx >= 0 {
					name = name[idx+1:]
				}
				result = append(result, AdminISOResponse{
					VolID:   iso.VolID,
					Name:    name,
					Size:    iso.Size,
					Storage: s.Storage,
					Node:    node,
					Enabled: enabledSet[iso.VolID],
				})
			}
		}
	}
	if result == nil {
		result = []AdminISOResponse{}
	}
	writeJSON(w, result)
}

// CloudInitStorages handles GET /api/v1/admin/cloudinit/storages.
// Returns storages that support snippets content type.
func (h *AdminHandler) CloudInitStorages(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		writeJSON(w, []string{})
		return
	}
	restyClient, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
	if err != nil {
		errInternal(w)
		return
	}
	storages, err := proxmox.GetSnippetsStoragesResty(r.Context(), restyClient)
	if err != nil {
		writeJSON(w, []string{})
		return
	}
	names := make([]string, 0, len(storages))
	for _, s := range storages {
		names = append(names, s.Storage)
	}
	writeJSON(w, names)
}

// Version handles GET /api/v1/public/version.
// Public endpoint that returns only the application version.
func (h *AdminHandler) Version(w http.ResponseWriter, _ *http.Request) {
	version := os.Getenv("PVMSS_VERSION")
	if version == "" {
		version = constants.AppVersion
	}
	resp := map[string]string{
		"version": version,
	}
	w.Header().Set("Cache-Control", "public, max-age=3600")
	writeJSON(w, resp)
}

// AppInfo handles GET /api/v1/admin/appinfo.
func (h *AdminHandler) AppInfo(w http.ResponseWriter, r *http.Request) {
	connected, _ := h.state.GetProxmoxStatus()
	nodes, _ := h.state.GetNodeCache()

	totalVMs := 0
	snap := h.state.GetProxmoxSnapshot()
	if snap != nil {
		totalVMs = len(snap.VMs)
	}

	envCfg := h.state.GetEnvConfig()
	resp := AdminAppInfoResponse{
		Version:          os.Getenv("PVMSS_VERSION"),
		Environment:      envCfg.Environment,
		GoVersion:        runtime.Version(),
		Platform:         runtime.GOOS + "/" + runtime.GOARCH,
		ProxmoxConnected: connected,
		ProxmoxURL:       envCfg.ProxmoxURL,
		OfflineMode:      h.state.IsOfflineMode(),
		TotalNodes:       len(nodes),
		TotalVMs:         totalVMs,
		ClusterInfo:      fetchClusterInfo(r, h.state.IsOfflineMode()),
		EnvVars:          collectSafeEnvVars(),
	}

	writeJSON(w, resp)
}

// fetchClusterInfo retrieves cluster information from Proxmox.
// Returns nil if in offline mode or on error.
func fetchClusterInfo(r *http.Request, offlineMode bool) *AdminClusterInfoResponse {
	if offlineMode {
		return nil
	}
	restyClient, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
	if err != nil {
		return nil
	}
	info, err := proxmox.GetClusterStatusResty(r.Context(), restyClient)
	if err != nil || info == nil {
		return nil
	}
	return &AdminClusterInfoResponse{
		IsCluster:   info.IsCluster,
		ClusterName: info.ClusterName,
		NodeCount:   info.NodeCount,
	}
}

// collectSafeEnvVars returns a map of safe, non-empty environment variables.
func collectSafeEnvVars() map[string]string {
	safeKeys := []string{
		"LOG_LEVEL", "LOG_FORMAT", "LOG_OUTPUT",
		"PROXMOX_URL", "PROXMOX_VERIFY_SSL",
		"PVMSS_ENV", "PVMSS_OFFLINE", "PVMSS_DB_PATH",
	}
	vars := make(map[string]string, len(safeKeys))
	for _, key := range safeKeys {
		if val := os.Getenv(key); val != "" {
			vars[key] = val
		}
	}
	if len(vars) == 0 {
		return nil
	}
	return vars
}

// Settings handles GET /api/v1/admin/settings.
func (h *AdminHandler) Settings(w http.ResponseWriter, _ *http.Request) {
	settings := h.state.GetSettings()
	writeJSON(w, settings)
}
