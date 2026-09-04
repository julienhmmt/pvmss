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
	// ErrNoPool — a non-admin actor has no personal pool, so nothing can own
	// the VM (FR-005).
	ErrNoPool = errors.New("no personal pool")
	// ErrAdminCannotCreate — an administrator (local or cluster) cannot create
	// VMs through the self-service portal. VM ownership requires a personal
	// pool, which admins do not have. Admins manage VMs through the admin
	// pages or directly in Proxmox.
	ErrAdminCannotCreate = errors.New("administrators cannot create VMs")
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
	// ErrInvalidSource — the request carries more than one VM source (ISO,
	// template, cloud image) or none (US2/issue-02 D2a). Mapped to 400 by
	// the handler.
	ErrInvalidSource = errors.New("invalid vm source")
	// ErrInvalidRequest — the request carries an impossible combination of
	// options (US6/issue-06: TPM without UEFI). Mapped to 400 by the handler.
	ErrInvalidRequest = errors.New("invalid request")
	// ErrDiskReduction — the requested disk size is smaller than the
	// template's disk (US2/issue-02 D2c: Proxmox does not reduce disks).
	ErrDiskReduction = errors.New("disk size below template")
	// ErrInsufficientDiskSpace — the target storage does not have enough
	// free space for the requested disk (US3/issue-04 D4b: hard refusal
	// before VMID consumption).
	ErrInsufficientDiskSpace = errors.New("insufficient disk space")
	// ErrNameTaken — the actor already has a VM with the requested name in
	// their personal pool (US5/issue-05 D5b: per-pool uniqueness so two VMs
	// in a user's list are never indistinguishable). Mapped to 400 by the
	// handler with the code "name_taken".
	ErrNameTaken = errors.New("name already taken")
	// ErrNoSnippetStorage — a cloud-init template was requested but no
	// snippet-capable storage exists on the chosen node, so the snippet could
	// never be uploaded. Refused before VMID allocation (ticket 04: the same
	// "never spend a VMID on a request that will be rejected" discipline as
	// the template resolution) instead of creating a VM whose cloud-init is
	// silently absent.
	ErrNoSnippetStorage = errors.New("no snippet-capable storage on the selected node")
	// ErrDiskBelowImage — the requested disk size is smaller than the cloud
	// image being imported (the import lands at the image's size and only
	// grows). Refused before VMID allocation.
	ErrDiskBelowImage = errors.New("disk size below cloud image")
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

// Image-mode fallback hardware — applied only when no profile was selected
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
// override this with their own bus value (FR-009).
const defaultDiskBus = "scsi"

// maxVMIDRetries is the maximum number of VMID collision retries after the
// first attempt (US5/issue-05 D5c: max 3 attempts total). GET /cluster/nextid
// returns the smallest free ID without reserving it, so two concurrent
// creations can collide; retrying with a fresh VMID is not a mutation replay.
const maxVMIDRetries = 2 // 1 initial + 2 retries = 3 attempts

