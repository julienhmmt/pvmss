package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strconv"
	"strings"
)

// VMDetail serves the four VM-detail endpoints, all gated by the same
// vm.Resolve() (FR-001, SC-005): GET /vms/:cluster/:vmid (detail),
// POST /vms/:cluster/:vmid/actions (power), DELETE /vms/:cluster/:vmid (delete),
// PATCH /vms/:cluster/:vmid (rename/description). 403/404 semantics are
// byte-identical across all four (contracts behavioural rule).
type VMDetail struct {
	projection *inventory.Projection
	auth       *Auth
	writer     cluster.Writer
	store      *store.Store
	refresher  vm.IndexRefresher
	policy     *policy.Policy
	log        *slog.Logger
}

// NewVMDetail creates the handler. The writer is the cluster.Writer (separate
// from the read Client — constitution IV); the refresher rebuilds the Index
// after a write (FR-010).
func NewVMDetail(projection *inventory.Projection, authHandler *Auth, writer cluster.Writer, st *store.Store, refresher vm.IndexRefresher, log *slog.Logger, services ...*policy.Policy) *VMDetail {
	var policyService *policy.Policy
	if len(services) > 0 {
		policyService = services[0]
	}

	if policyService == nil && st != nil {
		policyService = policy.New(st, projection, nil)
	}

	return &VMDetail{projection: projection, auth: authHandler, writer: writer, store: st, refresher: refresher, policy: policyService, log: log}
}

type vmDetailDTO struct {
	VMID              int                        `json:"vmid"`
	Name              string                     `json:"name"`
	Node              string                     `json:"node"`
	Pool              string                     `json:"pool"`
	Status            string                     `json:"status"`
	Tags              []string                   `json:"tags"`
	CPUCores          int                        `json:"cpuCores"`
	MemoryTotal       int64                      `json:"memoryTotal"`
	DiskTotal         int64                      `json:"diskTotal"`
	Sockets           int                        `json:"sockets"`
	Cores             int                        `json:"cores"`
	Disks             []cluster.Disk             `json:"disks"`
	CDROM             cluster.CDROMState         `json:"cdrom"`
	NetworkInterfaces []cluster.NetworkInterface `json:"networkInterfaces"`
	UptimeSeconds     int64                      `json:"uptimeSeconds,omitempty"`
	Description       string                     `json:"description,omitempty"`
}

type diskRequest struct {
	Bus     string `json:"bus"`
	Storage string `json:"storage"`
	SizeGB  int    `json:"sizeGB"`
}

type resizeDiskRequest struct {
	SizeGB int `json:"sizeGB"`
}

type cdromRequest struct {
	Action   string `json:"action"`
	ISOVolID string `json:"isoVolId,omitempty"`
}

type networkRequest struct {
	Interfaces []networkInterfaceRequest `json:"interfaces"`
}

type networkInterfaceRequest struct {
	Index    int    `json:"index"`
	Bridge   string `json:"bridge"`
	Model    string `json:"model"`
	VLAN     *int   `json:"vlan"`
	RateMbps *int   `json:"rateMbps"`
}

type hardwareRequest struct {
	Sockets  *int      `json:"sockets"`
	Cores    *int      `json:"cores"`
	MemoryMB *int      `json:"memoryMB"`
	Tags     *[]string `json:"tags"`
}

type hardwareOptionsDTO struct {
	Storages []hardwareStorageDTO `json:"storages"`
	Bridges  []hardwareBridgeDTO  `json:"bridges"`
	ISOs     []hardwareISODTO     `json:"isos"`
	Limits   vmLimitsDTO          `json:"limits"`
}

type hardwareStorageDTO struct {
	Node    string `json:"node"`
	Storage string `json:"storage"`
	Type    string `json:"type"`
}

type hardwareBridgeDTO struct {
	Node   string `json:"node"`
	Bridge string `json:"bridge"`
}

type hardwareISODTO struct {
	VolID     string `json:"volId"`
	Node      string `json:"node"`
	Storage   string `json:"storage"`
	Name      string `json:"name"`
	SizeBytes int64  `json:"sizeBytes"`
}

