package vm

import (
	"context"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/policy"
)

// createFromImage is the cloud-image path: CreateVM with import-from (the
// image becomes the primary disk at the image's size, grown to the requested
// size after the task), then cloud-init delivered through Proxmox's native
// keys (see applyImageCloudInitConfig - the REST API cannot write a per-VM
// snippet file). Cloud-init is mandatory in image mode - the image is a full
// disk, so there is no installer and no second boot path. The VM is never
// started inside the create task: the network/identity config is not applied
// yet, and cloud-init does not replay on the next boot without
// `cloud-init clean`. It is started explicitly after that config lands.
func createFromImage(ctx context.Context, policyService *policy.Policy, deps CreateDeps, clusterName string, actor auth.Identity, req CreateRequest) (CreateResult, error) {
	defaultImageHardware(&req)

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
		return CreateResult{}, allocateVMIDError(err)
	}

	spec := buildCreateSpec(actor, req, plan, vmid)

	finalVMID, upid, err := dispatchCreateWithRetry(ctx, deps, spec)
	if err != nil {
		return CreateResult{}, err
	}

	result := CreateResult{Cluster: clusterName, VMID: finalVMID, Name: req.Name, Node: plan.node, UPID: upid, FromImage: true}

	// The create task must finish before the snippet is attached - the PUT
	// hits a 500 "VM is locked (create)" otherwise. A wait failure is a
	// failed create task: the half-made VM is purged best-effort.
	if waitErr := waitCreateTask(ctx, deps.Creator, upid); waitErr != nil {
		return failImageCreate(ctx, deps, imageCreateFailure{
			actor:       actor,
			clusterName: clusterName,
			vmid:        finalVMID,
			node:        spec.Node,
			waitErr:     waitErr,
			result:      result,
		})
	}

	resizeImageDisk(ctx, deps, clusterName, plan, spec, finalVMID, &result)

	applyImageCloudInitConfig(ctx, imageCloudInitApply{
		Deps: deps, Actor: actor, ClusterName: clusterName,
		Spec: spec, VMID: finalVMID, Document: plan.document, SkipReason: plan.documentSkipReason,
		CloudInit: req.Image.CloudInit,
	}, &result)

	// Persist the baseline delivery state so the VM detail page can report
	// it. Best-effort: a store failure logs but does not abort.
	if result.BaselineState != "" {
		if err := deps.Store.PutBaselineState(ctx, clusterName, finalVMID, result.BaselineState, result.BaselineError); err != nil {
			deps.Log.ErrorContext(ctx, "persist baseline state failed", "component", "vm", "cluster", clusterName, "vmid", finalVMID, "error", err)
		}
	}

	startImageVM(ctx, deps, clusterName, spec, finalVMID, startAfterCreate, &result)

	if err := deps.Audit.RecordAction(ctx, actor.Username, clusterName, finalVMID, "vm_create"); err != nil {
		deps.Log.ErrorContext(ctx, auditLogMsg, "component", "vm", "cluster", clusterName, "vmid", finalVMID, "error", err)
	}

	return result, nil
}

