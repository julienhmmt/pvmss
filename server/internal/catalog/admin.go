package catalog

import (
	"context"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
)

// NodeApproval is one discovered node with its admin approval state.
// Missing is true for a stored approval whose node Proxmox no longer reports
// - the row stays listed (greyed out) so the admin can remove it. Only
// disabled orphans are surfaced; enabled orphans are auto-removed since they
// would otherwise be offered to users on a node that no longer exists.
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
	Missing      bool
}

// StorageApproval is one discovered storage with its admin approval state.
// Missing is true for a stored approval whose storage Proxmox no longer
// reports (see NodeApproval for the enabled-orphan auto-remove rule).
type StorageApproval struct {
	Name    string
	Node    string
	Type    string
	Total   int64
	Used    int64
	Enabled bool
	Missing bool
}

// BridgeApproval is one discovered bridge with its admin approval state.
// Missing is true for a stored approval whose bridge Proxmox no longer
// reports (see NodeApproval for the enabled-orphan auto-remove rule).
type BridgeApproval struct {
	Name    string
	Node    string
	Active  bool
	Comment string
	Enabled bool
	Missing bool
}

// ISOApproval is one discovered ISO with its admin approval state.
// Missing is true for a stored approval whose ISO file Proxmox no longer
// reports (see NodeApproval for the enabled-orphan auto-remove rule).
type ISOApproval struct {
	Storage   string
	Node      string
	File      string
	SizeBytes int64
	Enabled   bool
	Missing   bool
}

// TemplateApproval is one discovered Proxmox template with its admin approval
// state. The admin sees all templates the cluster reports and
// toggles which are offered in the create wizard. Missing is true for a
// stored approval whose template Proxmox no longer reports - the
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
	// DiskUnreadable is true when the template's config read failed: the row is shown greyed out
	// and enabling is refused.
	DiskUnreadable bool
	// OverrideDiscovery is true when an admin pinned the editable fields.
	// The list then shows the stored (overridden) values
	// instead of the discovered ones and skips the drift write-back.
	OverrideDiscovery bool
}

// AdminListNodes returns every node the cluster reports, unioned with its
// stored approval state. A node with no catalog row reports enabled=false
// (every resource, not only approved ones).
//
// Stored approvals whose node Proxmox no longer reports are orphans: an
// enabled orphan is auto-removed (it would otherwise be offered to users on a
// node that no longer exists), a disabled orphan is surfaced with Missing=true
// so the admin can remove it.
func AdminListNodes(ctx context.Context, st *store.Store, client cluster.Client, clusterName string) ([]NodeApproval, error) {
	snap, err := client.Snapshot(ctx)
	if err != nil {
		return nil, err
	}

	enabledRows, err := st.CatalogNodesEnabled(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	enabledByName := make(map[string]bool, len(enabledRows))
	discoveredByName := make(map[string]bool, len(snap.Nodes))

	for _, n := range enabledRows {
		enabledByName[n.Name] = n.Enabled
	}

	for _, node := range snap.Nodes {
		discoveredByName[node.Name] = true
	}

	// Count VMs per node from the snapshot.
	vmCountByNode := make(map[string]int)
	for _, vm := range snap.VMs {
		vmCountByNode[vm.Node]++
	}

	out := make([]NodeApproval, 0, len(snap.Nodes)+len(enabledRows))
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

	if err := sweepOrphans(ctx, st, clusterName, orphanSweep[string, NodeApproval]{
		rows:       toOrphanRows(enabledRows, nodeOrphanRow),
		discovered: discoveredByName,
		remove:     removeNodeOrphan,
		missing:    missingNodeApproval,
		out:        &out,
	}); err != nil {
		return nil, err
	}

	return out, nil
}

// AdminListStorages returns every VM-capable storage the cluster reports,
// unioned with its stored approval state per (name, node) pair. Orphan
// approvals (storage gone from Proxmox) are handled as in AdminListNodes.
func AdminListStorages(ctx context.Context, st *store.Store, client cluster.Client, clusterName string) ([]StorageApproval, error) {
	snap, err := client.Snapshot(ctx)
	if err != nil {
		return nil, err
	}

	enabledRows, err := st.CatalogStoragesEnabled(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	enabledByKey := make(map[nameNodeKey]bool, len(enabledRows))
	discoveredByKey := make(map[nameNodeKey]bool, len(snap.Storages))

	for _, s := range enabledRows {
		enabledByKey[nameNodeKey{Name: s.Name, Node: s.Node}] = s.Enabled
	}

	for _, storage := range snap.Storages {
		if !cluster.IsVMCapableStorage(storage) {
			continue
		}

		discoveredByKey[nameNodeKey{Name: storage.Name, Node: storage.Node}] = true
	}

	out := make([]StorageApproval, 0, len(snap.Storages)+len(enabledRows))
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
			Enabled: enabledByKey[nameNodeKey{Name: storage.Name, Node: storage.Node}],
		})
	}

	if err := sweepOrphans(ctx, st, clusterName, orphanSweep[nameNodeKey, StorageApproval]{
		rows:       toOrphanRows(enabledRows, storageOrphanRow),
		discovered: discoveredByKey,
		remove:     removeStorageOrphan,
		missing:    missingStorageApproval,
		out:        &out,
	}); err != nil {
		return nil, err
	}

	return out, nil
}