type vmLimitsDTO struct {
	MaxSockets        int            `json:"maxSockets"`
	MaxCores          int            `json:"maxCores"`
	MaxMemoryMB       int            `json:"maxMemoryMB"`
	MaxDiskPerVMGB    int            `json:"maxDiskPerVMGB"`
	MaxNetworkCards   int            `json:"maxNetworkCards"`
	RemainingBusSlots map[string]int `json:"remainingBusSlots"`
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

func (h *VMDetail) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/hardware-options") {
		h.handleHardwareOptions(w, r)
		return
	}

	if strings.HasSuffix(r.URL.Path, "/disks") || r.PathValue("diskKey") != "" {
		h.handleDisk(w, r)
		return
	}

	if strings.HasSuffix(r.URL.Path, "/cdrom") {
		h.handleCDROM(w, r)
		return
	}

	if strings.HasSuffix(r.URL.Path, "/network") {
		h.handleNetwork(w, r)
		return
	}

	if strings.HasSuffix(r.URL.Path, "/hardware") {
		h.handleHardware(w, r)
		return
	}

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
func (h *VMDetail) handleGet(w http.ResponseWriter, r *http.Request) {
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
func (h *VMDetail) handleAction(w http.ResponseWriter, r *http.Request) {
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
func (h *VMDetail) handleDelete(w http.ResponseWriter, r *http.Request) {
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
func (h *VMDetail) handlePatch(w http.ResponseWriter, r *http.Request) {
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

//nolint:gocyclo // one handler owns the shared Resolve/catalog setup for three disk verbs
func (h *VMDetail) handleDisk(w http.ResponseWriter, r *http.Request) {
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

	resources, err := catalog.ApprovedResources(r.Context(), h.store, clusterName)
	if err != nil {
		h.log.Error("read hardware catalog failed", "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", "internal server error")

		return
	}

	deps := vm.DiskDependencies{
		Index:       index,
		Actor:       identity,
		ClusterName: clusterName,
		VMID:        vmid,
		Writer:      h.writer,
		Resources:   resources,
		Policy:      h.policy,
		Audit:       h.store,
		Refresher:   h.refresher,
	}

	switch r.Method {
	case http.MethodPost:
		var request diskRequest
		if err := decodeJSON(w, r, &request); err != nil {
			h.writeDetailError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
			return
		}

		disk, err := vm.AddDisk(r.Context(), deps, cluster.DiskBus(request.Bus), request.Storage, request.SizeGB)
		if err != nil {
			h.writeDiskError(w, err)
			return
		}

		h.writeJSONStatus(w, http.StatusOK, disk)
	case http.MethodPut:
		var request resizeDiskRequest
		if err := decodeJSON(w, r, &request); err != nil {
			h.writeDetailError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
			return
		}

		if err := vm.ResizeDisk(r.Context(), deps, r.PathValue("diskKey"), request.SizeGB); err != nil {
			h.writeDiskError(w, err)
			return
		}

		refreshed := h.projection.Load()
		if refreshed == nil {
			h.writeDetailError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		entity, err := vm.Resolve(refreshed, identity, clusterName, vmid)
		if err != nil {
			h.writeResolveError(w, err)
			return
		}

		for _, disk := range entity.Disks {
			if disk.Key == r.PathValue("diskKey") {
				h.writeJSONStatus(w, http.StatusOK, disk)
				return
			}
		}

		h.writeDiskError(w, vm.ErrDiskNotFound)
	case http.MethodDelete:
		if err := vm.DeleteDisk(r.Context(), deps, r.PathValue("diskKey")); err != nil {
			h.writeDiskError(w, err)
			return
		}

		h.writeJSONStatus(w, http.StatusOK, deleteResponse{Status: "deleted"})
	default:
		w.Header().Set("Allow", "POST, PUT, DELETE")
		h.writeDetailError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
	}
}

func (h *VMDetail) handleCDROM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		w.Header().Set("Allow", "PATCH")
		h.writeDetailError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")

		return
	}

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

	resources, err := catalog.ApprovedResources(r.Context(), h.store, clusterName)
	if err != nil {
		h.log.Error("read hardware catalog failed", "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", "internal server error")

		return
	}

	var request cdromRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	state, err := vm.SetCDROM(r.Context(), vm.CDROMDependencies{
		Index:       index,
		Actor:       identity,
		ClusterName: clusterName,
		VMID:        vmid,
		Writer:      h.writer,
		Resources:   resources,
		Audit:       h.store,
		Refresher:   h.refresher,
	}, request.Action, request.ISOVolID)
	if err != nil {
		h.writeCDROMError(w, err)
		return
	}

	h.writeJSONStatus(w, http.StatusOK, state)
}

func (h *VMDetail) handleHardware(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", "PUT")
		h.writeDetailError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")

		return
	}

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

	var request hardwareRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	if request.Sockets == nil && request.Cores == nil && request.MemoryMB == nil && request.Tags == nil {
		h.writeDetailError(w, http.StatusBadRequest, "empty_patch", "at least one hardware field is required")
		return
	}

	err = vm.UpdateHardware(r.Context(), vm.HardwareDependencies{
		Index: index, Actor: identity, ClusterName: clusterName, VMID: vmid, Writer: h.writer,
		Policy: h.policy, Audit: h.store, Refresher: h.refresher,
	}, vm.HardwarePatch{Sockets: request.Sockets, Cores: request.Cores, MemoryMB: request.MemoryMB, Tags: request.Tags})
	if err != nil {
		h.writeHardwareError(w, err)
		return
	}

	refreshed := h.projection.Load()
	if refreshed == nil {
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	entity, err := vm.Resolve(refreshed, identity, clusterName, vmid)
	if err != nil {
		h.writeResolveError(w, err)
		return
	}

	h.writeEntity(w, entity)
}

func (h *VMDetail) writeHardwareError(w http.ResponseWriter, err error) {
	if h.writeCommonVMError(w, err) {
		return
	}

	switch {
	case errors.Is(err, vm.ErrEmptyHardwarePatch):
		h.writeDetailError(w, http.StatusBadRequest, "empty_patch", err.Error())
	case errors.Is(err, policy.ErrNodeCapacityExceeded):
		h.writeDetailError(w, http.StatusBadRequest, "capacity_exceeded", err.Error())
	case errors.Is(err, policy.ErrUnavailable):
		h.writeDetailError(w, http.StatusServiceUnavailable, "policy_unavailable", "policy service is not configured")
	case errors.Is(err, vm.ErrHardwareExceedsLimit):
		h.writeDetailError(w, http.StatusBadRequest, "hardware_exceeds_limit", err.Error())
	default:
		h.writeUnhandledVMError(w, "vm hardware operation failed", err)
	}
}

func (h *VMDetail) writeCommonVMError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeDetailError(w, http.StatusForbidden, "forbidden", "not your VM")
	case errors.Is(err, vm.ErrNotFound):
		h.writeDetailError(w, http.StatusNotFound, "not_found", "VM not found")
	default:
		return false
	}

	return true
}

func (h *VMDetail) writeUnhandledVMError(w http.ResponseWriter, message string, err error) {
	h.log.Error(message, "component", "httpapi", "error", err)
	h.writeDetailError(w, http.StatusInternalServerError, "internal_error", "internal server error")
}

func (h *VMDetail) handleNetwork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", "PUT")
		h.writeDetailError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")

		return
	}

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

	resources, err := catalog.ApprovedResources(r.Context(), h.store, clusterName)
	if err != nil {
		h.log.Error("read hardware catalog failed", "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", "internal server error")

		return
	}

	var request networkRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", "invalid request body")
		return
	}

	interfaces := make([]cluster.NetworkInterface, 0, len(request.Interfaces))
	for _, iface := range request.Interfaces {
		interfaces = append(interfaces, cluster.NetworkInterface{
			Index: iface.Index, Bridge: iface.Bridge, Model: iface.Model, VLAN: iface.VLAN, RateMbps: iface.RateMbps,
		})
	}

	updated, err := vm.UpdateNetwork(r.Context(), vm.NetworkDependencies{
		Index: index, Actor: identity, ClusterName: clusterName, VMID: vmid, Writer: h.writer,
		Resources: resources, Policy: h.policy, Audit: h.store, Refresher: h.refresher,
	}, interfaces)
	if err != nil {
		h.writeNetworkError(w, err)
		return
	}

	h.writeJSONStatus(w, http.StatusOK, updated)
}

