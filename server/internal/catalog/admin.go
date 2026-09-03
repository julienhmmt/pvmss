package catalog

import (
	"context"
	"database/sql"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"slices"
)

// NodeApproval is one discovered node with its admin approval state.
type NodeApproval struct {
	Name         string
	Status       string
	CPUCores     int
	CPUUsage     float64
	MemoryTotal  int64
	MemoryUsed   int64
	StorageTotal int64
	StorageUsed  int64
	VMCount      int
	Enabled      bool
}

// StorageApproval is one discovered storage with its admin approval state.
type StorageApproval struct {
	Name    string
	Node    string
	Type    string
	Total   int64
	Used    int64
	Enabled bool
}

// BridgeApproval is one discovered bridge with its admin approval state.
type BridgeApproval struct {
	Name    string
	Node    string
	Active  bool
	Comment string
	Enabled bool
}

// ISOApproval is one discovered ISO with its admin approval state.
type ISOApproval struct {
	Storage   string
	Node      string
	File      string
	SizeBytes int64
	Enabled   bool
}

// TemplateApproval is one discovered Proxmox template with its admin approval
// state (US2/issue-02). The admin sees all templates the cluster reports and
// toggles which are offered in the create wizard. Missing is true for a
// stored approval whose template Proxmox no longer reports (issue 02) — the
// row stays visible so the admin can remove it.
type TemplateApproval struct {
	VMID             int
	Node             string
	Name             string
	CloudInitCapable bool
	DiskStorage      string
	DiskSizeGB       int
	DiskBus          string
	Enabled          bool
	Missing          bool
	// DiskUnreadable is true when the template's config read failed (issue
	// 03): the row is shown greyed out and enabling is refused.
	DiskUnreadable bool
}

