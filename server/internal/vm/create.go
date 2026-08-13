package vm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"slices"
)

// Sentinel errors for the creation validation pipeline (T06 data-model.md).
// The handler maps them to 400/403; everything else from the cluster client
// is a 502.
var (
	// ErrNoPool — the actor has no personal pool, so nothing can own the VM
	// (FR-005: local admin, or a cluster admin without a pool).
	ErrNoPool = errors.New("no personal pool")
	// ErrOutOfRange — CPU/memory/disk violate the fixed technical safety
	// ceiling (FR-008). Deliberately never called a "gabarit" (constitution
	// I: that word is reserved for T12's policy).
	ErrOutOfRange = errors.New("out of technical range")
	// ErrNotApproved — a referenced node, storage, bridge, ISO, or profile is
	// absent from the cluster's catalog (FR-003).
	ErrNotApproved = errors.New("not approved for this cluster")
	// ErrClusterCreate — the cluster client rejected or failed the dispatch
	// (mapped to 502 by the handler).
	ErrClusterCreate = errors.New("cluster create failed")
)

// Fixed technical safety ceilings (FR-008) — hardcoded anti-abuse bounds,
// not admin-configurable.
const (
	MinCPUCores = 1
	MaxCPUCores = 32
	MinMemoryMB = 128
	MaxMemoryMB = 65536
	MinDiskGB   = 1
	MaxDiskGB   = 2048
)

// defaultNetworkModel is applied when a request omits the NIC model (simple
// mode never asks for it).
const defaultNetworkModel = "virtio"

// defaultDiskBus is applied when no profile is used (detailed mode). Profiles
// override this with their own bus value (FR-009).
const defaultDiskBus = "scsi"

// allowedNetworkModels is the fixed whitelist of NIC models the server
// accepts (FR-003 spirit: the catalog constrains bridges, this constrains
// the model — a forged request with an arbitrary string is rejected).
var allowedNetworkModels = map[string]bool{
	"virtio":  true,
	"e1000":   true,
	"rtl8139": true,
	"vmxnet3": true,
}

// CreateRequest is the single creation request shape both frontend modes
// build (FR-001). It deliberately carries no pool field (FR-004 — nothing to
// forge) and no mode field (the server cannot tell and does not care which
// wizard produced it).
type CreateRequest struct {
	Cluster             string         `json:"cluster"`
	Name                string         `json:"name"`
	ProfileID           string         `json:"profileId,omitempty"`
	CloudInitTemplateID string         `json:"cloudInitTemplateId,omitempty"`
	Node                string         `json:"node,omitempty"`
	Tags                []string       `json:"tags,omitempty"`
	CPUCores            int            `json:"cpuCores,omitempty"`
	MemoryMB            int            `json:"memoryMB,omitempty"`
	Disk                DiskRequest    `json:"disk"`
	Network             NetworkRequest `json:"network"`
	ISO                 *ISORequest    `json:"iso,omitempty"`
	StartAfterCreate    bool           `json:"startAfterCreate,omitempty"`
}

// DiskRequest is the request's single initial disk.
type DiskRequest struct {
	Storage string `json:"storage,omitempty"`
	SizeGB  int    `json:"sizeGB,omitempty"`
}

// NetworkRequest is the request's single initial NIC.
type NetworkRequest struct {
	Bridge string `json:"bridge,omitempty"`
	Model  string `json:"model,omitempty"`
}

// ISORequest is an optional installation ISO.
type ISORequest struct {
	Storage string `json:"storage"`
	File    string `json:"file"`
}

// CloudInitPusher applies a cloud-init snippet to a VM's storage — T08's
// cluster.Writer.PushCloudInitSnippet, reused verbatim by the creation-time
// template apply step (FR-007). Defined here as a narrow consumer contract so
// vm.Create depends only on the push method it actually calls, not the full
// Writer surface; cluster.Fake and the real Proxmox client both satisfy it.
type CloudInitPusher interface {
	PushCloudInitSnippet(ctx context.Context, node, storage, filename string, vmid int, content string) error
}