func (h *VMDetail) writeNetworkError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeDetailError(w, http.StatusForbidden, "forbidden", "not your VM")
	case errors.Is(err, vm.ErrNotFound):
		h.writeDetailError(w, http.StatusNotFound, "not_found", "VM not found")
	case errors.Is(err, vm.ErrBridgeNotApproved):
		h.writeDetailError(w, http.StatusBadRequest, "bridge_not_approved", err.Error())
	case errors.Is(err, vm.ErrNetworkCardsExceedLimit):
		h.writeDetailError(w, http.StatusBadRequest, "network_cards_exceed_limit", err.Error())
	case errors.Is(err, policy.ErrUnavailable):
		h.writeDetailError(w, http.StatusServiceUnavailable, "policy_unavailable", "policy service is not configured")
	case errors.Is(err, vm.ErrInvalidNetworkModel), errors.Is(err, vm.ErrDuplicateNetworkIndex):
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", err.Error())
	default:
		h.log.Error("vm network operation failed", "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func (h *VMDetail) writeCDROMError(w http.ResponseWriter, err error) {
	if h.writeCommonVMError(w, err) {
		return
	}

	switch {
	case errors.Is(err, vm.ErrInvalidCDROMAction):
		h.writeDetailError(w, http.StatusBadRequest, "invalid_action", err.Error())
	case errors.Is(err, vm.ErrISOVolumeNotApproved):
		h.writeDetailError(w, http.StatusBadRequest, "iso_not_approved", err.Error())
	default:
		h.writeUnhandledVMError(w, "vm cdrom operation failed", err)
	}
}

