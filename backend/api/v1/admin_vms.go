package apiv1

import (
	"encoding/json"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/proxmox"
	"pvmss/state"
)

// hasPVMSSTag reports whether the semicolon-separated tags string contains "pvmss".
func hasPVMSSTag(tags string) bool {
	for _, tag := range strings.Split(tags, ";") {
		if strings.TrimSpace(tag) == "pvmss" {
			return true
		}
	}
	return false
}

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
		errInternal(w)
		return
	}
	vms, err := proxmox.GetVMsResty(r.Context(), restyClient)
	if err != nil {
		errInternal(w)
		return
	}
	result := make([]AdminVMResponse, 0, len(vms))
	for _, vm := range vms {
		if !hasPVMSSTag(vm.Tags) {
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
	slices.SortFunc(result, func(a, b AdminVMResponse) int { return a.VMID - b.VMID })
	writeJSON(w, result)
}

// DeleteVM handles DELETE /api/v1/admin/vms/:id.
// Permanently deletes the VM and all its associated disk files from Proxmox.
func (h *AdminVMsAPIHandler) DeleteVM(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		errOffline(w)
		return
	}
	ps := r.Context().Value(httprouter.ParamsKey).(httprouter.Params)
	vmid, err := strconv.Atoi(ps.ByName("id"))
	if err != nil || vmid <= 0 {
		errBadRequest(w, "invalid VM ID")
		return
	}
	restyClient, err := proxmox.MakeRestyClientFromEnv(30 * time.Second)
	if err != nil {
		errInternal(w)
		return
	}
	vms, err := proxmox.GetVMsResty(r.Context(), restyClient)
	if err != nil {
		errInternal(w)
		return
	}
	var node string
	for _, vm := range vms {
		if vm.VMID == vmid && hasPVMSSTag(vm.Tags) {
			node = vm.Node
			break
		}
	}
	if node == "" {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}
	if err := proxmox.DeleteVMResty(r.Context(), restyClient, node, vmid); err != nil {
		writeError(w, http.StatusInternalServerError, "delete_failed", err.Error())
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
	ps := r.Context().Value(httprouter.ParamsKey).(httprouter.Params)
	vmid := ps.ByName("id")
	if vmid == "" {
		errBadRequest(w, "missing VM ID")
		return
	}
	var req AdminVMActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid JSON body")
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
		errInternal(w)
		return
	}
	node := req.Node
	if node == "" {
		// node not provided by client: look it up from the VM list
		vms, err := proxmox.GetVMsResty(r.Context(), restyClient)
		if err != nil {
			errInternal(w)
			return
		}
		vmidInt, _ := strconv.Atoi(vmid)
		for _, vm := range vms {
			if vm.VMID == vmidInt {
				node = vm.Node
				break
			}
		}
		if node == "" {
			errBadRequest(w, "VM not found")
			return
		}
	}
	taskID, err := proxmox.VMActionResty(r.Context(), restyClient, node, vmid, req.Action)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "vm_action_failed", err.Error())
		return
	}
	writeJSON(w, VMActionResponse{Success: true, TaskID: taskID})
}