// CreateResult is what a successful creation returns — the task is accepted,
// the VM does not necessarily exist yet (FR-013).
type CreateResult struct {
	Cluster             string
	VMID                int
	Name                string
	Node                string
	UPID                string
	CloudInitTemplateID string
	CloudInitPushError  string
}

// CreateDeps groups the collaborators vm.Create needs beyond the per-request
// arguments (ctx, actor, clusterName, req). Bundling them keeps Create's
// parameter count under go:S107's ceiling without losing any dependency.
type CreateDeps struct {
	Store    *store.Store
	Creator  cluster.Creator
	Pusher   CloudInitPusher
	Audit    AuditRecorder
	Log      *slog.Logger
	Services []*policy.Policy
}

// Create validates a creation request and dispatches it as an asynchronous
// cluster task (T06 data-model.md, steps in order):
//
//  1. the actor must have a personal pool (FR-005)
//  2. the name must be a valid hostname (FR-007, T05's rule reused)
//  3. a profile's catalog values override any request hardware fields (FR-009)
//  4. CPU/memory/disk must be within the technical ceiling (FR-008)
//  5. unset node/storage/bridge are auto-selected from the first approved
//     catalog entries (FR-010); every referenced resource must be a catalog
//     member (FR-003)
//  6. the VMID comes from the cluster client's single allocation point
//     (FR-012); the pool is always the actor's own (FR-004); the pvmss tag
//     is always present (FR-006)
//  7. the dispatch is recorded in the audit log (FR-017)
//
// Index invalidation (FR-018) is NOT done here — the VM does not exist yet;
// the task-status handler invalidates when the task reaches ok.
//
// A step-7 audit-write failure does not fail the request: the cluster task
// from step 6 is already dispatched and real, so returning an error here
// would tell the client creation failed when it did not — the same
// log-don't-fail rule the task-status handler already applies to a failed
// post-completion invalidation (tasks.go).
func Create(ctx context.Context, actor auth.Identity, clusterName string, req CreateRequest, deps CreateDeps) (CreateResult, error) {
	policyService := selectPolicyService(deps.Store, deps.Services)

	if actor.Pool == "" {
		return CreateResult{}, ErrNoPool
	}

	plan, err := planCreate(ctx, policyService, deps.Store, clusterName, actor, req)
	if err != nil {
		return CreateResult{}, err
	}

	// T18 step (FR-006): resolve a cloud-init template BEFORE NextVMID so an
	// unknown or disabled id is rejected without burning a VMID — the same
	// "never spend a VMID on a request that will be rejected" discipline T06
	// applies to node/storage/bridge/ISO catalog membership.
	var cloudTemplate catalog.CloudInitTemplate
	if req.CloudInitTemplateID != "" {
		cloudTemplate, err = catalog.FindCloudInitTemplate(ctx, deps.Store, clusterName, req.CloudInitTemplateID)
		if err != nil {
			return CreateResult{}, fmt.Errorf("%w: cloud-init template %q is not approved for this cluster", ErrNotApproved, req.CloudInitTemplateID)
		}
	}

	vmid, err := deps.Creator.NextVMID(ctx)
	if err != nil {
		return CreateResult{}, fmt.Errorf("%w: allocate vmid: %w", ErrClusterCreate, err)
	}

	tags := append([]string(nil), req.Tags...)
	if !slices.Contains(tags, "pvmss") {
		tags = append(tags, "pvmss")
	}

	spec := cluster.VMSpec{
		VMID:             vmid,
		Node:             plan.node,
		Name:             req.Name,
		Pool:             actor.Pool,
		Tags:             tags,
		CPUCores:         plan.cpuCores,
		MemoryMB:         plan.memoryMB,
		Disk:             cluster.DiskSpec{Storage: plan.storage, SizeGB: plan.diskGB, Bus: plan.bus},
		Network:          cluster.NetworkSpec{Bridge: plan.bridge, Model: plan.model},
		StartAfterCreate: req.StartAfterCreate,
	}
	if req.ISO != nil {
		spec.ISO = &cluster.ISOSpec{Storage: req.ISO.Storage, File: req.ISO.File}
	}

	upid, err := deps.Creator.CreateVM(ctx, spec)
	if err != nil {
		return CreateResult{}, fmt.Errorf("%w: %w", ErrClusterCreate, err)
	}

	result := CreateResult{Cluster: clusterName, VMID: vmid, Name: req.Name, Node: plan.node, UPID: upid}

	// T18 step (FR-007): apply the resolved template via T08's existing snippet
	// write path — store.PutCloudInitSnippet then cluster push — reusing the
	// same functions T08's per-VM snippet editor calls, never a second write
	// mechanism. A failure here does NOT abort the creation (the task is
	// already dispatched and cannot be undone): it sets CloudInitPushError
	// (FR-008). On success, CloudInitTemplateID echoes the resolved id.
	if cloudTemplate.ID != "" {
		result.CloudInitTemplateID = cloudTemplate.ID
		filename := fmt.Sprintf("%s%d.yml", snippetFilenamePrefix, vmid)

		storage := spec.Disk.Storage
		if err := deps.Store.PutCloudInitSnippet(ctx, clusterName, vmid, storage, filename, cloudTemplate.Content, actor.Username); err != nil {
			deps.Log.Error("cloud-init template store failed", "component", "vm", "cluster", clusterName, "vmid", vmid, "error", err)
			result.CloudInitPushError = err.Error()
		} else if err := deps.Pusher.PushCloudInitSnippet(ctx, spec.Node, storage, filename, vmid, cloudTemplate.Content); err != nil {
			deps.Log.Error("cloud-init template push failed", "component", "vm", "cluster", clusterName, "vmid", vmid, "error", err)
			result.CloudInitPushError = err.Error()
		}
	}

	if err := deps.Audit.RecordAction(ctx, actor.Username, clusterName, vmid, "vm_create"); err != nil {
		deps.Log.Error("record audit failed", "component", "vm", "cluster", clusterName, "vmid", vmid, "error", err)
	}

	return result, nil
}

