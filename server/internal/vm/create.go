package vm

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
)

// Sentinel errors for the creation validation pipeline.
// The handler maps them to 400/403; everything else from the cluster client
// is a 502.
var (
	// ErrNoPool - a non-admin actor has no personal pool, so nothing can own
	// the VM.
	ErrNoPool = errors.New("no personal pool")
	// ErrAdminCannotCreate - an administrator (local or cluster) cannot create
	// VMs through the self-service portal. VM ownership requires a personal
	// pool, which admins do not have. Admins manage VMs through the admin
	// pages or directly in Proxmox.
	ErrAdminCannotCreate = errors.New("administrators cannot create VMs")
	// ErrOutOfRange - CPU/memory/disk violate the fixed technical safety
	// ceiling. Deliberately never called a "gabarit" (that word is reserved for the policy).
	ErrOutOfRange = errors.New("out of technical range")
	// ErrNotApproved - a referenced node, storage, bridge, ISO, or profile is
	// absent from the cluster's catalog.
	ErrNotApproved = errors.New("not approved for this cluster")
	// ErrClusterCreate - the cluster client rejected or failed the dispatch
	// (mapped to 502 by the handler).
	ErrClusterCreate = errors.New("cluster create failed")
	// ErrInvalidSource - the request carries more than one VM source (ISO,
	// template, cloud image) or none. Mapped to 400 by
	// the handler.
	ErrInvalidSource = errors.New("invalid vm source")
	// ErrInvalidRequest - the request carries an impossible combination of
	// options (TPM without UEFI). Mapped to 400 by the handler.
	ErrInvalidRequest = errors.New("invalid request")
	// ErrDiskReduction - the requested disk size is smaller than the
	// template's disk (Proxmox does not reduce disks).
	ErrDiskReduction = errors.New("disk size below template")
	// ErrInsufficientDiskSpace - the target storage does not have enough
	// free space for the requested disk (hard refusal before VMID consumption).
	ErrInsufficientDiskSpace = errors.New("insufficient disk space")
	// ErrNameTaken - the actor already has a VM with the requested name in
	// their personal pool (per-pool uniqueness so two VMs in a user's list are never
	// indistinguishable). Mapped to 400 by the
	// handler with the code "name_taken".
	ErrNameTaken = errors.New("name already taken")
)

// Fixed technical safety ceilings - hardcoded anti-abuse bounds,
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

// allocateVMIDError wraps a NextVMID failure in ErrClusterCreate.
func allocateVMIDError(err error) error {
	if errors.Is(err, ErrVMIDRangeExhausted) {
		return err
	}

	return fmt.Errorf("%w: allocate vmid: %w", ErrClusterCreate, err)
}

// notApprovedError wraps a catalog lookup failure in ErrNotApproved.
func notApprovedError(err error) error {
	return fmt.Errorf("%w: %s", ErrNotApproved, err.Error())
}

// defaultDiskBus is applied when no profile is used (detailed mode). Profiles
// override this with their own bus value.
const defaultDiskBus = "scsi"

// maxVMIDRetries is the maximum number of VMID collision retries after the
// first attempt (max 3 attempts total). GET /cluster/nextid
// returns the smallest free ID without reserving it, so two concurrent
// creations can collide; retrying with a fresh VMID is not a mutation replay.
const maxVMIDRetries = 2 // 1 initial + 2 retries = 3 attempts

// auditLogMsg is the slog message used when RecordAction fails - the audit
// trail is best-effort and never blocks a successful create/clone.
const auditLogMsg = "record audit failed"

// allowedNetworkModels is the fixed whitelist of NIC models the server
// accepts (spirit: the catalog constrains bridges, this constrains the model - a forged request
// with an arbitrary string is rejected).
var allowedNetworkModels = map[string]bool{
	"virtio":  true,
	"e1000":   true,
	"rtl8139": true,
	"vmxnet3": true,
}

