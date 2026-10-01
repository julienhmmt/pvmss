package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/policy"
)

// createFromTemplate is the clone path: CloneVM from an approved
// Proxmox template, wait for the clone task, then apply post-clone configuration
// (hardware overrides, disk resize, cloud-init, start). The clone stays on the
// template's node (cross-node clone is forbidden).
func createFromTemplate(ctx context.Context, policyService *policy.Policy, deps CreateDeps, clusterName string, actor auth.Identity, req CreateRequest) (CreateResult, error) {
	tmpl, err := resolveTemplate(ctx, deps.Store, clusterName, req.TemplateID)
	if err != nil {
		return CreateResult{}, err
	}

	tmpl, err = refreshTemplateFromDiscovery(ctx, deps, clusterName, tmpl)
	if err != nil {
		return CreateResult{}, err
	}

	// The clone stays on the template's node. Override any client
	// supplied node - the selector is hidden in the UI, but a forged request
	// must not place the clone on a different node.
	req.Node = tmpl.Node

	// A zero disk size means "use the template's size" - the clone
	// keeps the template's disk as-is, no resize needed. Default it before
	// planCreate so checkTechnicalRange (MinDiskGB=1) does not reject a
	// zero, and so applyPostCloneConfig sees the template's size (no
	// enlargement triggers).
	if req.Disk.SizeGB == 0 {
		req.Disk.SizeGB = tmpl.DiskSizeGB
	}

	// Simple-mode template requests omit cpuCores/memoryMB - the clone
	// inherits the template's hardware. Default the zeros to the minimums
	// so checkTechnicalRange passes, and track that no override was
	// requested so applyPostCloneConfig skips UpdateHardware (which would
	// otherwise shrink the clone to 1 vCPU / 128 MB).
	hardwareOverride := defaultTemplateHardware(&req)

	plan, err := planCreate(ctx, policyService, deps, clusterName, actor, req)
	if err != nil {
		return CreateResult{}, err
	}

	// Reject disk reduction before VMID allocation. Run after
	// planCreate so the check sees the resolved disk size - a forged
	// request carrying both a profileId (whose DiskGB may be smaller than
	// the template's) and a templateId would otherwise bypass this guard.
	// The plan's diskGB is what applyPostCloneConfig uses for the resize
	// decision, so it is the right value to compare against.
	if err := checkDiskReduction(plan.diskGB, tmpl); err != nil {
		return CreateResult{}, err
	}

	cloudDoc := plan.document

	// Do not start in the clone task when cloud-init is
	// requested. The VM is started after snippet attachment. Capture the
	// original request so applyPostCloneConfig can start the VM after
	// cloud-init is attached.
	startAfterCreate := req.StartAfterCreate
	if cloudDoc.present() {
		req.StartAfterCreate = false
	}

	vmid, err := deps.Creator.NextVMID(ctx)
	if err != nil {
		return CreateResult{}, allocateVMIDError(err)
	}

	cloneSpec := buildCloneSpec(tmpl, plan, req, vmid, actor.Pool)

	finalVMID, upid, err := dispatchCloneWithRetry(ctx, deps, cloneSpec)
	if err != nil {
		return CreateResult{}, err
	}

	cloneSpec.NewVMID = finalVMID
	result := CreateResult{Cluster: clusterName, VMID: finalVMID, Name: req.Name, Node: tmpl.Node, UPID: upid}

	// Wait for the clone task to finish before any post-clone
	// configuration. The VM does not exist until the task completes.
	// If the task fails, the half-made VM is purged
	// (best-effort) so it does not consume the user's quota.
	if waitErr := waitCreateTask(ctx, deps.Creator, upid); waitErr != nil {
		deps.Log.ErrorContext(ctx, "clone task wait failed", "component", "vm", "cluster", clusterName, "vmid", finalVMID, "error", waitErr)
		result.CloudInitPushError = waitErr.Error()

		rollbackFailedCreate(ctx, deps, actor, clusterName, finalVMID, tmpl.Node, "clone task failed")

		if err := deps.Audit.RecordAction(ctx, actor.Username, clusterName, finalVMID, "vm_create"); err != nil {
			deps.Log.ErrorContext(ctx, auditLogMsg, "component", "vm", "cluster", clusterName, "vmid", finalVMID, "error", err)
		}

		return result, nil
	}

	applyPostCloneConfig(ctx, postCloneConfig{
		Deps: deps, Actor: actor, ClusterName: clusterName,
		VMID: finalVMID, Node: tmpl.Node, Plan: plan, Template: tmpl,
		CloudDocument: cloudDoc, StartAfterCreate: startAfterCreate,
		Tags: buildTags(req), DiskKey: primaryDiskKey(tmpl.DiskBus),
		HardwareOverride: hardwareOverride,
	}, &result)

	if err := deps.Audit.RecordAction(ctx, actor.Username, clusterName, finalVMID, "vm_create"); err != nil {
		deps.Log.ErrorContext(ctx, auditLogMsg, "component", "vm", "cluster", clusterName, "vmid", finalVMID, "error", err)
	}

	return result, nil
}

// refreshTemplateFromDiscovery is the clone-time freshness backstop. The stored row is the
// approval; discovery is the truth about
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
		// Discovery is unavailable - the approval still stands; the clone
		// itself will fail if the cluster is truly unreachable.
		deps.Log.WarnContext(ctx, "template freshness check failed, proceeding with stored values", "component", "vm", "cluster", clusterName, "vmid", tmpl.VMID, "error", err)

		return tmpl, nil
	case live.DiskUnreadable:
		// Keep the discovered node; the stored disk fields were validated at
		// approval and are never empty.
		deps.Log.WarnContext(ctx, "template disk unreadable at clone time, using stored disk fields", "component", "vm", "cluster", clusterName, "vmid", tmpl.VMID)

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
