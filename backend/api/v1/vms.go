package apiv1

import (
	"cmp"
	"context"
	"net/http"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"pvmss/constants"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// VMHandler handles VM listing and detail endpoints.
type VMHandler struct {
	state state.StateManager
}

// MakeVMHandler creates a new VMHandler.
func MakeVMHandler(s state.StateManager) *VMHandler {
	return &VMHandler{state: s}
}

// isOffline returns true when offline mode is set in EnvConfig or state signals offline.
func (h *VMHandler) isOffline() bool {
	if h.state == nil {
		return true
	}
	cfg := h.state.GetEnvConfig()
	return (cfg != nil && cfg.Offline) || h.state.IsOfflineMode()
}

// ListVMs handles GET /api/v1/vms.
// Admin users see all VMs tagged constants.RequiredTag. Regular users see only VMs in their
// pool (pvmss_<username>) that are also tagged constants.RequiredTag.
func (h *VMHandler) ListVMs(w http.ResponseWriter, r *http.Request) {
	if h.isOffline() {
		writeJSON(w, VMListPaginatedResponse{
			VMs:        []VMSummary{},
			Pagination: PaginationMetadata{Total: 0, Page: 1, Limit: 25, TotalPages: 1},
		})
		return
	}

	// Parse pagination parameters.
	page := parseIntParam(r.URL.Query().Get("page"), 1)
	limit := parseIntParam(r.URL.Query().Get("limit"), 25)
	if limit > 100 {
		limit = 100
	}
	if limit < 1 {
		limit = 25
	}
	search := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("search")))
	sortBy := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_by")))
	sortOrder := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_order")))
	if sortBy == "" {
		sortBy = "vmid"
	}
	if sortOrder != "desc" {
		sortOrder = "asc"
	}

	// Legacy search params kept for backward compatibility (search/vms route).
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	if q != "" && search == "" {
		search = q
	}
	filterType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if filterType == "" {
		filterType = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("filter")))
	}
	filterStatus := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	filterNode := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("node")))

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	var summaries []VMSummary

	// For admin users, try the snapshot cache first to avoid a live Proxmox call.
	// Non-admin users need pool filtering which requires live data (SnapshotVM
	// lacks pool membership), so they always use the live API path.
	snapshot := h.state.GetProxmoxSnapshot()
	if isAdmin && snapshot != nil {
		for _, vm := range snapshot.VMs {
			if !filterUserVM(vm.Tags, vm.Node, vm.VMID, vm.Name, "", filterNode, search, filterType) {
				continue
			}
			if filterStatus != "" && !strings.EqualFold(vm.Status, filterStatus) {
				continue
			}
			summaries = append(summaries, snapshotVMToSummary(vm))
		}
	} else {
		cfg := h.state.GetEnvConfig()
		client, err := proxmox.MakeRestyClientFromEnvConfig(cfg, 30*time.Second)
		if err != nil {
			writeAppError(w, err)
			return
		}

		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()

		allVMs, err := proxmox.GetVMsResty(ctx, client)
		if err != nil {
			writeAppError(w, err)
			return
		}

		var poolVMIDs map[int]bool
		if !isAdmin && username != "" {
			poolVMIDs = fetchPoolVMIDs(ctx, client, constants.PoolPrefix+username)
		}

		for _, vm := range allVMs {
			if poolVMIDs != nil && !poolVMIDs[vm.VMID] {
				continue
			}
			if !filterUserVM(vm.Tags, vm.Node, vm.VMID, vm.Name, "", filterNode, search, filterType) {
				continue
			}
			if filterStatus != "" && !strings.EqualFold(vm.Status, filterStatus) {
				continue
			}
			summaries = append(summaries, vmToSummary(vm))
		}
	}

	validateVMSortBy(sortBy)
	sortVMs(summaries, sortBy, sortOrder)
	writeJSON(w, paginateVMs(summaries, page, limit))
}

