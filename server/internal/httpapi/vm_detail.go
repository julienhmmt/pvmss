package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
)

// VmDetail serves the four VM-detail endpoints, all gated by the same
// vm.Resolve() (FR-001, SC-005): GET /vms/:cluster/:vmid (detail),
// POST /vms/:cluster/:vmid/actions (power), DELETE /vms/:cluster/:vmid (delete),
// PATCH /vms/:cluster/:vmid (rename/description). 403/404 semantics are
// byte-identical across all four (contracts behavioural rule).
type VmDetail struct {
	projection *inventory.Projection
	auth       *Auth
	writer     cluster.Writer
	store      *store.Store
	refresher  vm.IndexRefresher
	log        *slog.Logger
}

// NewVmDetail creates the handler. The writer is the cluster.Writer (separate
// from the read Client — constitution IV); the refresher rebuilds the Index
// after a write (FR-010).
func NewVmDetail(projection *inventory.Projection, authHandler *Auth, writer cluster.Writer, st *store.Store, refresher vm.IndexRefresher, log *slog.Logger) *VmDetail {
	return &VmDetail{projection: projection, auth: authHandler, writer: writer, store: st, refresher: refresher, log: log}
}

type vmDetailDTO struct {
	VMID          int      `json:"vmid"`
	Name          string   `json:"name"`
	Node          string   `json:"node"`
	Pool          string   `json:"pool"`
	Status        string   `json:"status"`
	Tags          []string `json:"tags"`
	CPUCores      int      `json:"cpuCores"`
	MemoryTotal   int64    `json:"memoryTotal"`
	DiskTotal     int64    `json:"diskTotal"`
	UptimeSeconds int64    `json:"uptimeSeconds,omitempty"`
	Description   string   `json:"description,omitempty"`
}

type actionRequest struct {
	Action string `json:"action"`
}

type actionResponse struct {
	Status string `json:"status"`
}

type deleteResponse struct {
	Status string `json:"status"`
}

type patchRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (h *VmDetail) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.handleGet(w, r)
	case http.MethodPost:
		h.handleAction(w, r)
	case http.MethodDelete:
		h.handleDelete(w, r)
	case http.MethodPatch:
		h.handlePatch(w, r)
	default:
		w.Header().Set("Allow", "GET, POST, DELETE, PATCH")
		h.writeDetailError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

// handleGet serves GET /vms/:cluster/:vmid — the detail view (US1). Calls
// Resolve and encodes the Entity (FR-005).
func (h *VmDetail) handleGet(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeDetailError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	clusterName, vmid, ok := h.parsePath(r)
	if !ok {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", "invalid VM path")
		return
	}
	index := h.projection.Load()
	if index == nil {
		h.writeDetailError(w, http.StatusServiceUnavailable, "inventory_not_ready", "inventory has not been populated yet")
		return
	}
	entity, err := vm.Resolve(index, identity, clusterName, vmid)
	if err != nil {
		h.writeResolveError(w, err)
		return
	}
	h.writeEntity(w, entity)
}

// handleAction serves POST /vms/:cluster/:vmid/actions (US2, closes S01).
// The request body carries only {"action": Kind} — no node field exists in
// the schema, so there is nothing to forge (S01 root cause, structurally closed).
func (h *VmDetail) handleAction(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeDetailError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	clusterName, vmid, ok := h.parsePath(r)
	if !ok {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", "invalid VM path")
		return
	}
	var req actionRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	req.Action = strings.TrimSpace(req.Action)
	if !vm.IsValidAction(req.Action) {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_action", fmt.Sprintf("unknown action %q", req.Action))
		return
	}
	index := h.projection.Load()
	if index == nil {
		h.writeDetailError(w, http.StatusServiceUnavailable, "inventory_not_ready", "inventory has not been populated yet")
		return
	}
	if err := vm.Action(r.Context(), index, identity, clusterName, vmid, req.Action, h.writer, h.store, h.refresher); err != nil {
		h.writeActionError(w, err)
		return
	}
	h.writeJSONStatus(w, http.StatusOK, actionResponse{Status: "accepted"})
}

// handleDelete serves DELETE /vms/:cluster/:vmid (US3). Same Resolve() gate.
func (h *VmDetail) handleDelete(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeDetailError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	clusterName, vmid, ok := h.parsePath(r)
	if !ok {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", "invalid VM path")
		return
	}
	index := h.projection.Load()
	if index == nil {
		h.writeDetailError(w, http.StatusServiceUnavailable, "inventory_not_ready", "inventory has not been populated yet")
		return
	}
	if err := vm.Delete(r.Context(), index, identity, clusterName, vmid, h.writer, h.store, h.refresher); err != nil {
		h.writeActionError(w, err)
		return
	}
	h.writeJSONStatus(w, http.StatusOK, deleteResponse{Status: "deleted"})
}

