package vm

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"time"
)

// LeaseSweepDeps are the per-cluster capabilities the sweeper needs. They are
// closures so the caller decides how a cluster name maps to its index, writer
// and refresh; a missing or failing lookup skips that cluster for the cycle.
type LeaseSweepDeps struct {
	Store    *store.Store
	Log      *slog.Logger
	Clusters func() []string
	Index    func(name string) (*inventory.Index, error)
	Writer   func(name string) (cluster.Writer, error)
	// Refresh rebuilds one cluster's inventory. An error means the cluster is
	// unreachable or stale, so the sweeper must not act on it this cycle.
	Refresh func(ctx context.Context, name string) (time.Time, error)
}

// refresherFunc adapts a closure to IndexRefresher for WriteDeps.
type refresherFunc func(context.Context) (time.Time, error)

func (f refresherFunc) Refresh(ctx context.Context) (time.Time, error) { return f(ctx) }

// SweepExpiredLeases deletes every VM whose lease expired before now, and
// returns how many were deleted. Per cluster it refreshes the inventory first
// and skips the whole cluster on failure (never expire blind). Each VM goes
// through Delete with the system actor and Force, so a running VM is stopped
// first. A lock or any other failure keeps the lease row and is retried by
// the next sweep. A lease whose VM is gone from PVMSS scope is dropped.
func SweepExpiredLeases(ctx context.Context, deps LeaseSweepDeps, now time.Time) int {
	deleted := 0

	for _, name := range deps.Clusters() {
		deleted += sweepCluster(ctx, deps, name, now)
	}

	return deleted
}

func sweepCluster(ctx context.Context, deps LeaseSweepDeps, name string, now time.Time) int {
	refresh := refresherFunc(func(c context.Context) (time.Time, error) { return deps.Refresh(c, name) })

	if _, err := refresh(ctx); err != nil {
		deps.Log.WarnContext(ctx, "lease sweep skipped: cluster not refreshed", "component", "vm", "event", "lease_sweep_skipped", "cluster", name, "error", err)

		return 0
	}

	index, err := deps.Index(name)
	if err != nil {
		deps.Log.WarnContext(ctx, "lease sweep skipped: no index", "component", "vm", "event", "lease_sweep_skipped", "cluster", name, "error", err)

		return 0
	}

	writer, err := deps.Writer(name)
	if err != nil {
		deps.Log.WarnContext(ctx, "lease sweep skipped: no writer", "component", "vm", "event", "lease_sweep_skipped", "cluster", name, "error", err)

		return 0
	}

	// A cluster with leases off expires nothing, whatever stale rows remain.
	policy, err := deps.Store.PolicyRow(ctx, name)
	if errors.Is(err, sql.ErrNoRows) {
		return 0
	}

	if err != nil {
		deps.Log.ErrorContext(ctx, "lease sweep skipped: policy read failed", "component", "vm", "event", "lease_sweep_failed", "cluster", name, "error", err)

		return 0
	}

	if policy.LeaseDays <= 0 {
		return 0
	}

	expired, err := deps.Store.ListExpiredVMLeases(ctx, name, now)
	if err != nil {
		deps.Log.ErrorContext(ctx, "lease sweep: list expired leases failed", "component", "vm", "event", "lease_sweep_failed", "cluster", name, "error", err)

		return 0
	}

	deleted := 0

	for _, lease := range expired {
		if expireLease(ctx, deps, lease, index, writer, refresh, now) {
			deleted++
		}
	}

	return deleted
}

