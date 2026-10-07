package httpapi

import (
	"log/slog"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strings"
)

// VMDetail serves the four VM-detail endpoints, all gated by the same
// vm.Resolve(): GET /vms/:cluster/:vmid (detail),
// POST /vms/:cluster/:vmid/actions (power), DELETE /vms/:cluster/:vmid (delete),
// PATCH /vms/:cluster/:vmid (rename/description). 403/404 semantics are
// byte-identical across all four (contracts behavioural rule).
type VMDetail struct {
	projection     *inventory.Projection
	resolver       vm.ClusterIndexResolver
	auth           *Auth
	writer         cluster.Writer
	clients        cluster.ClientProvider
	store          *store.Store
	refresher      vm.IndexRefresher
	refreshers     ClusterRefresherResolver
	statusReader   cluster.VMStatusReader
	guestNetReader cluster.GuestNetworkReader
	policy         *policy.Policy
	log            *slog.Logger
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
		{"/boot-cdrom", h.handleBootCDROM},
		{"/cdrom", h.handleCDROM},
		{"/network", h.handleNetwork},
		{"/hardware", h.handleHardware},
		{"/serial", h.handleEnableSerial},
		{"/retrofit-seabios", h.handleRetrofitSeaBIOS},
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
// from the read Client); the refresher rebuilds the Index
// after a write. Bound to a single cluster; use
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

	if reader, ok := writer.(cluster.GuestNetworkReader); ok {
		h.guestNetReader = reader
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
	// GuestNetReader supplies the live per-NIC IP addresses the detail DTO
	// merges in - the inventory projection carries config-only interfaces.
	GuestNetReader cluster.GuestNetworkReader
	Log            *slog.Logger
}

// NewVMDetailWithRegistry adds cluster-aware reads and writes: every index
// load and cluster.Writer call below is resolved per-request from the
// request's own :cluster path value, never from a client bound once at
// startup (closes the same class of bug fixed for a single default cluster - see the
// metrics-history work that surfaced the single-client wiring pattern in main.go's
// initCluster).
func NewVMDetailWithRegistry(deps VMDetailDeps, services ...*policy.Policy) *VMDetail {
	handler := NewVMDetail(deps.Projection, deps.Auth, deps.Writer, deps.Store, deps.Refresher, deps.Log, services...)
	if registry, ok := deps.Source.(*inventory.Registry); ok {
		handler.resolver = registryResolver{registry: registry}
		handler.refreshers = registryRefresherResolver{registry: registry}
	}

	handler.clients = deps.Clients
	handler.statusReader = deps.StatusReader
	handler.guestNetReader = deps.GuestNetReader

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
// cluster client doesn't implement VMStatusReader) - callers that need
// escalation fall back to the immediate-shutdown path.
func (h *VMDetail) statusReaderFor(clusterName string) cluster.VMStatusReader {
	reader, err := resolveCapability(h.clients, h.statusReader, clusterName, "VMStatusReader")
	if err != nil {
		return nil
	}

	return reader
}

// guestNetReaderFor resolves the cluster.GuestNetworkReader for clusterName.
// Returns nil when unavailable - the IP column is best-effort, an absent
// reader means "no live addresses", never an error.
func (h *VMDetail) guestNetReaderFor(clusterName string) cluster.GuestNetworkReader {
	reader, err := resolveCapability(h.clients, h.guestNetReader, clusterName, "GuestNetworkReader")
	if err != nil {
		return nil
	}

	return reader
}

// refresherFor resolves the vm.IndexRefresher for clusterName. Unlike
// writerFor, a missing refresher must not fail an action already applied on
// the cluster - so it never writes an HTTP error. When the per-cluster
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
	Cluster string   `json:"cluster"`
	VMID    int      `json:"vmid"`
	Name    string   `json:"name"`
	Node    string   `json:"node"`
	Pool    string   `json:"pool"`
	Status  string   `json:"status"`
	Tags    []string `json:"tags"`
	// OSType is Proxmox's kernel family ("l26", "win11"), not a distribution.
	OSType      string             `json:"ostype"`
	CPUCores    int                `json:"cpuCores"`
	MemoryTotal int64              `json:"memoryTotal"`
	DiskTotal   int64              `json:"diskTotal"`
	Sockets     int                `json:"sockets"`
	Cores       int                `json:"cores"`
	Disks       []cluster.Disk     `json:"disks"`
	CDROM       cluster.CDROMState `json:"cdrom"`
	// BootOrder mirrors the VM's persistent boot=order=... key ("ide2",
	// "scsi0", ...). The Connect tab reads it to tell an ISO-first boot
	// (installation still in progress) from a disk-first boot even while an
	// ISO remains attached.
	BootOrder         []string                   `json:"bootOrder,omitempty"`
	NetworkInterfaces []cluster.NetworkInterface `json:"networkInterfaces"`
	HasSerial         bool                       `json:"hasSerial"`
	UptimeSeconds     int64                      `json:"uptimeSeconds,omitempty"`
	Description       string                     `json:"description,omitempty"`
	DescriptionHTML   string                     `json:"descriptionHtml,omitempty"`
	// Lock carries the live Proxmox lock name ("snapshot-delete", "backup",
	// ...) from a best-effort /status/current read - the page
	// shows a badge and the operator command to clear it. Empty when the VM
	// is unlocked or the live read failed.
	Lock string `json:"lock,omitempty"`
	// GuestAgent explains why networkInterfaces[].ipAddresses is populated or
	// not, for a running VM: "disabled" (agent=0 in the VM config - known
	// without probing), "unreachable" (enabled but the agent did not answer
	// - not installed in the guest or still starting), "ok" (answered; IPs
	// may still be empty while DHCP is pending). Empty when the VM is not
	// running - the status field already explains it.
	GuestAgent string `json:"guestAgent,omitempty"`
	// BaselineState is the delivery state of the generated cloud-init
	// baseline for image-mode VMs: "applied", "override", "not_delivered". Empty for non-image
	// VMs.
	BaselineState string `json:"baselineState,omitempty"`
	// BaselineError is the reason when BaselineState is "not_delivered".
	BaselineError string `json:"baselineError,omitempty"`
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
	Tags     []hardwareTagDTO     `json:"tags"`
	Limits   vmLimitsDTO          `json:"limits"`
}

// hardwareTagDTO is one admin-curated tag offered to the VM tag picker.
// The protected pvmss tag is excluded - users cannot toggle it.
type hardwareTagDTO struct {
	Name  string `json:"name"`
	Color string `json:"color"`
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
	// Force authorizes shutdown to skip the ACPI request and stop directly.
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
	Name string `json:"name"`
	// Description is a pointer so "" (clear it) differs from absent.
	Description *string `json:"description"`
}
