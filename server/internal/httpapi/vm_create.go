package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"time"
)

// createWriteDeadlineMargin covers the post-clone/create configuration steps
// (hardware overrides, disk resize, cloud-init push) that run after
// vm.WaitCreateTask returns, before the response is written.
const createWriteDeadlineMargin = 2 * time.Minute

// VMCreate serves POST /api/v1/vms (the single creation endpoint for both
// simple and detailed modes) and GET /api/v1/vm-create/catalog.
// All validation lives in vm.Create; this handler only decodes,
// maps errors, and encodes.
type VMCreate struct {
	auth             *Auth
	store            *store.Store
	client           cluster.Client
	clients          cluster.ClientProvider
	creator          cluster.Creator
	pusher           vm.CloudInitPusher
	policy           *policy.Policy
	log              *slog.Logger
	trustedProxyHops int
	// refreshers, when set, rebuilds the target cluster's inventory right
	// after a successful create so the VM is in /api/v1/vms by the time
	// the wizard navigates to the list - instead of depending on the task
	// tray's poll landing after the list load.
	refreshers ClusterRefresherResolver
}

// SetInventoryRefreshers wires the post-create inventory refresh.
func (h *VMCreate) SetInventoryRefreshers(refreshers ClusterRefresherResolver) {
	h.refreshers = refreshers
}

// refreshAfterCreate rebuilds clusterName's projection. Every create path
// has already waited for its Proxmox task, so the VM exists now.
// Best-effort: a failure only delays visibility to the next cycle.
func (h *VMCreate) refreshAfterCreate(ctx context.Context, clusterName string) {
	if h.refreshers == nil {
		return
	}

	refresher, err := h.refreshers.RefresherFor(clusterName)
	if err != nil {
		return
	}

	if _, err := refresher.Refresh(ctx); err != nil {
		h.log.WarnContext(ctx, "post-create inventory refresh failed", "component", "httpapi", "cluster", clusterName, "error", err)
	}
}

// NewVMCreate creates the handler. The creator is the cluster client's
// creation contract (allocation + async dispatch), separate from reads and
// from existing-VM writes. The pusher is the same cluster
// client's cloud-init push contract, reused by vm.Create's template-apply
// step - never a second write mechanism.
func NewVMCreate(
	authHandler *Auth,
	st *store.Store,
	client cluster.Client,
	creator cluster.Creator,
	pusher vm.CloudInitPusher,
	log *slog.Logger,
	services ...*policy.Policy,
) *VMCreate {
	var policyService *policy.Policy
	if len(services) > 0 {
		policyService = services[0]
	}

	return &VMCreate{
		auth:    authHandler,
		store:   st,
		client:  client,
		creator: creator,
		pusher:  pusher,
		policy:  policyService,
		log:     log,
	}
}

// NewVMCreateWithRegistry creates a VM handler with cluster-aware catalog discovery.
func NewVMCreateWithRegistry(
	authHandler *Auth,
	st *store.Store,
	clients cluster.ClientProvider,
	creator cluster.Creator,
	pusher vm.CloudInitPusher,
	log *slog.Logger,
	services ...*policy.Policy,
) *VMCreate {
	handler := NewVMCreate(
		authHandler,
		st,
		nil,
		creator,
		pusher,
		log,
		services...,
	)
	handler.clients = clients

	return handler
}

// SetTrustedProxyHops configures how many X-Forwarded-For hops to trust for
// client IP extraction used in audit log entries.
func (h *VMCreate) SetTrustedProxyHops(n int) {
	h.trustedProxyHops = n
}

type createResultDTO struct {
	Cluster             string `json:"cluster"`
	VMID                int    `json:"vmid"`
	Name                string `json:"name"`
	Node                string `json:"node"`
	UPID                string `json:"upid"`
	CloudInitTemplateID string `json:"cloudInitTemplateId,omitempty"`
	CloudInitPushError  string `json:"cloudInitPushError,omitempty"`
	// FromImage is true when the VM was created from a cloud image
	// the create summary warns that SSH
	// is the only access until a console password is set.
	FromImage bool `json:"fromImage,omitempty"`
}

