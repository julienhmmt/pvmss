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

	tmpl, live, err := refreshTemplateFromDiscovery(ctx, deps, clusterName, tmpl)
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

	plan, err := planCreate(ctx, policyService, deps, clusterName, req)
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

	// Without an override the clone inherits the template's live hardware and
	// NICs, so the same gates an own-VM clone passes apply to it, before a
	// VMID is spent.
	if err := checkInheritedTemplate(ctx, policyService, clusterName, hardwareOverride, live); err != nil {
		return CreateResult{}, err
	}

	vmid, err := allocateVMID(ctx, deps)
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
	if waitErr := awaitCloneTask(ctx, deps.Creator, upid); waitErr != nil {
		cleanupFailedClone(ctx, deps, actor, clusterName, finalVMID, tmpl.Node, waitErr)

		if err := deps.Audit.RecordAction(ctx, actor.Username, clusterName, finalVMID, "vm_create"); err != nil {
			deps.Log.ErrorContext(ctx, auditLogMsg, "component", "vm", "cluster", clusterName, "vmid", finalVMID, "error", err)
		}

		return CreateResult{}, fmt.Errorf("%w: clone task: %w", ErrClusterCreate, waitErr)
	}

	applyPostCloneConfig(ctx, postCloneConfig{
		Deps: deps, Actor: actor, ClusterName: clusterName,
		VMID: finalVMID, Node: tmpl.Node, Plan: plan, Template: tmpl,
		StartAfterCreate: req.StartAfterCreate,
		Tags:             buildTags(req), DiskKey: primaryDiskKey(tmpl.DiskBus),
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
//
// It also returns the live discovery row (zero when discovery is skipped or
// unavailable); the clone gates read its hardware and NICs.
func refreshTemplateFromDiscovery(ctx context.Context, deps CreateDeps, clusterName string, tmpl catalog.Template) (catalog.Template, cluster.TemplateVM, error) {
	if deps.Templates == nil {
		return tmpl, cluster.TemplateVM{}, nil
	}

	live, err := deps.Templates.TemplateByVMID(ctx, tmpl.VMID)
	switch {
	case errors.Is(err, cluster.ErrNotFound):
		return catalog.Template{}, cluster.TemplateVM{}, fmt.Errorf("%w: template no longer exists", ErrNotApproved)
	case err != nil:
		// Discovery is unavailable - the approval still stands; the clone
		// itself will fail if the cluster is truly unreachable.
		deps.Log.WarnContext(ctx, "template freshness check failed, proceeding with stored values", "component", "vm", "cluster", clusterName, "vmid", tmpl.VMID, "error", err)

		return tmpl, cluster.TemplateVM{}, nil
	case live.DiskUnreadable:
		// Keep the discovered node; the stored disk fields were validated at
		// approval and are never empty. The live hardware is unknown.
		deps.Log.WarnContext(ctx, "template disk unreadable at clone time, using stored disk fields", "component", "vm", "cluster", clusterName, "vmid", tmpl.VMID)

		tmpl.Node = live.Node

		return tmpl, live, nil
	default:
		tmpl.Node = live.Node
		tmpl.Name = live.Name
		tmpl.DiskStorage = live.DiskStorage
		tmpl.DiskSizeGB = live.DiskSizeGB
		tmpl.DiskBus = live.DiskBus

		return tmpl, live, nil
	}
}

// checkInheritedTemplate runs the template gates when discovery returned the
// template's config. The isolation VLAN always applies: the clone's NICs come
// from the template whether or not hardware is overridden. The gabarit and
// node-capacity gates apply only when the clone inherits the template's
// hardware (no override); an overridden request is checked by planCreate.
func checkInheritedTemplate(ctx context.Context, policyService *policy.Policy, clusterName string, hardwareOverride bool, live cluster.TemplateVM) error {
	if !templateLiveKnown(live) {
		return nil
	}

	gabarit, err := policyService.Gabarit(ctx, clusterName)
	if err != nil {
		return fmt.Errorf("read vmid range: %w", err)
	}

	source := Entity{
		Node:              live.Node,
		Sockets:           live.Sockets,
		Cores:             live.Cores,
		MemoryTotal:       live.MemoryTotal,
		DiskTotal:         live.DiskTotal,
		NetworkInterfaces: live.NetworkInterfaces,
	}

	if err := checkCloneVLAN(gabarit.IsolationVLANTag, source); err != nil {
		return err
	}

	if hardwareOverride {
		return nil
	}

	if err := checkCloneGabarit(ctx, policyService, clusterName, source); err != nil {
		return err
	}

	return checkCloneCapacity(ctx, policyService, clusterName, source)
}

// templateLiveKnown reports whether discovery returned the template's config,
// so its hardware and NICs are real and the clone gates can run on them.
// shortcut: when discovery is unavailable or the config is unreadable the
// gates are skipped and the clone proceeds on stored values; upgrade by
// refusing the clone when the live config cannot be read.
func templateLiveKnown(live cluster.TemplateVM) bool {
	return live.VMID != 0 && !live.DiskUnreadable
}