// auditLogMsg is the slog message used when RecordAction fails — the audit
// trail is best-effort and never blocks a successful create/clone.
const auditLogMsg = "record audit failed"

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
//
// The VM source is exactly one of: an ISO (for OS without cloud images —
// Windows, appliances), a Proxmox template (for cloud-init-capable images),
// or a cloud image (imported as the primary disk, configured by cloud-init).
// The three are mutually exclusive (US2/issue-02 D2a): a request carrying
// more than one is rejected with ErrInvalidSource before any VMID is
// allocated.
type CreateRequest struct {
	Cluster             string         `json:"cluster"`
	Name                string         `json:"name"`
	ProfileID           string         `json:"profileId,omitempty"`
	CloudInitTemplateID string         `json:"cloudInitTemplateId,omitempty"`
	Node                string         `json:"node,omitempty"`
	Tags                []string       `json:"tags,omitempty"`
	Sockets             int            `json:"sockets,omitempty"`
	CPUCores            int            `json:"cpuCores,omitempty"`
	MemoryMB            int            `json:"memoryMB,omitempty"`
	Disk                DiskRequest    `json:"disk"`
	Network             NetworkRequest `json:"network"`
	ISO                 *ISORequest    `json:"iso,omitempty"`
	TemplateID          int            `json:"templateId,omitempty"`
	Image               *ImageRequest  `json:"image,omitempty"`
	// UEFI requests bios=ovmf + machine=q35 + efidisk0 (US6/issue-06 D6a).
	// Pointer so "omitted" (default true — modern OSes expect UEFI boot) is
	// distinguishable from an explicit false (legacy SeaBIOS). TPM requests
	// tpmstate0 alongside the EFI disk; ignored when UEFI is false — TPM 2.0
	// requires UEFI. TPM stays off by default even though UEFI doesn't.
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

// NetworkRequest is the request's list of initial NICs (US2/D3a: multi-NIC).
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
// boot. ImageCloudInit is mandatory in image mode — a cloud image has no
// installer, so cloud-init is the only way in.
type ImageRequest struct {
	Storage   string                `json:"storage"`
	File      string                `json:"file"`
	CloudInit ImageCloudInitRequest `json:"cloudInit"`
}

// ImageCloudInitRequest is the cloud-init configuration applied at first
// boot, delivered entirely through Proxmox's native cloud-init keys
// (ciuser/sshkeys/ipconfig0 — Writer.SetCloudInitConfig). Proxmox's REST API
// cannot write a per-VM snippet file (see cluster.Writer.HasSnippet), so
// there is deliberately no packages or raw user-data field here: neither can
// be delivered per VM. A fixed, admin-preplaced baseline snippet
// (imageBaselineSnippetFilename) is attached automatically when present,
// covering cluster-wide needs like installing qemu-guest-agent. Password is
// deliberately absent too — access is granted through SSH keys, and a
// password is set post-boot via the guest agent.
type ImageCloudInitRequest struct {
	User      string   `json:"user,omitempty"`
	SSHKeys   []string `json:"sshKeys,omitempty"`
	IPMode    string   `json:"ipMode,omitempty"` // "dhcp" (default) | "static"
	IPAddress string   `json:"ipAddress,omitempty"`
	Gateway   string   `json:"gateway,omitempty"`
}

// CloudInitPusher applies cloud-init configuration to a VM — T08's
// cluster.Writer.PushCloudInitSnippet, reused verbatim by the creation-time
// template apply step (FR-007), plus the native-key and baseline-snippet
// methods image mode uses (Proxmox's REST API cannot write a per-VM snippet
// file — see cluster.Writer.HasSnippet). Defined here as a narrow consumer
// contract so vm.Create depends only on the methods it actually calls, not
// the full Writer surface; cluster.Fake and the real Proxmox client both
// satisfy it.
type CloudInitPusher interface {
	PushCloudInitSnippet(ctx context.Context, node, storage, filename string, vmid int, content string) error
	AttachCloudInitSnippet(ctx context.Context, node, storage, filename string, vmid int) error
	SetCloudInitConfig(ctx context.Context, node string, vmid int, config cluster.CloudInitConfig) error
	HasSnippet(ctx context.Context, node, storage, filename string) (bool, error)
}

// HardwareUpdater is the post-clone mutation contract (US2/issue-02 +
// lifecycle-04). After the clone task completes, the caller applies hardware
// overrides (cores/memory/sockets), resizes the disk if enlargement is
// requested, and starts the VM if StartAfterCreate is set. Delete is included
// for the rollback path (US5/issue-05 D5a: purge a half-made VM after a failed
// create/clone task). Defined as a narrow interface so vm.Create depends only
// on the methods it calls, not the full Writer surface; cluster.Fake and the
// real Proxmox client both satisfy it.
type HardwareUpdater interface {
	UpdateHardware(ctx context.Context, node string, vmid, sockets, cores, memoryMB int, tags []string) error
	SetTags(ctx context.Context, node string, vmid int, tags []string) error
	ResizeDisk(ctx context.Context, node string, vmid int, diskKey string, sizeGB int) error
	Action(ctx context.Context, node string, vmid int, action string) error
	Delete(ctx context.Context, node string, vmid int) error
}

// FreeSpaceChecker reads live free space from a storage backend (US3/issue-04
// T045). Used by the create path's hard disk-space check before VMID
// allocation (D4b). Defined as a narrow interface so vm.Create depends only
// on the method it calls; cluster.Fake and the real Proxmox client both
// satisfy it.
type FreeSpaceChecker interface {
	StorageFreeSpace(ctx context.Context, node, storage string) (int64, error)
}

// SnippetStorageFinder resolves a snippet-capable storage on a node (ticket
// 04). planCreate resolves the snippet target at plan time — the same rule
// the snippet editor uses — instead of the create path guessing from the VM
// disk's storage, which is block-backed and cannot host a snippet. Narrow
// interface so vm.Create depends only on the method it calls; cluster.Fake
// and the real Proxmox client both satisfy it.
type SnippetStorageFinder interface {
	FindSnippetStorage(ctx context.Context, node string) (string, error)
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
	Store     *store.Store
	Creator   cluster.Creator
	Pusher    CloudInitPusher
	Writer    HardwareUpdater
	FreeSpace FreeSpaceChecker
	Snippets  SnippetStorageFinder
	Audit     AuditRecorder
	Log       *slog.Logger
	Services  []*policy.Policy
	// Templates is the clone-time freshness backstop's reader (T17, issue
	// 02): one TemplateByVMID call before a VMID is spent. Nil skips the
	// backstop (unit tests without the live path).
	Templates TemplateReader
}

// TemplateReader is the clone path's single-template discovery capability
// (issue 03's TemplateByVMID). Kept narrow so tests can stub discovery.
type TemplateReader interface {
	TemplateByVMID(ctx context.Context, vmid int) (cluster.TemplateVM, error)
}

// Create validates a creation request and dispatches it as an asynchronous
// cluster task (T06 data-model.md, steps in order):
//
//  1. a non-admin actor must have a personal pool (FR-005); admins are exempt
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

	// FR-005: administrators (local or cluster) cannot create VMs through the
	// self-service portal — VM ownership requires a personal pool, which admins
	// do not have. A non-admin must have a personal pool to own the VM.
	if actor.IsAdmin {
		return CreateResult{}, ErrAdminCannotCreate
	}

	if actor.Pool == "" {
		return CreateResult{}, ErrNoPool
	}

	// US2/issue-02 D2a: ISO, template, and cloud image are mutually
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

// createFromImage is the cloud-image path: CreateVM with import-from (the
// image becomes the primary disk at the image's size, grown to the requested
// size after the task), then cloud-init delivered through Proxmox's native
// keys (see applyImageCloudInitConfig — the REST API cannot write a per-VM
// snippet file). Cloud-init is mandatory in image mode — the image is a full
// disk, so there is no installer and no second boot path. The VM is never
// started inside the create task: the network/identity config is not applied
// yet, and cloud-init does not replay on the next boot without
// `cloud-init clean`. It is started explicitly after that config lands.
func createFromImage(ctx context.Context, policyService *policy.Policy, deps CreateDeps, clusterName string, actor auth.Identity, req CreateRequest) (CreateResult, error) {
	// Image mode without a profile: the wizard sends only the image and the
	// disk size, so CPU/memory come through zero. Default to
	// imageDefault{CPUCores,MemoryMB} rather than the shared technical
	// minimum (1 vCPU/128 MB) — a cloud image needs real headroom to boot.
	// req.ProfileID != "" skips this: resolveHardware overwrites these
	// fields with the profile's values regardless (FR-009).
	if req.ProfileID == "" {
		if req.CPUCores == 0 {
			req.CPUCores = imageDefaultCPUCores
		}

		if req.MemoryMB == 0 {
			req.MemoryMB = imageDefaultMemoryMB
		}

		if req.Disk.SizeGB == 0 {
			req.Disk.SizeGB = imageDefaultDiskGB
		}
	}

	plan, err := planCreate(ctx, policyService, deps, clusterName, actor, req)
	if err != nil {
		return CreateResult{}, err
	}

	// Never start inside the create task: the snippet is not attached yet,
	// and cloud-init does not replay on the next boot without
	// `cloud-init clean`. The original value drives the explicit start after
	// attachment.
	startAfterCreate := req.StartAfterCreate
	req.StartAfterCreate = false

	vmid, err := deps.Creator.NextVMID(ctx)
	if err != nil {
		return CreateResult{}, fmt.Errorf("%w: allocate vmid: %w", ErrClusterCreate, err)
	}

	spec := buildCreateSpec(actor, req, plan, vmid)

	finalVMID, upid, err := dispatchCreateWithRetry(ctx, deps, spec)
	if err != nil {
		return CreateResult{}, err
	}

	result := CreateResult{Cluster: clusterName, VMID: finalVMID, Name: req.Name, Node: plan.node, UPID: upid}

	// The create task must finish before the snippet is attached — the PUT
	// hits a 500 "VM is locked (create)" otherwise. A wait failure is a
	// failed create task: the half-made VM is purged best-effort (US5/
	// issue-05 D5a).
	if waitErr := waitCreateTask(ctx, deps.Creator, upid); waitErr != nil {
		deps.Log.Error("create task wait failed", "component", "vm", "cluster", clusterName, "vmid", finalVMID, "error", waitErr)
		result.CloudInitPushError = waitErr.Error()

		rollbackFailedCreate(ctx, deps, actor, clusterName, finalVMID, spec.Node, "create task failed")

		if err := deps.Audit.RecordAction(ctx, actor.Username, clusterName, finalVMID, "vm_create"); err != nil {
			deps.Log.Error(auditLogMsg, "component", "vm", "cluster", clusterName, "vmid", finalVMID, "error", err)
		}

		return result, nil
	}

	// import-from lands the disk at the source image's size (Proxmox
	// requires the :0 target syntax); grow it to the requested size now that
	// the create task released the VM lock. ResizeDisk only grows, so the
	// call is skipped when the request matches the image size. A resize
	// failure does not abort — the VM exists — it is recorded like the other
	// post-create steps.
	if plan.imageSizeGB > 0 && plan.diskGB > plan.imageSizeGB && deps.Writer != nil {
		if err := deps.Writer.ResizeDisk(ctx, spec.Node, finalVMID, spec.Disk.Bus+"0", plan.diskGB); err != nil {
			deps.Log.Error("image disk resize failed", "component", "vm", "cluster", clusterName, "vmid", finalVMID, "error", err)
			result.CloudInitPushError = err.Error()
		}
	}

	applyImageCloudInitConfig(ctx, imageCloudInitApply{
		Deps: deps, ClusterName: clusterName,
		Spec: spec, VMID: finalVMID, SnippetStorage: plan.snippetStorage, CloudInit: req.Image.CloudInit,
	}, &result)

	// Auto-start (image mode): the VM is fully configured at first boot, so
	// start it explicitly after the snippet is attached — the first boot sees
	// cloud-init.
	if startAfterCreate && result.CloudInitPushError == "" && deps.Writer != nil {
		if err := deps.Writer.Action(ctx, spec.Node, finalVMID, "start"); err != nil {
			deps.Log.Error("post-cloudinit start failed", "component", "vm", "cluster", clusterName, "vmid", finalVMID, "error", err)
		}
	}

	if err := deps.Audit.RecordAction(ctx, actor.Username, clusterName, finalVMID, "vm_create"); err != nil {
		deps.Log.Error(auditLogMsg, "component", "vm", "cluster", clusterName, "vmid", finalVMID, "error", err)
	}

	return result, nil
}

// imageBaselineSnippetFilename is the fixed, admin-preplaced vendor-data
// snippet image mode attaches when present. Proxmox's REST API cannot write
// a snippets file (Writer.HasSnippet), so PVMSS never generates or uploads
// one — an admin creates this ONE file once (e.g. to install
// qemu-guest-agent) via direct filesystem access to the node, and every
// image-mode VM picks it up automatically. Absent is a normal, silent state,
// not an error: most clusters simply have not set one up yet.
const imageBaselineSnippetFilename = "pvmss-baseline.yml"

// imageCloudInitApply bundles the inputs to applyImageCloudInitConfig.
type imageCloudInitApply struct {
	Deps           CreateDeps
	ClusterName    string
	Spec           cluster.VMSpec
	VMID           int
	SnippetStorage string
	CloudInit      ImageCloudInitRequest
}

// applyImageCloudInitConfig delivers image-mode cloud-init through Proxmox's
// native keys (ciuser/sshkeys/ipconfig0 — the only per-VM mechanism the REST
// API actually supports; see cluster.Writer.HasSnippet), then attaches the
// fixed baseline snippet when an admin has placed one. A failure does NOT
// abort the creation (the task succeeded and the VM exists): it records the
// error on result.CloudInitPushError (FR-008), which also blocks the
// post-attach auto-start.
func applyImageCloudInitConfig(ctx context.Context, cfg imageCloudInitApply, result *CreateResult) {
	config := cluster.CloudInitConfig{
		User:    cfg.CloudInit.User,
		SSHKeys: cfg.CloudInit.SSHKeys,
		IPMode:  cluster.CloudInitIPModeDHCP,
	}

	if cfg.CloudInit.IPMode == "static" && cfg.CloudInit.IPAddress != "" {
		config.IPMode = cluster.CloudInitIPModeStatic
		config.IPAddress = cfg.CloudInit.IPAddress
		config.Gateway = cfg.CloudInit.Gateway
	}

	if err := cfg.Deps.Pusher.SetCloudInitConfig(ctx, cfg.Spec.Node, cfg.VMID, config); err != nil {
		cfg.Deps.Log.Error("cloud-init config set failed", "component", "vm", "cluster", cfg.ClusterName, "vmid", cfg.VMID, "error", err)
		result.CloudInitPushError = err.Error()

		return
	}

	present, err := cfg.Deps.Pusher.HasSnippet(ctx, cfg.Spec.Node, cfg.SnippetStorage, imageBaselineSnippetFilename)
	if err != nil {
		// Best-effort: a lookup failure just means the baseline is skipped —
		// the VM still got its identity and network from the native keys
		// above, so this does not block start.
		cfg.Deps.Log.Error("baseline snippet lookup failed", "component", "vm", "cluster", cfg.ClusterName, "vmid", cfg.VMID, "error", err)

		return
	}

	if !present {
		return
	}

	if err := cfg.Deps.Pusher.AttachCloudInitSnippet(ctx, cfg.Spec.Node, cfg.SnippetStorage, imageBaselineSnippetFilename, cfg.VMID); err != nil {
		cfg.Deps.Log.Error("baseline snippet attach failed", "component", "vm", "cluster", cfg.ClusterName, "vmid", cfg.VMID, "error", err)
		result.CloudInitPushError = err.Error()
	}
}

// createFromISO is the original creation path (T06): CreateVM with an optional
// ISO, then cloud-init snippet attachment. lifecycle-04 adds waitCreateTask
// before cloud-init attachment so the Proxmox create lock is released, and
// forces StartAfterCreate off when a cloud-init template is requested (the VM
// is started explicitly after the snippet is attached, preventing a first boot
// without cloud-init).
func createFromISO(ctx context.Context, policyService *policy.Policy, deps CreateDeps, clusterName string, actor auth.Identity, req CreateRequest) (CreateResult, error) {
	plan, err := planCreate(ctx, policyService, deps, clusterName, actor, req)
	if err != nil {
		return CreateResult{}, err
	}

	// T18 step (FR-006): resolve a cloud-init template BEFORE NextVMID so an
	// unknown or disabled id is rejected without burning a VMID — the same
	// "never spend a VMID on a request that will be rejected" discipline T06
	// applies to node/storage/bridge/ISO catalog membership.
	cloudTemplate, err := resolveCloudInitTemplate(ctx, deps.Store, clusterName, req.CloudInitTemplateID)
	if err != nil {
		return CreateResult{}, err
	}

	// lifecycle-04: when a cloud-init template is requested, do not let
	// Proxmox start the VM in the same create task — the snippet is not
	// attached yet, and cloud-init does not replay on the next boot without
	// `cloud-init clean`. The VM is started explicitly after attachment.
	// Capture the original request so applyCloudInitAfterWait can start the
	// VM after cloud-init is attached.
	startAfterCreate := req.StartAfterCreate
	if cloudTemplate.ID != "" {
		req.StartAfterCreate = false
	}

	vmid, err := deps.Creator.NextVMID(ctx)
	if err != nil {
		return CreateResult{}, fmt.Errorf("%w: allocate vmid: %w", ErrClusterCreate, err)
	}

	spec := buildCreateSpec(actor, req, plan, vmid)

	finalVMID, upid, err := dispatchCreateWithRetry(ctx, deps, spec)
	if err != nil {
		return CreateResult{}, err
	}

	spec.VMID = finalVMID
	result := CreateResult{Cluster: clusterName, VMID: finalVMID, Name: req.Name, Node: plan.node, UPID: upid}

	// lifecycle-04: wait for the create task to finish before attaching
	// cloud-init. Without this, the PUT /nodes/{node}/qemu/{vmid}/config
	// hits a 500 "VM is locked (create)" from Proxmox. Only wait when a
	// cloud-init template is requested — a simple ISO creation with no
	// post-processing must not become a long HTTP request.
	if cloudTemplate.ID != "" {
		applyCloudInitAfterWait(ctx, cloudInitWaitRequest{
			Deps: deps, Actor: actor, ClusterName: clusterName,
			Spec: spec, VMID: finalVMID, Template: cloudTemplate, UPID: upid,
			StartAfterCreate: startAfterCreate,
			SnippetStorage:   plan.snippetStorage,
		}, &result)
	}

	if err := deps.Audit.RecordAction(ctx, actor.Username, clusterName, finalVMID, "vm_create"); err != nil {
		deps.Log.Error(auditLogMsg, "component", "vm", "cluster", clusterName, "vmid", finalVMID, "error", err)
	}

	return result, nil
}

// cloudInitWaitRequest bundles the inputs to applyCloudInitAfterWait
// (lifecycle-04). Extracted from createFromISO to keep the nesting under
// nestif's ceiling.
type cloudInitWaitRequest struct {
	Deps             CreateDeps
	Actor            auth.Identity
	ClusterName      string
	Spec             cluster.VMSpec
	VMID             int
	Template         catalog.CloudInitTemplate
	UPID             string
	StartAfterCreate bool
	// SnippetStorage is the plan-time-resolved snippet-capable storage
	// (ticket 04).
	SnippetStorage string
}

// applyCloudInitAfterWait waits for the create task, then attaches the
// cloud-init snippet and starts the VM if requested. A wait failure is
// treated as a failed create task (US5/issue-05 D5a): the half-made VM is
// purged best-effort so it does not consume the user's quota. An attach
// failure is recorded on result.CloudInitPushError but does not abort — the
// task succeeded and the VM exists (lifecycle-04).
func applyCloudInitAfterWait(ctx context.Context, req cloudInitWaitRequest, result *CreateResult) {
	if waitErr := waitCreateTask(ctx, req.Deps.Creator, req.UPID); waitErr != nil {
		req.Deps.Log.Error("create task wait failed", "component", "vm", "cluster", req.ClusterName, "vmid", req.VMID, "error", waitErr)
		result.CloudInitPushError = waitErr.Error()

		// US5/issue-05 D5a: the create task failed, so the VM is half-made.
		// Purge it best-effort so it does not eat the user's quota.
		rollbackFailedCreate(ctx, req.Deps, req.Actor, req.ClusterName, req.VMID, req.Spec.Node, "create task failed")

		return
	}

	applyCloudInitTemplate(ctx, cloudInitApplyRequest{
		Deps: req.Deps, ClusterName: req.ClusterName,
		Spec: req.Spec, VMID: req.VMID, Template: req.Template,
		SnippetStorage: req.SnippetStorage,
	}, result)

	// lifecycle-04: start the VM explicitly after the snippet is attached,
	// so the first boot sees cloud-init.
	if req.StartAfterCreate && result.CloudInitPushError == "" && req.Deps.Writer != nil {
		if startErr := req.Deps.Writer.Action(ctx, req.Spec.Node, req.VMID, "start"); startErr != nil {
			req.Deps.Log.Error("post-cloudinit start failed", "component", "vm", "cluster", req.ClusterName, "vmid", req.VMID, "error", startErr)
		}
	}
}

// createFromTemplate is the clone path (US2/issue-02): CloneVM from an approved
// Proxmox template, wait for the clone task, then apply post-clone configuration
// (hardware overrides, disk resize, cloud-init, start). The clone stays on the
// template's node (D2b: cross-node clone is forbidden).
func createFromTemplate(ctx context.Context, policyService *policy.Policy, deps CreateDeps, clusterName string, actor auth.Identity, req CreateRequest) (CreateResult, error) {
	tmpl, err := resolveTemplate(ctx, deps.Store, clusterName, req.TemplateID)
	if err != nil {
		return CreateResult{}, err
	}

	tmpl, err = refreshTemplateFromDiscovery(ctx, deps, clusterName, tmpl)
	if err != nil {
		return CreateResult{}, err
	}

	// D2b: the clone stays on the template's node. Override any client-
	// supplied node — the selector is hidden in the UI, but a forged request
	// must not place the clone on a different node.
	req.Node = tmpl.Node

	// D2c: a zero disk size means "use the template's size" — the clone
	// keeps the template's disk as-is, no resize needed. Default it before
	// planCreate so checkTechnicalRange (MinDiskGB=1) does not reject a
	// zero, and so applyPostCloneConfig sees the template's size (no
	// enlargement triggers).
	if req.Disk.SizeGB == 0 {
		req.Disk.SizeGB = tmpl.DiskSizeGB
	}

	// Simple-mode template requests omit cpuCores/memoryMB — the clone
	// inherits the template's hardware. Default the zeros to the minimums
	// so checkTechnicalRange passes, and track that no override was
	// requested so applyPostCloneConfig skips UpdateHardware (which would
	// otherwise shrink the clone to 1 vCPU / 128 MB).
	hardwareOverride := defaultTemplateHardware(&req)

	plan, err := planCreate(ctx, policyService, deps, clusterName, actor, req)
	if err != nil {
		return CreateResult{}, err
	}

	// D2c: reject disk reduction before VMID allocation. Run after
	// planCreate so the check sees the resolved disk size — a forged
	// request carrying both a profileId (whose DiskGB may be smaller than
	// the template's) and a templateId would otherwise bypass this guard.
	// The plan's diskGB is what applyPostCloneConfig uses for the resize
	// decision, so it is the right value to compare against.
	if err := checkDiskReduction(plan.diskGB, tmpl); err != nil {
		return CreateResult{}, err
	}

	cloudTemplate, err := resolveCloudInitTemplate(ctx, deps.Store, clusterName, req.CloudInitTemplateID)
	if err != nil {
		return CreateResult{}, err
	}

	// lifecycle-04: do not start in the clone task when cloud-init is
	// requested. The VM is started after snippet attachment. Capture the
	// original request so applyPostCloneConfig can start the VM after
	// cloud-init is attached.
	startAfterCreate := req.StartAfterCreate
	if cloudTemplate.ID != "" {
		req.StartAfterCreate = false
	}

	vmid, err := deps.Creator.NextVMID(ctx)
	if err != nil {
		return CreateResult{}, fmt.Errorf("%w: allocate vmid: %w", ErrClusterCreate, err)
	}

	cloneSpec := buildCloneSpec(tmpl, plan, req, vmid, actor.Pool)

	finalVMID, upid, err := dispatchCloneWithRetry(ctx, deps, cloneSpec)
	if err != nil {
		return CreateResult{}, err
	}

	cloneSpec.NewVMID = finalVMID
	result := CreateResult{Cluster: clusterName, VMID: finalVMID, Name: req.Name, Node: tmpl.Node, UPID: upid}

	// lifecycle-04: wait for the clone task to finish before any post-clone
	// configuration. The VM does not exist until the task completes.
	// US5/issue-05 D5a: if the task fails, the half-made VM is purged
	// (best-effort) so it does not consume the user's quota.
	if waitErr := waitCreateTask(ctx, deps.Creator, upid); waitErr != nil {
		deps.Log.Error("clone task wait failed", "component", "vm", "cluster", clusterName, "vmid", finalVMID, "error", waitErr)
		result.CloudInitPushError = waitErr.Error()

		rollbackFailedCreate(ctx, deps, actor, clusterName, finalVMID, tmpl.Node, "clone task failed")

		if err := deps.Audit.RecordAction(ctx, actor.Username, clusterName, finalVMID, "vm_create"); err != nil {
			deps.Log.Error(auditLogMsg, "component", "vm", "cluster", clusterName, "vmid", finalVMID, "error", err)
		}

		return result, nil
	}

	applyPostCloneConfig(ctx, postCloneConfig{
		Deps: deps, ClusterName: clusterName,
		VMID: finalVMID, Node: tmpl.Node, Plan: plan, Template: tmpl,
		CloudTemplate: cloudTemplate, StartAfterCreate: startAfterCreate,
		Tags: buildTags(req), DiskKey: primaryDiskKey(tmpl.DiskBus),
		HardwareOverride: hardwareOverride,
	}, &result)

	if err := deps.Audit.RecordAction(ctx, actor.Username, clusterName, finalVMID, "vm_create"); err != nil {
		deps.Log.Error(auditLogMsg, "component", "vm", "cluster", clusterName, "vmid", finalVMID, "error", err)
	}

	return result, nil
}

// refreshTemplateFromDiscovery is the clone-time freshness backstop (T17,
// issue 02). The stored row is the approval; discovery is the truth about
// the template's current values. Re-check before a VMID is spent: a template
// deleted since approval fails fast (ErrNotApproved) instead of failing
// after a VMID is consumed.
func refreshTemplateFromDiscovery(ctx context.Context, deps CreateDeps, clusterName string, tmpl catalog.Template) (catalog.Template, error) {
	if deps.Templates == nil {
		return tmpl, nil
	}

	live, err := deps.Templates.TemplateByVMID(ctx, tmpl.VMID)
	switch {
	case errors.Is(err, cluster.ErrNotFound):
		return catalog.Template{}, fmt.Errorf("%w: template no longer exists", ErrNotApproved)
	case err != nil:
		// Discovery is unavailable — the approval still stands; the clone
		// itself will fail if the cluster is truly unreachable.
		deps.Log.Warn("template freshness check failed, proceeding with stored values", "component", "vm", "cluster", clusterName, "vmid", tmpl.VMID, "error", err)

		return tmpl, nil
	case live.DiskUnreadable:
		// Keep the discovered node; the stored disk fields were validated at
		// approval and are never empty.
		deps.Log.Warn("template disk unreadable at clone time, using stored disk fields", "component", "vm", "cluster", clusterName, "vmid", tmpl.VMID)

		tmpl.Node = live.Node

		return tmpl, nil
	default:
		tmpl.Node = live.Node
		tmpl.Name = live.Name
		tmpl.CloudInitCapable = live.CloudInitCapable
		tmpl.DiskStorage = live.DiskStorage
		tmpl.DiskSizeGB = live.DiskSizeGB
		tmpl.DiskBus = live.DiskBus

		return tmpl, nil
	}
}

// dispatchCreateWithRetry dispatches a CreateVM call, retrying with a fresh
// VMID when Proxmox reports a collision (US5/issue-05 D5c: max 3 attempts).
// A retry with a new VMID is not a mutation replay — the original create never
// succeeded, so ProxMate's idempotency concern does not apply. Returns the
// final VMID (which may differ from spec.VMID after a retry) and the UPID.
func dispatchCreateWithRetry(ctx context.Context, deps CreateDeps, spec cluster.VMSpec) (int, string, error) {
	return retryWithFreshVMID(ctx, deps, spec.VMID, "create", func(vmid int) (string, error) {
		spec.VMID = vmid

		return deps.Creator.CreateVM(ctx, spec)
	})
}

// dispatchCloneWithRetry dispatches a CloneVM call with the same VMID collision
// retry as dispatchCreateWithRetry (US5/issue-05 D5c). Returns the final
// NewVMID (which may differ after a retry) and the UPID.
func dispatchCloneWithRetry(ctx context.Context, deps CreateDeps, spec cluster.CloneSpec) (int, string, error) {
	return retryWithFreshVMID(ctx, deps, spec.NewVMID, "clone", func(vmid int) (string, error) {
		spec.NewVMID = vmid

		return deps.Creator.CloneVM(ctx, spec)
	})
}

// retryWithFreshVMID is the shared VMID-collision retry loop (US5/issue-05
// D5c). dispatch is called with the current VMID; on ErrVMIDTaken it allocates
// a fresh VMID and retries, up to maxVMIDRetries times. label is "create" or
// "clone" for the error message and log.
func retryWithFreshVMID(ctx context.Context, deps CreateDeps, initialVMID int, label string, dispatch func(vmid int) (string, error)) (int, string, error) {
	vmid := initialVMID

	for attempt := 0; ; attempt++ {
		upid, err := dispatch(vmid)
		if err == nil {
			return vmid, upid, nil
		}

		if !errors.Is(err, cluster.ErrVMIDTaken) || attempt >= maxVMIDRetries {
			return 0, "", fmt.Errorf("%w: %s: %w", ErrClusterCreate, label, err)
		}

		deps.Log.Info("vmid collision, retrying", "component", "vm", "vmid", vmid, "attempt", attempt+1)

		newVMID, err := deps.Creator.NextVMID(ctx)
		if err != nil {
			return 0, "", fmt.Errorf("%w: allocate vmid on retry: %w", ErrClusterCreate, err)
		}

		vmid = newVMID
	}
}

// rollbackFailedCreate purges a half-made VM after a failed create or clone
// task (US5/issue-05 D5a). The cleanup is best-effort: a failure is logged and
// does not mask the original error. An audit entry is recorded so the orphan
// is traceable — ProxMate deliberately kept half-made VMs, but in a self-
// service portal an orphan consuming the user's quota is indefensible.
// The Writer interface is used (not Creator) because Delete is a Writer
// method; when no Writer is wired (unit tests without the live path), the
// rollback is skipped.
func rollbackFailedCreate(ctx context.Context, deps CreateDeps, actor auth.Identity, clusterName string, vmid int, node, reason string) {
	if deps.Writer == nil {
		return
	}

	if err := deps.Writer.Delete(ctx, node, vmid); err != nil {
		// Best-effort: log and move on. The original error is what the
		// caller reports; a failed cleanup must not mask it.
		deps.Log.Error("rollback: failed to purge half-made vm", "component", "vm", "cluster", clusterName, "vmid", vmid, "node", node, "reason", reason, "error", err)

		return
	}

	deps.Log.Info("rollback: purged half-made vm", "component", "vm", "cluster", clusterName, "vmid", vmid, "node", node, "reason", reason)

	if err := deps.Audit.RecordAction(ctx, actor.Username, clusterName, vmid, "vm_create_rollback"); err != nil {
		deps.Log.Error(auditLogMsg, "component", "vm", "cluster", clusterName, "vmid", vmid, "error", err)
	}
}

// defaultTemplateHardware defaults zero CPU/memory to the technical
// minimums so checkTechnicalRange passes, and returns whether the caller
// explicitly supplied hardware (so applyPostCloneConfig can skip the
// post-clone UpdateHardware that would otherwise shrink the clone).
func defaultTemplateHardware(req *CreateRequest) bool {
	override := req.CPUCores != 0 || req.MemoryMB != 0
	if req.CPUCores == 0 {
		req.CPUCores = MinCPUCores
	}

	if req.MemoryMB == 0 {
		req.MemoryMB = MinMemoryMB
	}

	return override
}

// resolveTemplate looks up the approved Proxmox template before any VMID is
// allocated (US2/issue-02). Returns ErrNotApproved for an unknown or
// disabled template — same "never spend a VMID on a rejected request"
// discipline as ISO validation.
func resolveTemplate(ctx context.Context, st *store.Store, clusterName string, templateID int) (catalog.Template, error) {
	templates, err := catalog.Templates(ctx, st, clusterName)
	if err != nil {
		return catalog.Template{}, fmt.Errorf("read templates: %w", err)
	}

	tmpl, err := catalog.FindTemplate(templates, templateID)
	if err != nil {
		return catalog.Template{}, fmt.Errorf("%w: %s", ErrNotApproved, err.Error())
	}

	return tmpl, nil
}

// checkDiskReduction rejects a disk size smaller than the template's disk
// (US2/issue-02 D2c: Proxmox does not reduce disks). The caller passes the
// resolved disk size (from planCreate, which applies profile overrides), so
// a forged request carrying both a profileId and a templateId cannot bypass
// this guard with a profile whose DiskGB is smaller than the template's.
func checkDiskReduction(diskGB int, tmpl catalog.Template) error {
	if diskGB < tmpl.DiskSizeGB {
		return fmt.Errorf("%w: requested %d GB, template disk is %d GB", ErrDiskReduction, diskGB, tmpl.DiskSizeGB)
	}

	return nil
}

// checkDiskAboveImage rejects a disk size smaller than the cloud image being
// imported (image-mode D2c: the import lands at the image's size and only
// grows afterwards, so a smaller request is a reduction). The caller passes
// the resolved disk size (from planCreate, which applies profile overrides)
// and the resolved node, so a forged request cannot bypass this guard with a
// profile whose DiskGB is smaller than the image. Returns the image's size in
// whole GB (rounded up) so the caller can grow the imported disk afterwards.
func checkDiskAboveImage(resources catalog.Resources, req CreateRequest, node string, diskGB int) (int, error) {
	image, err := resources.FindCloudImage(req.Image.Storage, req.Image.File, node)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", ErrNotApproved, err.Error())
	}

	minGB := int((image.SizeBytes + bytesPerGB - 1) / bytesPerGB)
	if int64(diskGB)*bytesPerGB < image.SizeBytes {
		return 0, fmt.Errorf("%w: requested %d GB, image %q is %d GB", ErrDiskBelowImage, diskGB, image.File, minGB)
	}

	return minGB, nil
}

