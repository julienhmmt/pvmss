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
	// ErrNoSnippetStorage - a cloud-init template was requested but no
	// snippet-capable storage exists on the chosen node, so the snippet could
	// never be uploaded. Refused before VMID allocation (the same "never spend a VMID on a request
	// that will be rejected" discipline as the template resolution) instead of creating a VM whose
	// cloud-init is
	// silently absent.
	ErrNoSnippetStorage = errors.New("no snippet-capable storage on the selected node")
	// ErrCloudInitWriteUnavailable - a cloud-init document was requested but
	// the cluster has no snippet write target (Infrastructure › Clusters). Refused at
	// plan time, before a VMID is spent. Mapped to 409.
	ErrCloudInitWriteUnavailable = errors.New("cloud-init documents are not enabled on this cluster")
	// ErrDiskBelowImage - the requested disk size is smaller than the cloud
	// image being imported (the import lands at the image's size and only
	// grows). Refused before VMID allocation.
	ErrDiskBelowImage = errors.New("disk size below cloud image")
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
	return fmt.Errorf("%w: allocate vmid: %w", ErrClusterCreate, err)
}

// notApprovedError wraps a catalog lookup failure in ErrNotApproved.
func notApprovedError(err error) error {
	return fmt.Errorf("%w: %s", ErrNotApproved, err.Error())
}

// Image-mode fallback hardware - applied only when no profile was selected
// (a cluster with no profiles configured yet). Deliberately bigger than the
// shared technical minimum (1 vCPU/128 MB): a cloud image needs real
// headroom to boot, not just pass checkTechnicalRange. The wizard's manual
// disk field always sends a nonzero size in practice; imageDefaultDiskGB is
// a defensive floor for a direct API caller that omits it.
const (
	imageDefaultCPUCores = 1
	imageDefaultMemoryMB = 2048
	imageDefaultDiskGB   = 12
)

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
	// CloudInitTemplateID names an admin cloud-init template published on
	// the cluster. Users never author cloud-init YAML themselves.
	CloudInitTemplateID string `json:"cloudInitTemplateId,omitempty"`

	Node       string         `json:"node,omitempty"`
	Tags       []string       `json:"tags,omitempty"`
	Sockets    int            `json:"sockets,omitempty"`
	CPUCores   int            `json:"cpuCores,omitempty"`
	MemoryMB   int            `json:"memoryMB,omitempty"`
	Disk       DiskRequest    `json:"disk"`
	Network    NetworkRequest `json:"network"`
	ISO        *ISORequest    `json:"iso,omitempty"`
	TemplateID int            `json:"templateId,omitempty"`
	Image      *ImageRequest  `json:"image,omitempty"`
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

// ImageRequest is an optional cloud image source: the image is imported as
// the VM's primary disk (import-from) and configured by cloud-init on first
// boot. ImageCloudInit is mandatory in image mode - a cloud image has no
// installer, so cloud-init is the only way in.
type ImageRequest struct {
	Storage   string                `json:"storage"`
	File      string                `json:"file"`
	CloudInit ImageCloudInitRequest `json:"cloudInit"`
}

// ImageCloudInitRequest is the cloud-init configuration applied at first
// boot, delivered entirely through Proxmox's native cloud-init keys
// (ciuser/sshkeys/ipconfig0 - Writer.SetCloudInitConfig). Proxmox's REST API
// cannot write a per-VM snippet file (see cluster.Writer.HasSnippet), so
// there is deliberately no packages or raw user-data field here: neither can
// be delivered per VM. A fixed, admin-preplaced baseline snippet
// (imageBaselineSnippetFilename) is attached automatically when present,
// covering cluster-wide needs like installing qemu-guest-agent. Password is
// deliberately absent too - access is granted through SSH keys, and a
// password is set post-boot via the guest agent.
type ImageCloudInitRequest struct {
	User      string   `json:"user,omitempty"`
	SSHKeys   []string `json:"sshKeys,omitempty"`
	IPMode    string   `json:"ipMode,omitempty"` // "dhcp" (default) | "static"
	IPAddress string   `json:"ipAddress,omitempty"`
	Gateway   string   `json:"gateway,omitempty"`
}