// createPlan holds the resolved and validated values for a VM creation request.
type createPlan struct {
	node     string
	storage  string
	bridge   string
	model    string
	cpuCores int
	memoryMB int
	diskGB   int
	bus      string
}

// planCreate runs all pre-allocation validation: quota, name, catalog,
// hardware ranges, gabarit, resource resolution, and node capacity.
func planCreate(ctx context.Context, policyService *policy.Policy, st *store.Store, clusterName string, actor auth.Identity, req CreateRequest) (createPlan, error) {
	if err := policyService.CheckQuota(ctx, clusterName, actor); err != nil {
		return createPlan{}, err
	}

	if err := ValidateName(req.Name); err != nil {
		return createPlan{}, err
	}

	resources, err := catalog.ApprovedResources(ctx, st, clusterName)
	if err != nil {
		return createPlan{}, fmt.Errorf("read catalog: %w", err)
	}

	cpuCores, memoryMB, diskGB, bus, err := resolveHardware(ctx, st, clusterName, req)
	if err != nil {
		return createPlan{}, err
	}

	if err := checkTechnicalRange(cpuCores, memoryMB, diskGB); err != nil {
		return createPlan{}, err
	}

	if err := policyService.CheckGabarit(ctx, clusterName, 1, cpuCores, memoryMB, diskGB, 1); err != nil {
		return createPlan{}, err
	}

	node, storage, bridge, model, err := resolveResources(req, resources)
	if err != nil {
		return createPlan{}, err
	}

	if err := validateCatalog(req, resources, node, storage, bridge, model); err != nil {
		return createPlan{}, err
	}

	if err := policyService.CheckNodeCapacity(ctx, clusterName, node, 1, cpuCores, memoryMB, 0); err != nil {
		return createPlan{}, err
	}

	return createPlan{node: node, storage: storage, bridge: bridge, model: model, cpuCores: cpuCores, memoryMB: memoryMB, diskGB: diskGB, bus: bus}, nil
}

