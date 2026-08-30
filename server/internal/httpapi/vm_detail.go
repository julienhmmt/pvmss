package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strconv"
	"strings"
	"time"
)

// VMDetail serves the four VM-detail endpoints, all gated by the same
// vm.Resolve() (FR-001, SC-005): GET /vms/:cluster/:vmid (detail),
// POST /vms/:cluster/:vmid/actions (power), DELETE /vms/:cluster/:vmid (delete),
// PATCH /vms/:cluster/:vmid (rename/description). 403/404 semantics are
// byte-identical across all four (contracts behavioural rule).
type VMDetail struct {
	projection   *inventory.Projection
	resolver     vm.ClusterIndexResolver
	auth         *Auth
	writer       cluster.Writer
	clients      cluster.ClientProvider
	store        *store.Store
	refresher    vm.IndexRefresher
	refreshers   ClusterRefresherResolver
	statusReader cluster.VMStatusReader
	policy       *policy.Policy
	log          *slog.Logger
}

// ServeHTTP dispatches to the sub-handlers.
func (h *VMDetail) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h.dispatchBySuffix(w, r) {
		return
	}

	h.dispatchByMethod(w, r)
}

// dispatchBySuffix routes sub-resource paths identified by their URL suffix
// (e.g. /hardware-options, /disks, /status). Returns true when the request
// was handled, false when it should fall through to the method-based switch.
func (h *VMDetail) dispatchBySuffix(w http.ResponseWriter, r *http.Request) bool {
	cases := []struct {
		suffix  string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{"/hardware-options", h.handleHardwareOptions},
		{"/cdrom", h.handleCDROM},
		{"/network", h.handleNetwork},
		{"/hardware", h.handleHardware},
		{"/serial", h.handleEnableSerial},
		{"/audit", h.handleAudit},
		{"/status", h.handleStatus},
	}

	for _, c := range cases {
		if strings.HasSuffix(r.URL.Path, c.suffix) {
			c.handler(w, r)
			return true
		}
	}

	// /disks and the per-disk routes (which carry a diskKey path value) share
	// the disk handler.
	if strings.HasSuffix(r.URL.Path, "/disks") || r.PathValue("diskKey") != "" {
		h.handleDisk(w, r)
		return true
	}

	return false
}

// dispatchByMethod routes the base /vms/{cluster}/{vmid} path by HTTP method.
func (h *VMDetail) dispatchByMethod(w http.ResponseWriter, r *http.Request) {
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
		h.writeDetailError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)
	}
}

// NewVMDetail creates the handler. The writer is the cluster.Writer (separate
// from the read Client — constitution IV); the refresher rebuilds the Index
// after a write (FR-010). Bound to a single cluster; use
// NewVMDetailWithRegistry for multi-cluster deployments.
func NewVMDetail(projection *inventory.Projection, authHandler *Auth, writer cluster.Writer, st *store.Store, refresher vm.IndexRefresher, log *slog.Logger, services ...*policy.Policy) *VMDetail {
	var policyService *policy.Policy
	if len(services) > 0 {
		policyService = services[0]
	}

	if policyService == nil && st != nil {
		policyService = policy.New(st, projection, nil)
	}

	h := &VMDetail{projection: projection, resolver: singleClusterResolver{projection: projection}, auth: authHandler, writer: writer, store: st, refresher: refresher, policy: policyService, log: log}
	// In single-cluster mode, the writer is typically the same client that
	// implements VMStatusReader (Fake or Proxmox both do). Wire it so the
	// /status endpoint works without the WithRegistry constructor.
	if reader, ok := writer.(cluster.VMStatusReader); ok {
		h.statusReader = reader
	}

	return h
}

// VMDetailDeps groups the shared dependencies for constructing a VMDetail
// handler. It collapses the seven positional parameters NewVMDetailWithRegistry
// used to take (SonarQube go:S107).
type VMDetailDeps struct {
	Source       inventory.LookupSource
	Projection   *inventory.Projection
	Auth         *Auth
	Writer       cluster.Writer
	Clients      cluster.ClientProvider
	Store        *store.Store
	Refresher    vm.IndexRefresher
	StatusReader cluster.VMStatusReader
	Log          *slog.Logger
}