// expireLease deletes one expired VM. It reports whether the VM was deleted.
func expireLease(ctx context.Context, deps LeaseSweepDeps, lease store.VMLease, index *inventory.Index, writer cluster.Writer, refresh refresherFunc, now time.Time) bool {
	// Re-read before acting: a concurrent owner extension must win over an
	// expiry listed a moment ago.
	current, err := deps.Store.GetVMLease(ctx, lease.Cluster, lease.VMID)
	if errors.Is(err, store.ErrLeaseNotFound) {
		return false
	}

	if err != nil || !current.ExpiresAt.Before(now) {
		return false
	}

	entity, err := Resolve(index, systemActor, lease.Cluster, lease.VMID, deps.Store)
	if errors.Is(err, ErrNotFound) {
		// The VM is gone or out of PVMSS scope: nothing left to expire.
		dropLease(ctx, deps, lease)

		return false
	}

	if err != nil {
		deps.Log.WarnContext(ctx, "lease expiry deferred: VM not resolvable", "component", "vm", "event", "lease_expiry_deferred", "cluster", lease.Cluster, "vmid", lease.VMID, "error", err)

		return false
	}

	// Fail closed: a pool mismatch means the VMID now belongs to another
	// owner. Keep the row and delete nothing; the row is cleared by the next
	// create or delete on this VMID.
	if entity.Pool != lease.Pool {
		deps.Log.WarnContext(ctx, "lease pool mismatch: not deleted", "component", "vm", "event", "lease_pool_mismatch", "cluster", lease.Cluster, "vmid", lease.VMID)

		return false
	}

	if !matchesIdentity(ctx, deps, lease, writer, entity.Node) {
		return false
	}

	err = Delete(ctx, WriteDeps{
		Index: index, Actor: systemActor, ClusterName: lease.Cluster, VMID: lease.VMID,
		Writer: writer, Audit: deps.Store, Refresher: refresh, Store: deps.Store, Log: deps.Log, Force: true,
	})
	if errors.Is(err, ErrNotFound) {
		// The VM is gone or out of PVMSS scope: nothing left to expire.
		dropLease(ctx, deps, lease)

		return false
	}

	if err != nil {
		deps.Log.WarnContext(ctx, "lease expiry deferred: will retry next sweep", "component", "vm", "event", "lease_expiry_deferred", "cluster", lease.Cluster, "vmid", lease.VMID, "error", err)

		return false
	}

	dropLease(ctx, deps, lease)

	if err := deps.Store.RecordAdminAction(ctx, systemActor.Username, "vm.lease_expired", "vm", FormatVMRef(lease.Cluster, lease.VMID), leaseDetail("lease expired, vm deleted", map[string]any{"field": "expiresAt", "old": current.ExpiresAt.Format(time.RFC3339), "new": "deleted"}), ""); err != nil {
		deps.Log.ErrorContext(ctx, "lease expired audit failed", "component", "vm", "event", "audit_failed", "cluster", lease.Cluster, "vmid", lease.VMID, "error", err)
	}

	deps.Log.InfoContext(ctx, "vm lease expired and deleted", "component", "vm", "event", "lease_expired", "cluster", lease.Cluster, "vmid", lease.VMID)

	return true
}

func dropLease(ctx context.Context, deps LeaseSweepDeps, lease store.VMLease) {
	if err := deps.Store.DeleteVMLease(ctx, lease.Cluster, lease.VMID); err != nil {
		deps.Log.ErrorContext(ctx, "drop vm lease failed", "component", "vm", "event", "lease_drop_failed", "cluster", lease.Cluster, "vmid", lease.VMID, "error", err)
	}
}

// matchesIdentity reports whether the live VM is the one the lease was bound
// to. A missing or different identity drops the row without deleting. A failed
// config read keeps the row for the next sweep: it is not evidence of a
// recycled VMID.
func matchesIdentity(ctx context.Context, deps LeaseSweepDeps, lease store.VMLease, writer cluster.Writer, node string) bool {
	reader, ok := writer.(cluster.SnapshotConfigReader)
	if !ok {
		deps.Log.WarnContext(ctx, "lease identity unreadable: not deleted", "component", "vm", "event", "lease_identity_unreadable", "cluster", lease.Cluster, "vmid", lease.VMID)

		return false
	}

	cfg, err := reader.SnapshotConfig(ctx, node, lease.VMID, "current")
	if err != nil {
		deps.Log.WarnContext(ctx, "lease identity read failed: retried next sweep", "component", "vm", "event", "lease_identity_read_failed", "cluster", lease.Cluster, "vmid", lease.VMID, "error", err)

		return false
	}

	live := CreationIdentity(cfg)

	// Unknown on either side: keep the row and delete nothing. Binding the
	// live identity here would attach the lease to whatever VM holds the VMID.
	if lease.Identity == "" || live == "" {
		deps.Log.WarnContext(ctx, "lease identity unknown: kept, not deleted", "component", "vm", "event", "lease_identity_unknown", "cluster", lease.Cluster, "vmid", lease.VMID)

		return false
	}

	// A positive mismatch: the VMID now belongs to another VM. Drop the row
	// and delete nothing.
	if live != lease.Identity {
		deps.Log.WarnContext(ctx, "lease identity mismatch: not deleted, lease dropped", "component", "vm", "event", "lease_identity_mismatch", "cluster", lease.Cluster, "vmid", lease.VMID)
		dropLease(ctx, deps, lease)

		return false
	}

	return true
}
