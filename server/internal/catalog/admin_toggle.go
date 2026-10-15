package catalog

import (
	"context"
	"database/sql"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
)

// SetNodeEnabled toggles the enabled flag on a catalog node approval.
// It returns cluster.ErrNotFound if the node is not in the current discovery
// set (never a delete, but toggling an undiscovered resource is a 404). A cluster discovery
// error is surfaced verbatim so the caller can map
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

// ISORef identifies one discovered ISO by its (node, storage, file) triple -
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

// ErrNodeNotFound is returned when a node approval row does not exist for the
// cluster (removing an orphan approval whose node was deleted in Proxmox).
var ErrNodeNotFound = errors.New("node not found")

// ErrStorageNotFound is returned when a storage approval row does not exist.
var ErrStorageNotFound = errors.New("storage not found")

// ErrBridgeNotFound is returned when a bridge approval row does not exist.
var ErrBridgeNotFound = errors.New("bridge not found")

// ErrISONotFound is returned when an ISO approval row does not exist.
var ErrISONotFound = errors.New("iso not found")

// DeleteNode removes a node approval row. Returns ErrNodeNotFound when the
// cluster has no approval for the node - the admin UI offers Remove only on
// missing (orphaned) rows, but the API deletes any approval row.
func DeleteNode(ctx context.Context, st *store.Store, cluster, name string) error {
	err := st.DeleteNode(ctx, cluster, name)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNodeNotFound
	}

	return err
}

// DeleteStorage removes a storage approval row. Returns ErrStorageNotFound when
// the cluster has no approval for the (name, node) pair.
func DeleteStorage(ctx context.Context, st *store.Store, cluster, name, node string) error {
	err := st.DeleteStorage(ctx, cluster, name, node)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrStorageNotFound
	}

	return err
}

// DeleteBridge removes a bridge approval row. Returns ErrBridgeNotFound when
// the cluster has no approval for the (node, name) pair.
func DeleteBridge(ctx context.Context, st *store.Store, cluster, node, name string) error {
	err := st.DeleteBridge(ctx, cluster, node, name)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrBridgeNotFound
	}

	return err
}

// DeleteISO removes an ISO approval row. Returns ErrISONotFound when the
// cluster has no approval for the (node, storage, file) triple.
func DeleteISO(ctx context.Context, st *store.Store, cluster, node, storage, file string) error {
	err := st.DeleteISO(ctx, cluster, node, storage, file)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrISONotFound
	}

	return err
}
