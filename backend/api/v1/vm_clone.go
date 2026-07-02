package apiv1

import (
	"context"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"pvmss/constants"
	"pvmss/logger"
	"pvmss/proxmox"
)

// VMCloneRequest is the body for POST /api/v1/vms/:id/clone.
type VMCloneRequest struct {
	Name       string `json:"name"`        // required, hostname of the clone
	Full       *bool  `json:"full"`        // nil → default full=true
	TargetNode string `json:"target_node"` // optional; source node if empty
	Storage    string `json:"storage"`     // optional; target storage for full clone
}

// VMCloneResponse is returned after a clone task is accepted.
type VMCloneResponse struct {
	VMID int    `json:"vmid"`
	Task string `json:"task,omitempty"`
}

// isValidHostname reports whether name is a valid single-label or dotted DNS
// hostname (letters, digits, hyphen, dot; no leading/trailing hyphen per label).
// Proxmox rejects invalid names anyway, but failing fast gives a clearer error.
func isValidHostname(name string) bool {
	if name == "" || len(name) > 63 {
		return false
	}
	for _, label := range strings.Split(name, ".") {
		if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
				return false
			}
		}
	}
	return true
}

// CloneVM handles POST /api/v1/vms/:id/clone. It clones a VM the caller owns,
// enforcing the same guards as VM create (quota, node aggregate limits,
// allowlists) before dispatching the asynchronous Proxmox clone task.
func (h *VMDetailsHandler) CloneVM(w http.ResponseWriter, r *http.Request) {
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}

	var req VMCloneRequest
	if !decodeBody(w, r, &req) {
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	if !isValidHostname(req.Name) {
		errBadRequest(w, "invalid name: use a valid hostname (letters, digits, hyphens; max 63 chars)")
		return
	}
	full := true
	if req.Full != nil {
		full = *req.Full
	}

	if h.isOffline() {
		errOffline(w)
		return
	}

	settings := h.state.GetSettings()
	if settings == nil {
		writeError(w, http.StatusInternalServerError, "settings_unavailable", "Settings not available")
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

	// Ownership + quota share a single pool lookup for non-admin callers.
	if !isAdmin {
		poolIDs := fetchPoolVMIDs(ctx, client, constants.PoolPrefix+username)
		if !poolIDs[vmid] {
			writeError(w, http.StatusNotFound, "not_found", "VM not found")
			return
		}
		if settings.MaxVMPerUser > 0 && len(poolIDs) >= settings.MaxVMPerUser {
			writeError(w, http.StatusConflict, "quota_exceeded",
				strconv.Itoa(len(poolIDs))+"/"+strconv.Itoa(settings.MaxVMPerUser)+" VMs — maximum reached")
			return
		}
	}

	// Resolve source node and specs from a single VM-list call.
	vms, err := proxmox.GetVMsResty(ctx, client)
	if err != nil {
		writeAppError(w, err)
		return
	}
	var src *proxmox.VM
	for i := range vms {
		if vms[i].VMID == vmid {
			src = &vms[i]
			break
		}
	}
	if src == nil {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	targetNode := strings.TrimSpace(req.TargetNode)
	if targetNode == "" {
		targetNode = src.Node
	}
	if !nodeAllowed(settings.EnabledNodes, targetNode) {
		errBadRequest(w, "Invalid selection")
		return
	}
	if req.Storage != "" && !storageAllowed(settings.EnabledStorages, req.Storage) {
		errBadRequest(w, "Invalid selection")
		return
	}

	// Node aggregate limits: the clone inherits the source's specs.
	srcCores := src.CPUs
	if srcCores <= 0 {
		srcCores = 1
	}
	srcMemMB := int(src.MaxMem / (1024 * 1024))
	if err := validateNodeAggregateLimits(h.state, targetNode, 1, srcCores, srcMemMB); err != nil {
		writeError(w, http.StatusConflict, nodeLimitCode(err), err.Error())
		return
	}
	committed := false
	defer func() {
		if !committed {
			releaseNodeAggregateReservation(targetNode, 1, srcCores, srcMemMB)
		}
	}()

	newID, err := allocateNextVMID(ctx, h.state, client)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "vmid_error", "Failed to get next VMID")
		return
	}

	pool := ""
	if !isAdmin && username != "" {
		pool = constants.PoolPrefix + username
	}

	upid, err := proxmox.CloneVMResty(ctx, client, src.Node, strconv.Itoa(vmid), proxmox.VMCloneConfig{
		NewID:   newID,
		Name:    req.Name,
		Target:  req.TargetNode,
		Full:    full,
		Storage: req.Storage,
		Pool:    pool,
	})
	if err != nil {
		logger.Get().Error().Err(err).Int("vmid", vmid).Int("newid", newID).Msg("api/v1: VM clone failed")
		writeError(w, http.StatusBadGateway, "proxmox_error", "Failed to clone VM")
		return
	}
	committed = true

	logger.VMEvent("api_vm_clone", newID, targetNode).
		Str("username", username).
		Int("source_vmid", vmid).
		Bool("full", full).
		Str("upid", upid).
		Msg("VM clone requested via API")

	h.state.RequestSnapshotRefresh()

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, VMCloneResponse{VMID: newID, Task: upid})
}

// nodeAllowed reports whether node is permitted given the enabled-nodes
// allowlist. An empty allowlist means no restriction.
func nodeAllowed(enabled []string, node string) bool {
	return len(enabled) == 0 || slices.Contains(enabled, node)
}

// storageAllowed reports whether storage is permitted given the
// enabled-storages allowlist (entries are "node:storage" or "storage").
// An empty allowlist means no restriction.
func storageAllowed(enabled []string, storage string) bool {
	if len(enabled) == 0 {
		return true
	}
	for _, s := range enabled {
		name := s
		if parts := strings.SplitN(s, ":", 2); len(parts) == 2 {
			name = parts[1]
		}
		if name == storage {
			return true
		}
	}
	return false
}
