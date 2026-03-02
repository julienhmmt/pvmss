package apiv1

import (
	"context"
	"net/http"
	"strings"
	"time"

	"pvmss/proxmox"
	"pvmss/state"
)

// ProfileHandler serves per-user VM data.
type ProfileHandler struct {
	state state.StateManager
}

// NewProfileHandler creates a new ProfileHandler.
func NewProfileHandler(s state.StateManager) *ProfileHandler {
	return &ProfileHandler{state: s}
}

// vmBelongsToUser returns true when the VM's tags contain the username.
// Tags are semicolon-separated (e.g. "pvmss;alice;prod").
func vmBelongsToUser(vm proxmox.VM, username string) bool {
	if username == "" {
		return false
	}
	lower := strings.ToLower(username)
	for _, tag := range strings.Split(vm.Tags, ";") {
		if strings.TrimSpace(strings.ToLower(tag)) == lower {
			return true
		}
	}
	return false
}

// ListMyVMs handles GET /api/v1/profile/vms — returns VMs owned by the requesting user.
func (h *ProfileHandler) ListMyVMs(w http.ResponseWriter, r *http.Request) {
	if h.state != nil && h.state.IsOfflineMode() {
		writeJSON(w, VMListResponse{VMs: []VMSummary{}, Total: 0})
		return
	}

	client, err := restyClient()
	if err != nil {
		errInternal(w)
		return
	}

	username := usernameFromCtx(r)

	ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
	defer cancel()

	vms, err := proxmox.GetVMsResty(ctx, client)
	if err != nil {
		errInternal(w)
		return
	}

	owned := make([]VMSummary, 0)
	for _, vm := range vms {
		if vmBelongsToUser(vm, username) {
			owned = append(owned, vmToSummary(vm))
		}
	}

	writeJSON(w, VMListResponse{VMs: owned, Total: len(owned)})
}
