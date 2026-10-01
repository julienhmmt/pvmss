package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"slices"
)

// dispatchCreateWithRetry dispatches a CreateVM call, retrying with a fresh
// VMID when Proxmox reports a collision (max 3 attempts).
// A retry with a new VMID is not a mutation replay - the original create never
// succeeded, so ProxMate's idempotency concern does not apply. Returns the
// final VMID (which may differ from spec.VMID after a retry) and the UPID.
func dispatchCreateWithRetry(ctx context.Context, deps CreateDeps, spec cluster.VMSpec) (int, string, error) {
	return retryWithFreshVMID(ctx, deps, spec.VMID, "create", func(vmid int) (string, error) {
		spec.VMID = vmid

		return deps.Creator.CreateVM(ctx, spec)
	})
}

// dispatchCloneWithRetry dispatches a CloneVM call with the same VMID collision
// retry as dispatchCreateWithRetry. Returns the final
// NewVMID (which may differ after a retry) and the UPID.
func dispatchCloneWithRetry(ctx context.Context, deps CreateDeps, spec cluster.CloneSpec) (int, string, error) {
	return retryWithFreshVMID(ctx, deps, spec.NewVMID, "clone", func(vmid int) (string, error) {
		spec.NewVMID = vmid

		return deps.Creator.CloneVM(ctx, spec)
	})
}

// retryWithFreshVMID is the shared VMID-collision retry loop. dispatch is called with the
// current VMID; on ErrVMIDTaken it allocates
// a fresh VMID and retries, up to maxVMIDRetries times. label is "create" or "clone" for the
// error message and log.
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

		deps.Log.InfoContext(ctx, "vmid collision, retrying", "component", "vm", "vmid", vmid, "attempt", attempt+1)

		newVMID, err := deps.Creator.NextVMID(ctx)
		if err != nil {
			return 0, "", fmt.Errorf("%w: allocate vmid on retry: %w", ErrClusterCreate, err)
		}

		vmid = newVMID
	}
}

