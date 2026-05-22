package apiv1

import (
	"cmp"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// AdminVMsAPIHandler handles admin VM endpoints.
type AdminVMsAPIHandler struct {
	state state.StateManager
}

// MakeAdminVMsAPIHandler creates a new AdminVMsAPIHandler.
func MakeAdminVMsAPIHandler(s state.StateManager) *AdminVMsAPIHandler {
	return &AdminVMsAPIHandler{state: s}
}

// ListAllVMs handles GET /api/v1/admin/vms.
func (h *AdminVMsAPIHandler) ListAllVMs(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		writeJSON(w, []AdminVMResponse{})
		return
	}
	restyClient, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}
	vms, err := proxmox.GetVMsResty(r.Context(), restyClient)
	if err != nil {
		writeAppError(w, err)
		return
	}
	result := make([]AdminVMResponse, 0, len(vms))
	for _, vm := range vms {
		if !hasTag(vm.Tags, "pvmss") {
			continue
		}
		result = append(result, AdminVMResponse{
			VMID:    vm.VMID,
			Name:    vm.Name,
			Node:    vm.Node,
			Status:  vm.Status,
			CPU:     vm.CPU,
			CPUs:    vm.CPUs,
			Mem:     vm.Mem,
			MaxMem:  vm.MaxMem,
			MaxDisk: vm.MaxDisk,
			Uptime:  vm.Uptime,
			Tags:    vm.Tags,
		})
	}
	slices.SortFunc(result, func(a, b AdminVMResponse) int { return cmp.Compare(a.VMID, b.VMID) })
	writeJSON(w, result)
}

// DeleteVM handles DELETE /api/v1/admin/vms/:id.
// Permanently deletes the VM and all its associated disk files from Proxmox.
func (h *AdminVMsAPIHandler) DeleteVM(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		errOffline(w)
		return
	}
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}
	restyClient, err := proxmox.MakeRestyClientFromEnv(30 * time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}
	vms, err := proxmox.GetVMsResty(r.Context(), restyClient)
	if err != nil {
		writeAppError(w, err)
		return
	}
	var node string
	for _, vm := range vms {
		if vm.VMID == vmid && hasTag(vm.Tags, "pvmss") {
			node = vm.Node
			break
		}
	}
	if node == "" {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}
	if err := proxmox.DeleteVMResty(r.Context(), restyClient, node, vmid); err != nil {
		logger.Get().Error().Err(err).Int("vmid", vmid).Str("node", node).Msg("api/v1: VM deletion failed")
		writeError(w, http.StatusInternalServerError, "delete_failed", "Failed to delete VM")
		return
	}
	writeJSON(w, map[string]bool{"success": true})
}

// VMAction handles POST /api/v1/admin/vms/:id/action.
func (h *AdminVMsAPIHandler) VMAction(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		errOffline(w)
		return
	}
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}
	var req AdminVMActionRequest
	if !decodeBody(w, r, &req) {
		return
	}
	validActions := map[string]bool{
		"start": true, "stop": true, "shutdown": true, "reboot": true, "reset": true,
	}
	if !validActions[req.Action] {
		errBadRequest(w, "invalid action: must be start, stop, shutdown, reboot, or reset")
		return
	}
	restyClient, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}
	node := req.Node
	if node == "" {
		// node not provided by client: look it up from the VM list
		vms, err := proxmox.GetVMsResty(r.Context(), restyClient)
		if err != nil {
			writeAppError(w, err)
			return
		}
		for _, vm := range vms {
			if vm.VMID == vmid {
				node = vm.Node
				break
			}
		}
		if node == "" {
			errBadRequest(w, "VM not found")
			return
		}
	}
	taskID, err := proxmox.VMActionResty(r.Context(), restyClient, node, strconv.Itoa(vmid), req.Action)
	if err != nil {
		logger.Get().Error().Err(err).Int("vmid", vmid).Str("node", node).Str("action", req.Action).Msg("api/v1: VM action failed")
		writeError(w, http.StatusInternalServerError, "vm_action_failed", "Failed to perform VM action")
		return
	}
	writeJSON(w, VMActionResponse{Success: true, TaskID: taskID})
}