// CreateRequest is the single creation request shape both frontend modes
// build. It deliberately carries no pool field (nothing to forge) and no mode field (the server
// cannot tell and does not care which
// wizard produced it).
//
// The VM source is exactly one of: an ISO (for OS without cloud images -
// Windows, appliances), a Proxmox template (for cloud-init-capable images),
// or a cloud image (imported as the primary disk, configured by cloud-init).
// The three are mutually exclusive: a request carrying
// more than one is rejected with ErrInvalidSource before any VMID is
// allocated.
type CreateRequest struct {
	Cluster   string `json:"cluster"`
	Name      string `json:"name"`
	ProfileID string `json:"profileId,omitempty"`

	Node       string         `json:"node,omitempty"`
	Tags       []string       `json:"tags,omitempty"`
	Sockets    int            `json:"sockets,omitempty"`
	CPUCores   int            `json:"cpuCores,omitempty"`
	MemoryMB   int            `json:"memoryMB,omitempty"`
	Disk       DiskRequest    `json:"disk"`
	Network    NetworkRequest `json:"network"`
	ISO        *ISORequest    `json:"iso,omitempty"`
	TemplateID int            `json:"templateId,omitempty"`
	// UEFI requests bios=ovmf + machine=q35 + efidisk0.
	// Pointer so "omitted" (default true - modern OSes expect UEFI boot) is
	// distinguishable from an explicit false (legacy SeaBIOS). TPM requests
	// tpmstate0 alongside the EFI disk; ignored when UEFI is false - TPM 2.0
	// requires UEFI. TPM stays off by default even though UEFI doesn't.
	// There is deliberately no Secure Boot field: the EFI disk is always
	// provisioned with an empty key store, because an approved ISO's signing
	// is unknown and an unsigned one would never boot (see
	// cluster.setUEFIFormKeys).
	UEFI             *bool `json:"uefi,omitempty"`
	TPM              bool  `json:"tpm,omitempty"`
	StartAfterCreate bool  `json:"startAfterCreate,omitempty"`
}

// DiskRequest is the request's single initial disk.
type DiskRequest struct {
	Storage string `json:"storage,omitempty"`
	SizeGB  int    `json:"sizeGB,omitempty"`
}

// NICRequest is one network interface card the request asks to attach.
type NICRequest struct {
	Bridge string `json:"bridge,omitempty"`
	Model  string `json:"model,omitempty"`
}

// NetworkRequest is the request's list of initial NICs (multi-NIC).
// Simple mode sends one entry; detailed mode may send several. A nil or empty
// list is treated as a single auto-selected NIC by resolveResources.
type NetworkRequest []NICRequest

// ISORequest is an optional installation ISO.
type ISORequest struct {
	Storage string `json:"storage"`
	File    string `json:"file"`
}

// HardwareUpdater is the post-clone mutation contract. After the clone task completes, the
// caller applies hardware
// overrides (cores/memory/sockets), resizes the disk if enlargement is
// requested, and starts the VM if StartAfterCreate is set. Delete is included
// for the rollback path (purge a half-made VM after a failed create/clone task). Defined as a
// narrow interface so vm.Create depends only
// on the methods it calls, not the full Writer surface; cluster.Fake and the
// real Proxmox client both satisfy it.
type HardwareUpdater interface {
	UpdateHardware(ctx context.Context, node string, vmid, sockets, cores, memoryMB int, tags []string) error
	SetTags(ctx context.Context, node string, vmid int, tags []string) error
	ResizeDisk(ctx context.Context, node string, vmid int, diskKey string, sizeGB int) error
	Action(ctx context.Context, node string, vmid int, action string) error
	Delete(ctx context.Context, node string, vmid int) error
}

// FreeSpaceChecker reads live free space from a storage backend. Used by the create path's hard
// disk-space check before VMID
// allocation. Defined as a narrow interface so vm.Create depends only
// on the method it calls; cluster.Fake and the real Proxmox client both
// satisfy it.
type FreeSpaceChecker interface {
	StorageFreeSpace(ctx context.Context, node, storage string) (int64, error)
}

// CreateResult is what a successful creation returns - the task is accepted,
// the VM does not necessarily exist yet.
type CreateResult struct {
	Cluster string
	VMID    int
	Name    string
	Node    string
	UPID    string
}