// NewVMDetailWithRegistry adds cluster-aware reads and writes: every index
// load and cluster.Writer call below is resolved per-request from the
// request's own :cluster path value, never from a client bound once at
// startup (closes the same class of bug S01 fixed for a single default
// cluster — see the metrics-history ticket that surfaced the single-client
// wiring pattern in main.go's initCluster).
func NewVMDetailWithRegistry(deps VMDetailDeps, services ...*policy.Policy) *VMDetail {
	handler := NewVMDetail(deps.Projection, deps.Auth, deps.Writer, deps.Store, deps.Refresher, deps.Log, services...)
	if registry, ok := deps.Source.(*inventory.Registry); ok {
		handler.resolver = registryResolver{registry: registry}
		handler.refreshers = registryRefresherResolver{registry: registry}
	}

	handler.clients = deps.Clients
	handler.statusReader = deps.StatusReader

	return handler
}

// index resolves the current Index for clusterName, writing the appropriate
// error response on failure.
func (h *VMDetail) index(w http.ResponseWriter, clusterName string) (*inventory.Index, bool) {
	return loadClusterIndex(h.resolver, clusterName, func(status int, code, message string) { h.writeDetailError(w, status, code, message) })
}

// writerFor resolves the cluster.Writer for clusterName, writing a 404 on an
// unknown cluster name.
func (h *VMDetail) writerFor(w http.ResponseWriter, clusterName string) (cluster.Writer, bool) {
	writer, err := resolveCapability(h.clients, h.writer, clusterName, "Writer")
	if err != nil {
		h.writeDetailError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return nil, false
	}

	return writer, true
}

// statusReaderFor resolves the cluster.VMStatusReader for clusterName. Returns
// nil when no reader is available (single-cluster mode without one, or the
// cluster client doesn't implement VMStatusReader) — callers that need
// escalation fall back to the immediate-shutdown path.
func (h *VMDetail) statusReaderFor(clusterName string) cluster.VMStatusReader {
	reader, err := resolveCapability(h.clients, h.statusReader, clusterName, "VMStatusReader")
	if err != nil {
		return nil
	}

	return reader
}

// refresherFor resolves the vm.IndexRefresher for clusterName. Unlike
// writerFor, a missing refresher must not fail an action already applied on
// the cluster — so it never writes an HTTP error. When the per-cluster
// resolver is unset (single-cluster mode) or the cluster is unknown, it
// returns the fallback refresher and logs a warning. The result is never nil
// when the fallback is non-nil.
func (h *VMDetail) refresherFor(clusterName string) vm.IndexRefresher {
	if h.refreshers == nil {
		return h.refresher
	}

	refresher, err := h.refreshers.RefresherFor(clusterName)
	if err != nil {
		h.log.Warn("refresher not found for cluster, using fallback", "component", "httpapi", "cluster", clusterName, "error", err)
		return h.refresher
	}

	return refresher
}

type vmDetailDTO struct {
	Cluster           string                     `json:"cluster"`
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
	HasSerial         bool                       `json:"hasSerial"`
	UptimeSeconds     int64                      `json:"uptimeSeconds,omitempty"`
	Description       string                     `json:"description,omitempty"`
	// Lock carries the live Proxmox lock name ("snapshot-delete", "backup",
	// ...) from a best-effort /status/current read (ticket 06) — the page
	// shows a badge and the operator command to clear it. Empty when the VM
	// is unlocked or the live read failed.
	Lock string `json:"lock,omitempty"`
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
	// Force authorizes shutdown to skip ACPI and go directly to stop (ticket 05).
	// Only meaningful for shutdown; ignored for other actions.
	Force bool `json:"force"`
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

// handleGet serves GET /vms/:cluster/:vmid — the detail view (US1). Calls
// Resolve and encodes the Entity (FR-005).
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

// handleStatus serves GET /vms/:cluster/:vmid/status — the live status read
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
		h.log.Error("live status read failed", "component", "httpapi", "cluster", clusterName, "vmid", vmid, "error", err)
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

// handleAction serves POST /vms/:cluster/:vmid/actions (US2, closes S01).
// The request body carries only {"action": Kind} — no node field exists in
// the schema, so there is nothing to forge (S01 root cause, structurally closed).
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

	// Refresh the projection once after the action (ticket 09: the caller
	// owns refresh, not Action). Best-effort — the action already succeeded.
	if refresher := h.refresherFor(clusterName); refresher != nil {
		if _, err := refresher.Refresh(r.Context()); err != nil {
			h.log.Warn("post-action refresh failed", "component", "httpapi", "cluster", clusterName, "error", err)
		}
	}

	h.writeJSONStatus(w, http.StatusOK, actionResponse{Status: "accepted"})
}