// rollbackFailedCreate purges a half-made VM after a failed create or clone
// task. The cleanup is best-effort: a failure is logged and
// does not mask the original error. An audit entry is recorded so the orphan
// is traceable - ProxMate deliberately kept half-made VMs, but in a self-
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
		deps.Log.ErrorContext(ctx, "rollback: failed to purge half-made vm", "component", "vm", "cluster", clusterName, "vmid", vmid, "node", node, "reason", reason, "error", err)

		return
	}

	deps.Log.InfoContext(ctx, "rollback: purged half-made vm", "component", "vm", "cluster", clusterName, "vmid", vmid, "node", node, "reason", reason)

	if err := deps.Audit.RecordAction(ctx, actor.Username, clusterName, vmid, "vm_create_rollback"); err != nil {
		deps.Log.ErrorContext(ctx, auditLogMsg, "component", "vm", "cluster", clusterName, "vmid", vmid, "error", err)
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
// allocated. Returns ErrNotApproved for an unknown or
// disabled template - same "never spend a VMID on a rejected request"
// discipline as ISO validation.
func resolveTemplate(ctx context.Context, st *store.Store, clusterName string, templateID int) (catalog.Template, error) {
	templates, err := catalog.Templates(ctx, st, clusterName)
	if err != nil {
		return catalog.Template{}, fmt.Errorf("read templates: %w", err)
	}

	tmpl, err := catalog.FindTemplate(templates, templateID)
	if err != nil {
		return catalog.Template{}, notApprovedError(err)
	}

	return tmpl, nil
}

// checkDiskReduction rejects a disk size smaller than the template's disk
// (Proxmox does not reduce disks). The caller passes the
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
// imported (image-mode: the import lands at the image's size and only grows afterwards, so a
// smaller request is a reduction). The caller passes
// the resolved disk size (from planCreate, which applies profile overrides)
// and the resolved node, so a forged request cannot bypass this guard with a
// profile whose DiskGB is smaller than the image. Returns the image's size in
// whole GB (rounded up) so the caller can grow the imported disk afterwards.
func checkDiskAboveImage(resources catalog.Resources, req CreateRequest, node string, diskGB int) (int, error) {
	image, err := resources.FindCloudImage(req.Image.Storage, req.Image.File, node)
	if err != nil {
		return 0, notApprovedError(err)
	}

	minGB := int((image.SizeBytes + bytesPerGB - 1) / bytesPerGB)
	if int64(diskGB)*bytesPerGB < image.SizeBytes {
		return 0, fmt.Errorf("%w: requested %d GB, image %q is %d GB", ErrDiskBelowImage, diskGB, image.File, minGB)
	}

	return minGB, nil
}

// buildCloneSpec assembles the CloneSpec from the resolved template, plan,
// request, and allocated VMID. Full clone when the template
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

// buildCreateSpec assembles the cluster.VMSpec from the validated plan,
// request, actor identity, and allocated VMID. The "pvmss" tag is always
// present. Image-mode VMs also carry "pvmss-image" so the console
// can default to the readable text tab.
func buildCreateSpec(actor auth.Identity, req CreateRequest, plan createPlan, vmid int) cluster.VMSpec {
	tags := append([]string(nil), req.Tags...)
	if !slices.Contains(tags, "pvmss") {
		tags = append(tags, "pvmss")
	}

	if req.Image != nil && !slices.Contains(tags, "pvmss-image") {
		tags = append(tags, "pvmss-image")
	}

	// Stamp the admin-imposed isolation VLAN on every NIC.
	// The tag comes from the per-cluster gabarit; tenants never choose
	// it. Firewall is always true (the Proxmox per-VM firewall is armed by default, not
	// user-exposed).
	nics := make([]cluster.NICSpec, 0, len(plan.nics))
	for _, nic := range plan.nics {
		spec := cluster.NICSpec{Bridge: nic.bridge, Model: nic.model, Firewall: true}

		if plan.isolationVLANTag > 0 {
			tag := plan.isolationVLANTag
			spec.VLAN = &tag
		}

		nics = append(nics, spec)
	}

	// UEFI maps to bios=ovmf; the cluster create path
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

// postCloneConfig bundles the inputs to applyPostCloneConfig.
type postCloneConfig struct {
	Deps             CreateDeps
	Actor            auth.Identity
	ClusterName      string
	VMID             int
	Node             string
	Plan             createPlan
	Template         catalog.Template
	CloudDocument    publishedDocument
	StartAfterCreate bool
	Tags             []string
	DiskKey          string
	// HardwareOverride is false when the request omitted cpuCores/memoryMB
	// (simple template mode). The clone inherits the template's hardware
	// and UpdateHardware is skipped.
	HardwareOverride bool
}

// applyPostCloneConfig runs the post-clone configuration sequence (in the order ProxMate uses):
// hardware overrides → disk resize → cloud-init
// → start. Each step is best-effort: a failure is logged and recorded on
// result.CloudInitPushError but does not abort the remaining steps - the clone
// is already real and the VM exists.
func applyPostCloneConfig(ctx context.Context, cfg postCloneConfig, result *CreateResult) {
	applyCloneHardware(ctx, cfg, result)
	applyCloneDiskResize(ctx, cfg, result)

	// 3. Cloud-init document attach (same mechanism as the ISO path).
	if cfg.CloudDocument.present() {
		applyCloudInitDocument(ctx, cfg.Deps, cfg.Actor, documentTarget{Cluster: cfg.ClusterName, Node: cfg.Node, VMID: cfg.VMID}, cfg.CloudDocument, result)
	}

	// 4. Start the VM if requested (after cloud-init attachment so the first boot sees the
	// snippet).
	if cfg.StartAfterCreate && result.CloudInitPushError == "" && cfg.Deps.Writer != nil {
		if err := cfg.Deps.Writer.Action(ctx, cfg.Node, cfg.VMID, "start"); err != nil {
			cfg.Deps.Log.ErrorContext(ctx, "post-clone start failed", "component", "vm", "cluster", cfg.ClusterName, "vmid", cfg.VMID, "error", err)
		}
	}
}

// applyCloneHardware applies hardware overrides when the caller explicitly
// supplied CPU/memory, and always stamps the mandatory pvmss tag.
// In simple template mode (no hardware override), SetTags is used so the
// clone inherits the template's hardware unchanged - UpdateHardware would
// shrink it to the plan's minimums (1 vCPU / 128 MB).
func applyCloneHardware(ctx context.Context, cfg postCloneConfig, result *CreateResult) {
	if cfg.Deps.Writer == nil {
		return
	}

	if cfg.HardwareOverride {
		if err := cfg.Deps.Writer.UpdateHardware(ctx, cfg.Node, cfg.VMID, cfg.Plan.sockets, cfg.Plan.cpuCores, cfg.Plan.memoryMB, cfg.Tags); err != nil {
			cfg.Deps.Log.ErrorContext(ctx, "post-clone hardware update failed", "component", "vm", "cluster", cfg.ClusterName, "vmid", cfg.VMID, "error", err)
			result.CloudInitPushError = err.Error()

			return
		}

		return
	}

	// No hardware override - still stamp the pvmss tag so the clone is
	// visible to PVMSS. Without this, a simple-mode clone exists in
	// Proxmox but Resolve() returns ErrNotFound.
	if err := cfg.Deps.Writer.SetTags(ctx, cfg.Node, cfg.VMID, cfg.Tags); err != nil {
		cfg.Deps.Log.ErrorContext(ctx, "post-clone set tags failed", "component", "vm", "cluster", cfg.ClusterName, "vmid", cfg.VMID, "error", err)
		result.CloudInitPushError = err.Error()
	}
}

// applyCloneDiskResize enlarges the clone's disk when the plan's size exceeds
// the template's (Proxmox does not reduce disks).
func applyCloneDiskResize(ctx context.Context, cfg postCloneConfig, result *CreateResult) {
	if cfg.Deps.Writer == nil || cfg.Plan.diskGB <= cfg.Template.DiskSizeGB || cfg.DiskKey == "" {
		return
	}

	if err := cfg.Deps.Writer.ResizeDisk(ctx, cfg.Node, cfg.VMID, cfg.DiskKey, cfg.Plan.diskGB); err != nil {
		cfg.Deps.Log.ErrorContext(ctx, "post-clone disk resize failed", "component", "vm", "cluster", cfg.ClusterName, "vmid", cfg.VMID, "error", err)

		if result.CloudInitPushError == "" {
			result.CloudInitPushError = err.Error()
		}
	}
}

// buildTags returns the request's tags with the mandatory "pvmss" tag appended
// when absent. Shared by both creation paths.
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
