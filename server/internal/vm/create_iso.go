package vm

import (
	"context"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/policy"
)

// createFromISO is the original creation path: CreateVM with an optional
// ISO, then cloud-init snippet attachment. adds waitCreateTask
// before cloud-init attachment so the Proxmox create lock is released, and
// forces StartAfterCreate off when a cloud-init template is requested (the VM
// is started explicitly after the snippet is attached, preventing a first boot
// without cloud-init).
func createFromISO(ctx context.Context, policyService *policy.Policy, deps CreateDeps, clusterName string, actor auth.Identity, req CreateRequest) (CreateResult, error) {
	plan, err := planCreate(ctx, policyService, deps, clusterName, req)
	if err != nil {
		return CreateResult{}, err
	}

	vmid, err := allocateVMID(ctx, deps)
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

	if err := deps.Audit.RecordAction(ctx, actor.Username, clusterName, finalVMID, "vm_create"); err != nil {
		deps.Log.ErrorContext(ctx, auditLogMsg, "component", "vm", "cluster", clusterName, "vmid", finalVMID, "error", err)
	}

	return result, nil
}
