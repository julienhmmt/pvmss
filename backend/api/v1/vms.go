package apiv1

import (
	"context"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

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

// isOffline returns true when PVMSS_OFFLINE=true or state signals offline.
func (h *VMHandler) isOffline() bool {
	return strings.EqualFold(os.Getenv("PVMSS_OFFLINE"), "true") ||
		h.state == nil || h.state.IsOfflineMode()
}

// restyClient returns a fresh Resty client from env vars with a 30s timeout.
func restyClient() (*proxmox.RestyClient, error) {
	return proxmox.MakeRestyClientFromEnv(30 * time.Second)
}

// ListVMs handles GET /api/v1/vms.
// Admin users see all VMs tagged "pvmss". Regular users see only VMs in their
// pool (pvmss_<username>) that are also tagged "pvmss".
func (h *VMHandler) ListVMs(w http.ResponseWriter, r *http.Request) {
	if h.isOffline() {
		writeJSON(w, VMListResponse{VMs: []VMSummary{}, Total: 0})
		return
	}

	username := usernameFromCtx(r)
	isAdmin := isAdminFromCtx(r)

	client, err := restyClient()
	if err != nil {
		logger.Get().Error().Err(err).Msg("api/v1: failed to create resty client for ListVMs")
		errInternal(w)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	allVMs, err := proxmox.GetVMsResty(ctx, client)
	if err != nil {
		logger.Get().Error().Err(err).Msg("api/v1: GetVMsResty failed")
		errInternal(w)
		return
	}

	// For non-admin users, restrict to VMs in their pool (pvmss_<username>).
	var poolVMIDs map[int]bool
	if !isAdmin && username != "" {
		poolVMIDs = fetchPoolVMIDs(ctx, client, "pvmss_"+username)
	}

	// Optional search/filter params: ?q=, ?type=name|tag|vmid, ?status=, ?node=
	q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	filterType := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("type")))
	if filterType == "" {
		filterType = strings.ToLower(strings.TrimSpace(r.URL.Query().Get("filter")))
	}
	filterStatus := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	filterNode := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("node")))

	summaries := make([]VMSummary, 0, len(allVMs))
	for _, vm := range allVMs {
		// Pool filter: non-admin users see only their pool's VMs.
		if poolVMIDs != nil && !poolVMIDs[vm.VMID] {
			continue
		}
		// Tag filter: VM must carry the "pvmss" tag.
		if !hasTag(vm.Tags, "pvmss") {
			continue
		}
		// Search filter: match by type (name|tag|vmid) or all three when unspecified.
		if q != "" {
			vmidStr := strconv.Itoa(vm.VMID)
			var matched bool
			switch filterType {
			case "name":
				matched = strings.Contains(strings.ToLower(vm.Name), q)
			case "tag":
				matched = containsTagSubstring(vm.Tags, q)
			case "vmid":
				matched = strings.Contains(vmidStr, q)
			default:
				matched = strings.Contains(strings.ToLower(vm.Name), q) ||
					strings.Contains(vmidStr, q) ||
					containsTagSubstring(vm.Tags, q)
			}
			if !matched {
				continue
			}
		}
		// Status filter.
		if filterStatus != "" && !strings.EqualFold(vm.Status, filterStatus) {
			continue
		}
		// Node filter.
		if filterNode != "" && !strings.EqualFold(vm.Node, filterNode) {
			continue
		}
		summaries = append(summaries, vmToSummary(vm))
	}

	writeJSON(w, VMListResponse{VMs: summaries, Total: len(summaries)})
}

// GetVM handles GET /api/v1/vms/:id.
func (h *VMHandler) GetVM(w http.ResponseWriter, r *http.Request) {
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

	var poolVMIDs map[int]bool
	if !isAdmin && username != "" {
		poolVMIDs = fetchPoolVMIDs(ctx, client, "pvmss_"+username)
	}

	allVMs, err := proxmox.GetVMsResty(ctx, client)
	if err != nil {
		errInternal(w)
		return
	}
	for _, vm := range allVMs {
		if vm.VMID != vmid {
			continue
		}
		if poolVMIDs != nil && !poolVMIDs[vm.VMID] {
			break // VM exists but not in user's pool
		}
		if !hasTag(vm.Tags, "pvmss") {
			break
		}
		writeJSON(w, vmToSummary(vm))
		return
	}
	writeError(w, http.StatusNotFound, "not_found", "VM not found")
}

// DeleteVM handles DELETE /api/v1/vms/:id.
// Admins may delete any pvmss-tagged VM; regular users may only delete VMs in their pool.
func (h *VMHandler) DeleteVM(w http.ResponseWriter, r *http.Request) {
	if h.isOffline() {
		errOffline(w)
		return
	}
	ps := httprouter.ParamsFromContext(r.Context())
	vmid, err := strconv.Atoi(ps.ByName("id"))
	if err != nil || vmid <= 0 {
		errBadRequest(w, "invalid vm id")
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

	allVMs, err := proxmox.GetVMsResty(ctx, client)
	if err != nil {
		errInternal(w)
		return
	}

	var targetNode string
	for _, vm := range allVMs {
		if vm.VMID == vmid && hasTag(vm.Tags, "pvmss") {
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
		poolVMIDs := fetchPoolVMIDs(ctx, client, "pvmss_"+username)
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
		if strings.EqualFold(trimmed, "pvmss") {
			continue
		}
		if strings.Contains(strings.ToLower(trimmed), sub) {
			return true
		}
	}
	return false
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
