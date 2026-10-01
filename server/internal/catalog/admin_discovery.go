package catalog

import (
	"context"
	"pvmss/server/internal/cluster"
	"slices"
)

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
