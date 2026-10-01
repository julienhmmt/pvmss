package httpapi

import (
	"fmt"
	"net/http"
	"pvmss/server/internal/logctx"
	"pvmss/server/internal/vm"
	"strings"
)

// handleGet serves GET /vms/:cluster/:vmid - the detail view. Calls
// Resolve and encodes the Entity.
func (h *VMDetail) handleGet(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeDetailError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	clusterName, vmid, ok := h.parsePath(r)
	if !ok {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidVMPath)
		return
	}

	index, ok := h.index(w, clusterName)
	if !ok {
		return
	}

	entity, err := vm.Resolve(index, identity, clusterName, vmid)
	if err != nil {
		h.writeResolveError(w, err)
		return
	}

	h.writeEntity(w, r, entity)
}

// handleStatus serves GET /vms/:cluster/:vmid/status - the live status read
// (ADR 0001). Unlike handleGet which reads the projection, this reads the
// cluster's live /status/current via VMStatusReader, so the front's converge
// loop sees the real power state immediately after an action, not the
// projection's up-to-30s-stale view. Read-only: never writes the projection.
func (h *VMDetail) handleStatus(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeDetailError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	clusterName, vmid, ok := h.parsePath(r)
	if !ok {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidVMPath)
		return
	}

	statusReader := h.statusReaderFor(clusterName)
	if statusReader == nil {
		h.writeDetailError(w, http.StatusServiceUnavailable, "no_status_reader", "live status reader not configured")
		return
	}

	index, ok := h.index(w, clusterName)
	if !ok {
		return
	}

	entity, err := vm.Resolve(index, identity, clusterName, vmid)
	if err != nil {
		h.writeResolveError(w, err)
		return
	}

	live, err := statusReader.VMStatus(r.Context(), entity.Node, vmid)
	if err != nil {
		SetErrorMsg(w, "live status read failed", err)
		h.writeDetailError(w, http.StatusBadGateway, "cluster_error", "failed to read live status")

		return
	}

	h.writeJSONStatus(w, http.StatusOK, vmLiveStatusDTO{
		Status: string(live.Status),
		Lock:   live.Lock,
		Uptime: int64(live.Uptime.Seconds()),
	})
}

// vmLiveStatusDTO is the response shape for GET /vms/:cluster/:vmid/status.
type vmLiveStatusDTO struct {
	Status string `json:"status"`
	Lock   string `json:"lock,omitempty"`
	Uptime int64  `json:"uptime"`
}

// handleAction serves POST /vms/:cluster/:vmid/actions.
// The request body carries only {"action": Kind} - no node field exists in
// the schema, so there is nothing to forge (root cause, structurally closed).
func (h *VMDetail) handleAction(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeDetailError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	clusterName, vmid, ok := h.parsePath(r)
	if !ok {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidVMPath)
		return
	}

	var req actionRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}

	req.Action = strings.TrimSpace(req.Action)
	if !vm.IsValidAction(req.Action) {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_action", fmt.Sprintf("unknown action %q", req.Action))
		return
	}

	index, ok := h.index(w, clusterName)
	if !ok {
		return
	}

	writer, ok := h.writerFor(w, clusterName)
	if !ok {
		return
	}

	if err := vm.Action(r.Context(), vm.BulkDeps{
		Actor:        identity,
		Writer:       writer,
		Audit:        h.store,
		Refresher:    h.refresherFor(clusterName),
		StatusReader: h.statusReaderFor(clusterName),
		Force:        req.Force,
	}, index, clusterName, vmid, req.Action); err != nil {
		h.writeActionError(w, err)
		return
	}

	// Refresh the projection once after the action (the caller owns refresh, not Action).
	// Best-effort - the action already succeeded.
	if refresher := h.refresherFor(clusterName); refresher != nil {
		if _, err := refresher.Refresh(r.Context()); err != nil {
			logctx.FromOr(r.Context(), h.log).WarnContext(r.Context(), "post-action refresh failed", "component", "httpapi", "cluster", clusterName, "error", err)
		}
	}

	h.writeJSONStatus(w, http.StatusOK, actionResponse{Status: "accepted"})
}

// handleDelete serves DELETE /vms/:cluster/:vmid. Same Resolve() gate.
// The optional ?force=true query parameter authorizes a force-stop of a running
// VM before the destroy - the UI only sends it after the user has confirmed the
// force-stop in the delete dialog. Without it, a running VM is rejected with 409 (code
// "vm_running") so the client can prompt for confirmation.
func (h *VMDetail) handleDelete(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeDetailError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	clusterName, vmid, ok := h.parsePath(r)
	if !ok {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidVMPath)
		return
	}

	index, ok := h.index(w, clusterName)
	if !ok {
		return
	}

	writer, ok := h.writerFor(w, clusterName)
	if !ok {
		return
	}

	if err := vm.Delete(r.Context(), vm.WriteDeps{Index: index, Actor: identity, ClusterName: clusterName, VMID: vmid, Writer: writer, Audit: h.store, Refresher: h.refresherFor(clusterName), Store: h.store, Log: h.log, Force: r.URL.Query().Get("force") == "true"}); err != nil {
		h.writeActionError(w, err)
		return
	}

	h.writeJSONStatus(w, http.StatusOK, deleteResponse{Status: "deleted"})
}

// handlePatch serves PATCH /vms/:cluster/:vmid. Accepts name and/or
// description; at least one must be present. Name is validated as a hostname
// before Resolve is called (malformed input rejected first).
func (h *VMDetail) handlePatch(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeDetailError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	clusterName, vmid, ok := h.parsePath(r)
	if !ok {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidVMPath)
		return
	}

	var req patchRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)

	index, ok := h.index(w, clusterName)
	if !ok {
		return
	}

	writer, ok := h.writerFor(w, clusterName)
	if !ok {
		return
	}

	if err := vm.Patch(r.Context(), vm.WriteDeps{Index: index, Actor: identity, ClusterName: clusterName, VMID: vmid, Writer: writer, Audit: h.store, Refresher: h.refresherFor(clusterName)}, req.Name, req.Description); err != nil {
		h.writePatchError(w, err)
		return
	}
	// Re-resolve from the refreshed projection to return the updated Entity
	// (contracts: PATCH 200 returns the updated Entity, same shape as GET).
	refreshed, ok := h.index(w, clusterName)
	if !ok {
		return
	}

	entity, err := vm.Resolve(refreshed, identity, clusterName, vmid)
	if err != nil {
		// The write succeeded but the re-resolve failed (e.g. a race deleted
		// the VM between the patch and the re-read). Return a generic success
		// rather than a confusing 404 after a 200-worthy write.
		SetErrorMsg(w, "post-patch re-resolve failed", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	h.writeEntity(w, r, entity)
}