// resolveHardware returns the effective CPU, memory, disk, and bus values,
// applying the profile's catalog values when a profile is selected (FR-009).
func resolveHardware(ctx context.Context, st *store.Store, clusterName string, req CreateRequest) (cpuCores, memoryMB, diskGB int, bus string, err error) {
	cpuCores, memoryMB, diskGB = req.CPUCores, req.MemoryMB, req.Disk.SizeGB
	bus = defaultDiskBus

	if req.ProfileID == "" {
		return cpuCores, memoryMB, diskGB, bus, nil
	}

	profiles, err := catalog.Profiles(ctx, st, clusterName)
	if err != nil {
		return 0, 0, 0, "", fmt.Errorf("read profiles: %w", err)
	}

	profile, err := catalog.FindProfile(profiles, req.ProfileID)
	if err != nil {
		return 0, 0, 0, "", fmt.Errorf("%w: %s", ErrNotApproved, err.Error())
	}

	// FR-009: the profile's catalog values are authoritative — hardware
	// fields the request also carries are ignored, never merged.
	return profile.CPUCores, profile.MemoryMB, profile.DiskGB, profile.Bus, nil
}

// resolveResources resolves the node, storage, bridge, and network model,
// applying auto-selection defaults when the request omits them.
func resolveResources(req CreateRequest, resources catalog.Resources) (node, storage, bridge, model string, err error) {
	node = req.Node
	if node == "" {
		if len(resources.Nodes) == 0 {
			return "", "", "", "", fmt.Errorf("%w: no approved node in catalog", ErrNotApproved)
		}

		node = resources.Nodes[0].Name
	}

	storage = req.Disk.Storage
	if storage == "" {
		storage = firstStorageOnNode(resources, node)
		if storage == "" {
			return "", "", "", "", fmt.Errorf("%w: no approved storage on node %q", ErrNotApproved, node)
		}
	}

	bridge = req.Network.Bridge
	if bridge == "" {
		if len(resources.Bridges) == 0 {
			return "", "", "", "", fmt.Errorf("%w: no approved bridge in catalog", ErrNotApproved)
		}

		bridge = resources.Bridges[0]
	}

	model = req.Network.Model
	if model == "" {
		model = defaultNetworkModel
	}

	return node, storage, bridge, model, nil
}

// validateCatalog checks that the resolved node, storage, bridge, model, and
// optional ISO are all present in the approved catalog.
func validateCatalog(req CreateRequest, resources catalog.Resources, node, storage, bridge, model string) error {
	if !allowedNetworkModels[model] {
		return fmt.Errorf("%w: network model %q", ErrNotApproved, model)
	}

	if !resources.HasNode(node) {
		return fmt.Errorf("%w: node %q", ErrNotApproved, node)
	}

	if !resources.HasStorage(storage, node) {
		return fmt.Errorf("%w: storage %q on node %q", ErrNotApproved, storage, node)
	}

	if !resources.HasBridge(bridge) {
		return fmt.Errorf("%w: bridge %q", ErrNotApproved, bridge)
	}

	if req.ISO != nil && !resources.HasISO(req.ISO.Storage, req.ISO.File) {
		return fmt.Errorf("%w: iso %q on storage %q", ErrNotApproved, req.ISO.File, req.ISO.Storage)
	}

	return nil
}

// checkTechnicalRange enforces FR-008's fixed anti-abuse bounds.
func checkTechnicalRange(cpuCores, memoryMB, diskGB int) error {
	switch {
	case cpuCores < MinCPUCores || cpuCores > MaxCPUCores:
		return fmt.Errorf("%w: cpuCores must be between %d and %d", ErrOutOfRange, MinCPUCores, MaxCPUCores)
	case memoryMB < MinMemoryMB || memoryMB > MaxMemoryMB:
		return fmt.Errorf("%w: memoryMB must be between %d and %d", ErrOutOfRange, MinMemoryMB, MaxMemoryMB)
	case diskGB < MinDiskGB || diskGB > MaxDiskGB:
		return fmt.Errorf("%w: disk sizeGB must be between %d and %d", ErrOutOfRange, MinDiskGB, MaxDiskGB)
	}

	return nil
}

// firstStorageOnNode returns the first approved storage attached to node, or
// "" when none is (catalog queries are ordered, so this is deterministic).
func firstStorageOnNode(resources catalog.Resources, node string) string {
	for _, storage := range resources.Storages {
		if storage.Node == node {
			return storage.Name
		}
	}

	return ""
}