// buildCloneSpec assembles the CloneSpec from the resolved template, plan,
// request, and allocated VMID (US2/issue-02 §5). Full clone when the template
// is cloud-init capable (lvmthin cannot linked-clone an imported disk), or
// when the target storage differs from the template's disk storage. Linked
// otherwise.
func buildCloneSpec(tmpl catalog.Template, plan createPlan, req CreateRequest, vmid int, pool string) cluster.CloneSpec {
	full := tmpl.CloudInitCapable || (plan.storage != "" && plan.storage != tmpl.DiskStorage)

	spec := cluster.CloneSpec{
		SourceVMID: tmpl.VMID,
		SourceNode: tmpl.Node,
		NewVMID:    vmid,
		Name:       req.Name,
		Full:       full,
		Pool:       pool,
		DiskBus:    tmpl.DiskBus,
	}

	if full && plan.storage != "" && plan.storage != tmpl.DiskStorage {
		spec.Storage = plan.storage
	}

	return spec
}

// resolveCloudInitTemplate looks up the requested cloud-init template id
// before any VMID is allocated. An empty id means no template was requested.
func resolveCloudInitTemplate(ctx context.Context, st *store.Store, clusterName, templateID string) (catalog.CloudInitTemplate, error) {
	if templateID == "" {
		return catalog.CloudInitTemplate{}, nil
	}

	tmpl, err := catalog.FindCloudInitTemplate(ctx, st, clusterName, templateID)
	if err != nil {
		return catalog.CloudInitTemplate{}, fmt.Errorf("%w: cloud-init template %q is not approved for this cluster", ErrNotApproved, templateID)
	}

	return tmpl, nil
}

