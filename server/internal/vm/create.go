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
	Cluster          string         `json:"cluster"`
	Name             string         `json:"name"`
	ProfileID        string         `json:"profileId,omitempty"`
	Node             string         `json:"node,omitempty"`
	Tags             []string       `json:"tags,omitempty"`
	CPUCores         int            `json:"cpuCores,omitempty"`
	MemoryMB         int            `json:"memoryMB,omitempty"`
	Disk             DiskRequest    `json:"disk"`
	Network          NetworkRequest `json:"network"`
	ISO              *ISORequest    `json:"iso,omitempty"`
	StartAfterCreate bool           `json:"startAfterCreate,omitempty"`
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

// CreateResult is what a successful creation returns — the task is accepted,
// the VM does not necessarily exist yet (FR-013).
type CreateResult struct {
	Cluster string
	VMID    int
	Name    string
	Node    string
	UPID    string
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
func Create(ctx context.Context, actor auth.Identity, clusterName string, req CreateRequest, st *store.Store, creator cluster.Creator, audit AuditRecorder, log *slog.Logger, services ...*policy.Policy) (CreateResult, error) {
	policyService := selectPolicyService(st, services)

	if actor.Pool == "" {
		return CreateResult{}, ErrNoPool
	}

	plan, err := planCreate(ctx, policyService, st, clusterName, actor, req)
	if err != nil {
		return CreateResult{}, err
	}

	vmid, err := creator.NextVMID(ctx)
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

	upid, err := creator.CreateVM(ctx, spec)
	if err != nil {
		return CreateResult{}, fmt.Errorf("%w: %w", ErrClusterCreate, err)
	}

	if err := audit.RecordAction(ctx, actor.Username, clusterName, vmid, "vm_create"); err != nil {
		log.Error("record audit failed", "component", "vm", "cluster", clusterName, "vmid", vmid, "error", err)
	}

	return CreateResult{Cluster: clusterName, VMID: vmid, Name: req.Name, Node: plan.node, UPID: upid}, nil
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