// CloudInitPusher applies cloud-init configuration to a VM: the native keys
// (ciuser/sshkeys/ipconfig0) and the vendor-data cicustom pointing at an
// admin-published document. It never writes a file - publishing is an admin
// action (the pasted command, catalog.WriteCommand) - and HasSnippet proves the
// file is on the VM's node before any cicustom is set. Narrow consumer
// contract: cluster.Fake and the real Proxmox client both satisfy it.
type CloudInitPusher interface {
	AttachCloudInitSnippet(ctx context.Context, node, storage, filename string, vmid int) error
	SetCloudInitConfig(ctx context.Context, node string, vmid int, config cluster.CloudInitConfig) error
	HasSnippet(ctx context.Context, node, storage, filename string) (bool, error)
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

// SnippetStorageFinder resolves a snippet-capable storage on a node. planCreate resolves the
// snippet target at plan time - the same rule
// the snippet editor uses - instead of the create path guessing from the VM
// disk's storage, which is block-backed and cannot host a snippet. Narrow
// interface so vm.Create depends only on the method it calls; cluster.Fake
// and the real Proxmox client both satisfy it.
type SnippetStorageFinder interface {
	FindSnippetStorage(ctx context.Context, node string) (string, error)
}

// CreateResult is what a successful creation returns - the task is accepted,
// the VM does not necessarily exist yet.
type CreateResult struct {
	Cluster             string
	VMID                int
	Name                string
	Node                string
	UPID                string
	CloudInitTemplateID string
	CloudInitPushError  string
	// BaselineState is the delivery state of the generated cloud-init
	// baseline for image-mode VMs:
	// - "applied" - a published document (the baseline, or a template that
	//   embeds it) was attached
	// - "not_delivered" - nothing could be attached (see BaselineError)
	// - "" - not an image-mode VM (no baseline)
	BaselineState string
	// BaselineError is the reason the baseline could not be delivered, when
	// BaselineState is "not_delivered". Does NOT block the VM start - the
	// native keys (ciuser/sshkeys/ipconfig0) were already set.
	BaselineError string
	// FromImage is true when the VM was created from a cloud image
	// the create summary warns that SSH is
	// the only access until a console password is set.
	FromImage bool
}

// CreateDeps groups the collaborators vm.Create needs beyond the per-request
// arguments (ctx, actor, clusterName, req). Bundling them keeps Create's
// parameter count under go:S107's ceiling without losing any dependency.
type CreateDeps struct {
	Store     *store.Store
	Creator   cluster.Creator
	Pusher    CloudInitPusher
	Writer    HardwareUpdater
	FreeSpace FreeSpaceChecker
	Snippets  SnippetStorageFinder
	Audit     AuditRecorder
	Log       *slog.Logger
	Services  []*policy.Policy
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

	// Administrators (local or cluster) cannot create VMs through the
	// self-service portal - VM ownership requires a personal pool, which admins
	// do not have. A non-admin must have a personal pool to own the VM.
	if actor.IsAdmin {
		return CreateResult{}, ErrAdminCannotCreate
	}

	if actor.Pool == "" {
		return CreateResult{}, ErrNoPool
	}

	// ISO, template, and cloud image are mutually
	// exclusive sources.
	sources := 0
	if req.ISO != nil {
		sources++
	}

	if req.TemplateID != 0 {
		sources++
	}

	if req.Image != nil {
		sources++
	}

	if sources > 1 {
		return CreateResult{}, fmt.Errorf("%w: request carries more than one of iso, templateId, image", ErrInvalidSource)
	}

	if req.Image != nil {
		return createFromImage(ctx, policyService, deps, clusterName, actor, req)
	}

	if req.TemplateID != 0 {
		return createFromTemplate(ctx, policyService, deps, clusterName, actor, req)
	}

	return createFromISO(ctx, policyService, deps, clusterName, actor, req)
}
