package apiv1

import (
	"net/http"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

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
	result := make([]AdminNodeResponse, 0, len(cached))
	for _, details := range cached {
		result = append(result, AdminNodeResponse{
			Name:      details.Node,
			Status:    details.Status,
			CPU:       details.CPU,
			MaxCPU:    details.MaxCPU,
			Memory:    details.Memory,
			MaxMemory: details.MaxMemory,
			Disk:      details.Disk,
			MaxDisk:   details.MaxDisk,
			Uptime:    details.Uptime,
		})
	}
	writeJSON(w, result)
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
	var result []AdminVMBRResponse
	for _, node := range nodeNames {
		bridges, err := proxmox.GetVMBRsResty(r.Context(), restyClient, node)
		if err != nil {
			continue
		}
		for _, b := range bridges {
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
				Node:        node,
				Enabled:     enabledSet[node+":"+b.Iface],
			})
		}
	}
	if result == nil {
		result = []AdminVMBRResponse{}
	}
	writeJSON(w, result)
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
	// Find storages that support ISO content
	var isoStorages []string
	for _, s := range storages {
		if strings.Contains(s.Content, "iso") {
			isoStorages = append(isoStorages, s.Storage)
		}
	}
	var result []AdminISOResponse
	for _, node := range nodeNames {
		for _, storage := range isoStorages {
			isos, err := proxmox.GetISOListResty(r.Context(), restyClient, node, storage)
			if err != nil {
				continue
			}
			for _, iso := range isos {
				result = append(result, AdminISOResponse{
					VolID:   iso.VolID,
					Name:    iso.VolID,
					Size:    iso.Size,
					Storage: storage,
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

// AppInfo handles GET /api/v1/admin/appinfo.
func (h *AdminHandler) AppInfo(w http.ResponseWriter, r *http.Request) {
	connected, _ := h.state.GetProxmoxStatus()
	nodes, _ := h.state.GetNodeCache()

	totalVMs := 0
	snap := h.state.GetProxmoxSnapshot()
	if snap != nil {
		totalVMs = len(snap.VMs)
	}

	resp := AdminAppInfoResponse{
		Version:          os.Getenv("PVMSS_VERSION"),
		Environment:      os.Getenv("PVMSS_ENV"),
		GoVersion:        runtime.Version(),
		Platform:         runtime.GOOS + "/" + runtime.GOARCH,
		ProxmoxConnected: connected,
		ProxmoxURL:       os.Getenv("PM_API_URL"),
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
		"PVMSS_ENV", "PVMSS_OFFLINE", "PVMSS_SETTINGS_PATH",
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