// buildCreateSpec assembles the cluster.VMSpec from the validated plan,
// request, actor identity, and allocated VMID. The "pvmss" tag is always
// present (FR-004).
func buildCreateSpec(actor auth.Identity, req CreateRequest, plan createPlan, vmid int) cluster.VMSpec {
	tags := append([]string(nil), req.Tags...)
	if !slices.Contains(tags, "pvmss") {
		tags = append(tags, "pvmss")
	}

	// US6/issue-06 D6b: stamp the admin-imposed isolation VLAN on every NIC.
	// The tag comes from the per-cluster gabarit (Q18); tenants never choose
	// it. Firewall is always true (D6a — the Proxmox per-VM firewall is
	// armed by default, not user-exposed).
	nics := make([]cluster.NICSpec, 0, len(plan.nics))
	for _, nic := range plan.nics {
		spec := cluster.NICSpec{Bridge: nic.bridge, Model: nic.model, Firewall: true}

		if plan.isolationVLANTag > 0 {
			tag := plan.isolationVLANTag
			spec.VLAN = &tag
		}

		nics = append(nics, spec)
	}

	// US6/issue-06 D6a: UEFI maps to bios=ovmf; the cluster create path
	// forces machine=q35 and provisions efidisk0 (+ tpmstate0 when TPM).
	bios := ""
	if plan.uefi {
		bios = "ovmf"
	}

	spec := cluster.VMSpec{
		VMID:             vmid,
		Node:             plan.node,
		Name:             req.Name,
		Pool:             actor.Pool,
		Tags:             tags,
		Sockets:          plan.sockets,
		CPUCores:         plan.cpuCores,
		MemoryMB:         plan.memoryMB,
		Disk:             cluster.DiskSpec{Storage: plan.storage, SizeGB: plan.diskGB, Bus: plan.bus},
		Network:          cluster.NetworkSpec(nics),
		BIOS:             bios,
		TPM:              plan.tpm,
		StartAfterCreate: req.StartAfterCreate,
	}
	if req.ISO != nil {
		spec.ISO = &cluster.ISOSpec{Storage: req.ISO.Storage, File: req.ISO.File}
	}

	if req.Image != nil {
		spec.Image = &cluster.ImageSpec{Storage: req.Image.Storage, File: req.Image.File}
	}

	return spec
}