// defaultImageHardware fills the zero-value hardware fields of an image-mode
// request with the image defaults: the wizard sends only the image and the
// disk size, so CPU/memory come through zero. imageDefault{CPUCores,MemoryMB}
// applies rather than the shared technical minimum (1 vCPU/128 MB) - a cloud
// image needs real headroom to boot. A set ProfileID skips this:
// resolveHardware overwrites these fields with the profile's values
// regardless.
func defaultImageHardware(req *CreateRequest) {
	if req.ProfileID != "" {
		return
	}

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

// imageCreateFailure carries the context of a failed image-mode create to
// failImageCreate: the half-made VM's identity, the wait error, and the
// accumulated result.
type imageCreateFailure struct {
	actor       auth.Identity
	clusterName string
	vmid        int
	node        string
	waitErr     error
	result      CreateResult
}

// failImageCreate handles a failed create-task wait on the image path: the
// half-made VM is purged best-effort, the wait error is
// recorded on the result, and the create is audited.
func failImageCreate(ctx context.Context, deps CreateDeps, f imageCreateFailure) (CreateResult, error) {
	deps.Log.ErrorContext(ctx, "create task wait failed", "component", "vm", "cluster", f.clusterName, "vmid", f.vmid, "error", f.waitErr)
	f.result.CloudInitPushError = f.waitErr.Error()

	rollbackFailedCreate(ctx, deps, f.actor, f.clusterName, f.vmid, f.node, "create task failed")

	if err := deps.Audit.RecordAction(ctx, f.actor.Username, f.clusterName, f.vmid, "vm_create"); err != nil {
		deps.Log.ErrorContext(ctx, auditLogMsg, "component", "vm", "cluster", f.clusterName, "vmid", f.vmid, "error", err)
	}

	return f.result, nil
}

// resizeImageDisk grows the imported disk to the requested size. import-from
//
//	lands the disk at the source image's size (Proxmox requires the:0 target
//
// syntax), so the grow runs now that the create task released the VM lock.
// ResizeDisk only grows, so the call is skipped when the request matches the
// image size. A resize failure does not abort - the VM exists - it is
// recorded like the other post-create steps.
func resizeImageDisk(ctx context.Context, deps CreateDeps, clusterName string, plan createPlan, spec cluster.VMSpec, vmid int, result *CreateResult) {
	if plan.imageSizeGB <= 0 || plan.diskGB <= plan.imageSizeGB || deps.Writer == nil {
		return
	}

	if err := deps.Writer.ResizeDisk(ctx, spec.Node, vmid, spec.Disk.Bus+"0", plan.diskGB); err != nil {
		deps.Log.ErrorContext(ctx, "image disk resize failed", "component", "vm", "cluster", clusterName, "vmid", vmid, "error", err)
		result.CloudInitPushError = err.Error()
	}
}

// startImageVM implements auto-start for image mode: the VM is fully
// configured at first boot, so it is started explicitly after the snippet is
// attached - the first boot sees cloud-init. A recorded cloud-init failure
// (result.CloudInitPushError) blocks the start.
func startImageVM(ctx context.Context, deps CreateDeps, clusterName string, spec cluster.VMSpec, vmid int, startAfterCreate bool, result *CreateResult) {
	if !startAfterCreate || result.CloudInitPushError != "" || deps.Writer == nil {
		return
	}

	if err := deps.Writer.Action(ctx, spec.Node, vmid, "start"); err != nil {
		deps.Log.ErrorContext(ctx, "post-cloudinit start failed", "component", "vm", "cluster", clusterName, "vmid", vmid, "error", err)
	}
}

// Baseline delivery states recorded on CreateResult.BaselineState and
// persisted in vm_baseline_state. "override" is historical (the removed
// hand-placed pvmss-baseline.yml); it is still displayed for old rows.
const (
	BaselineStateApplied      = "applied"
	BaselineStateOverride     = "override"
	BaselineStateNotDelivered = "not_delivered"
)

// imageCloudInitApply bundles the inputs to applyImageCloudInitConfig.
type imageCloudInitApply struct {
	Deps        CreateDeps
	Actor       auth.Identity
	ClusterName string
	Spec        cluster.VMSpec
	VMID        int
	// Document is the plan-time resolved, node-verified published file:
	// the chosen template (which embeds the baseline) or the standalone
	// baseline. Empty when none could be resolved (SkipReason says why).
	Document   publishedDocument
	SkipReason string
	CloudInit  ImageCloudInitRequest
}

// applyImageCloudInitConfig delivers image-mode cloud-init: the native keys
// (ciuser/sshkeys/ipconfig0) first - they always work - then the published
// vendor-data document. A missing baseline does not block the start (the
// VM boots on the native keys, BaselineState "not_delivered"); a template
// the user chose that cannot be attached does (CloudInitPushError), like
// the ISO and template paths.
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
		cfg.Deps.Log.ErrorContext(ctx, "cloud-init config set failed", "component", "vm", "cluster", cfg.ClusterName, "vmid", cfg.VMID, "error", err)
		result.CloudInitPushError = err.Error()

		return
	}

	result.CloudInitTemplateID = cfg.Document.TemplateID

	if !cfg.Document.present() {
		result.BaselineState = BaselineStateNotDelivered
		result.BaselineError = cfg.SkipReason

		if result.BaselineError == "" {
			result.BaselineError = "no published cloud-init document on this cluster"
		}

		return
	}

	if err := attachPublishedDocument(ctx, cfg.Deps, cfg.Actor, documentTarget{Cluster: cfg.ClusterName, Node: cfg.Spec.Node, VMID: cfg.VMID}, cfg.Document); err != nil {
		cfg.Deps.Log.ErrorContext(ctx, "cloud-init document attach failed", "component", "vm", "cluster", cfg.ClusterName, "vmid", cfg.VMID, "error", err)

		result.BaselineState = BaselineStateNotDelivered
		result.BaselineError = err.Error()

		if cfg.Document.TemplateID != "" {
			result.CloudInitPushError = err.Error()
		}

		return
	}

	result.BaselineState = BaselineStateApplied
}