// AdminListBridges returns every bridge the cluster reports, unioned with its
// stored approval state per (node, name) pair. Orphan approvals (bridge gone
// from Proxmox) are handled as in AdminListNodes.
func AdminListBridges(ctx context.Context, st *store.Store, client cluster.Client, clusterName string) ([]BridgeApproval, error) {
	discovered, err := client.ListBridges(ctx)
	if err != nil {
		return nil, err
	}

	enabledRows, err := st.CatalogBridgesEnabled(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	enabledByKey := make(map[nameNodeKey]bool, len(enabledRows))
	discoveredByKey := make(map[nameNodeKey]bool, len(discovered))

	for _, b := range enabledRows {
		enabledByKey[nameNodeKey{Name: b.Name, Node: b.Node}] = b.Enabled
	}

	for _, bridge := range discovered {
		discoveredByKey[nameNodeKey{Name: bridge.Name, Node: bridge.Node}] = true
	}

	out := make([]BridgeApproval, 0, len(discovered)+len(enabledRows))
	for _, bridge := range discovered {
		out = append(out, BridgeApproval{
			Name:    bridge.Name,
			Node:    bridge.Node,
			Active:  bridge.Active,
			Comment: bridge.Comment,
			Enabled: enabledByKey[nameNodeKey{Name: bridge.Name, Node: bridge.Node}],
		})
	}

	if err := sweepOrphans(ctx, st, clusterName, orphanSweep[nameNodeKey, BridgeApproval]{
		rows:       toOrphanRows(enabledRows, bridgeOrphanRow),
		discovered: discoveredByKey,
		remove:     removeBridgeOrphan,
		missing:    missingBridgeApproval,
		out:        &out,
	}); err != nil {
		return nil, err
	}

	return out, nil
}

// AdminListISOs returns every ISO the cluster reports, unioned with its stored
// approval state keyed by (node, storage, file). Orphan approvals (ISO file
// gone from Proxmox) are handled as in AdminListNodes: enabled orphans are
// auto-removed (a vanished ISO cannot be used and would mislead users),
// disabled orphans are surfaced with Missing=true for manual removal.
func AdminListISOs(ctx context.Context, st *store.Store, client cluster.Client, clusterName string) ([]ISOApproval, error) {
	discovered, err := client.ListISOs(ctx)
	if err != nil {
		return nil, err
	}

	return adminListFiles(ctx, st, clusterName, fileListing[store.CatalogISOEnabled, ISOApproval]{
		discovered:   isoFileEntries(discovered),
		listRows:     st.CatalogISOsEnabled,
		toOrphanRow:  isoOrphanRow,
		removeOrphan: removeISOOrphan,
		viewOf:       isoApprovalView,
		missing:      missingISOApproval,
	})
}

// AdminListTemplates returns every Proxmox template the cluster reports,
// unioned with its stored approval state keyed by VMID.
// Discovery is the truth about a template's field values; the stored row is
// the truth about approval only. When they disagree, the list
// shows the discovered values and the stored row is reconciled with
// UpdateTemplate - a drift write, not a human mutation, so it is not audited.
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
		approval, err := reconcileTemplateApproval(ctx, st, clusterName, tmpl, storedByVMID)
		if err != nil {
			return nil, err
		}

		out = append(out, approval)
	}

	// Orphan approvals: the template is gone from Proxmox but the row (and
	// its enabled flag) lives on. Surface it so the admin can remove it -
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
		approval.OverrideDiscovery = row.OverrideDiscovery

		out = append(out, approval)
	}

	return out, nil
}