// cloudInitApplyRequest bundles the per-creation inputs to
// applyCloudInitTemplate. Keeping them in a struct holds the helper under
// go:S107's ceiling and makes the single call site self-documenting.
type cloudInitApplyRequest struct {
	Deps        CreateDeps
	ClusterName string
	Spec        cluster.VMSpec
	VMID        int
	Template    catalog.CloudInitTemplate
	// SnippetStorage is the plan-time-resolved snippet-capable storage
	// (ticket 04) — never the VM disk's storage, which is block-backed.
	SnippetStorage string
}

// templateSnippetFilename is the fixed, admin-preplaced snippet filename for
// one cloud-init template — one file per template (not per VM), reused by
// every clone. templateID is a slug (catalog.DeriveCloudInitTemplateID),
// filesystem-safe as-is.
func templateSnippetFilename(templateID string) string {
	return fmt.Sprintf("pvmss-template-%s.yml", templateID)
}

// applyCloudInitTemplate attaches the fixed, admin-preplaced snippet file for
// the resolved cloud-init template. Proxmox's REST API cannot write a
// snippets-content file (upload/download-url both reject content=snippets —
// see cluster.Writer.HasSnippet), so the template's Content field stored in
// PVMSS's catalog is reference/documentation only: an admin must place the
// matching file on the cluster (<storage>/snippets/<filename>, filesystem or
// SSH access) and keep it in sync with catalog edits themselves. A missing
// file is NOT silently skipped like image mode's optional baseline — the
// user explicitly chose a cloud-init template expecting it to apply, so its
// absence is recorded on result.CloudInitPushError like any other failure.
// A failure does NOT abort the creation (the task is already dispatched and
// cannot be undone).
func applyCloudInitTemplate(ctx context.Context, req cloudInitApplyRequest, result *CreateResult) {
	if req.Template.ID == "" {
		return
	}

	result.CloudInitTemplateID = req.Template.ID
	filename := templateSnippetFilename(req.Template.ID)
	storage := req.SnippetStorage

	present, err := req.Deps.Pusher.HasSnippet(ctx, req.Spec.Node, storage, filename)
	if err != nil {
		req.Deps.Log.Error("cloud-init template snippet lookup failed", "component", "vm", "cluster", req.ClusterName, "vmid", req.VMID, "error", err)
		result.CloudInitPushError = err.Error()

		return
	}

	if !present {
		req.Deps.Log.Error("cloud-init template snippet missing on cluster", "component", "vm", "cluster", req.ClusterName, "vmid", req.VMID, "template", req.Template.ID, "expected_filename", filename)
		result.CloudInitPushError = fmt.Sprintf("cloud-init template %q has no matching snippet file on the cluster (expected %q on a snippets-capable storage) — an admin must place it", req.Template.ID, filename)

		return
	}

	// Point the VM at the snippet through the vendor-data slot so the guest
	// actually receives it (REPORT.md addendum: previously a silent no-op).
	if err := req.Deps.Pusher.AttachCloudInitSnippet(ctx, req.Spec.Node, storage, filename, req.VMID); err != nil {
		req.Deps.Log.Error("cloud-init template attach failed", "component", "vm", "cluster", req.ClusterName, "vmid", req.VMID, "error", err)
		result.CloudInitPushError = err.Error()
	}
}

// postCloneConfig bundles the inputs to applyPostCloneConfig (US2/issue-02).
type postCloneConfig struct {
	Deps             CreateDeps
	ClusterName      string
	VMID             int
	Node             string
	Plan             createPlan
	Template         catalog.Template
	CloudTemplate    catalog.CloudInitTemplate
	StartAfterCreate bool
	Tags             []string
	DiskKey          string
	// HardwareOverride is false when the request omitted cpuCores/memoryMB
	// (simple template mode). The clone inherits the template's hardware
	// and UpdateHardware is skipped.
	HardwareOverride bool
}