type catalogStorageDTO struct {
	Name string `json:"name"`
	Node string `json:"node"`
}

// catalogBridgeDTO is one approved bridge on one node - bridge approval is
// per-node (like storage), so the client needs the node to both label the
// option and filter to the VM's chosen node.
type catalogBridgeDTO struct {
	Name    string `json:"name"`
	Node    string `json:"node"`
	Comment string `json:"comment,omitempty"`
}

type catalogISODTO struct {
	Storage string `json:"storage"`
	Node    string `json:"node"`
	File    string `json:"file"`
}

// catalogImageDTO is one approved cloud image (import-from source). SizeBytes
// lets the UI enforce the minimum disk size (reductions are rejected).
type catalogImageDTO struct {
	Storage   string `json:"storage"`
	Node      string `json:"node"`
	File      string `json:"file"`
	SizeBytes int64  `json:"sizeBytes"`
}

type catalogProfileDTO struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Sockets  int    `json:"sockets"`
	CPUCores int    `json:"cpuCores"`
	MemoryMB int    `json:"memoryMB"`
	DiskGB   int    `json:"diskGB"`
	Bus      string `json:"bus"`
}

type catalogCloudInitTemplateDTO struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	// Nodes lists the nodes whose snippet storage has the template's file:
	// the wizard offers the template only on those nodes.
	Nodes []string `json:"nodes"`
}

// catalogTemplateDTO is one approved Proxmox template. The
// VMID is the Proxmox VMID of the template; the node determines where the
// clone lands (cross-node clone is forbidden, so the UI hides the node selector when a template
// is chosen). CloudInitCapable signals the UI that
// the template supports cloud-init. DiskSizeGB lets the UI show the minimum
// disk size (reductions are rejected). DiskStorage is the template disk's
// source storage - the UI uses it to warn when the chosen target storage
// forces a full copy (buildCloneSpec's rule).
type catalogTemplateDTO struct {
	VMID             int    `json:"vmid"`
	Node             string `json:"node"`
	Name             string `json:"name"`
	CloudInitCapable bool   `json:"cloudInitCapable"`
	DiskSizeGB       int    `json:"diskSizeGB"`
	DiskStorage      string `json:"diskStorage"`
}

type catalogTagDTO struct {
	Name  string `json:"name"`
	Color string `json:"color"`
}

// catalogGabaritDTO is the administrator-editable per-VM size ceiling (gabarit) - the client
// uses it to validate hardware/disk fields before
// submit and to show the user what they're allowed, not just what failed.
type catalogGabaritDTO struct {
	MaxSockets       int `json:"maxSockets"`
	MaxCores         int `json:"maxCores"`
	MaxMemoryMB      int `json:"maxMemoryMB"`
	MaxDiskPerVMGB   int `json:"maxDiskPerVMGB"`
	MaxNetworkCards  int `json:"maxNetworkCards"`
	MaxSnapshots     int `json:"maxSnapshots"`
	IsolationVLANTag int `json:"isolationVlanTag"`
}

// catalogQuotaDTO is the caller's own VM count against the cluster's
// per-user allowance. Allowed is -1 for unlimited (policy.Quota contract).
type catalogQuotaDTO struct {
	Used    int `json:"used"`
	Allowed int `json:"allowed"`
}

// catalogNodeCapacityDTO is one approved node's configured aggregate
// capacité, live usage, and physical facts (policy.Capacity). Omitted from
// the response for a node with no capacité configured (all-zero row).
type catalogNodeCapacityDTO struct {
	Node          string `json:"node"`
	MaxVMs        int    `json:"maxVMs"`
	MaxVCPUs      int    `json:"maxVCPUs"`
	MaxRAMGB      int    `json:"maxRAMGB"`
	MaxDiskGB     int    `json:"maxDiskGB"`
	UsedVMs       int    `json:"usedVMs"`
	UsedVCPUs     int    `json:"usedVCPUs"`
	UsedRAMGB     int    `json:"usedRAMGB"`
	UsedDiskGB    int    `json:"usedDiskGB"`
	PhysicalVCPUs int    `json:"physicalVCPUs"`
	PhysicalRAMGB int    `json:"physicalRAMGB"`
}

