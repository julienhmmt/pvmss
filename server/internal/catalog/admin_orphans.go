package catalog

import (
	"context"
	"fmt"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
)

// nameNodeKey is a composite map key for resources approved by (name, node)
// - storages and bridges - avoiding string-concat collisions when a name
// contains "@".
type nameNodeKey struct {
	Name string
	Node string
}

// fileKey is a composite map key for file approvals (ISOs, cloud images)
// keyed by (node, storage, file), avoiding string-concat collisions when a
// storage or file contains ":".
type fileKey struct {
	Node    string
	Storage string
	File    string
}

// orphanRow flattens one stored approval row to the two fields the orphan
// sweep needs: its discovery key and its enabled flag.
type orphanRow[Key comparable] struct {
	key     Key
	enabled bool
}

// toOrphanRows flattens stored approval rows for sweepOrphans via toRow.
func toOrphanRows[Row any, Key comparable](rows []Row, toRow func(Row) orphanRow[Key]) []orphanRow[Key] {
	out := make([]orphanRow[Key], 0, len(rows))
	for _, row := range rows {
		out = append(out, toRow(row))
	}

	return out
}

// orphanSweep carries the inputs of sweepOrphans: the flattened stored rows,
// the keys discovery still reports, the remove/missing functions, and the
// output disabled orphans append to.
type orphanSweep[Key comparable, A any] struct {
	rows       []orphanRow[Key]
	discovered map[Key]bool
	remove     func(context.Context, *store.Store, string, orphanRow[Key]) error
	missing    func(orphanRow[Key]) A
	out        *[]A
}

// sweepOrphans applies the orphan rule shared by every AdminList* function:
// a stored approval whose key discovery no longer reports is an orphan - an
// enabled orphan is auto-removed via remove (it would otherwise be offered
// to users on a resource that no longer exists), a disabled orphan is
// appended to out via missing (surfaced as Missing=true) so the admin can
// remove it.
func sweepOrphans[Key comparable, A any](
	ctx context.Context,
	st *store.Store,
	clusterName string,
	s orphanSweep[Key, A],
) error {
	for _, row := range s.rows {
		if s.discovered[row.key] {
			continue
		}

		if row.enabled {
			if err := s.remove(ctx, st, clusterName, row); err != nil {
				return err
			}

			continue
		}

		*s.out = append(*s.out, s.missing(row))
	}

	return nil
}

// nodeOrphanRow flattens a stored node approval row for sweepOrphans.
func nodeOrphanRow(row store.CatalogNodeEnabled) orphanRow[string] {
	return orphanRow[string]{key: row.Name, enabled: row.Enabled}
}

// storageOrphanRow flattens a stored storage approval row for sweepOrphans.
func storageOrphanRow(row store.CatalogStorageEnabled) orphanRow[nameNodeKey] {
	return orphanRow[nameNodeKey]{key: nameNodeKey{Name: row.Name, Node: row.Node}, enabled: row.Enabled}
}

// bridgeOrphanRow flattens a stored bridge approval row for sweepOrphans.
func bridgeOrphanRow(row store.CatalogBridgeEnabled) orphanRow[nameNodeKey] {
	return orphanRow[nameNodeKey]{key: nameNodeKey{Name: row.Name, Node: row.Node}, enabled: row.Enabled}
}

// isoOrphanRow flattens a stored ISO approval row for sweepOrphans.
func isoOrphanRow(row store.CatalogISOEnabled) orphanRow[fileKey] {
	return orphanRow[fileKey]{key: fileKey{Node: row.Node, Storage: row.Storage, File: row.File}, enabled: row.Enabled}
}

// imageOrphanRow flattens a stored cloud image approval row for sweepOrphans.
func imageOrphanRow(row store.CatalogImageEnabled) orphanRow[fileKey] {
	return orphanRow[fileKey]{key: fileKey{Node: row.Node, Storage: row.Storage, File: row.File}, enabled: row.Enabled}
}

// removeNodeOrphan deletes an enabled orphan node approval row.
func removeNodeOrphan(ctx context.Context, st *store.Store, clusterName string, row orphanRow[string]) error {
	if err := st.DeleteNode(ctx, clusterName, row.key); err != nil {
		return fmt.Errorf("auto-remove orphan node %q: %w", row.key, err)
	}

	return nil
}

// removeStorageOrphan deletes an enabled orphan storage approval row.
func removeStorageOrphan(ctx context.Context, st *store.Store, clusterName string, row orphanRow[nameNodeKey]) error {
	if err := st.DeleteStorage(ctx, clusterName, row.key.Name, row.key.Node); err != nil {
		return fmt.Errorf("auto-remove orphan storage %q on %q: %w", row.key.Name, row.key.Node, err)
	}

	return nil
}

// removeBridgeOrphan deletes an enabled orphan bridge approval row.
func removeBridgeOrphan(ctx context.Context, st *store.Store, clusterName string, row orphanRow[nameNodeKey]) error {
	if err := st.DeleteBridge(ctx, clusterName, row.key.Node, row.key.Name); err != nil {
		return fmt.Errorf("auto-remove orphan bridge %q on %q: %w", row.key.Name, row.key.Node, err)
	}

	return nil
}

// removeISOOrphan deletes an enabled orphan ISO approval row.
func removeISOOrphan(ctx context.Context, st *store.Store, clusterName string, row orphanRow[fileKey]) error {
	if err := st.DeleteISO(ctx, clusterName, row.key.Node, row.key.Storage, row.key.File); err != nil {
		return fmt.Errorf("auto-remove orphan iso %q on %q: %w", row.key.File, row.key.Node, err)
	}

	return nil
}

