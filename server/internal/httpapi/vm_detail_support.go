package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strconv"
	"strings"
	"time"
)

// parsePath extracts :cluster and :vmid from the route pattern
//
// /api/v1/vms/{cluster}/{vmid}[...]. Returns ok=false if vmid is not a valid int.
func (h *VMDetail) parsePath(r *http.Request) (string, int, bool) {
	clusterName := r.PathValue("cluster")
	if clusterName == "" {
		return "", 0, false
	}

	vmid, err := parseIntPathValue(r, "vmid")
	if err != nil {
		return "", 0, false
	}

	return clusterName, vmid, true
}

// parseIntPathValue reads a path parameter as a positive integer.
func parseIntPathValue(r *http.Request, key string) (int, error) {
	raw := r.PathValue(key)
	if raw == "" {
		return 0, fmt.Errorf("missing path value %q", key)
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 {
		return 0, fmt.Errorf("invalid path value %q: %q", key, raw)
	}

	return value, nil
}

func (h *VMDetail) writeEntity(w http.ResponseWriter, r *http.Request, entity vm.Entity) {
	dto := vmDetailDTO{
		Cluster:           entity.Cluster,
		VMID:              entity.VMID,
		Name:              entity.Name,
		Node:              entity.Node,
		Pool:              entity.Pool,
		Status:            string(entity.Status),
		Tags:              entity.Tags,
		OSType:            entity.OSType,
		CPUCores:          entity.CPUCores,
		MemoryTotal:       entity.MemoryTotal,
		DiskTotal:         entity.DiskTotal,
		Sockets:           entity.Sockets,
		Cores:             entity.Cores,
		Disks:             entity.Disks,
		CDROM:             entity.CDROM,
		BootOrder:         entity.BootOrder,
		NetworkInterfaces: entity.NetworkInterfaces,
		HasSerial:         entity.HasSerial,
		Description:       entity.Description,
		DescriptionHTML:   renderMarkdownToHTML(entity.Description),
	}
	if entity.Uptime > 0 {
		dto.UptimeSeconds = int64(entity.Uptime.Seconds())
	}

	// The detail DTO carries the live Proxmox lock (best-effort
	// a failed live read must not fail the whole detail) so the page can show
	// the lock badge; the convergence loop keeps it fresh after actions.
	if reader := h.statusReaderFor(entity.Cluster); reader != nil {
		if live, err := reader.VMStatus(r.Context(), entity.Node, entity.VMID); err == nil {
			dto.Lock = live.Lock
		}
	}

	h.fillBaselineState(r.Context(), entity, &dto)

	h.fillGuestAgent(r.Context(), entity, &dto)

	h.writeJSONStatus(w, http.StatusOK, dto)
}

// fillBaselineState carries the baseline delivery state for image-mode VMs so
// the page can report it (best-effort - a store failure must not fail the
// whole detail). A "not_delivered" value is a creation-time snapshot: a
// document attached since (snippets enabled later) supersedes it.
func (h *VMDetail) fillBaselineState(ctx context.Context, entity vm.Entity, dto *vmDetailDTO) {
	if h.store == nil {
		return
	}

	state, found, err := h.store.GetBaselineState(ctx, entity.Cluster, entity.VMID)
	if err != nil || !found {
		return
	}

	dto.BaselineState = state.State
	dto.BaselineError = state.Error

	if state.State == vm.BaselineStateNotDelivered {
		if _, attached, err := h.store.GetVMCloudInitDocument(ctx, entity.Cluster, entity.VMID); err == nil && attached {
			dto.BaselineState, dto.BaselineError = "", ""
		}
	}
}

// fillGuestAgent probes the guest agent for a running VM's live IPs and
// reports the channel's state on the DTO. The projection carries config-only
// interfaces - parseNetworkInterfaces deliberately skips the per-VM agent
// round trip - so a running VM's live IPs are asked of the guest agent here
// instead (best-effort, like Lock). The config's agent= flag already tells
// us when probing is pointless; a failed probe on an enabled channel is
// itself the communication test.
func (h *VMDetail) fillGuestAgent(ctx context.Context, entity vm.Entity, dto *vmDetailDTO) {
	if entity.Status != cluster.VMRunning {
		return
	}

	if !entity.Agent {
		dto.GuestAgent = "disabled"
		return
	}

	reader := h.guestNetReaderFor(entity.Cluster)
	if reader == nil {
		return
	}

	guests, err := reader.GuestNetworkInterfaces(ctx, entity.Node, entity.VMID)
	if err != nil {
		dto.GuestAgent = "unreachable"
		return
	}

	dto.GuestAgent = "ok"
	mergeGuestIPs(dto.NetworkInterfaces, guests)
}

// mergeGuestIPs copies each guest-reported IP list onto the configured NIC
// with the same MAC. The agent reports lowercase addresses while the Proxmox
// config stores them uppercase, so the correlation is case-insensitive.
// Guest interfaces with no matching NIC (lo, hot-plugged) are dropped.
func mergeGuestIPs(nics []cluster.NetworkInterface, guests []cluster.GuestInterface) {
	byMAC := make(map[string][]string, len(guests))
	for _, guest := range guests {
		byMAC[strings.ToLower(guest.MAC)] = guest.IPAddresses
	}

	for i := range nics {
		if ips, ok := byMAC[strings.ToLower(nics[i].MAC)]; ok {
			nics[i].IPAddresses = ips
		}
	}
}

func (h *VMDetail) writeJSONStatus(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		SetErrorMsg(w, "failed to marshal response", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	if err := writeJSON(w, status, body); err != nil {
		h.log.Warn("failed to write response", "component", "httpapi", "error", err)
	}
}

// handleAudit serves GET /vms/:cluster/:vmid/audit - paginated, VM-scoped audit trail.
func (h *VMDetail) handleAudit(w http.ResponseWriter, r *http.Request) {
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

	// Verify ownership via Resolve (same gate as GET detail).
	index, ok := h.index(w, clusterName)
	if !ok {
		return
	}

	if _, err := vm.Resolve(index, identity, clusterName, vmid); err != nil {
		h.writeResolveError(w, err)
		return
	}

	page, ok := h.parseAuditPage(r)
	if !ok {
		return
	}

	filter := store.AuditFilter{
		Cluster:  clusterName,
		VMID:     &vmid,
		Page:     page,
		PageSize: 20,
	}
	if actor := r.URL.Query().Get("actor"); actor != "" {
		filter.Actor = actor
	}

	if action := r.URL.Query().Get("action"); action != "" {
		filter.Action = action
	}

	result, err := h.store.ListAuditLog(r.Context(), filter)
	if err != nil {
		SetErrorMsg(w, "vm audit list failed", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	items := make([]auditEntryDTO, len(result.Items))
	for i, e := range result.Items {
		items[i] = auditEntryDTO{
			ID:        e.ID,
			Actor:     e.Actor,
			Cluster:   e.Cluster,
			VMID:      e.VMID,
			Action:    e.Action,
			Timestamp: e.Timestamp.Format(time.RFC3339Nano),
		}
	}

	h.writeJSONStatus(w, http.StatusOK, auditPageDTO{
		Items:    items,
		Total:    result.Total,
		Page:     result.Page,
		PageSize: result.PageSize,
	})
}

func (h *VMDetail) parseAuditPage(r *http.Request) (int, bool) {
	raw := r.URL.Query().Get("page")
	if raw == "" {
		return 1, true
	}

	page, err := strconv.Atoi(raw)
	if err != nil || page < 1 {
		return 0, false
	}

	return page, true
}

// writeResolveError maps vm.Resolve errors to HTTP statuses. 403 and 404 are
// byte-identical in shape across all four endpoints.
func (h *VMDetail) writeResolveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeDetailError(w, http.StatusForbidden, "forbidden", msgNotYourVM)
	case errors.Is(err, vm.ErrNotFound):
		h.writeDetailError(w, http.StatusNotFound, "not_found", msgVMNotFound)
	default:
		SetErrorMsg(w, "unexpected resolve error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
	}
}

// writeActionError maps vm.Action / vm.Delete errors to HTTP statuses.
func (h *VMDetail) writeActionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeDetailError(w, http.StatusForbidden, "forbidden", msgNotYourVM)
	case errors.Is(err, vm.ErrNotFound):
		h.writeDetailError(w, http.StatusNotFound, "not_found", msgVMNotFound)
	case errors.Is(err, vm.ErrActionRejected):
		h.writeDetailError(w, http.StatusBadRequest, "invalid_action", err.Error())
	case errors.Is(err, cluster.ErrNotFound):
		SetErrorMsg(w, "cluster writer: VM not found after Resolve", err)
		h.writeDetailError(w, http.StatusBadGateway, "cluster_error", msgClusterRejected)
	case errors.Is(err, cluster.ErrUnreachable):
		h.writeDetailError(w, http.StatusBadGateway, "cluster_unreachable", "cluster is not reachable")
	case errors.Is(err, cluster.ErrInvalidStateTransition):
		h.writeDetailError(w, http.StatusConflict, "invalid_state_transition", err.Error())
	case errors.Is(err, cluster.ErrVMRunning):
		h.writeDetailError(w, http.StatusConflict, "vm_running", msgVMRunning)
	case errors.Is(err, cluster.ErrClusterRejected):
		code, message, _ := clusterRejectionResponse(err)
		h.writeDetailError(w, http.StatusBadGateway, code, message)
	default:
		SetErrorMsg(w, "vm action failed", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
	}
}

// writePatchError maps vm.Patch errors to HTTP statuses.
func (h *VMDetail) writePatchError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeDetailError(w, http.StatusForbidden, "forbidden", msgNotYourVM)
	case errors.Is(err, vm.ErrNotFound):
		h.writeDetailError(w, http.StatusNotFound, "not_found", msgVMNotFound)
	case errors.Is(err, vm.ErrInvalidName):
		h.writeDetailError(w, http.StatusBadRequest, "invalid_name", "name must be a valid hostname (lowercase alphanumeric and hyphen, no leading/trailing hyphen, max 63 chars)")
	case errors.Is(err, vm.ErrEmptyPatch):
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", "at least one of name or description is required")
	case errors.Is(err, vm.ErrDescriptionTooLong):
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("description exceeds %d characters", vm.MaxDescriptionLength))
	case errors.Is(err, cluster.ErrNotFound):
		SetErrorMsg(w, "cluster writer: VM not found after Resolve", err)
		h.writeDetailError(w, http.StatusBadGateway, "cluster_error", msgClusterRejected)
	case errors.Is(err, cluster.ErrClusterRejected):
		code, message, _ := clusterRejectionResponse(err)
		h.writeDetailError(w, http.StatusBadGateway, code, message)
	default:
		SetErrorMsg(w, "vm patch failed", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
	}
}

func (h *VMDetail) writeDetailError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Warn("failed to write error response", "component", "httpapi", "code", code, "error", err)
	}
}