// applyPostCloneConfig runs the post-clone configuration sequence (US2/issue-02
// §4, in the order ProxMate uses): hardware overrides → disk resize → cloud-init
// → start. Each step is best-effort: a failure is logged and recorded on
// result.CloudInitPushError but does not abort the remaining steps — the clone
// is already real and the VM exists.
func applyPostCloneConfig(ctx context.Context, cfg postCloneConfig, result *CreateResult) {
	applyCloneHardware(ctx, cfg, result)
	applyCloneDiskResize(ctx, cfg, result)

	// 3. Cloud-init snippet attachment (same mechanism as the ISO path).
	if cfg.CloudTemplate.ID != "" {
		applyCloudInitTemplate(ctx, cloudInitApplyRequest{
			Deps: cfg.Deps, ClusterName: cfg.ClusterName,
			Spec: cluster.VMSpec{Node: cfg.Node, Disk: cluster.DiskSpec{Storage: cfg.Plan.storage}},
			VMID: cfg.VMID, Template: cfg.CloudTemplate,
			SnippetStorage: cfg.Plan.snippetStorage,
		}, result)
	}

	// 4. Start the VM if requested (lifecycle-04: after cloud-init attachment
	// so the first boot sees the snippet).
	if cfg.StartAfterCreate && result.CloudInitPushError == "" && cfg.Deps.Writer != nil {
		if err := cfg.Deps.Writer.Action(ctx, cfg.Node, cfg.VMID, "start"); err != nil {
			cfg.Deps.Log.Error("post-clone start failed", "component", "vm", "cluster", cfg.ClusterName, "vmid", cfg.VMID, "error", err)
		}
	}
}

// applyCloneHardware applies hardware overrides when the caller explicitly
// supplied CPU/memory, and always stamps the mandatory pvmss tag (FR-006).
// In simple template mode (no hardware override), SetTags is used so the
// clone inherits the template's hardware unchanged — UpdateHardware would
// shrink it to the plan's minimums (1 vCPU / 128 MB).
func applyCloneHardware(ctx context.Context, cfg postCloneConfig, result *CreateResult) {
	if cfg.Deps.Writer == nil {
		return
	}

	if cfg.HardwareOverride {
		if err := cfg.Deps.Writer.UpdateHardware(ctx, cfg.Node, cfg.VMID, cfg.Plan.sockets, cfg.Plan.cpuCores, cfg.Plan.memoryMB, cfg.Tags); err != nil {
			cfg.Deps.Log.Error("post-clone hardware update failed", "component", "vm", "cluster", cfg.ClusterName, "vmid", cfg.VMID, "error", err)
			result.CloudInitPushError = err.Error()

			return
		}

		return
	}

	// No hardware override — still stamp the pvmss tag so the clone is
	// visible to PVMSS (FR-006). Without this, a simple-mode clone exists in
	// Proxmox but Resolve() returns ErrNotFound.
	if err := cfg.Deps.Writer.SetTags(ctx, cfg.Node, cfg.VMID, cfg.Tags); err != nil {
		cfg.Deps.Log.Error("post-clone set tags failed", "component", "vm", "cluster", cfg.ClusterName, "vmid", cfg.VMID, "error", err)
		result.CloudInitPushError = err.Error()
	}
}

// applyCloneDiskResize enlarges the clone's disk when the plan's size exceeds
// the template's (D2c: Proxmox does not reduce disks).
func applyCloneDiskResize(ctx context.Context, cfg postCloneConfig, result *CreateResult) {
	if cfg.Deps.Writer == nil || cfg.Plan.diskGB <= cfg.Template.DiskSizeGB || cfg.DiskKey == "" {
		return
	}

	if err := cfg.Deps.Writer.ResizeDisk(ctx, cfg.Node, cfg.VMID, cfg.DiskKey, cfg.Plan.diskGB); err != nil {
		cfg.Deps.Log.Error("post-clone disk resize failed", "component", "vm", "cluster", cfg.ClusterName, "vmid", cfg.VMID, "error", err)

		if result.CloudInitPushError == "" {
			result.CloudInitPushError = err.Error()
		}
	}
}

// buildTags returns the request's tags with the mandatory "pvmss" tag appended
// when absent (FR-006). Shared by both creation paths.
func buildTags(req CreateRequest) []string {
	tags := append([]string(nil), req.Tags...)
	if !slices.Contains(tags, "pvmss") {
		tags = append(tags, "pvmss")
	}

	return tags
}

// primaryDiskKey returns the Proxmox disk key (e.g. "scsi0") for the clone's
// primary disk, derived from the template's disk bus. The clone inherits the
// template's disk bus, not the plan's catalog-approved default.
func primaryDiskKey(bus string) string {
	if bus == "" {
		return "scsi0"
	}

	return bus + "0"
}

// createPlan holds the resolved and validated values for a VM creation request.
type createPlan struct {
	node    string
	storage string
	// snippetStorage is the snippet-capable storage resolved at plan time
	// when a cloud-init template was requested (ticket 04). Empty when no
	// template was requested — resolution costs a cluster read and must not
	// run on the plain ISO path.
	snippetStorage string
	sockets        int
	cpuCores       int
	memoryMB       int
	diskGB         int
	bus            string
	// imageSizeGB is the cloud image's size in whole GB (rounded up), set
	// only in image mode. import-from lands the disk at this size; the
	// caller grows it to diskGB after the create task completes.
	imageSizeGB      int
	nics             []nicPlan
	isolationVLANTag int
	uefi             bool
	tpm              bool
}

// nicPlan is one resolved and validated NIC.
type nicPlan struct {
	bridge string
	model  string
}

// checkName validates the hostname form (FR-008) then checks per-pool name
// uniqueness (US5/issue-05 D5b). A malformed name reports ErrInvalidName
// before the duplicate check runs. Extracted from planCreate to keep its
// cyclomatic complexity under gocyclo's ceiling.
func checkName(policyService *policy.Policy, pool, name string) error {
	if err := ValidateName(name); err != nil {
		return err
	}

	// US5/issue-05 D5b: per-pool name uniqueness. The name is the only
	// identifier the user manipulates in the portal; two VMs with the same
	// name in one user's list are indistinguishable. Checked before any
	// VMID is consumed, like every other rejection.
	if policyService.PoolHasName(pool, name) {
		return fmt.Errorf("%w: %q is already used by a VM in your pool", ErrNameTaken, name)
	}

	return nil
}

// resolveUEFI defaults UEFI to true when the request omits it (US6/issue-06:
// modern OSes expect UEFI boot). An explicit false selects legacy SeaBIOS.
func resolveUEFI(req CreateRequest) bool {
	if req.UEFI == nil {
		return true
	}

	return *req.UEFI
}

// checkUEFICompat rejects the impossible TPM-without-UEFI combination early
// (US6/issue-06 D6a: TPM 2.0 requires UEFI). Extracted from planCreate to
// keep its cyclomatic complexity under gocyclo's ceiling.
func checkUEFICompat(req CreateRequest) error {
	if req.TPM && !resolveUEFI(req) {
		return fmt.Errorf("%w: tpm requires uefi", ErrInvalidRequest)
	}

	return nil
}

// planCreate runs all pre-allocation validation: quota, name, catalog,
// hardware ranges, gabarit, resource resolution, node capacity, and live
// disk-space check (US3/issue-04). Name uniqueness by pool (US5/issue-05 D5b)
// is checked after ValidateName so a malformed name reports ErrInvalidName
// before the duplicate check runs.
func planCreate(ctx context.Context, policyService *policy.Policy, deps CreateDeps, clusterName string, actor auth.Identity, req CreateRequest) (createPlan, error) {
	if err := policyService.CheckQuota(ctx, clusterName, actor); err != nil {
		return createPlan{}, err
	}

	if err := checkName(policyService, actor.Pool, req.Name); err != nil {
		return createPlan{}, err
	}

	// US6/issue-06 D6a: TPM 2.0 requires UEFI — reject the impossible
	// combination early, before any VMID or catalog work.
	if err := checkUEFICompat(req); err != nil {
		return createPlan{}, err
	}

	resources, err := catalog.ApprovedResources(ctx, deps.Store, clusterName)
	if err != nil {
		return createPlan{}, fmt.Errorf("read catalog: %w", err)
	}

	sockets, cpuCores, memoryMB, diskGB, bus, err := resolveHardware(ctx, deps.Store, clusterName, req)
	if err != nil {
		return createPlan{}, err
	}

	if err := checkTechnicalRange(cpuCores, memoryMB, diskGB); err != nil {
		return createPlan{}, err
	}

	// US3/issue-04: fetch node capacities for placement scoring and storage
	// free space from the projection for best-storage selection, then resolve
	// and validate the placement (node/storage/NICs against the catalog).
	node, storage, nics, err := resolvePlacement(ctx, req, policyService, clusterName, resources, deps.Log)
	if err != nil {
		return createPlan{}, err
	}

	// D2c (image mode): reject a disk size below the cloud image before any
	// VMID is spent — the import lands at the image's size and only grows.
	// Run after planCreate so the check sees the resolved disk size (profile
	// overrides applied). The image size rides on the plan so createFromImage
	// can grow the imported disk to the requested size.
	var imageSizeGB int

	if req.Image != nil {
		imageSizeGB, err = checkDiskAboveImage(resources, req, node, diskGB)
		if err != nil {
			return createPlan{}, err
		}
	}

	if err := checkGabaritAndCapacity(ctx, policyService, gabaritRequest{
		clusterName: clusterName, node: node,
		sockets: sockets, cpuCores: cpuCores, memoryMB: memoryMB, diskGB: diskGB,
		nicCount: len(nics),
	}); err != nil {
		return createPlan{}, err
	}

	vlanTag, err := finalizePlanChecks(ctx, deps, policyService, clusterName, node, storage, diskGB)
	if err != nil {
		return createPlan{}, err
	}

	snippetStorage, err := resolvePlanSnippetStorage(ctx, deps, req, node)
	if err != nil {
		return createPlan{}, err
	}

	return createPlan{
		node: node, storage: storage, snippetStorage: snippetStorage,
		sockets: sockets, cpuCores: cpuCores,
		memoryMB: memoryMB, diskGB: diskGB, bus: bus, nics: nics,
		isolationVLANTag: vlanTag, uefi: resolveUEFI(req), tpm: req.TPM,
		imageSizeGB: imageSizeGB,
	}, nil
}

