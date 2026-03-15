package apiv1

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"

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
	writeJSON(w, result)
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
	// Find VM node
	vms, err := proxmox.GetVMsResty(r.Context(), restyClient)
	if err != nil {
		errInternal(w)
		return
	}
	vmidInt, _ := strconv.Atoi(vmid)
	var node string
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
	taskID, err := proxmox.VMActionResty(r.Context(), restyClient, node, vmid, req.Action)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "vm_action_failed", err.Error())
		return
	}
	writeJSON(w, VMActionResponse{Success: true, TaskID: taskID})
}