// handleDelete serves DELETE /vms/:cluster/:vmid (US3). Same Resolve() gate.
// The optional ?force=true query parameter authorizes a force-stop of a running
// VM before the destroy — the UI only sends it after the user has confirmed the
// force-stop in the delete dialog. Without it, a running VM is rejected with
// 409 (code "vm_running") so the client can prompt for confirmation.
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

	if err := vm.Delete(r.Context(), vm.WriteDeps{Index: index, Actor: identity, ClusterName: clusterName, VMID: vmid, Writer: writer, Audit: h.store, Refresher: h.refresherFor(clusterName), Force: r.URL.Query().Get("force") == "true"}); err != nil {
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
		h.log.Error("post-patch re-resolve failed", "component", "httpapi", "vmid", vmid, "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	h.writeEntity(w, r, entity)
}

func (h *VMDetail) handleDisk(w http.ResponseWriter, r *http.Request) {
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

	resources, err := catalog.ApprovedResources(r.Context(), h.store, clusterName)
	if err != nil {
		h.log.Error(msgHardwareCatalogFailed, "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	deps := vm.DiskDependencies{
		Index:       index,
		Actor:       identity,
		ClusterName: clusterName,
		VMID:        vmid,
		Writer:      writer,
		Resources:   resources,
		Policy:      h.policy,
		Audit:       h.store,
		Refresher:   h.refresherFor(clusterName),
	}

	switch r.Method {
	case http.MethodPost:
		h.handleDiskCreate(w, r, deps, vmid)
	case http.MethodPut:
		h.handleDiskResize(w, r, deps, identity, clusterName, vmid)
	case http.MethodDelete:
		h.handleDiskDelete(w, r, deps, r.PathValue("diskKey"))
	default:
		w.Header().Set("Allow", "POST, PUT, DELETE")
		h.writeDetailError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)
	}
}

// handleDiskCreate adds a new disk to the VM from a POST body.
func (h *VMDetail) handleDiskCreate(w http.ResponseWriter, r *http.Request, deps vm.DiskDependencies, _ int) {
	var request diskRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}

	disk, err := vm.AddDisk(r.Context(), deps, cluster.DiskBus(request.Bus), request.Storage, request.SizeGB)
	if err != nil {
		h.writeDiskError(w, err)
		return
	}

	h.writeJSONStatus(w, http.StatusOK, disk)
}

// handleDiskResize grows an existing disk from a PUT body, then re-resolves the
// VM to return the updated disk.
func (h *VMDetail) handleDiskResize(w http.ResponseWriter, r *http.Request, deps vm.DiskDependencies, identity auth.Identity, clusterName string, vmid int) {
	var request resizeDiskRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}

	if err := vm.ResizeDisk(r.Context(), deps, r.PathValue("diskKey"), request.SizeGB); err != nil {
		h.writeDiskError(w, err)
		return
	}

	refreshed, ok := h.index(w, clusterName)
	if !ok {
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
}

// handleDiskDelete removes a disk by key.
func (h *VMDetail) handleDiskDelete(w http.ResponseWriter, r *http.Request, deps vm.DiskDependencies, diskKey string) {
	if err := vm.DeleteDisk(r.Context(), deps, diskKey); err != nil {
		h.writeDiskError(w, err)
		return
	}

	h.writeJSONStatus(w, http.StatusOK, deleteResponse{Status: "deleted"})
}