type catalogDTO struct {
	Cluster            string                        `json:"cluster"`
	Nodes              []string                      `json:"nodes"`
	Storages           []catalogStorageDTO           `json:"storages"`
	Bridges            []catalogBridgeDTO            `json:"bridges"`
	ISOs               []catalogISODTO               `json:"isos"`
	Images             []catalogImageDTO             `json:"images"`
	Profiles           []catalogProfileDTO           `json:"profiles"`
	Templates          []catalogTemplateDTO          `json:"templates"`
	CloudInitTemplates []catalogCloudInitTemplateDTO `json:"cloudInitTemplates"`
	// CloudInitWriteEnabled reports whether this cluster has a snippet write
	// target configured (Infrastructure › Clusters): cloud-init documents can be
	// written, so the wizard may offer the document picker.
	CloudInitWriteEnabled bool `json:"cloudInitWriteEnabled"`
	// CloudInitBaselineID is the document id of the standalone baseline
	// (published for image VMs with no template). The web client compares a
	// VM's document id against it instead of hardcoding the value.
	CloudInitBaselineID string                   `json:"cloudInitBaselineId"`
	Tags                []catalogTagDTO          `json:"tags"`
	Gabarit             *catalogGabaritDTO       `json:"gabarit,omitempty"`
	Quota               *catalogQuotaDTO         `json:"quota,omitempty"`
	NodeCapacities      []catalogNodeCapacityDTO `json:"nodeCapacities,omitempty"`
}

// ServeHTTP handles POST /api/v1/vms. Creation is asynchronous:
// 202 means the task was accepted, not that the VM exists.
func (h *VMCreate) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeCreateError(w, http.StatusUnauthorized, "unauthenticated", msgAuthRequired)
		return
	}

	// Creation blocks on vm.WaitCreateTask (up to vm.MaxCreateTaskWait) before
	// writing any response - the server's global WriteTimeout (10s,
	// cmd/pvmss/main.go) is nowhere near enough for a template clone or image
	// import. Extend just this response's write deadline so a slow clone
	// still reaches the client instead of the connection dying underneath a
	// creation that actually succeeded (report: VM created in Proxmox, PVMSS
	// showed "Échec de la création").
	if rc := http.NewResponseController(w); rc != nil {
		_ = rc.SetWriteDeadline(time.Now().Add(vm.MaxCreateTaskWait + createWriteDeadlineMargin))
	}

	var req vm.CreateRequest
	if err := decodeJSON(w, r, &req); err != nil {
		h.writeCreateError(w, http.StatusBadRequest, codeInvalidRequest, msgInvalidRequestBody)
		return
	}

	target, ok := h.resolveCreateTarget(w, req.Cluster)
	if !ok {
		return
	}

	ctx := policy.ContextWithAuditIP(r.Context(), clientIP(r, h.trustedProxyHops))

	result, err := vm.Create(ctx, identity, target.clusterName, req, vm.CreateDeps{
		Store:     h.store,
		Creator:   target.creator,
		Pusher:    target.pusher,
		Writer:    target.writer,
		FreeSpace: target.freeSpace,
		Snippets:  target.snippets,
		Audit:     h.store,
		Log:       h.log,
		Services:  []*policy.Policy{h.policy},
		Templates: target.templates,
	})
	if err != nil {
		h.writeCreateFailure(w, err)
		return
	}

	h.refreshAfterCreate(r.Context(), target.clusterName)

	h.writeCreateJSON(w, http.StatusAccepted, createResultDTO{
		Cluster:             result.Cluster,
		VMID:                result.VMID,
		Name:                result.Name,
		Node:                result.Node,
		UPID:                result.UPID,
		CloudInitTemplateID: result.CloudInitTemplateID,
		CloudInitPushError:  result.CloudInitPushError,
		FromImage:           result.FromImage,
	})
}