// handlePatch serves PATCH /vms/:cluster/:vmid (US4). Accepts name and/or
// description; at least one must be present. Name is validated as a hostname
// before Resolve is called (constitution XIII: malformed input rejected first).
func (h *VmDetail) handlePatch(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeDetailError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}
	clusterName, vmid, ok := h.parsePath(r)
	if !ok {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", "invalid VM path")
		return
	}
	var req patchRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Description = strings.TrimSpace(req.Description)
	index := h.projection.Load()
	if index == nil {
		h.writeDetailError(w, http.StatusServiceUnavailable, "inventory_not_ready", "inventory has not been populated yet")
		return
	}
	if err := vm.Patch(r.Context(), index, identity, clusterName, vmid, req.Name, req.Description, h.writer, h.store, h.refresher); err != nil {
		h.writePatchError(w, err)
		return
	}
	// Re-resolve from the refreshed projection to return the updated Entity
	// (contracts: PATCH 200 returns the updated Entity, same shape as GET).
	refreshed := h.projection.Load()
	if refreshed == nil {
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	entity, err := vm.Resolve(refreshed, identity, clusterName, vmid)
	if err != nil {
		// The write succeeded but the re-resolve failed (e.g. a race deleted
		// the VM between the patch and the re-read). Return a generic success
		// rather than a confusing 404 after a 200-worthy write.
		h.log.Error("post-patch re-resolve failed", "component", "httpapi", "vmid", vmid, "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	h.writeEntity(w, entity)
}

// parsePath extracts :cluster and :vmid from the route pattern
// /api/v1/vms/{cluster}/{vmid}[...]. Returns ok=false if vmid is not a valid int.
func (h *VmDetail) parsePath(r *http.Request) (string, int, bool) {
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

func (h *VmDetail) writeEntity(w http.ResponseWriter, entity vm.Entity) {
	dto := vmDetailDTO{
		VMID:        entity.VMID,
		Name:        entity.Name,
		Node:        entity.Node,
		Pool:        entity.Pool,
		Status:      string(entity.Status),
		Tags:        entity.Tags,
		CPUCores:    entity.CPUCores,
		MemoryTotal: entity.MemoryTotal,
		DiskTotal:   entity.DiskTotal,
		Description: entity.Description,
	}
	if entity.Uptime > 0 {
		dto.UptimeSeconds = int64(entity.Uptime.Seconds())
	}
	h.writeJSONStatus(w, http.StatusOK, dto)
}

func (h *VmDetail) writeJSONStatus(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		h.log.Error("failed to marshal response", "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}
	if err := writeJSON(w, status, body); err != nil {
		h.log.Error("failed to write response", "component", "httpapi", "error", err)
	}
}

// writeResolveError maps vm.Resolve errors to HTTP statuses. 403 and 404 are
// byte-identical in shape across all four endpoints (contracts).
func (h *VmDetail) writeResolveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeDetailError(w, http.StatusForbidden, "forbidden", "not your VM")
	case errors.Is(err, vm.ErrNotFound):
		h.writeDetailError(w, http.StatusNotFound, "not_found", "VM not found")
	default:
		h.log.Error("unexpected resolve error", "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

// writeActionError maps vm.Action / vm.Delete errors to HTTP statuses.
func (h *VmDetail) writeActionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeDetailError(w, http.StatusForbidden, "forbidden", "not your VM")
	case errors.Is(err, vm.ErrNotFound):
		h.writeDetailError(w, http.StatusNotFound, "not_found", "VM not found")
	case errors.Is(err, vm.ErrActionRejected):
		h.writeDetailError(w, http.StatusBadRequest, "invalid_action", err.Error())
	case errors.Is(err, cluster.ErrNotFound):
		h.log.Error("cluster writer: VM not found after Resolve", "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusBadGateway, "cluster_error", "cluster rejected the request")
	case errors.Is(err, cluster.ErrUnreachable):
		h.writeDetailError(w, http.StatusBadGateway, "cluster_unreachable", "cluster is not reachable")
	default:
		h.log.Error("vm action failed", "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

// writePatchError maps vm.Patch errors to HTTP statuses.
func (h *VmDetail) writePatchError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeDetailError(w, http.StatusForbidden, "forbidden", "not your VM")
	case errors.Is(err, vm.ErrNotFound):
		h.writeDetailError(w, http.StatusNotFound, "not_found", "VM not found")
	case errors.Is(err, vm.ErrInvalidName):
		h.writeDetailError(w, http.StatusBadRequest, "invalid_name", "name must be a valid hostname (lowercase alphanumeric and hyphen, no leading/trailing hyphen, max 63 chars)")
	case errors.Is(err, vm.ErrEmptyPatch):
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", "at least one of name or description is required")
	case errors.Is(err, vm.ErrDescriptionTooLong):
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", fmt.Sprintf("description exceeds %d characters", vm.MaxDescriptionLength))
	case errors.Is(err, cluster.ErrNotFound):
		h.log.Error("cluster writer: VM not found after Resolve", "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusBadGateway, "cluster_error", "cluster rejected the request")
	default:
		h.log.Error("vm patch failed", "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func (h *VmDetail) writeDetailError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Error("failed to write error response", "component", "httpapi", "code", code, "error", err)
	}
}