// CreateDeps groups the collaborators vm.Create needs beyond the per-request
// arguments (ctx, actor, clusterName, req). Bundling them keeps Create's
// parameter count under go:S107's ceiling without losing any dependency.
type CreateDeps struct {
	Store     *store.Store
	Creator   cluster.Creator
	Writer    HardwareUpdater
	FreeSpace FreeSpaceChecker
	Audit     AuditRecorder
	Log       *slog.Logger
	Services  []*policy.Policy
	// VMIDMin and VMIDMax bound the ids given to new VMs (both 0 = Proxmox's
	// next free id). Create fills them from the cluster policy.
	VMIDMin, VMIDMax int
	// Templates is the clone-time freshness backstop's reader: one TemplateByVMID call before a
	// VMID is spent. Nil skips the
	// backstop (unit tests without the live path).
	Templates TemplateReader
}

// TemplateReader is the clone path's single-template discovery capability
// (TemplateByVMID). Kept narrow so tests can stub discovery.
type TemplateReader interface {
	TemplateByVMID(ctx context.Context, vmid int) (cluster.TemplateVM, error)
}

// Create validates a creation request and dispatches it as an asynchronous
// cluster task:
//
// 1. a non-admin actor must have a personal pool; admins are exempt
// 2. the name must be a valid hostname
// 3. a profile's catalog values override any request hardware fields
// 4. CPU/memory/disk must be within the technical ceiling
// 5. unset node/storage/bridge are auto-selected from the first approved
// catalog entries; every referenced resource must be a catalog
// member
// 6. the VMID comes from the cluster client's single allocation point
// the pool is always the actor's own; the pvmss tag
// is always present
// 7. the dispatch is recorded in the audit log
//
// Index invalidation is NOT done here - the VM does not exist yet;
// the task-status handler invalidates when the task reaches ok.
//
// A step-7 audit-write failure does not fail the request: the cluster task
// from step 6 is already dispatched and real, so returning an error here
// would tell the client creation failed when it did not - the same
// log-don't-fail rule the task-status handler already applies to a failed
// post-completion invalidation (tasks.go).
func Create(ctx context.Context, actor auth.Identity, clusterName string, req CreateRequest, deps CreateDeps) (CreateResult, error) {
	policyService := selectPolicyService(deps.Store, deps.Services)

	gabarit, err := policyService.Gabarit(ctx, clusterName)
	if err != nil {
		return CreateResult{}, fmt.Errorf("read vmid range: %w", err)
	}

	deps.VMIDMin, deps.VMIDMax = gabarit.VMIDMin, gabarit.VMIDMax

	// Administrators (local or cluster) cannot create VMs through the
	// self-service portal - VM ownership requires a personal pool, which admins
	// do not have. A non-admin must have a personal pool to own the VM.
	if actor.IsAdmin {
		return CreateResult{}, ErrAdminCannotCreate
	}

	if actor.Pool == "" {
		return CreateResult{}, ErrNoPool
	}

	// ISO and template are mutually
	// exclusive sources.
	sources := 0
	if req.ISO != nil {
		sources++
	}

	if req.TemplateID != 0 {
		sources++
	}

	if sources > 1 {
		return CreateResult{}, fmt.Errorf("%w: request carries more than one of iso, templateId", ErrInvalidSource)
	}

	release, err := reserveCreate(ctx, policyService, clusterName, actor, req.Name)
	if err != nil {
		return CreateResult{}, err
	}

	result, err := createFromSource(ctx, policyService, deps, clusterName, actor, req)
	if err != nil {
		release()
	}

	return result, err
}

// reserveCreate validates the name, then checks quota and per-pool name
// uniqueness against the projection plus creates still in flight, so two
// quick requests cannot both pass. A malformed name reports ErrInvalidName
// before the duplicate check runs.
func reserveCreate(ctx context.Context, policyService *policy.Policy, clusterName string, actor auth.Identity, name string) (func(), error) {
	if err := ValidateName(name); err != nil {
		return nil, err
	}

	release, err := policyService.ReserveCreate(ctx, clusterName, actor, name)
	if errors.Is(err, policy.ErrNameReserved) {
		return nil, fmt.Errorf("%w: %q is already used by a VM in your pool", ErrNameTaken, name)
	}

	return release, err
}

func createFromSource(ctx context.Context, policyService *policy.Policy, deps CreateDeps, clusterName string, actor auth.Identity, req CreateRequest) (CreateResult, error) {
	if req.TemplateID != 0 {
		return createFromTemplate(ctx, policyService, deps, clusterName, actor, req)
	}

	return createFromISO(ctx, policyService, deps, clusterName, actor, req)
}
