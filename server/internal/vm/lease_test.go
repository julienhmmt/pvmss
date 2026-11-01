package vm_test

import (
	"context"
	"errors"
	"log/slog"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"testing"
	"time"
)

const metaKey = "meta"

const leaseDayDuration = 24 * time.Hour

func adminIdentity() auth.Identity {
	return auth.Identity{Username: testAdminUser, IsAdmin: true}
}

func silentLog() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// leaseSweepDeps wires the sweeper to one cluster: the given index resolver,
// writer, and a refresh that fails with refreshErr (nil = reachable).
func leaseSweepDeps(st *store.Store, idx *inventory.Index, writer cluster.Writer, refreshErr error) vm.LeaseSweepDeps {
	return vm.LeaseSweepDeps{
		Store:    st,
		Log:      silentLog(),
		Clusters: func() []string { return []string{testClusterName} },
		Index:    func(string) (*inventory.Index, error) { return idx, nil },
		Writer:   func(string) (cluster.Writer, error) { return writer, nil },
		Refresh:  func(context.Context, string) (time.Time, error) { return time.Now(), refreshErr },
	}
}

func setLease(t *testing.T, st *store.Store, vmid int, expires time.Time) {
	t.Helper()

	// A VMID with no VM (orphan lease) keeps an empty identity.
	machine, _ := buildResolveIndex(t).Lookup(testClusterName, vmid)

	lease := store.VMLease{Cluster: testClusterName, VMID: vmid, ExpiresAt: expires, Pool: machine.Pool, Identity: cluster.FakeCreationTime}
	if err := st.SetVMLease(context.Background(), lease); err != nil {
		t.Fatalf("SetVMLease(%d): %v", vmid, err)
	}
}

// enableLeases gives the test cluster a lease length, as the sweeper requires.
func enableLeases(t *testing.T, st *store.Store, days int) {
	t.Helper()

	row := store.PolicyRow{Cluster: testClusterName, MaxSockets: 4, MaxCores: 8, MaxMemoryMB: 16384, MaxDiskPerVMGB: 500, MaxNetworkCards: 4, MaxSnapshots: 5, MaxVMPerUser: -1, LeaseDays: days}
	if err := st.UpsertPolicyRow(context.Background(), row); err != nil {
		t.Fatalf("UpsertPolicyRow: %v", err)
	}
}

func leaseExists(t *testing.T, st *store.Store, vmid int) bool {
	t.Helper()

	_, err := st.GetVMLease(context.Background(), testClusterName, vmid)
	if errors.Is(err, store.ErrLeaseNotFound) {
		return false
	}

	if err != nil {
		t.Fatalf("GetVMLease(%d): %v", vmid, err)
	}

	return true
}