func (h *VMDetail) handleCDROM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPatch {
		w.Header().Set("Allow", "PATCH")
		h.writeDetailError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)

		return
	}

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

	resources, err := catalog.ApprovedResources(r.Context(), h.store, clusterName)
	if err != nil {
		h.log.Error(msgHardwareCatalogFailed, "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	var request cdromRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}

	state, err := vm.SetCDROM(r.Context(), vm.CDROMDependencies{
		Index:       index,
		Actor:       identity,
		ClusterName: clusterName,
		VMID:        vmid,
		Writer:      writer,
		Resources:   resources,
		Audit:       h.store,
		Refresher:   h.refresherFor(clusterName),
	}, request.Action, request.ISOVolID)
	if err != nil {
		h.writeCDROMError(w, err)
		return
	}

	h.writeJSONStatus(w, http.StatusOK, state)
}

// parseHardwareRequest decodes and validates the PUT .../hardware body: at
// least one field must be present. Split out of handleHardware to keep its
// cyclomatic complexity under the linter's ceiling.
func (h *VMDetail) parseHardwareRequest(w http.ResponseWriter, r *http.Request) (hardwareRequest, bool) {
	var request hardwareRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return hardwareRequest{}, false
	}

	if request.Sockets == nil && request.Cores == nil && request.MemoryMB == nil && request.Tags == nil {
		h.writeDetailError(w, http.StatusBadRequest, "empty_patch", "at least one hardware field is required")
		return hardwareRequest{}, false
	}

	return request, true
}

func (h *VMDetail) handleHardware(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", "PUT")
		h.writeDetailError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)

		return
	}

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

	request, ok := h.parseHardwareRequest(w, r)
	if !ok {
		return
	}

	err = vm.UpdateHardware(r.Context(), vm.HardwareDependencies{
		Index: index, Actor: identity, ClusterName: clusterName, VMID: vmid, Writer: writer,
		Policy: h.policy, Audit: h.store, Refresher: h.refresherFor(clusterName),
	}, vm.HardwarePatch{Sockets: request.Sockets, Cores: request.Cores, MemoryMB: request.MemoryMB, Tags: request.Tags})
	if err != nil {
		h.writeHardwareError(w, err)
		return
	}

	refreshed, ok := h.index(w, clusterName)
	if !ok {
		return
	}

	entity, err := vm.Resolve(refreshed, identity, clusterName, vmid)
	if err != nil {
		h.writeResolveError(w, err)
		return
	}

	h.writeEntity(w, r, entity)
}

// handleEnableSerial serves POST /vms/:cluster/:vmid/serial — the serial-
// console retrofit for VMs created before serial0 was added at create time.
// Reuses vm.EnableSerialConsole (Resolve ownership gate → Writer.EnableSerial
// → audit + inventory refresh) and returns the refreshed entity so the UI can
// flip its "no serial" state without a poll cycle.
func (h *VMDetail) handleEnableSerial(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		h.writeDetailError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)

		return
	}

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

	err = vm.EnableSerialConsole(r.Context(), vm.EnableSerialDependencies{
		Index:       index,
		Actor:       identity,
		ClusterName: clusterName,
		VMID:        vmid,
		Writer:      writer,
		Audit:       h.store,
		Refresher:   h.refresherFor(clusterName),
	})
	if err != nil {
		if h.writeCommonVMError(w, err) {
			return
		}

		h.log.Error("enable serial console failed", "component", "httpapi", "cluster", clusterName, "vmid", vmid, "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	refreshed, ok := h.index(w, clusterName)
	if !ok {
		return
	}

	entity, err := vm.Resolve(refreshed, identity, clusterName, vmid)
	if err != nil {
		h.writeResolveError(w, err)
		return
	}

	h.writeEntity(w, r, entity)
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
		h.writeDetailError(w, http.StatusServiceUnavailable, "policy_unavailable", msgPolicyUnavailable)
	case errors.Is(err, vm.ErrHardwareExceedsLimit):
		h.writeDetailError(w, http.StatusBadRequest, "hardware_exceeds_limit", err.Error())
	default:
		h.writeUnhandledVMError(w, "vm hardware operation failed", err)
	}
}

func (h *VMDetail) writeCommonVMError(w http.ResponseWriter, err error) bool {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeDetailError(w, http.StatusForbidden, "forbidden", msgNotYourVM)
	case errors.Is(err, vm.ErrNotFound):
		h.writeDetailError(w, http.StatusNotFound, "not_found", msgVMNotFound)
	default:
		return false
	}

	return true
}

func (h *VMDetail) writeUnhandledVMError(w http.ResponseWriter, message string, err error) {
	// A cluster rejection is not an unhandled error: surface Proxmox's own
	// message with its machine code instead of a generic 500 (ADR 0002).
	if code, msg, ok := clusterRejectionResponse(err); ok {
		h.writeDetailError(w, http.StatusBadGateway, code, msg)

		return
	}

	h.log.Error(message, "component", "httpapi", "error", err)
	h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
}

