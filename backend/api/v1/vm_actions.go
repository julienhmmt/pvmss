package apiv1

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

var allowedActions = map[string]bool{
	"start":    true,
	"stop":     true,
	"shutdown": true,
	"reboot":   true,
	"reset":    true,
}

// VMActionHandler handles VM lifecycle action requests.
type VMActionHandler struct {
	state state.StateManager
}

// MakeVMActionHandler creates a new VMActionHandler.
func MakeVMActionHandler(s state.StateManager) *VMActionHandler {
	return &VMActionHandler{state: s}
}

// VMAction handles POST /api/v1/vms/:id/action.
// Body: {"action":"start|stop|shutdown|reboot|reset", "node":"nodename"}
func (h *VMActionHandler) VMAction(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	vmid, err := strconv.Atoi(ps.ByName("id"))
	if err != nil || vmid <= 0 {
		errBadRequest(w, "invalid vm id")
		return
	}

	var req VMActionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errBadRequest(w, "invalid JSON body")
		return
	}
	if !allowedActions[req.Action] {
		errBadRequest(w, "action must be one of: start, stop, shutdown, reboot, reset")
		return
	}
	if req.Node == "" {
		errBadRequest(w, "node is required")
		return
	}

	if h.state == nil || h.state.IsOfflineMode() {
		errOffline(w)
		return
	}

	client, err := proxmox.MakeRestyClientFromEnv(60 * time.Second)
	if err != nil {
		logger.Get().Error().Err(err).Msg("api/v1: failed to create resty client for VMAction")
		errInternal(w)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	logger.VMEvent("api_vm_action", vmid, req.Node).
		Str("username", usernameFromCtx(r)).
		Str("action", req.Action).
		Msg("VM action requested via API")

	upid, err := proxmox.VMActionResty(ctx, client, req.Node, strconv.Itoa(vmid), req.Action)
	if err != nil {
		logger.Get().Error().Err(err).Int("vmid", vmid).Str("action", req.Action).Msg("api/v1: VMActionResty failed")
		writeError(w, http.StatusBadGateway, "proxmox_error", "Failed to perform VM action")
		return
	}

	writeJSON(w, VMActionResponse{Success: true, TaskID: upid})
}