func (h *VMDetail) handleHardwareOptions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		h.writeDetailError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")

		return
	}

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

	resources, err := catalog.ApprovedResources(r.Context(), h.store, clusterName)
	if err != nil {
		h.log.Error("read hardware catalog failed", "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", "internal server error")

		return
	}

	if h.policy == nil {
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", "internal server error")
		return
	}

	gabarit, err := h.policy.Gabarit(r.Context(), clusterName)
	if err != nil {
		h.log.Error("read gabarit failed", "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", "internal server error")

		return
	}

	remaining := map[string]int{}

	for bus, max := range map[cluster.DiskBus]int{
		cluster.DiskBusVirtio: 16,
		cluster.DiskBusSCSI:   31,
		cluster.DiskBusSATA:   6,
		cluster.DiskBusIDE:    3,
	} {
		used := 0

		for _, disk := range entity.Disks {
			if disk.Bus == bus {
				used++
			}
		}

		remaining[string(bus)] = max - used
	}

	h.writeJSONStatus(w, http.StatusOK, hardwareOptionsDTO{
		Storages: hardwareStorages(resources.Storages, index),
		Bridges:  hardwareBridges(resources.Bridges, entity.Node),
		ISOs:     hardwareISOs(resources.ISOs, resources.Storages),
		Limits: vmLimitsDTO{
			MaxSockets:        gabarit.MaxSockets,
			MaxCores:          gabarit.MaxCores,
			MaxMemoryMB:       gabarit.MaxMemoryMB,
			MaxDiskPerVMGB:    gabarit.MaxDiskPerVMGB,
			MaxNetworkCards:   gabarit.MaxNetworkCards,
			RemainingBusSlots: remaining,
		},
	})
}

func hardwareStorages(storages []catalog.Storage, index *inventory.Index) []hardwareStorageDTO {
	result := make([]hardwareStorageDTO, 0, len(storages))
	for _, storage := range storages {
		storageType := ""

		for _, available := range index.StoragesByNode[storage.Node] {
			if available.Name == storage.Name {
				storageType = available.Type
				break
			}
		}

		result = append(result, hardwareStorageDTO{Node: storage.Node, Storage: storage.Name, Type: storageType})
	}

	return result
}