func vmPresent(t *testing.T, fake *cluster.Fake, vmid int) bool {
	t.Helper()

	snap, err := fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for _, v := range snap.VMs {
		if v.VMID == vmid {
			return true
		}
	}

	return false
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestSweep_ExpiredVMDeletedAndLeaseDropped(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	enableLeases(t, st, 30)

	now := time.Now().UTC()

	setLease(t, st, 101, now.Add(-leaseDayDuration))
	setLease(t, st, 102, now.Add(leaseDayDuration))

	if got := vm.SweepExpiredLeases(context.Background(), leaseSweepDeps(st, idx, fake, nil), now); got != 1 {
		t.Fatalf("deleted = %d, want 1", got)
	}

	if vmPresent(t, fake, 101) {
		t.Error("expired VM 101 still present")
	}

	if leaseExists(t, st, 101) {
		t.Error("lease for expired VM 101 not dropped")
	}

	if !vmPresent(t, fake, 102) || !leaseExists(t, st, 102) {
		t.Error("VM 102 with future expiry must be untouched")
	}

	rows, err := st.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	var sawExpired bool

	for _, row := range rows {
		if row.Action == "vm.lease_expired" && row.Actor == "system" {
			sawExpired = true
		}
	}

	if !sawExpired {
		t.Errorf("audit rows = %+v, want a vm.lease_expired row by system", rows)
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestSweep_LockedVMKeptForNextCycle(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	enableLeases(t, st, 30)

	now := time.Now().UTC()

	setLease(t, st, 101, now.Add(-leaseDayDuration))

	locked := failingWriter{Fake: *fake, deleteErr: errors.New("VM is locked (backup)")}
	if got := vm.SweepExpiredLeases(context.Background(), leaseSweepDeps(st, idx, locked, nil), now); got != 0 {
		t.Fatalf("deleted = %d, want 0 while locked", got)
	}

	if !leaseExists(t, st, 101) {
		t.Error("lease must be kept so the next sweep retries a locked VM")
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestSweep_UnreachableClusterSkipped(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	enableLeases(t, st, 30)

	now := time.Now().UTC()

	setLease(t, st, 101, now.Add(-leaseDayDuration))

	deps := leaseSweepDeps(st, idx, fake, errors.New("cluster unreachable"))
	if got := vm.SweepExpiredLeases(context.Background(), deps, now); got != 0 {
		t.Fatalf("deleted = %d, want 0 when the cluster cannot be refreshed", got)
	}

	if !vmPresent(t, fake, 101) || !leaseExists(t, st, 101) {
		t.Error("never expire blind: VM and lease must survive an unreachable cluster")
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestSweep_VMGoneDropsOrphanLease(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	enableLeases(t, st, 30)

	now := time.Now().UTC()

	setLease(t, st, 999, now.Add(-leaseDayDuration))

	if got := vm.SweepExpiredLeases(context.Background(), leaseSweepDeps(st, idx, fake, nil), now); got != 0 {
		t.Fatalf("deleted = %d, want 0 for a VM that no longer exists", got)
	}

	if leaseExists(t, st, 999) {
		t.Error("orphan lease for a missing VM must be dropped")
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestExtendLease_ClampsToPolicyAndAudits(t *testing.T) {
	actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	now := time.Now().UTC()

	setLease(t, st, 101, now.Add(leaseDayDuration))

	got, err := vm.ExtendLease(context.Background(), idx, st, vm.LeaseExtension{
		Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 101, Days: 90, MaxDays: 30,
	}, now)
	if err != nil {
		t.Fatalf("ExtendLease: %v", err)
	}

	want := now.Add(30 * leaseDayDuration).Truncate(time.Second)
	if !got.ExpiresAt.Equal(want) {
		t.Errorf("expiresAt = %v, want clamped %v", got.ExpiresAt, want)
	}

	rows, err := st.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	if len(rows) == 0 || rows[len(rows)-1].Action != "vm.lease_extended" {
		t.Errorf("audit rows = %+v, want a trailing vm.lease_extended", rows)
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestExtendLease_Refusals(t *testing.T) {
	actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	now := time.Now().UTC()

	setLease(t, st, 101, now.Add(leaseDayDuration))

	cases := []struct {
		name    string
		req     vm.LeaseExtension
		wantErr error
	}{
		{"leases disabled", vm.LeaseExtension{Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 101, Days: 5, MaxDays: 0}, vm.ErrLeaseDisabled},
		{"zero days", vm.LeaseExtension{Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 101, Days: 0, MaxDays: 30}, vm.ErrLeaseDays},
		{"no lease row", vm.LeaseExtension{Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 102, Days: 5, MaxDays: 30}, store.ErrLeaseNotFound},
		{"not owner", vm.LeaseExtension{Actor: bobIdentity(), ClusterName: testClusterName, VMID: 101, Days: 5, MaxDays: 30}, vm.ErrForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := vm.ExtendLease(context.Background(), idx, st, tc.req, now)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestRemoveLease_ExemptsAndAudits(t *testing.T) {
	actionsIndex(t)
	st := bulkTestStore(t)
	now := time.Now().UTC()

	setLease(t, st, 101, now.Add(leaseDayDuration))

	if err := vm.RemoveLease(context.Background(), st, adminIdentity(), testClusterName, 101, ""); err != nil {
		t.Fatalf("RemoveLease: %v", err)
	}

	if leaseExists(t, st, 101) {
		t.Error("lease still present after RemoveLease")
	}

	err := vm.RemoveLease(context.Background(), st, adminIdentity(), testClusterName, 101, "")
	if !errors.Is(err, store.ErrLeaseNotFound) {
		t.Errorf("second RemoveLease err = %v, want ErrLeaseNotFound", err)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestStartLease_ZeroDaysWritesNothing(t *testing.T) {
	st := bulkTestStore(t)

	if err := vm.StartLease(context.Background(), st, testClusterName, 101, 0, "", time.Now()); err != nil {
		t.Fatalf("StartLease: %v", err)
	}

	if leaseExists(t, st, 101) {
		t.Error("lease written although the cluster has leases off")
	}

	if err := vm.StartLease(context.Background(), st, testClusterName, 101, 7, "pool-x", time.Now()); err != nil {
		t.Fatalf("StartLease: %v", err)
	}

	if !leaseExists(t, st, 101) {
		t.Error("lease not written for lease_days = 7")
	}
}

// A lease written for another pool must not expire the VM now holding the
// VMID: nothing is deleted and the row is kept (fail closed).
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestSweep_PoolMismatchNotDeletedAndKept(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	enableLeases(t, st, 30)

	now := time.Now().UTC()

	stale := store.VMLease{Cluster: testClusterName, VMID: 101, ExpiresAt: now.Add(-leaseDayDuration), Pool: "pool-previous"}
	if err := st.SetVMLease(context.Background(), stale); err != nil {
		t.Fatalf("SetVMLease: %v", err)
	}

	if got := vm.SweepExpiredLeases(context.Background(), leaseSweepDeps(st, idx, fake, nil), now); got != 0 {
		t.Fatalf("deleted = %d, want 0 on a pool mismatch", got)
	}

	if !vmPresent(t, fake, 101) {
		t.Fatal("VM 101 was deleted through a lease that belongs to another pool")
	}

	if !leaseExists(t, st, 101) {
		t.Error("lease row must be kept on a pool mismatch")
	}
}

// Renaming a leased VM through vm.Patch keeps its expiry enforced: the
// expired renamed VM is still deleted by the sweep.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestSweep_RenamedVMStillExpires(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	enableLeases(t, st, 30)

	now := time.Now().UTC()

	setLease(t, st, 101, now.Add(-leaseDayDuration))

	err := vm.Patch(context.Background(), vm.WriteDeps{Index: idx, Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 101, Writer: fake, Audit: st, Refresher: noopRefresher{}, Store: st}, "renamed-vm", "")
	if err != nil {
		t.Fatalf("Patch: %v", err)
	}

	if got := vm.SweepExpiredLeases(context.Background(), leaseSweepDeps(st, idx, fake, nil), now); got != 1 {
		t.Fatalf("deleted = %d, want 1 for the expired renamed VM", got)
	}

	if vmPresent(t, fake, 101) {
		t.Error("expired renamed VM still present")
	}
}

// A cluster whose lease_days is 0 expires nothing, even with an expired row.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestSweep_LeaseDaysZeroSkipsCluster(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	enableLeases(t, st, 0)

	now := time.Now().UTC()

	setLease(t, st, 101, now.Add(-leaseDayDuration))

	if got := vm.SweepExpiredLeases(context.Background(), leaseSweepDeps(st, idx, fake, nil), now); got != 0 {
		t.Fatalf("deleted = %d, want 0 when lease_days = 0", got)
	}

	if !vmPresent(t, fake, 101) || !leaseExists(t, st, 101) {
		t.Error("cluster with leases off must not touch the VM or its row")
	}
}

// Deleting a leased VM removes its lease, so the VMID carries no expiry.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestDelete_RemovesLease(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	setLease(t, st, 101, time.Now().UTC().Add(leaseDayDuration))

	err := vm.Delete(context.Background(), vm.WriteDeps{Index: idx, Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 101, Writer: fake, Audit: st, Refresher: noopRefresher{}, Store: st})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if leaseExists(t, st, 101) {
		t.Error("lease still present after the VM was deleted")
	}
}

// StartLease on a VMID that still has a row from an earlier VM overwrites it
// with the new VM's identity, and clears it when the cluster has no leases.
//
//nolint:paralleltest // serial: shared database fixture
func TestStartLease_OverwritesStaleRow(t *testing.T) {
	st := bulkTestStore(t)
	ctx := context.Background()

	stale := store.VMLease{Cluster: testClusterName, VMID: 101, ExpiresAt: time.Now().Add(time.Hour), Pool: "pool-previous"}
	if err := st.SetVMLease(ctx, stale); err != nil {
		t.Fatalf("SetVMLease: %v", err)
	}

	if err := vm.StartLease(ctx, st, testClusterName, 101, 7, "pool-new", time.Now()); err != nil {
		t.Fatalf("StartLease: %v", err)
	}

	got, err := st.GetVMLease(ctx, testClusterName, 101)
	if err != nil || got.Pool != "pool-new" {
		t.Fatalf("lease = %+v, err = %v, want new identity", got, err)
	}

	if err := vm.StartLease(ctx, st, testClusterName, 101, 0, "", time.Now()); err != nil {
		t.Fatalf("StartLease(0): %v", err)
	}

	if leaseExists(t, st, 101) {
		t.Error("stale row survived a create on a cluster with leases off")
	}
}

// metaWriter is the fake cluster with the VM config's meta property replaced,
// to model a VMID reused by a different VM (another creation time) or a
// config without an identity.
type metaWriter struct {
	cluster.Fake
	meta string
}

func (w metaWriter) SnapshotConfig(ctx context.Context, node string, vmid int, name string) (map[string]string, error) {
	cfg, err := w.Fake.SnapshotConfig(ctx, node, vmid, name)
	if err != nil {
		return nil, err
	}

	cfg[metaKey] = w.meta

	return cfg, nil
}

// A recycled VMID inside the same pool: the lease was bound to the previous
// VM's creation time. The live VM has another one, so nothing is deleted and
// the row is dropped.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestSweep_RecycledVMIDSamePoolNotDeleted(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	enableLeases(t, st, 30)

	now := time.Now().UTC()

	setLease(t, st, 101, now.Add(-leaseDayDuration))

	writer := metaWriter{Fake: *fake, meta: "creation-time=1800000000,vmgenid=11111111-1111-1111-1111-111111111111"}
	if got := vm.SweepExpiredLeases(context.Background(), leaseSweepDeps(st, idx, writer, nil), now); got != 0 {
		t.Fatalf("deleted = %d, want 0 for a recycled VMID", got)
	}

	if !vmPresent(t, fake, 101) {
		t.Fatal("VM 101 was deleted through a lease bound to another VM")
	}

	if leaseExists(t, st, 101) {
		t.Error("lease of a VM that no longer exists must be dropped")
	}
}

// A live config without an identity cannot be matched: nothing is deleted and
// the row is dropped.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestSweep_MissingIdentityNotDeleted(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	enableLeases(t, st, 30)

	now := time.Now().UTC()

	setLease(t, st, 101, now.Add(-leaseDayDuration))

	writer := metaWriter{Fake: *fake, meta: ""}
	if got := vm.SweepExpiredLeases(context.Background(), leaseSweepDeps(st, idx, writer, nil), now); got != 0 {
		t.Fatalf("deleted = %d, want 0 without a live identity", got)
	}

	if !vmPresent(t, fake, 101) {
		t.Error("VM must survive when the live identity is missing")
	}

	if !leaseExists(t, st, 101) {
		t.Error("row must be kept when the live identity is unknown")
	}
}

// A lease never bound to an identity (its create task never finished) is
// never deleted.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestSweep_UnboundLeaseNotDeleted(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	enableLeases(t, st, 30)

	now := time.Now().UTC()

	machine, _ := idx.Lookup(testClusterName, 101)

	unbound := store.VMLease{Cluster: testClusterName, VMID: 101, ExpiresAt: now.Add(-leaseDayDuration), Pool: machine.Pool}
	if err := st.SetVMLease(context.Background(), unbound); err != nil {
		t.Fatalf("SetVMLease: %v", err)
	}

	if got := vm.SweepExpiredLeases(context.Background(), leaseSweepDeps(st, idx, fake, nil), now); got != 0 {
		t.Fatalf("deleted = %d, want 0 for an unbound lease", got)
	}

	if !vmPresent(t, fake, 101) {
		t.Error("unbound lease deleted a VM")
	}

	if !leaseExists(t, st, 101) {
		t.Error("unbound lease must be kept, not dropped")
	}
}

func TestCreationIdentity(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		cfg  map[string]string
		want string
	}{
		"creation time":      {map[string]string{metaKey: "creation-time=1700000000,vmgenid=abc"}, "1700000000"},
		"vmgenid fallback":   {map[string]string{metaKey: "vmgenid=abc"}, "abc"},
		"no meta":            {map[string]string{}, ""},
		"meta without value": {map[string]string{metaKey: "garbage"}, ""},
		"top-level vmgenid":  {map[string]string{"vmgenid": "top"}, "top"},
		"meta beats vmgenid": {map[string]string{metaKey: "ctime=7", "vmgenid": "top"}, "7"},
	}

	for name, tc := range cases {
		if got := vm.CreationIdentity(tc.cfg); got != tc.want {
			t.Errorf("%s: VMIdentity = %q, want %q", name, got, tc.want)
		}
	}
}

// Extending a lease after its identity was bound keeps the identity: the
// extension writes the expiry only.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestExtendLease_KeepsBoundIdentity(t *testing.T) {
	actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	now := time.Now().UTC()

	setLease(t, st, 101, now.Add(time.Hour))

	_, err := vm.ExtendLease(context.Background(), idx, st, vm.LeaseExtension{
		Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 101, Days: 5, MaxDays: 30,
	}, now)
	if err != nil {
		t.Fatalf("ExtendLease: %v", err)
	}

	got, err := st.GetVMLease(context.Background(), testClusterName, 101)
	if err != nil {
		t.Fatalf("GetVMLease: %v", err)
	}

	if got.Identity != cluster.FakeCreationTime {
		t.Errorf("identity = %q after extend, want %q", got.Identity, cluster.FakeCreationTime)
	}
}