// ListAllVMsPaginated handles GET /api/v1/admin/vms/paginated.
func (h *AdminVMsAPIHandler) ListAllVMsPaginated(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		writeJSON(w, AdminVMListPaginatedResponse{
			VMs:        []AdminVMResponse{},
			Pagination: PaginationMetadata{Total: 0, Page: 1, Limit: 25, TotalPages: 1},
		})
		return
	}

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
	filterNode := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("node")))

	var result []AdminVMResponse

	snapshot := h.state.GetProxmoxSnapshot()
	if snapshot != nil {
		for _, vm := range snapshot.VMs {
			if !filterAdminVM(vm.Tags, vm.Node, vm.VMID, vm.Name, filterNode, search) {
				continue
			}
			result = append(result, snapshotVMToAdminResponse(vm))
		}
	} else {
		rc, err := proxmox.MakeRestyClientFromEnv(10 * time.Second)
		if err != nil {
			writeAppError(w, err)
			return
		}
		vms, err := proxmox.GetVMsResty(r.Context(), rc)
		if err != nil {
			writeAppError(w, err)
			return
		}
		for _, vm := range vms {
			if !filterAdminVM(vm.Tags, vm.Node, vm.VMID, vm.Name, filterNode, search) {
				continue
			}
			result = append(result, AdminVMResponse{
				VMID:    vm.VMID,
				Name:    vm.Name,
				Node:    vm.Node,
				Status:  vm.Status,
				CPU:     vm.CPU,
				CPUs:    vm.CPUs,
				Mem:     vm.Mem,
				MaxMem:  vm.MaxMem,
				MaxDisk: vm.MaxDisk,
				Uptime:  vm.Uptime,
				Tags:    vm.Tags,
			})
		}
	}

	validateAdminSortBy(sortBy)
	sortAdminVMs(result, sortBy, sortOrder)
	writeJSON(w, paginateAdminVMs(result, page, limit))
}

// filterAdminVM returns true if the VM passes tag, node, and search filters.
func filterAdminVM(tags, node string, vmid int, name, filterNode, search string) bool {
	if !hasTag(tags, "pvmss") {
		return false
	}
	if filterNode != "" && !strings.EqualFold(node, filterNode) {
		return false
	}
	if search != "" {
		vmidStr := strconv.Itoa(vmid)
		matched := strings.Contains(strings.ToLower(name), search) ||
			strings.Contains(vmidStr, search) ||
			containsTagSubstring(tags, search)
		if !matched {
			return false
		}
	}
	return true
}

func snapshotVMToAdminResponse(vm state.SnapshotVM) AdminVMResponse {
	return AdminVMResponse{
		VMID:    vm.VMID,
		Name:    vm.Name,
		Node:    vm.Node,
		Status:  vm.Status,
		CPU:     0,
		CPUs:    vm.Cores,
		Mem:     0,
		MaxMem:  vm.MemoryMB * 1024 * 1024,
		MaxDisk: 0,
		Uptime:  0,
		Tags:    vm.Tags,
	}
}

// validAdminSortKeys lists accepted sort_by values for the admin VMs endpoint.
var validAdminSortKeys = map[string]bool{"vmid": true, "name": true, "status": true, "cpu": true, "memory": true}

// validateAdminSortBy logs a warning for unrecognised sort keys.
func validateAdminSortBy(sortBy string) {
	if sortBy != "" && !validAdminSortKeys[sortBy] {
		logger.Get().Warn().Str("sort_by", sortBy).Str("endpoint", "admin/vms/paginated").Msg("Unrecognised sort_by value; falling back to vmid")
	}
}

func sortAdminVMs(vms []AdminVMResponse, sortBy, sortOrder string) {
	ascending := sortOrder != "desc"
	switch sortBy {
	case "name":
		slices.SortFunc(vms, func(a, b AdminVMResponse) int {
			cmp := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
			if ascending {
				return cmp
			}
			return -cmp
		})
	case "status":
		slices.SortFunc(vms, func(a, b AdminVMResponse) int {
			cmp := strings.Compare(a.Status, b.Status)
			if ascending {
				return cmp
			}
			return -cmp
		})
	case "cpu":
		slices.SortFunc(vms, func(a, b AdminVMResponse) int {
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
		slices.SortFunc(vms, func(a, b AdminVMResponse) int {
			if a.MaxMem < b.MaxMem {
				if ascending {
					return -1
				}
				return 1
			}
			if a.MaxMem > b.MaxMem {
				if ascending {
					return 1
				}
				return -1
			}
			return 0
		})
	default: // "vmid" and fallback
		slices.SortFunc(vms, func(a, b AdminVMResponse) int {
			c := cmp.Compare(a.VMID, b.VMID)
			if ascending {
				return c
			}
			return -c
		})
	}
}

func paginateAdminVMs(vms []AdminVMResponse, page, limit int) AdminVMListPaginatedResponse {
	total := len(vms)
	running := 0
	stopped := 0
	for _, vm := range vms {
		switch vm.Status {
		case "running":
			running++
		case "stopped":
			stopped++
		}
	}
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
	var paged []AdminVMResponse
	if offset < total {
		paged = vms[offset:end]
	} else {
		paged = []AdminVMResponse{}
	}
	return AdminVMListPaginatedResponse{
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
		},
	}
}