// removeImageOrphan deletes an enabled orphan cloud image approval row.
func removeImageOrphan(ctx context.Context, st *store.Store, clusterName string, row orphanRow[fileKey]) error {
	if err := st.DeleteImage(ctx, clusterName, row.key.Node, row.key.Storage, row.key.File); err != nil {
		return fmt.Errorf("auto-remove orphan image %q on %q: %w", row.key.File, row.key.Node, err)
	}

	return nil
}

// missingNodeApproval builds the Missing=true view of a disabled orphan node.
func missingNodeApproval(row orphanRow[string]) NodeApproval {
	return NodeApproval{Name: row.key, Missing: true}
}

// missingStorageApproval builds the Missing=true view of a disabled orphan
// storage.
func missingStorageApproval(row orphanRow[nameNodeKey]) StorageApproval {
	return StorageApproval{Name: row.key.Name, Node: row.key.Node, Missing: true}
}

// missingBridgeApproval builds the Missing=true view of a disabled orphan
// bridge.
func missingBridgeApproval(row orphanRow[nameNodeKey]) BridgeApproval {
	return BridgeApproval{Name: row.key.Name, Node: row.key.Node, Missing: true}
}

// fileEntry is one discovered file resource (ISO or cloud image) normalized
// for the shared approval pipeline - both have the same fields.
type fileEntry struct {
	Storage   string
	Node      string
	File      string
	SizeBytes int64
}

// key returns the (node, storage, file) approval key of the file.
func (f fileEntry) key() fileKey {
	return fileKey{Node: f.Node, Storage: f.Storage, File: f.File}
}

// isoFileEntries normalizes discovered ISOs for adminListFiles.
func isoFileEntries(discovered []cluster.ISOImage) []fileEntry {
	out := make([]fileEntry, 0, len(discovered))
	for _, iso := range discovered {
		out = append(out, fileEntry{Storage: iso.Storage, Node: iso.Node, File: iso.File, SizeBytes: iso.SizeBytes})
	}

	return out
}

// imageFileEntries normalizes discovered cloud images for adminListFiles.
func imageFileEntries(discovered []cluster.CloudImage) []fileEntry {
	out := make([]fileEntry, 0, len(discovered))
	for _, image := range discovered {
		out = append(out, fileEntry{Storage: image.Storage, Node: image.Node, File: image.File, SizeBytes: image.SizeBytes})
	}

	return out
}

// isoApprovalView builds the approval view of one discovered ISO.
func isoApprovalView(file fileEntry, enabled bool) ISOApproval {
	return ISOApproval{
		Storage: file.Storage, Node: file.Node, File: file.File,
		SizeBytes: file.SizeBytes, Enabled: enabled,
	}
}

// imageApprovalView builds the approval view of one discovered cloud image.
func imageApprovalView(file fileEntry, enabled bool) ImageApproval {
	return ImageApproval{
		Storage: file.Storage, Node: file.Node, File: file.File,
		SizeBytes: file.SizeBytes, Enabled: enabled,
	}
}

// missingISOApproval builds the Missing=true view of a disabled orphan ISO.
func missingISOApproval(row orphanRow[fileKey]) ISOApproval {
	return ISOApproval{Storage: row.key.Storage, Node: row.key.Node, File: row.key.File, Missing: true}
}

// missingImageApproval builds the Missing=true view of a disabled orphan
// cloud image.
func missingImageApproval(row orphanRow[fileKey]) ImageApproval {
	return ImageApproval{Storage: row.key.Storage, Node: row.key.Node, File: row.key.File, Missing: true}
}

// fileListing carries the inputs of adminListFiles: the discovered files and
// the per-resource functions that adapt stored rows, orphan removal, and
// approval views.
type fileListing[Row, A any] struct {
	discovered   []fileEntry
	listRows     func(context.Context, string) ([]Row, error)
	toOrphanRow  func(Row) orphanRow[fileKey]
	removeOrphan func(context.Context, *store.Store, string, orphanRow[fileKey]) error
	viewOf       func(fileEntry, bool) A
	missing      func(orphanRow[fileKey]) A
}

// adminListFiles is the shared list-and-sweep pipeline for the two file
// resources: ISOs and cloud images have identical shapes end to end (same
// discovery fields, same (node, storage, file) approval key, same approval
// view). Every discovered file gets an approval view with Enabled from the
// stored row; stored rows whose file vanished are swept as orphans.
func adminListFiles[Row, A any](
	ctx context.Context,
	st *store.Store,
	clusterName string,
	l fileListing[Row, A],
) ([]A, error) {
	rows, err := l.listRows(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	orphans := toOrphanRows(rows, l.toOrphanRow)

	enabledByKey := make(map[fileKey]bool, len(orphans))
	discoveredByKey := make(map[fileKey]bool, len(l.discovered))

	for _, row := range orphans {
		enabledByKey[row.key] = row.enabled
	}

	for _, file := range l.discovered {
		discoveredByKey[file.key()] = true
	}

	out := make([]A, 0, len(l.discovered)+len(orphans))
	for _, file := range l.discovered {
		out = append(out, l.viewOf(file, enabledByKey[file.key()]))
	}

	if err := sweepOrphans(ctx, st, clusterName, orphanSweep[fileKey, A]{
		rows:       orphans,
		discovered: discoveredByKey,
		remove:     l.removeOrphan,
		missing:    l.missing,
		out:        &out,
	}); err != nil {
		return nil, err
	}

	return out, nil
}