// resolvePlacement fetches node capacities and storage free space from the
// projection (US3/issue-04), resolves node/storage/NICs, validates the choice
// against the catalog, and logs the placement decision when auto-selection
// ran (T042). Extracted from planCreate to keep its cyclomatic complexity
// under gocyclo's ceiling.
func resolvePlacement(ctx context.Context, req CreateRequest, policyService *policy.Policy, clusterName string, resources catalog.Resources, log *slog.Logger) (node, storage string, nics []nicPlan, err error) {
	capacities := fetchNodeCapacities(ctx, policyService, clusterName, resources.Nodes)
	storageFree := fetchStorageFreeBytes(policyService, resources.Storages)

	node, storage, nics, err = resolveResources(req, resources, capacities, storageFree)
	if err != nil {
		return "", "", nil, err
	}

	if err := validateCatalog(req, resources, node, storage, nics); err != nil {
		return "", "", nil, err
	}

	if req.Node == "" && log != nil {
		logPlacement(log, node, resources.Nodes, capacities, req)
	}

	return node, storage, nics, nil
}

// resolvePlanSnippetStorage resolves the snippet target at plan time when a
// cloud-init template or a cloud image was requested — before NextVMID, so a
// node without any snippet-capable storage is refused without burning a VMID
// (the same discipline as the template resolution). The VM disk's storage is
// block-backed (ZFS/LVM-thin/Ceph) and cannot host a snippet, so the editor's
// FindSnippetStorage rule is the only correct source (ticket 04). Returns ""
// when neither was requested: the resolution costs a cluster read and must
// not run on the plain ISO path.
func resolvePlanSnippetStorage(ctx context.Context, deps CreateDeps, req CreateRequest, node string) (string, error) {
	if req.CloudInitTemplateID == "" && req.Image == nil {
		return "", nil
	}

	if deps.Snippets == nil {
		return "", fmt.Errorf("%w: no snippet storage resolver wired", ErrClusterCreate)
	}

	storage, err := deps.Snippets.FindSnippetStorage(ctx, node)
	if err != nil {
		return "", fmt.Errorf("%w: %s: enable the snippets content type on a storage of this node (%w)", ErrNoSnippetStorage, node, err)
	}

	return storage, nil
}

// gabaritRequest groups the resolved hardware dimensions a gabarit + capacity
// check needs. Extracted from checkGabaritAndCapacity's parameter list to
// stay under go:S107's 7-parameter ceiling.
type gabaritRequest struct {
	clusterName string
	node        string
	sockets     int
	cpuCores    int
	memoryMB    int
	diskGB      int
	nicCount    int
}

// checkGabaritAndCapacity runs the gabarit ceiling check and the node-capacity
// check together (US2/D3a + US3/issue-04). Extracted from planCreate to keep
// its cyclomatic complexity under gocyclo's ceiling.
func checkGabaritAndCapacity(ctx context.Context, policyService *policy.Policy, req gabaritRequest) error {
	if err := policyService.CheckGabarit(ctx, req.clusterName, req.sockets, req.cpuCores, req.memoryMB, req.diskGB, req.nicCount); err != nil {
		return err
	}

	return policyService.CheckNodeCapacity(ctx, req.clusterName, req.node, policy.CapacityDelta{
		Sockets: req.sockets, Cores: req.cpuCores, MemoryMB: req.memoryMB, DiskGB: req.diskGB,
	})
}

// finalizePlanChecks runs the post-capacity checks: the live disk-space
// check (US3/issue-04 D4b) and the gabarit VLAN read (US6/issue-06 D6b).
// Returns the per-cluster isolation VLAN tag (0 = none imposed). Extracted
// from planCreate to keep its cyclomatic complexity under gocyclo's ceiling.
func finalizePlanChecks(ctx context.Context, deps CreateDeps, policyService *policy.Policy, clusterName, node, storage string, diskGB int) (int, error) {
	if err := checkLiveDiskSpace(ctx, deps.FreeSpace, node, storage, diskGB); err != nil {
		return 0, err
	}

	gabarit, err := policyService.Gabarit(ctx, clusterName)
	if err != nil {
		return 0, fmt.Errorf("read gabarit for vlan: %w", err)
	}

	return gabarit.IsolationVLANTag, nil
}

// checkLiveDiskSpace verifies the target storage has enough free space for the
// requested disk (US3/issue-04 D4b). Skipped when no FreeSpaceChecker is wired
// (unit tests that don't need the live check) or when diskGB is zero.
func checkLiveDiskSpace(ctx context.Context, freeSpace FreeSpaceChecker, node, storage string, diskGB int) error {
	if freeSpace == nil || diskGB <= 0 {
		return nil
	}

	freeBytes, err := freeSpace.StorageFreeSpace(ctx, node, storage)
	if err != nil {
		return fmt.Errorf("%w: read free space on %q/%q: %w", ErrClusterCreate, node, storage, err)
	}

	needed := int64(diskGB) * bytesPerGB
	if freeBytes < needed {
		return fmt.Errorf("%w: storage %q on node %q has %d GB free, request needs %d GB", ErrInsufficientDiskSpace, storage, node, freeBytes/bytesPerGB, int64(diskGB))
	}

	return nil
}

// bytesPerGB is the conversion factor for disk-space checks.
const bytesPerGB int64 = 1024 * 1024 * 1024

// fetchNodeCapacities reads the capacity of each approved node from the policy
// service. Nodes with no configured capacité return a zero-value Capacity
// (scoreNode handles this gracefully).
func fetchNodeCapacities(ctx context.Context, policyService *policy.Policy, clusterName string, nodes []catalog.Node) map[string]policy.Capacity {
	capacities := make(map[string]policy.Capacity, len(nodes))

	for _, n := range nodes {
		nodeCap, err := policyService.NodeCapacity(ctx, clusterName, n.Name)
		if err != nil {
			continue
		}

		capacities[n.Name] = nodeCap
	}

	return capacities
}

// fetchStorageFreeBytes reads the projected free bytes for each approved
// storage from the policy service's in-memory projection. Storages not in the
// projection get 0 (bestStorageOnNode treats 0 as a valid candidate).
func fetchStorageFreeBytes(policyService *policy.Policy, storages []catalog.Storage) map[string]int64 {
	free := make(map[string]int64, len(storages))

	for _, s := range storages {
		free[s.Name] = policyService.StorageFreeBytes(s.Node, s.Name)
	}

	return free
}

// logPlacement emits a [placement] log line naming each candidate and its
// score, like ProxMate's scheduler log.
func logPlacement(log *slog.Logger, selected string, candidates []catalog.Node, capacities map[string]policy.Capacity, req CreateRequest) {
	scores := make([]string, 0, len(candidates))

	for _, n := range candidates {
		score := scoreNode(capacities[n.Name], req)
		scores = append(scores, fmt.Sprintf("%s=%.3f", n.Name, score))
	}

	log.Info("[placement] auto-selected node", "component", "vm", "selected", selected, "candidates", scores)
}

// resolveHardware returns the effective sockets, CPU, memory, disk, and bus
// values, applying the profile's catalog values when a profile is selected
// (FR-009). Sockets defaults to 1 when the request omits it (zero value).
func resolveHardware(ctx context.Context, st *store.Store, clusterName string, req CreateRequest) (sockets, cpuCores, memoryMB, diskGB int, bus string, err error) {
	sockets, cpuCores, memoryMB, diskGB = defaultSockets(req.Sockets), req.CPUCores, req.MemoryMB, req.Disk.SizeGB
	bus = defaultDiskBus

	if req.ProfileID == "" {
		return sockets, cpuCores, memoryMB, diskGB, bus, nil
	}

	profiles, err := catalog.Profiles(ctx, st, clusterName)
	if err != nil {
		return 0, 0, 0, 0, "", fmt.Errorf("read profiles: %w", err)
	}

	profile, err := catalog.FindProfile(profiles, req.ProfileID)
	if err != nil {
		return 0, 0, 0, 0, "", fmt.Errorf("%w: %s", ErrNotApproved, err.Error())
	}

	// FR-009: the profile's catalog values are authoritative — hardware
	// fields the request also carries are ignored, never merged.
	return profile.Sockets, profile.CPUCores, profile.MemoryMB, profile.DiskGB, profile.Bus, nil
}

// defaultSockets returns n or 1 when n is zero — the Proxmox default and the
// value every pre-US2 request implicitly used.
func defaultSockets(n int) int {
	if n == 0 {
		return 1
	}

	return n
}

// Placement scoring weights (US3/issue-04 D4a: fixed, matching ProxMate).
// Revisit only if a real deployment demonstrates bad placement.
const (
	placementWeightMem  = 0.5
	placementWeightCPU  = 0.35
	placementWeightDisk = 0.15
	placementFitBonus   = 1.0
)