func (h *VMDetail) handleNetwork(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		w.Header().Set("Allow", "PUT")
		h.writeDetailError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)

		return
	}

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

	resources, err := catalog.ApprovedResources(r.Context(), h.store, clusterName)
	if err != nil {
		h.log.Error(msgHardwareCatalogFailed, "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	var request networkRequest
	if err := decodeJSON(w, r, &request); err != nil {
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", msgInvalidRequestBody)
		return
	}

	interfaces := make([]cluster.NetworkInterface, 0, len(request.Interfaces))
	for _, iface := range request.Interfaces {
		interfaces = append(interfaces, cluster.NetworkInterface{
			Index: iface.Index, Bridge: iface.Bridge, Model: iface.Model, VLAN: iface.VLAN, RateMbps: iface.RateMbps,
		})
	}

	updated, err := vm.UpdateNetwork(r.Context(), vm.NetworkDependencies{
		Index: index, Actor: identity, ClusterName: clusterName, VMID: vmid, Writer: writer,
		Resources: resources, Policy: h.policy, Audit: h.store, Refresher: h.refresherFor(clusterName),
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
		h.writeDetailError(w, http.StatusForbidden, "forbidden", msgNotYourVM)
	case errors.Is(err, vm.ErrNotFound):
		h.writeDetailError(w, http.StatusNotFound, "not_found", msgVMNotFound)
	case errors.Is(err, vm.ErrBridgeNotApproved):
		h.writeDetailError(w, http.StatusBadRequest, "bridge_not_approved", err.Error())
	case errors.Is(err, vm.ErrNetworkCardsExceedLimit):
		h.writeDetailError(w, http.StatusBadRequest, "network_cards_exceed_limit", err.Error())
	case errors.Is(err, policy.ErrUnavailable):
		h.writeDetailError(w, http.StatusServiceUnavailable, "policy_unavailable", msgPolicyUnavailable)
	case errors.Is(err, vm.ErrInvalidNetworkModel), errors.Is(err, vm.ErrDuplicateNetworkIndex):
		h.writeDetailError(w, http.StatusBadRequest, "invalid_request", err.Error())
	case errors.Is(err, cluster.ErrClusterRejected):
		code, message, _ := clusterRejectionResponse(err)
		h.writeDetailError(w, http.StatusBadGateway, code, message)
	default:
		h.log.Error("vm network operation failed", "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
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
		h.writeDetailError(w, http.StatusMethodNotAllowed, "method_not_allowed", msgMethodNotAllowed)

		return
	}

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

	resources, err := catalog.ApprovedResources(r.Context(), h.store, clusterName)
	if err != nil {
		h.log.Error(msgHardwareCatalogFailed, "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	if h.policy == nil {
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
		return
	}

	gabarit, err := h.policy.Gabarit(r.Context(), clusterName)
	if err != nil {
		h.log.Error("read gabarit failed", "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

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
		ISOs:     hardwareISOs(resources.ISOs),
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
		available, ok := vmCapableStorage(storage, index.StoragesByNode[storage.Node])
		if !ok {
			continue
		}

		result = append(result, hardwareStorageDTO{Node: storage.Node, Storage: storage.Name, Type: available.Type})
	}

	return result
}

func hardwareBridges(bridges []catalog.Bridge, node string) []hardwareBridgeDTO {
	result := make([]hardwareBridgeDTO, 0, len(bridges))
	for _, bridge := range bridges {
		if bridge.Node == node {
			result = append(result, hardwareBridgeDTO{Node: bridge.Node, Bridge: bridge.Name})
		}
	}

	return result
}

func hardwareISOs(isos []catalog.ISO) []hardwareISODTO {
	result := make([]hardwareISODTO, 0, len(isos))
	for _, iso := range isos {
		result = append(result, hardwareISODTO{
			VolID: iso.Storage + ":iso/" + iso.File,
			Node:  iso.Node, Storage: iso.Storage, Name: iso.File,
		})
	}

	return result
}

func (h *VMDetail) writeDiskError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeDetailError(w, http.StatusForbidden, "forbidden", msgNotYourVM)
	case errors.Is(err, vm.ErrNotFound):
		h.writeDetailError(w, http.StatusNotFound, "not_found", msgVMNotFound)
	case errors.Is(err, vm.ErrDiskNotFound):
		h.writeDetailError(w, http.StatusNotFound, "disk_not_found", err.Error())
	case errors.Is(err, vm.ErrBootDiskProtected):
		h.writeDetailError(w, http.StatusBadRequest, "boot_disk_protected", "the boot disk cannot be deleted")
	case errors.Is(err, vm.ErrVMNotStopped):
		h.writeDetailError(w, http.StatusBadRequest, "vm_not_stopped", err.Error())
	case errors.Is(err, vm.ErrDiskStorageNotApproved):
		h.writeDetailError(w, http.StatusBadRequest, "storage_not_approved", err.Error())
	case errors.Is(err, policy.ErrUnavailable):
		h.writeDetailError(w, http.StatusServiceUnavailable, "policy_unavailable", msgPolicyUnavailable)
	case errors.Is(err, vm.ErrDiskSizeExceedsLimit):
		h.writeDetailError(w, http.StatusBadRequest, "disk_size_exceeds_limit", err.Error())
	case errors.Is(err, vm.ErrDiskSizeNotGreater):
		h.writeDetailError(w, http.StatusBadRequest, "disk_size_not_greater", err.Error())
	case errors.Is(err, vm.ErrBusFull):
		h.writeDetailError(w, http.StatusBadRequest, "bus_full", err.Error())
	case errors.Is(err, cluster.ErrNotFound):
		h.writeDetailError(w, http.StatusBadGateway, "cluster_error", msgClusterRejected)
	case errors.Is(err, cluster.ErrClusterRejected):
		code, message, _ := clusterRejectionResponse(err)
		h.writeDetailError(w, http.StatusBadGateway, code, message)
	default:
		h.log.Error("vm disk operation failed", "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
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

func (h *VMDetail) writeEntity(w http.ResponseWriter, r *http.Request, entity vm.Entity) {
	dto := vmDetailDTO{
		Cluster:           entity.Cluster,
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
		HasSerial:         entity.HasSerial,
		Description:       entity.Description,
	}
	if entity.Uptime > 0 {
		dto.UptimeSeconds = int64(entity.Uptime.Seconds())
	}

	// Ticket 06: the detail DTO carries the live Proxmox lock (best-effort —
	// a failed live read must not fail the whole detail) so the page can show
	// the lock badge; the convergence loop keeps it fresh after actions.
	if reader := h.statusReaderFor(entity.Cluster); reader != nil {
		if live, err := reader.VMStatus(r.Context(), entity.Node, entity.VMID); err == nil {
			dto.Lock = live.Lock
		}
	}

	h.writeJSONStatus(w, http.StatusOK, dto)
}

func (h *VMDetail) writeJSONStatus(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		h.log.Error("failed to marshal response", "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)

		return
	}

	if err := writeJSON(w, status, body); err != nil {
		h.log.Error("failed to write response", "component", "httpapi", "error", err)
	}
}

// handleAudit serves GET /vms/:cluster/:vmid/audit — paginated, VM-scoped audit trail.
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
		h.log.Error("vm audit list failed", "component", "httpapi", "error", err)
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
// byte-identical in shape across all four endpoints (contracts).
func (h *VMDetail) writeResolveError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeDetailError(w, http.StatusForbidden, "forbidden", msgNotYourVM)
	case errors.Is(err, vm.ErrNotFound):
		h.writeDetailError(w, http.StatusNotFound, "not_found", msgVMNotFound)
	default:
		h.log.Error("unexpected resolve error", "component", "httpapi", "error", err)
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
		h.log.Error("cluster writer: VM not found after Resolve", "component", "httpapi", "error", err)
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
		h.log.Error("vm action failed", "component", "httpapi", "error", err)
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
		h.log.Error("cluster writer: VM not found after Resolve", "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusBadGateway, "cluster_error", msgClusterRejected)
	case errors.Is(err, cluster.ErrClusterRejected):
		code, message, _ := clusterRejectionResponse(err)
		h.writeDetailError(w, http.StatusBadGateway, code, message)
	default:
		h.log.Error("vm patch failed", "component", "httpapi", "error", err)
		h.writeDetailError(w, http.StatusInternalServerError, "internal_error", msgInternalServerError)
	}
}

func (h *VMDetail) writeDetailError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Error("failed to write error response", "component", "httpapi", "code", code, "error", err)
	}
}
