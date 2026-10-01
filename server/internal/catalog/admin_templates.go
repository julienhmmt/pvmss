package catalog

import (
	"context"
	"database/sql"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
)

// reconcileTemplateApproval builds the approval view for one discovered
// template, applying the stored approval row when one exists: enabled flag,
// admin pin (OverrideDiscovery), and drift write-back. Extracted from
// AdminListTemplates to keep its cognitive complexity under the go:S3776
// limit.
func reconcileTemplateApproval(
	ctx context.Context,
	st *store.Store,
	clusterName string,
	tmpl cluster.TemplateVM,
	storedByVMID map[int]store.CatalogTemplateEnabled,
) (TemplateApproval, error) {
	approval := newTemplateApproval(tmpl)
	approval.DiskUnreadable = tmpl.DiskUnreadable

	stored, ok := storedByVMID[tmpl.VMID]
	if !ok {
		return approval, nil
	}

	approval.Enabled = stored.Enabled
	approval.OverrideDiscovery = stored.OverrideDiscovery

	// When the admin pinned the row (schemaV26), the stored values
	// are authoritative - show them instead of the discovered ones
	// and skip the drift write-back so the pin survives the next
	// list. An unreadable discovery reports empty disk fields
	// never write them over the stored values: the
	// clone-time fallback relies on them being non-empty.
	if stored.OverrideDiscovery {
		approval.Node = stored.Node
		approval.Name = stored.Name
		approval.CloudInitCapable = stored.CloudInitCapable
		approval.DiskStorage = stored.DiskStorage
		approval.DiskSizeGB = stored.DiskSizeGB
		approval.DiskBus = stored.DiskBus

		return approval, nil
	}

	if !tmpl.DiskUnreadable && templateDrift(stored, tmpl) {
		values := store.TemplateValues{
			Node: tmpl.Node, Name: tmpl.Name, CloudInitCapable: tmpl.CloudInitCapable,
			DiskStorage: tmpl.DiskStorage, DiskSizeGB: tmpl.DiskSizeGB, DiskBus: tmpl.DiskBus,
		}
		if err := st.UpdateTemplate(ctx, clusterName, tmpl.VMID, values); err != nil {
			return TemplateApproval{}, err
		}
	}

	return approval, nil
}

// newTemplateApproval builds the approval view of one discovered template
// with Enabled left false - the caller sets it from the stored row.
func newTemplateApproval(tmpl cluster.TemplateVM) TemplateApproval {
	return TemplateApproval{
		VMID:             tmpl.VMID,
		Node:             tmpl.Node,
		Name:             tmpl.Name,
		CloudInitCapable: tmpl.CloudInitCapable,
		DiskStorage:      tmpl.DiskStorage,
		DiskSizeGB:       tmpl.DiskSizeGB,
		DiskBus:          tmpl.DiskBus,
	}
}

// templateDrift reports whether a stored approval row's field values differ
// from what discovery currently reports (the stored row is an approval-time snapshot and can go
// stale).
func templateDrift(stored store.CatalogTemplateEnabled, tmpl cluster.TemplateVM) bool {
	return stored.Node != tmpl.Node || stored.Name != tmpl.Name ||
		stored.CloudInitCapable != tmpl.CloudInitCapable ||
		stored.DiskStorage != tmpl.DiskStorage || stored.DiskSizeGB != tmpl.DiskSizeGB ||
		stored.DiskBus != tmpl.DiskBus
}

// TemplateRef identifies one discovered template by its VMID.
type TemplateRef struct {
	VMID int
}

// ErrTemplateNotFound is returned when a template approval row does not exist
// for the cluster (removing an orphan approval).
var ErrTemplateNotFound = errors.New("template not found")

// DeleteTemplate removes a template approval row. Returns
// ErrTemplateNotFound when the cluster has no approval for the vmid - the
// admin UI only offers Remove on missing (orphaned) rows, but the API deletes
// any approval row.
func DeleteTemplate(ctx context.Context, st *store.Store, cluster string, vmid int) error {
	err := st.DeleteTemplate(ctx, cluster, vmid)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTemplateNotFound
	}

	return err
}

// UpdateTemplate overrides an approved template's editable fields and pins the
// row against discovery-wins write-back (schemaV26). Returns
// ErrTemplateNotFound when the cluster has no approval for the vmid. The
// override is a human mutation, so the caller (HTTP handler) audits it; this
// function only persists. DiskSizeGB must be >= 0; the HTTP handler validates
// the upper bound against the gabarit.
func UpdateTemplate(ctx context.Context, st *store.Store, cluster string, vmid int, values store.TemplateValues) error {
	values.OverrideDiscovery = true

	err := st.UpdateTemplate(ctx, cluster, vmid, values)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTemplateNotFound
	}

	return err
}

// ErrTemplateUnreadable is returned when approving a template whose disk
// config could not be read: the approval row would carry empty
// disk_bus/disk_storage and break the post-clone resize. Disabling stays
// possible.
var ErrTemplateUnreadable = errors.New("template disk unreadable")

// SetTemplateEnabled upserts the enabled state for one template. Returns
// cluster.ErrNotFound if the template is not in the current discovery set.
// See SetNodeEnabled for the discovery-error contract.
//
// The discovered template's field values are used to populate the row on
// first approval (so the row is complete, not a stub with empty fields).
// The lookup is a single TemplateByVMID call - not a full
// ListTemplates re-hydration per toggle.
func SetTemplateEnabled(ctx context.Context, st *store.Store, client cluster.Client, clusterName string, ref TemplateRef, enabled bool) error {
	found, err := client.TemplateByVMID(ctx, ref.VMID)
	if err != nil {
		return err
	}

	// Approving an unreadable template would store empty disk fields, which
	// breaks the post-clone resize. Disabling an approved one stays possible.
	if enabled && found.DiskUnreadable {
		return ErrTemplateUnreadable
	}

	existing, err := st.CatalogTemplatesEnabled(ctx, clusterName)
	if err != nil {
		return err
	}

	for _, row := range existing {
		if row.VMID == ref.VMID {
			return st.SetTemplateEnabled(ctx, clusterName, ref.VMID, enabled)
		}
	}

	// Not yet in the table - insert with discovered values so the row is
	// complete, not just a stub with empty fields. The enabled state is the
	// caller's (admin toggle), not hardcoded to 1.
	values := store.TemplateValues{
		Node: found.Node, Name: found.Name, CloudInitCapable: found.CloudInitCapable,
		DiskStorage: found.DiskStorage, DiskSizeGB: found.DiskSizeGB, DiskBus: found.DiskBus,
	}

	return st.InsertTemplate(ctx, clusterName, ref.VMID, values, enabled)
}