func hardwareBridges(bridges []string, node string) []hardwareBridgeDTO {
	result := make([]hardwareBridgeDTO, 0, len(bridges))
	for _, bridge := range bridges {
		result = append(result, hardwareBridgeDTO{Node: node, Bridge: bridge})
	}

	return result
}

func hardwareISOs(isos []catalog.ISO, storages []catalog.Storage) []hardwareISODTO {
	result := make([]hardwareISODTO, 0, len(isos))
	for _, iso := range isos {
		node := ""

		for _, storage := range storages {
			if storage.Name == iso.Storage {
				node = storage.Node
				break
			}
		}

		result = append(result, hardwareISODTO{
			VolID: iso.Storage + ":iso/" + iso.File,
			Node:  node, Storage: iso.Storage, Name: iso.File,
		})
	}

	return result
}

func (h *VMDetail) writeDiskError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeDetailError(w, http.StatusForbidden, "forbidden", "not your VM")
	case errors.Is(err, vm.ErrNotFound):
		h.writeDetailError(w, http.StatusNotFound, "not_found", "VM not found")
	case errors.Is(err, vm.ErrDiskNotFound):
		h.writeDetailError(w, http.StatusNotFound, "disk_not_found", err.Error())
	case errors.Is(err, vm.ErrBootDiskProtected):
		h.writeDetailError(w, http.StatusBadRequest, "boot_disk_protected", "the boot disk cannot be deleted")
	case errors.Is(err, vm.ErrVMNotStopped):
		h.writeDetailError(w, http.StatusBadRequest, "vm_not_stopped", err.Error())
	case errors.Is(err, vm.ErrDiskStorageNotApproved):
		h.writeDetailError(w, http.StatusBadRequest, "storage_not_approved", err.Error())
	case errors.Is(err, policy.ErrUnavailable):
		h.writeDetailError(w, http.StatusServiceUnavailable, "policy_unavailable", "policy service is not configured")
	case errors.Is(err, vm.ErrDiskSizeExceedsLimit):
		h.writeDetailError(w, http.StatusBadRequest, "disk_size_exceeds_limit", err.Error())
	case errors.Is(err, vm.ErrDiskSizeNotGreater):
		h.writeDetailError(w, http.StatusBadRequest, "disk_size_not_greater", err.Error())
	case errors.Is(err, vm.ErrBusFull):
		h.writeDetailError(w, http.StatusBadRequest, "bus_full", err.Error())
	case errors.Is(err, cluster.ErrNotFound):
		h.writeDetailError(w, http.StatusBadGateway, "cluster_error", "cluster rejected the request")
	default:
		h.log.Error("vm disk operation failed", "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

// parsePath extracts :cluster and :vmid from the route pattern
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

func (h *VMDetail) writeEntity(w http.ResponseWriter, entity vm.Entity) {
	dto := vmDetailDTO{
		VMID:              entity.VMID,
		Name:              entity.Name,
		Node:              entity.Node,
		Pool:              entity.Pool,
		Status:            string(entity.Status),
		Tags:              entity.Tags,
		CPUCores:          entity.CPUCores,
		MemoryTotal:       entity.MemoryTotal,
		DiskTotal:         entity.DiskTotal,
		Sockets:           entity.Sockets,
		Cores:             entity.Cores,
		Disks:             entity.Disks,
		CDROM:             entity.CDROM,
		NetworkInterfaces: entity.NetworkInterfaces,
		Description:       entity.Description,
	}
	if entity.Uptime > 0 {
		dto.UptimeSeconds = int64(entity.Uptime.Seconds())
	}

	h.writeJSONStatus(w, http.StatusOK, dto)
}

func (h *VMDetail) writeJSONStatus(w http.ResponseWriter, status int, value any) {
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
func (h *VMDetail) writeResolveError(w http.ResponseWriter, err error) {
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
func (h *VMDetail) writeActionError(w http.ResponseWriter, err error) {
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
func (h *VMDetail) writePatchError(w http.ResponseWriter, err error) {
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

func (h *VMDetail) writeDetailError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Error("failed to write error response", "component", "httpapi", "code", code, "error", err)
	}
}