// DeleteVM handles DELETE /api/v1/vms/:id.
// Admins may delete any pvmss-tagged VM; regular users may only delete VMs in their pool.
func (h *VMHandler) DeleteVM(w http.ResponseWriter, r *http.Request) {
	if h.isOffline() {
		errOffline(w)
		return
	}
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}
	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	cfg := h.state.GetEnvConfig()
	client, err := proxmox.MakeRestyClientFromEnvConfig(cfg, 30*time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	allVMs, err := proxmox.GetVMsResty(ctx, client)
	if err != nil {
		writeAppError(w, err)
		return
	}

	var targetNode string
	for _, vm := range allVMs {
		if vm.VMID == vmid && hasTag(vm.Tags, constants.RequiredTag) {
			targetNode = vm.Node
			break
		}
	}
	if targetNode == "" {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	// Non-admin: verify VM is in the user's pool.
	if !isAdmin {
		poolVMIDs := fetchPoolVMIDs(ctx, client, constants.PoolPrefix+username)
		if !poolVMIDs[vmid] {
			errForbidden(w)
			return
		}
	}

	if err := proxmox.DeleteVMResty(ctx, client, targetNode, vmid); err != nil {
		logger.Get().Error().Err(err).Int("vmid", vmid).Str("node", targetNode).Msg("api/v1: VM deletion failed")
		writeError(w, http.StatusInternalServerError, "delete_failed", "Failed to delete VM")
		return
	}
	writeJSON(w, map[string]bool{"success": true})
}

// fetchPoolVMIDs returns the set of QEMU VM IDs in the given Proxmox pool.
// Returns an empty map on error so callers can treat it as "no VMs".
func fetchPoolVMIDs(ctx context.Context, client *proxmox.RestyClient, poolName string) map[int]bool {
	ids := make(map[int]bool)
	var resp struct {
		Data struct {
			Members []struct {
				Type     string `json:"type"`
				VMID     int    `json:"vmid"`
				Template int    `json:"template"`
			} `json:"members"`
		} `json:"data"`
	}
	if err := client.Get(ctx, "/pools/"+poolName, &resp); err != nil {
		logger.Get().Warn().Err(err).Str("pool", poolName).Msg("api/v1: failed to fetch pool members")
		return ids
	}
	for _, m := range resp.Data.Members {
		if m.Template == 1 || m.VMID <= 0 {
			continue
		}
		if strings.EqualFold(m.Type, "qemu") {
			ids[m.VMID] = true
		}
	}
	return ids
}

// hasTag returns true if the semicolon-separated tags string contains target.
func hasTag(tags, target string) bool {
	for _, t := range strings.Split(tags, ";") {
		if strings.EqualFold(strings.TrimSpace(t), target) {
			return true
		}
	}
	return false
}

// containsTagSubstring returns true if any tag in the semicolon-separated list
// contains the given substring (case-insensitive).
func containsTagSubstring(tags, sub string) bool {
	for _, t := range strings.Split(tags, ";") {
		trimmed := strings.TrimSpace(t)
		if strings.EqualFold(trimmed, constants.RequiredTag) {
			continue
		}
		if strings.Contains(strings.ToLower(trimmed), sub) {
			return true
		}
	}
	return false
}

// filterUserVM returns true if the VM passes tag, node, and search filters.
// filterType controls the search scope: "name", "tag", "vmid", or "" for all.
func filterUserVM(tags, node string, vmid int, name, filterPool, filterNode, search, filterType string) bool {
	if !hasTag(tags, constants.RequiredTag) {
		return false
	}
	if filterNode != "" && !strings.EqualFold(node, filterNode) {
		return false
	}
	if search != "" {
		vmidStr := strconv.Itoa(vmid)
		var matched bool
		switch filterType {
		case "name":
			matched = strings.Contains(strings.ToLower(name), search)
		case "tag":
			matched = containsTagSubstring(tags, search)
		case "vmid":
			matched = strings.Contains(vmidStr, search)
		default:
			matched = strings.Contains(strings.ToLower(name), search) ||
				strings.Contains(vmidStr, search) ||
				containsTagSubstring(tags, search)
		}
		if !matched {
			return false
		}
	}
	return true
}

// snapshotVMToSummary converts a state.SnapshotVM to the API VMSummary.
// SnapshotVM lacks live metrics (CPU, Mem, Uptime, Disk) — these are zeroed.
func snapshotVMToSummary(vm state.SnapshotVM) VMSummary {
	return VMSummary{
		VMID:     vm.VMID,
		Name:     vm.Name,
		Node:     vm.Node,
		Status:   vm.Status,
		CPU:      0,
		CPUs:     vm.Cores,
		MemMB:    0,
		MaxMemMB: vm.MemoryMB,
		DiskMB:   0,
		Uptime:   0,
		Tags:     vm.Tags,
	}
}

// vmToSummary converts a proxmox.VM to the API VMSummary.
// Proxmox reports Mem/MaxMem/MaxDisk in bytes; we convert to MB.
func vmToSummary(vm proxmox.VM) VMSummary {
	const bytesPerMB = int64(1024 * 1024)
	return VMSummary{
		VMID:     vm.VMID,
		Name:     vm.Name,
		Node:     vm.Node,
		Status:   vm.Status,
		CPU:      vm.CPU,
		CPUs:     vm.CPUs,
		MemMB:    vm.Mem / bytesPerMB,
		MaxMemMB: vm.MaxMem / bytesPerMB,
		DiskMB:   vm.MaxDisk / bytesPerMB,
		Uptime:   vm.Uptime,
		Tags:     vm.Tags,
	}
}

// parseIntParam parses a string query param as int, returning defaultValue on error.
func parseIntParam(s string, defaultValue int) int {
	if s == "" {
		return defaultValue
	}
	i, err := strconv.Atoi(s)
	if err != nil || i < 0 {
		return defaultValue
	}
	return i
}

// validVMSortKeys lists accepted sort_by values for the user VMs endpoint.
var validVMSortKeys = map[string]bool{"vmid": true, "name": true, "status": true, "cpu": true, "memory": true, "node": true}

// validateVMSortBy logs a warning for unrecognised sort keys.
func validateVMSortBy(sortBy string) {
	if sortBy != "" && !validVMSortKeys[sortBy] {
		logger.Get().Warn().Str("sort_by", sortBy).Str("endpoint", "vms").Msg("Unrecognised sort_by value; falling back to vmid")
	}
}

// paginateVMs slices summaries based on page/limit and builds the paginated response.
// It also computes a Nodes facet (distinct nodes in the filtered set) for filter UIs.
func paginateVMs(summaries []VMSummary, page, limit int) VMListPaginatedResponse {
	total := len(summaries)
	running := 0
	stopped := 0
	nodeSet := make(map[string]struct{}, 8)
	for _, s := range summaries {
		switch s.Status {
		case "running":
			running++
		case "stopped":
			stopped++
		}
		if s.Node != "" {
			nodeSet[s.Node] = struct{}{}
		}
	}
	nodes := make([]string, 0, len(nodeSet))
	for n := range nodeSet {
		nodes = append(nodes, n)
	}
	sort.Strings(nodes)

	totalPages := (total + limit - 1) / limit
	if totalPages == 0 {
		totalPages = 1
	}
	if page < 1 {
		page = 1
	}
	if page > totalPages {
		page = totalPages
	}
	offset := (page - 1) * limit
	end := offset + limit
	if end > total {
		end = total
	}
	var paged []VMSummary
	if offset < total {
		paged = summaries[offset:end]
	} else {
		paged = []VMSummary{}
	}
	return VMListPaginatedResponse{
		VMs: paged,
		Pagination: PaginationMetadata{
			Total:        total,
			Page:         page,
			Limit:        limit,
			TotalPages:   totalPages,
			HasNext:      page < totalPages,
			HasPrev:      page > 1,
			RunningCount: running,
			StoppedCount: stopped,
			Nodes:        nodes,
		},
	}
}

// sortVMs sorts summaries in-place by the given field and order.
func sortVMs(vms []VMSummary, sortBy, sortOrder string) {
	ascending := sortOrder != "desc"
	switch sortBy {
	case "name":
		slices.SortFunc(vms, func(a, b VMSummary) int {
			cmp := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
			if ascending {
				return cmp
			}
			return -cmp
		})
	case "status":
		slices.SortFunc(vms, func(a, b VMSummary) int {
			cmp := strings.Compare(a.Status, b.Status)
			if ascending {
				return cmp
			}
			return -cmp
		})
	case "cpu":
		slices.SortFunc(vms, func(a, b VMSummary) int {
			if a.CPU < b.CPU {
				if ascending {
					return -1
				}
				return 1
			}
			if a.CPU > b.CPU {
				if ascending {
					return 1
				}
				return -1
			}
			return 0
		})
	case "memory":
		slices.SortFunc(vms, func(a, b VMSummary) int {
			if a.MaxMemMB < b.MaxMemMB {
				if ascending {
					return -1
				}
				return 1
			}
			if a.MaxMemMB > b.MaxMemMB {
				if ascending {
					return 1
				}
				return -1
			}
			return 0
		})
	case "node":
		slices.SortFunc(vms, func(a, b VMSummary) int {
			cmp := strings.Compare(strings.ToLower(a.Node), strings.ToLower(b.Node))
			if ascending {
				return cmp
			}
			return -cmp
		})
	default: // "vmid" and fallback
		slices.SortFunc(vms, func(a, b VMSummary) int {
			c := cmp.Compare(a.VMID, b.VMID)
			if ascending {
				return c
			}
			return -c
		})
	}
}
