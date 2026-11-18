package catalog

import (
	"context"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"slices"
)

// storageFileKey identifies one ISO on a shared storage, whatever node sees it.
type storageFileKey struct {
	Storage string
	File    string
}

// sharedISOGroup is one shared-storage ISO with every node that reports it.
type sharedISOGroup struct {
	key       storageFileKey
	nodes     []string
	sizeBytes int64
}

// AdminListISOs returns every ISO the cluster reports, unioned with its stored
// approval state. An ISO on local storage gets one row per (node, storage,
// file). An ISO on shared storage gets ONE row (Shared true, NodeCount nodes)
// enabled when any of its per-node rows is; an approved one is extended to
// nodes that have no row yet (nodes added later). Orphan approvals are handled
// as in AdminListNodes; a shared orphan is one row, gone from every node.
func AdminListISOs(ctx context.Context, st *store.Store, client cluster.Client, clusterName string) ([]ISOApproval, error) {
	discovered, err := client.ListISOs(ctx)
	if err != nil {
		return nil, err
	}

	shared, err := sharedStorageNames(ctx, client)
	if err != nil {
		return nil, err
	}

	rows, err := st.CatalogISOsEnabled(ctx, clusterName)
	if err != nil {
		return nil, err
	}

	isShared := func(storage string) bool { return shared[storage] }
	localISOs := slices.DeleteFunc(slices.Clone(discovered), func(i cluster.ISOImage) bool { return isShared(i.Storage) })
	localRows := slices.DeleteFunc(slices.Clone(rows), func(r store.CatalogISOEnabled) bool { return isShared(r.Storage) })
	sharedRows := slices.DeleteFunc(slices.Clone(rows), func(r store.CatalogISOEnabled) bool { return !isShared(r.Storage) })

	out, err := adminListFiles(ctx, st, clusterName, fileListing[store.CatalogISOEnabled, ISOApproval]{
		discovered: isoFileEntries(localISOs),
		listRows: func(context.Context, string) ([]store.CatalogISOEnabled, error) {
			return localRows, nil
		},
		toOrphanRow:  isoOrphanRow,
		removeOrphan: removeISOOrphan,
		viewOf:       isoApprovalView,
		missing:      missingISOApproval,
	})
	if err != nil {
		return nil, err
	}

	sharedOut, err := listSharedISOs(ctx, st, clusterName, groupSharedISOs(discovered, shared), sharedRows)
	if err != nil {
		return nil, err
	}

	return append(out, sharedOut...), nil
}

// groupSharedISOs collapses the per-node sightings of shared-storage ISOs into
// one group per (storage, file), in discovery order.
func groupSharedISOs(discovered []cluster.ISOImage, shared map[string]bool) []sharedISOGroup {
	var groups []sharedISOGroup

	index := make(map[storageFileKey]int)

	for _, iso := range discovered {
		if !shared[iso.Storage] {
			continue
		}

		key := storageFileKey{Storage: iso.Storage, File: iso.File}

		i, ok := index[key]
		if !ok {
			i = len(groups)
			index[key] = i
			groups = append(groups, sharedISOGroup{key: key, sizeBytes: iso.SizeBytes})
		}

		if !slices.Contains(groups[i].nodes, iso.Node) {
			groups[i].nodes = append(groups[i].nodes, iso.Node)
		}
	}

	return groups
}

// listSharedISOs builds the single row of each shared ISO, extends approved
// ones to nodes without a row, and sweeps shared orphans per (storage, file).
func listSharedISOs(ctx context.Context, st *store.Store, clusterName string, groups []sharedISOGroup, rows []store.CatalogISOEnabled) ([]ISOApproval, error) {
	enabledAny := make(map[storageFileKey]bool)
	rowNodes := make(map[storageFileKey][]string)

	for _, row := range rows {
		key := storageFileKey{Storage: row.Storage, File: row.File}
		enabledAny[key] = enabledAny[key] || row.Enabled
		rowNodes[key] = append(rowNodes[key], row.Node)
	}

	out := make([]ISOApproval, 0, len(groups))
	discovered := make(map[storageFileKey]bool, len(groups))

	for _, group := range groups {
		discovered[group.key] = true

		if enabledAny[group.key] {
			if err := extendSharedISO(ctx, st, clusterName, group, rowNodes[group.key]); err != nil {
				return nil, err
			}
		}

		out = append(out, ISOApproval{
			Storage: group.key.Storage, Node: group.nodes[0], File: group.key.File,
			SizeBytes: group.sizeBytes, Enabled: enabledAny[group.key],
			Shared: true, NodeCount: len(group.nodes),
		})
	}

	return sweepSharedISOOrphans(ctx, st, clusterName, out, discovered, rowNodes, enabledAny)
}

// extendSharedISO approves the ISO on every node that sees it but has no row.
func extendSharedISO(ctx context.Context, st *store.Store, clusterName string, group sharedISOGroup, withRow []string) error {
	for _, node := range group.nodes {
		if slices.Contains(withRow, node) {
			continue
		}

		if err := st.SetISOEnabled(ctx, clusterName, node, group.key.Storage, group.key.File, true); err != nil {
			return err
		}
	}

	return nil
}

// sweepSharedISOOrphans applies the orphan rule to shared ISOs as one unit:
// a (storage, file) no node reports any more is auto-removed when enabled,
// otherwise surfaced once as Missing.
func sweepSharedISOOrphans(
	ctx context.Context,
	st *store.Store,
	clusterName string,
	out []ISOApproval,
	discovered map[storageFileKey]bool,
	rowNodes map[storageFileKey][]string,
	enabledAny map[storageFileKey]bool,
) ([]ISOApproval, error) {
	for key, nodes := range rowNodes {
		if discovered[key] {
			continue
		}

		if enabledAny[key] {
			if err := st.DeleteISOAllNodes(ctx, clusterName, key.Storage, key.File); err != nil {
				return nil, err
			}

			continue
		}

		out = append(out, ISOApproval{Storage: key.Storage, Node: nodes[0], File: key.File, Missing: true, Shared: true, NodeCount: len(nodes)})
	}

	return out, nil
}

// sharedStorageNames returns the names of the storages the cluster reports as
// shared (one datastore visible on every node).
func sharedStorageNames(ctx context.Context, client cluster.Client) (map[string]bool, error) {
	snap, err := cluster.ReadTopology(ctx, client)
	if err != nil {
		return nil, err
	}

	shared := make(map[string]bool)

	for _, s := range snap.Storages {
		if cluster.IsSharedStorage(s) {
			shared[s.Name] = true
		}
	}

	return shared, nil
}

// isSharedStorageName reports whether storage is shared per live topology.
func isSharedStorageName(ctx context.Context, client cluster.Client, storage string) (bool, error) {
	shared, err := sharedStorageNames(ctx, client)
	if err != nil {
		return false, err
	}

	return shared[storage], nil
}

// isoNodes returns every node that reports (storage, file).
func isoNodes(ctx context.Context, client cluster.Client, storage, file string) ([]string, error) {
	isos, err := client.ListISOs(ctx)
	if err != nil {
		return nil, err
	}

	var nodes []string

	for _, iso := range isos {
		if iso.Storage == storage && iso.File == file && !slices.Contains(nodes, iso.Node) {
			nodes = append(nodes, iso.Node)
		}
	}

	return nodes, nil
}
