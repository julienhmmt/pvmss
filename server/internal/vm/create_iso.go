package vm

import (
	"context"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/policy"
)

// createFromISO is the original creation path: CreateVM with an optional
// ISO, then cloud-init snippet attachment. adds waitCreateTask
// before cloud-init attachment so the Proxmox create lock is released, and
// forces StartAfterCreate off when a cloud-init template is requested (the VM
// is started explicitly after the snippet is attached, preventing a first boot
// without cloud-init).
func createFromISO(ctx context.Context, policyService *policy.Policy, deps CreateDeps, clusterName string, actor auth.Identity, req CreateRequest) (CreateResult, error) {
	plan, err := planCreate(ctx, policyService, deps, clusterName, actor, req)
	if err != nil {
		return CreateResult{}, err
	}

	// planCreate already resolved the cloud-init document and proved it is
	// on the VM's node, before any VMID is spent.
	cloudDoc := plan.document

	// When a cloud-init document is requested, do not let
	// Proxmox start the VM in the same create task - the snippet is not
	// attached yet, and cloud-init does not replay on the next boot without
	// `cloud-init clean`. The VM is started explicitly after attachment.
	// Capture the original request so applyCloudInitAfterWait can start the
	// VM after cloud-init is attached.
	startAfterCreate := req.StartAfterCreate
	if cloudDoc.present() {
		req.StartAfterCreate = false
	}

	vmid, err := deps.Creator.NextVMID(ctx)
	if err != nil {
		return CreateResult{}, allocateVMIDError(err)
	}

	spec := buildCreateSpec(actor, req, plan, vmid)

	finalVMID, upid, err := dispatchCreateWithRetry(ctx, deps, spec)
	if err != nil {
		return CreateResult{}, err
	}

	spec.VMID = finalVMID
	result := CreateResult{Cluster: clusterName, VMID: finalVMID, Name: req.Name, Node: plan.node, UPID: upid}

	// Wait for the create task to finish before attaching
	// cloud-init. Without this, the PUT /nodes/{node}/qemu/{vmid}/config
	// hits a 500 "VM is locked (create)" from Proxmox. Only wait when a
	// cloud-init document is requested - a simple ISO creation with no
	// post-processing must not become a long HTTP request.
	if cloudDoc.present() {
		applyCloudInitAfterWait(ctx, cloudInitWaitRequest{
			Deps: deps, Actor: actor, ClusterName: clusterName,
			Spec: spec, VMID: finalVMID, Document: cloudDoc, UPID: upid,
			StartAfterCreate: startAfterCreate,
		}, &result)
	}

	if err := deps.Audit.RecordAction(ctx, actor.Username, clusterName, finalVMID, "vm_create"); err != nil {
		deps.Log.ErrorContext(ctx, auditLogMsg, "component", "vm", "cluster", clusterName, "vmid", finalVMID, "error", err)
	}

	return result, nil
}

// cloudInitWaitRequest bundles the inputs to applyCloudInitAfterWait.
// Extracted from createFromISO to keep the nesting under
// nestif's ceiling.
type cloudInitWaitRequest struct {
	Deps             CreateDeps
	Actor            auth.Identity
	ClusterName      string
	Spec             cluster.VMSpec
	VMID             int
	Document         publishedDocument
	UPID             string
	StartAfterCreate bool
}

// applyCloudInitAfterWait waits for the create task, then attaches the
// cloud-init snippet and starts the VM if requested. A wait failure is
// treated as a failed create task: the half-made VM is
// purged best-effort so it does not consume the user's quota. An attach
// failure is recorded on result.CloudInitPushError but does not abort - the
// task succeeded and the VM exists.
func applyCloudInitAfterWait(ctx context.Context, req cloudInitWaitRequest, result *CreateResult) {
	if waitErr := waitCreateTask(ctx, req.Deps.Creator, req.UPID); waitErr != nil {
		req.Deps.Log.ErrorContext(ctx, "create task wait failed", "component", "vm", "cluster", req.ClusterName, "vmid", req.VMID, "error", waitErr)
		result.CloudInitPushError = waitErr.Error()

		// The create task failed, so the VM is half-made.
		// Purge it best-effort so it does not eat the user's quota.
		rollbackFailedCreate(ctx, req.Deps, req.Actor, req.ClusterName, req.VMID, req.Spec.Node, "create task failed")

		return
	}

	applyCloudInitDocument(ctx, req.Deps, req.Actor, documentTarget{Cluster: req.ClusterName, Node: req.Spec.Node, VMID: req.VMID}, req.Document, result)

	// Start the VM explicitly after the snippet is attached,
	// so the first boot sees cloud-init.
	if req.StartAfterCreate && result.CloudInitPushError == "" && req.Deps.Writer != nil {
		if startErr := req.Deps.Writer.Action(ctx, req.Spec.Node, req.VMID, "start"); startErr != nil {
			req.Deps.Log.ErrorContext(ctx, "post-cloudinit start failed", "component", "vm", "cluster", req.ClusterName, "vmid", req.VMID, "error", startErr)
		}
	}
}