// AdminListNodes returns every node the cluster reports, unioned with its
// stored approval state. A node with no catalog row reports enabled=false
// (FR-001: every resource, not only approved ones).
func AdminListNodes(ctx context.Context, st *store.Store, client cluster.Client, clusterName string) ([]NodeApproval, error) {
	snap, err := client.Snapshot(ctx)
	if err != nil {
		return nil, err
	}

	enabledMap, err := st.CatalogNodesEnabled(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	enabledByName := make(map[string]bool, len(enabledMap))
	for _, n := range enabledMap {
		enabledByName[n.Name] = n.Enabled
	}

	// Count VMs per node from the snapshot.
	vmCountByNode := make(map[string]int)
	for _, vm := range snap.VMs {
		vmCountByNode[vm.Node]++
	}

	out := make([]NodeApproval, 0, len(snap.Nodes))
	for _, node := range snap.Nodes {
		out = append(out, NodeApproval{
			Name:         node.Name,
			Status:       string(node.Status),
			CPUCores:     node.CPUCores,
			CPUUsage:     node.CPUUsage,
			MemoryTotal:  node.MemoryTotal,
			MemoryUsed:   node.MemoryUsed,
			StorageTotal: node.StorageTotal,
			StorageUsed:  node.StorageUsed,
			VMCount:      vmCountByNode[node.Name],
			Enabled:      enabledByName[node.Name],
		})
	}

	return out, nil
}

// AdminListStorages returns every storage the cluster reports, unioned with
// its stored approval state per (name, node) pair.
func AdminListStorages(ctx context.Context, st *store.Store, client cluster.Client, clusterName string) ([]StorageApproval, error) {
	snap, err := client.Snapshot(ctx)
	if err != nil {
		return nil, err
	}

	enabledRows, err := st.CatalogStoragesEnabled(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	enabledByKey := make(map[storageKey]bool, len(enabledRows))
	for _, s := range enabledRows {
		enabledByKey[storageKey{Name: s.Name, Node: s.Node}] = s.Enabled
	}

	out := make([]StorageApproval, 0, len(snap.Storages))
	for _, storage := range snap.Storages {
		if !cluster.IsVMCapableStorage(storage) {
			continue
		}

		out = append(out, StorageApproval{
			Name:    storage.Name,
			Node:    storage.Node,
			Type:    storage.Type,
			Total:   storage.Total,
			Used:    storage.Used,
			Enabled: enabledByKey[storageKey{Name: storage.Name, Node: storage.Node}],
		})
	}

	return out, nil
}

// AdminListBridges returns every bridge the cluster reports, unioned with its
// stored approval state per (node, name) pair.
func AdminListBridges(ctx context.Context, st *store.Store, client cluster.Client, clusterName string) ([]BridgeApproval, error) {
	discovered, err := client.ListBridges(ctx)
	if err != nil {
		return nil, err
	}

	enabledRows, err := st.CatalogBridgesEnabled(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	enabledByKey := make(map[bridgeKey]bool, len(enabledRows))
	for _, b := range enabledRows {
		enabledByKey[bridgeKey{Name: b.Name, Node: b.Node}] = b.Enabled
	}

	out := make([]BridgeApproval, 0, len(discovered))
	for _, bridge := range discovered {
		out = append(out, BridgeApproval{
			Name:    bridge.Name,
			Node:    bridge.Node,
			Active:  bridge.Active,
			Comment: bridge.Comment,
			Enabled: enabledByKey[bridgeKey{Name: bridge.Name, Node: bridge.Node}],
		})
	}

	return out, nil
}

// AdminListISOs returns every ISO the cluster reports, unioned with its stored
// approval state keyed by (node, storage, file).
func AdminListISOs(ctx context.Context, st *store.Store, client cluster.Client, clusterName string) ([]ISOApproval, error) {
	discovered, err := client.ListISOs(ctx)
	if err != nil {
		return nil, err
	}

	enabledRows, err := st.CatalogISOsEnabled(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	enabledByKey := make(map[isoKey]bool, len(enabledRows))
	for _, i := range enabledRows {
		enabledByKey[isoKey{Node: i.Node, Storage: i.Storage, File: i.File}] = i.Enabled
	}

	out := make([]ISOApproval, 0, len(discovered))
	for _, iso := range discovered {
		out = append(out, ISOApproval{
			Storage:   iso.Storage,
			Node:      iso.Node,
			File:      iso.File,
			SizeBytes: iso.SizeBytes,
			Enabled:   enabledByKey[isoKey{Node: iso.Node, Storage: iso.Storage, File: iso.File}],
		})
	}

	return out, nil
}

// AdminListTemplates returns every Proxmox template the cluster reports,
// unioned with its stored approval state keyed by VMID (US2/issue-02).
// Discovery is the truth about a template's field values; the stored row is
// the truth about approval only (issue 02). When they disagree, the list
// shows the discovered values and the stored row is reconciled with
// UpdateTemplate — a drift write, not a human mutation, so it is not audited.
// Stored rows with no discovered match are appended with Missing=true so the
// admin can see and remove an approval whose template was deleted in
// Proxmox.
func AdminListTemplates(ctx context.Context, st *store.Store, client cluster.Client, clusterName string) ([]TemplateApproval, error) {
	discovered, err := client.ListTemplates(ctx)
	if err != nil {
		return nil, err
	}

	storedRows, err := st.CatalogTemplatesEnabled(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	storedByVMID := make(map[int]store.CatalogTemplateEnabled, len(storedRows))
	for _, row := range storedRows {
		storedByVMID[row.VMID] = row
	}

	discoveredByVMID := make(map[int]bool, len(discovered))
	for _, tmpl := range discovered {
		discoveredByVMID[tmpl.VMID] = true
	}

	out := make([]TemplateApproval, 0, len(discovered)+len(storedRows))
	for _, tmpl := range discovered {
		approval := newTemplateApproval(tmpl)
		approval.DiskUnreadable = tmpl.DiskUnreadable

		if stored, ok := storedByVMID[tmpl.VMID]; ok {
			approval.Enabled = stored.Enabled

			// An unreadable discovery reports empty disk fields (issue 03) —
			// never write them over the stored, approval-time values: the
			// clone-time fallback (T17) relies on them being non-empty.
			if !tmpl.DiskUnreadable && templateDrift(stored, tmpl) {
				values := store.TemplateValues{
					Node: tmpl.Node, Name: tmpl.Name, CloudInitCapable: tmpl.CloudInitCapable,
					DiskStorage: tmpl.DiskStorage, DiskSizeGB: tmpl.DiskSizeGB, DiskBus: tmpl.DiskBus,
				}

				if err := st.UpdateTemplate(ctx, clusterName, tmpl.VMID, values); err != nil {
					return nil, err
				}
			}
		}

		out = append(out, approval)
	}

	// Orphan approvals: the template is gone from Proxmox but the row (and
	// its enabled flag) lives on. Surface it so the admin can remove it —
	// otherwise it would be invisible yet still offered to users.
	for _, row := range storedRows {
		if discoveredByVMID[row.VMID] {
			continue
		}

		approval := newTemplateApproval(cluster.TemplateVM{
			VMID: row.VMID, Node: row.Node, Name: row.Name, CloudInitCapable: row.CloudInitCapable,
			DiskStorage: row.DiskStorage, DiskSizeGB: row.DiskSizeGB, DiskBus: row.DiskBus,
		})
		approval.Enabled = row.Enabled
		approval.Missing = true

		out = append(out, approval)
	}

	return out, nil
}

// newTemplateApproval builds the approval view of one discovered template
// with Enabled left false — the caller sets it from the stored row.
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
// from what discovery currently reports (issue 02: the stored row is an
// approval-time snapshot and can go stale).
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
// for the cluster (issue 02: removing an orphan approval).
var ErrTemplateNotFound = errors.New("template not found")

// DeleteTemplate removes a template approval row. Returns
// ErrTemplateNotFound when the cluster has no approval for the vmid — the
// admin UI only offers Remove on missing (orphaned) rows, but the API deletes
// any approval row.
func DeleteTemplate(ctx context.Context, st *store.Store, cluster string, vmid int) error {
	err := st.DeleteTemplate(ctx, cluster, vmid)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrTemplateNotFound
	}

	return err
}

// ErrTemplateUnreadable is returned when approving a template whose disk
// config could not be read (issue 03): the approval row would carry empty
// disk_bus/disk_storage and break the post-clone resize. Disabling stays
// possible.
var ErrTemplateUnreadable = errors.New("template disk unreadable")

// SetTemplateEnabled upserts the enabled state for one template. Returns
// cluster.ErrNotFound if the template is not in the current discovery set.
// See SetNodeEnabled for the discovery-error contract.
//
// The discovered template's field values are used to populate the row on
// first approval (so the row is complete, not a stub with empty fields).
// The lookup is a single TemplateByVMID call (issue 03) — not a full
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

	// Not yet in the table — insert with discovered values so the row is
	// complete, not just a stub with empty fields. The enabled state is the
	// caller's (admin toggle), not hardcoded to 1.
	values := store.TemplateValues{
		Node: found.Node, Name: found.Name, CloudInitCapable: found.CloudInitCapable,
		DiskStorage: found.DiskStorage, DiskSizeGB: found.DiskSizeGB, DiskBus: found.DiskBus,
	}

	return st.InsertTemplate(ctx, clusterName, ref.VMID, values, enabled)
}

// SetNodeEnabled toggles the enabled flag on a catalog node approval.
// It returns cluster.ErrNotFound if the node is not in the current discovery
// set (FR-006: never a delete, but toggling an undiscovered resource is a
// 404). A cluster discovery error is surfaced verbatim so the caller can map
// it to 5xx instead of mistaking it for a 404.
func SetNodeEnabled(ctx context.Context, st *store.Store, client cluster.Client, clusterName, name string, enabled bool) error {
	discovered, err := nodeDiscovered(ctx, client, name)
	if err != nil {
		return err
	}

	if !discovered {
		return cluster.ErrNotFound
	}

	return st.SetNodeEnabled(ctx, clusterName, name, enabled)
}

// SetStorageEnabled upserts the enabled state for one (name, node) pair.
// See SetNodeEnabled for the discovery-error contract.
func SetStorageEnabled(ctx context.Context, st *store.Store, client cluster.Client, clusterName, name, node string, enabled bool) error {
	discovered, err := storageDiscovered(ctx, client, name, node)
	if err != nil {
		return err
	}

	if !discovered {
		return cluster.ErrNotFound
	}

	return st.SetStorageEnabled(ctx, clusterName, name, node, enabled)
}

// SetBridgeEnabled upserts the enabled state for one (node, name) pair.
// See SetNodeEnabled for the discovery-error contract.
func SetBridgeEnabled(ctx context.Context, st *store.Store, client cluster.Client, clusterName, node, name string, enabled bool) error {
	discovered, err := bridgeDiscovered(ctx, client, node, name)
	if err != nil {
		return err
	}

	if !discovered {
		return cluster.ErrNotFound
	}

	return st.SetBridgeEnabled(ctx, clusterName, node, name, enabled)
}

// ISORef identifies one discovered ISO by its (node, storage, file) triple —
// the same key the enabled-state store and discovery check use internally.
type ISORef struct {
	Node    string
	Storage string
	File    string
}

// SetISOEnabled upserts the enabled state for one ISO identified by ref.
// See SetNodeEnabled for the discovery-error contract.
func SetISOEnabled(ctx context.Context, st *store.Store, client cluster.Client, clusterName string, ref ISORef, enabled bool) error {
	discovered, err := isoDiscovered(ctx, client, ref.Node, ref.Storage, ref.File)
	if err != nil {
		return err
	}

	if !discovered {
		return cluster.ErrNotFound
	}

	return st.SetISOEnabled(ctx, clusterName, ref.Node, ref.Storage, ref.File, enabled)
}

// nodeDiscovered reports whether the cluster reports a node with the given
// name. A discovery error is returned verbatim so the caller can distinguish
// "not present" (404) from "cluster unreachable" (5xx).
func nodeDiscovered(ctx context.Context, client cluster.Client, name string) (bool, error) {
	snap, err := client.Snapshot(ctx)
	if err != nil {
		return false, err
	}

	return slices.ContainsFunc(snap.Nodes, func(n cluster.Node) bool { return n.Name == name }), nil
}

// storageDiscovered reports whether the cluster reports a (name, node) storage.
// See nodeDiscovered for the error contract.
func storageDiscovered(ctx context.Context, client cluster.Client, name, node string) (bool, error) {
	snap, err := client.Snapshot(ctx)
	if err != nil {
		return false, err
	}

	return slices.ContainsFunc(snap.Storages, func(s cluster.Storage) bool {
		return s.Name == name && s.Node == node && cluster.IsVMCapableStorage(s)
	}), nil
}

// bridgeDiscovered reports whether the cluster reports a bridge with the given
// (node, name) pair. See nodeDiscovered for the error contract.
func bridgeDiscovered(ctx context.Context, client cluster.Client, node, name string) (bool, error) {
	bridges, err := client.ListBridges(ctx)
	if err != nil {
		return false, err
	}

	return slices.ContainsFunc(bridges, func(b cluster.Bridge) bool {
		return b.Name == name && b.Node == node
	}), nil
}

// isoDiscovered reports whether the cluster reports an ISO with the given
// (node, storage, file) triple. See nodeDiscovered for the error contract.
func isoDiscovered(ctx context.Context, client cluster.Client, node, storage, file string) (bool, error) {
	isos, err := client.ListISOs(ctx)
	if err != nil {
		return false, err
	}

	return slices.ContainsFunc(isos, func(i cluster.ISOImage) bool {
		return i.Node == node && i.Storage == storage && i.File == file
	}), nil
}

// storageKey is a composite map key for storages, avoiding string-concat
// collisions when a name contains "@".
type storageKey struct {
	Name string
	Node string
}

type bridgeKey struct {
	Name string
	Node string
}

// isoKey is a composite map key for ISOs, avoiding string-concat collisions
// when a storage or file contains ":".
type isoKey struct {
	Node    string
	Storage string
	File    string
}