// resolveResources resolves the node, storage, and NICs, applying
// auto-selection defaults when the request omits them. When the request
// carries an ISO and no explicit node, candidate nodes are restricted to
// those that hold the ISO (US1: a node-local ISO silently fails on the wrong
// node — the refusal must arrive before VMID consumption).
//
// When no explicit node is selected, candidates are scored by free resource
// fractions (US3/issue-04): memFrac*0.5 + cpuFrac*0.35 + diskFrac*0.15, +1
// if the VM fits. Catalog order breaks ties for reproducibility.
func resolveResources(req CreateRequest, resources catalog.Resources, capacities map[string]policy.Capacity, storageFree map[string]int64) (node, storage string, nics []nicPlan, err error) {
	node = req.Node
	if node == "" {
		candidates := resources.Nodes
		if req.ISO != nil {
			candidates = nodesWithISO(resources, req.ISO.Storage, req.ISO.File)
			if len(candidates) == 0 {
				return "", "", nil, fmt.Errorf("%w: no approved node holds iso %q on storage %q", ErrNotApproved, req.ISO.File, req.ISO.Storage)
			}
		}

		if req.Image != nil {
			candidates = nodesWithImage(resources, req.Image.Storage, req.Image.File)
			if len(candidates) == 0 {
				return "", "", nil, fmt.Errorf("%w: no approved node holds image %q on storage %q", ErrNotApproved, req.Image.File, req.Image.Storage)
			}
		}

		// Hard filter: node must have at least one approved storage.
		candidates = nodesWithStorage(resources, candidates)
		if len(candidates) == 0 {
			return "", "", nil, fmt.Errorf("%w: no approved node with storage in catalog", ErrNotApproved)
		}

		node = pickBestNode(candidates, capacities, req)
	}

	storage = req.Disk.Storage
	if storage == "" {
		storage = bestStorageOnNode(resources, node, storageFree)
		if storage == "" {
			return "", "", nil, fmt.Errorf("%w: no approved storage on node %q", ErrNotApproved, node)
		}
	}

	nics, err = resolveNICs(req, resources, node)
	if err != nil {
		return "", "", nil, err
	}

	return node, storage, nics, nil
}

// pickBestNode scores each candidate node and returns the name of the highest
// scorer. Catalog order breaks ties (stable selection for reproducible tests).
func pickBestNode(candidates []catalog.Node, capacities map[string]policy.Capacity, req CreateRequest) string {
	best := candidates[0]
	bestScore := scoreNode(capacities[best.Name], req)

	for _, candidate := range candidates[1:] {
		score := scoreNode(capacities[candidate.Name], req)
		if score > bestScore {
			best = candidate
			bestScore = score
		}
	}

	return best.Name
}

// scoreNode computes a placement score from free resource fractions
// (US3/issue-04 D4a). The formula matches ProxMate's fixed weights:
// memFrac*0.5 + cpuFrac*0.35 + diskFrac*0.15, +1 if the VM fits. A node with
// no capacity data (zero value) scores 0 — still selectable as a fallback,
// but preferred less than any node with known headroom.
func scoreNode(capacity policy.Capacity, req CreateRequest) float64 {
	var memFrac, cpuFrac, diskFrac float64

	if capacity.PhysicalRAMGB > 0 {
		freeMem := capacity.PhysicalRAMGB - capacity.UsedRAMGB
		if freeMem > 0 {
			memFrac = float64(freeMem) / float64(capacity.PhysicalRAMGB)
		}
	}

	if capacity.PhysicalVCPUs > 0 {
		freeCPU := capacity.PhysicalVCPUs - capacity.UsedVCPUs
		if freeCPU > 0 {
			cpuFrac = float64(freeCPU) / float64(capacity.PhysicalVCPUs)
		}
	}

	if capacity.MaxDiskGB > 0 {
		freeDisk := capacity.MaxDiskGB - capacity.UsedDiskGB
		if freeDisk > 0 {
			diskFrac = float64(freeDisk) / float64(capacity.MaxDiskGB)
		}
	}

	score := memFrac*placementWeightMem + cpuFrac*placementWeightCPU + diskFrac*placementWeightDisk

	// Bonus if the VM actually fits (bonus, not barrier — under overcommit
	// a node is still returned rather than failing the create).
	requestedRAM := (req.MemoryMB + 1023) / 1024
	requestedCPU := defaultSockets(req.Sockets) * req.CPUCores

	fitsMem := capacity.PhysicalRAMGB == 0 || capacity.PhysicalRAMGB-capacity.UsedRAMGB >= requestedRAM
	fitsCPU := capacity.PhysicalVCPUs == 0 || capacity.PhysicalVCPUs-capacity.UsedVCPUs >= requestedCPU
	fitsDisk := capacity.MaxDiskGB == 0 || capacity.MaxDiskGB-capacity.UsedDiskGB >= req.Disk.SizeGB

	if fitsMem && fitsCPU && fitsDisk {
		score += placementFitBonus
	}

	return score
}

// nodesWithStorage filters candidates to those that have at least one approved
// storage in the catalog (US3/issue-04 hard filter).
func nodesWithStorage(resources catalog.Resources, candidates []catalog.Node) []catalog.Node {
	var filtered []catalog.Node

	for _, candidate := range candidates {
		for _, storage := range resources.Storages {
			if storage.Node == candidate.Name {
				filtered = append(filtered, candidate)
				break
			}
		}
	}

	return filtered
}

// resolveNICs builds the resolved NIC list from the request. An empty request
// list produces one auto-selected NIC (simple mode); each entry with an empty
// bridge gets the first approved bridge on the node, and each entry with an
// empty model gets the default model.
func resolveNICs(req CreateRequest, resources catalog.Resources, node string) ([]nicPlan, error) {
	requested := req.Network
	if len(requested) == 0 {
		bridge := firstBridgeOnNode(resources, node)
		if bridge == "" {
			return nil, fmt.Errorf("%w: no approved bridge on node %q", ErrNotApproved, node)
		}

		return []nicPlan{{bridge: bridge, model: defaultNetworkModel}}, nil
	}

	nics := make([]nicPlan, 0, len(requested))
	for _, reqNIC := range requested {
		bridge := reqNIC.Bridge
		if bridge == "" {
			bridge = firstBridgeOnNode(resources, node)
			if bridge == "" {
				return nil, fmt.Errorf("%w: no approved bridge on node %q", ErrNotApproved, node)
			}
		}

		model := reqNIC.Model
		if model == "" {
			model = defaultNetworkModel
		}

		nics = append(nics, nicPlan{bridge: bridge, model: model})
	}

	return nics, nil
}

// nodesWithISO returns the approved nodes that hold the given ISO, preserving
// catalog order so auto-selection is deterministic.
func nodesWithISO(resources catalog.Resources, storage, file string) []catalog.Node {
	var matched []catalog.Node

	for _, node := range resources.Nodes {
		if resources.HasISO(storage, file, node.Name) {
			matched = append(matched, node)
		}
	}

	return matched
}

// nodesWithImage returns the approved nodes that hold the given cloud image,
// preserving catalog order so auto-selection is deterministic.
func nodesWithImage(resources catalog.Resources, storage, file string) []catalog.Node {
	var matched []catalog.Node

	for _, node := range resources.Nodes {
		if resources.HasCloudImage(storage, file, node.Name) {
			matched = append(matched, node)
		}
	}

	return matched
}

// validateCatalog checks that the resolved node, storage, each NIC's bridge
// and model, the optional ISO, and every requested tag are all present in the
// approved catalog.
func validateCatalog(req CreateRequest, resources catalog.Resources, node, storage string, nics []nicPlan) error {
	if !resources.HasNode(node) {
		return fmt.Errorf("%w: node %q", ErrNotApproved, node)
	}

	if !resources.HasStorage(storage, node) {
		return fmt.Errorf("%w: storage %q on node %q", ErrNotApproved, storage, node)
	}

	for _, nic := range nics {
		if !allowedNetworkModels[nic.model] {
			return fmt.Errorf("%w: network model %q", ErrNotApproved, nic.model)
		}

		if !resources.HasBridge(nic.bridge, node) {
			return fmt.Errorf("%w: bridge %q on node %q", ErrNotApproved, nic.bridge, node)
		}
	}

	if req.ISO != nil && !resources.HasISO(req.ISO.Storage, req.ISO.File, node) {
		return fmt.Errorf("%w: iso %q on storage %q on node %q", ErrNotApproved, req.ISO.File, req.ISO.Storage, node)
	}

	if req.Image != nil && !resources.HasCloudImage(req.Image.Storage, req.Image.File, node) {
		return fmt.Errorf("%w: image %q on storage %q on node %q", ErrNotApproved, req.Image.File, req.Image.Storage, node)
	}

	for _, tag := range req.Tags {
		if !resources.HasTag(tag) {
			return fmt.Errorf("%w: tag %q", ErrNotApproved, tag)
		}
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
// bestStorageOnNode picks the approved storage with the most free space on
// the selected node (US3/issue-04 T041). Falls back to catalog order when
// free-space data is unavailable (zero free bytes). Catalog order breaks ties
// for reproducibility.
func bestStorageOnNode(resources catalog.Resources, node string, storageFree map[string]int64) string {
	best := ""
	bestFree := int64(-1)

	for _, storage := range resources.Storages {
		if storage.Node != node {
			continue
		}

		free := storageFree[storage.Name]
		if free > bestFree {
			best = storage.Name
			bestFree = free
		}
	}

	return best
}

func firstBridgeOnNode(resources catalog.Resources, node string) string {
	for _, bridge := range resources.Bridges {
		if bridge.Node == node {
			return bridge.Name
		}
	}

	return ""
}
