package vm_test

import (
	"context"
	"errors"
	"log/slog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"testing"
	"time"
)

// customLeaseDeps lets a test stub each closure independently.
func customLeaseDeps(st *store.Store, indexErr, writerErr error) vm.LeaseSweepDeps {
	return vm.LeaseSweepDeps{
		Store:    st,
		Log:      slog.New(slog.DiscardHandler),
		Clusters: func() []string { return []string{testClusterName} },
		Index: func(string) (*inventory.Index, error) {
			return nil, indexErr
		},
		Writer: func(string) (cluster.Writer, error) {
			return nil, writerErr
		},
		Refresh: func(context.Context, string) (time.Time, error) { return time.Now(), nil },
	}
}

func TestSweep_IndexErrorSkipsCluster(t *testing.T) {
	t.Parallel()

	st := bulkTestStore(t)
	enableLeases(t, st, 30)
	if got := vm.SweepExpiredLeases(context.Background(), customLeaseDeps(st, errBoom, nil), time.Now().UTC()); got != 0 {
		t.Fatalf("deleted = %d, want 0 on index failure", got)
	}
}

func TestSweep_WriterErrorSkipsCluster(t *testing.T) {
	t.Parallel()

	st := bulkTestStore(t)
	enableLeases(t, st, 30)
	if got := vm.SweepExpiredLeases(context.Background(), customLeaseDeps(st, nil, errBoom), time.Now().UTC()); got != 0 {
		t.Fatalf("deleted = %d, want 0 on writer failure", got)
	}
}

//nolint:paralleltest // serial: closes the store
func TestSweep_PolicyReadErrorSkipsCluster(t *testing.T) {
	st := bulkTestStore(t)
	idx := buildResolveIndex(t)
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	deps := customLeaseDeps(st, nil, nil)
	deps.Index = func(string) (*inventory.Index, error) { return idx, nil }
	deps.Writer = func(string) (cluster.Writer, error) { return cluster.Fake{}, nil }

	if got := vm.SweepExpiredLeases(context.Background(), deps, time.Now().UTC()); got != 0 {
		t.Fatalf("deleted = %d, want 0 on policy read failure", got)
	}
}

func TestSweep_NoPolicyRowSkipsCluster(t *testing.T) {
	t.Parallel()

	st := bulkTestStore(t)
	idx := buildResolveIndex(t)
	deps := customLeaseDeps(st, nil, nil)
	deps.Index = func(string) (*inventory.Index, error) { return idx, nil }
	deps.Writer = func(string) (cluster.Writer, error) { return cluster.Fake{}, nil }

	if got := vm.SweepExpiredLeases(context.Background(), deps, time.Now().UTC()); got != 0 {
		t.Fatalf("deleted = %d, want 0 without a policy row", got)
	}
}

// configFailWriter can read VM status but fails the identity config read, so
// the sweeper defers rather than delete a VM it cannot identify.
type configFailWriter struct {
	cluster.Fake
	failIdentity bool
	noReader     bool
}

func (w configFailWriter) SnapshotConfig(ctx context.Context, node string, vmid int, name string) (map[string]string, error) {
	if w.failIdentity {
		return nil, errBoom
	}

	return w.Fake.SnapshotConfig(ctx, node, vmid, name)
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestSweep_IdentityReadFailureKeepsLease(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	enableLeases(t, st, 30)

	now := time.Now().UTC()
	setLease(t, st, 101, now.Add(-leaseDayDuration))

	writer := configFailWriter{Fake: *fake, failIdentity: true}
	if got := vm.SweepExpiredLeases(context.Background(), leaseSweepDeps(st, idx, writer, nil), now); got != 0 {
		t.Fatalf("deleted = %d, want 0 on identity read failure", got)
	}
	if !leaseExists(t, st, 101) {
		t.Fatal("a failed identity read must keep the lease row")
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestSweep_IdentityMismatchDropsLease(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	enableLeases(t, st, 30)

	now := time.Now().UTC()
	machine, ok := idx.Lookup(testClusterName, 101)
	if !ok {
		t.Fatal("Lookup: VM 101 missing from index")
	}
	stale := store.VMLease{Cluster: testClusterName, VMID: 101, ExpiresAt: now.Add(-leaseDayDuration), Pool: machine.Pool, Identity: "999-recycled"}
	if err := st.SetVMLease(context.Background(), stale); err != nil {
		t.Fatalf("SetVMLease: %v", err)
	}

	if got := vm.SweepExpiredLeases(context.Background(), leaseSweepDeps(st, idx, fake, nil), now); got != 0 {
		t.Fatalf("deleted = %d, want 0 on identity mismatch", got)
	}
	if leaseExists(t, st, 101) {
		t.Fatal("an identity mismatch must drop the lease row without deleting the VM")
	}
	if !vmPresent(t, fake, 101) {
		t.Fatal("the VM must survive an identity mismatch")
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestSweep_EmptyIdentityKeepsLease(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	enableLeases(t, st, 30)

	now := time.Now().UTC()
	machine, ok := idx.Lookup(testClusterName, 101)
	if !ok {
		t.Fatal("Lookup: VM 101 missing from index")
	}
	stale := store.VMLease{Cluster: testClusterName, VMID: 101, ExpiresAt: now.Add(-leaseDayDuration), Pool: machine.Pool}
	if err := st.SetVMLease(context.Background(), stale); err != nil {
		t.Fatalf("SetVMLease: %v", err)
	}

	if got := vm.SweepExpiredLeases(context.Background(), leaseSweepDeps(st, idx, fake, nil), now); got != 0 {
		t.Fatalf("deleted = %d, want 0 on empty identity", got)
	}
	if !leaseExists(t, st, 101) {
		t.Fatal("an unknown identity must keep the lease row")
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestSweep_DeleteFailureKeepsLease(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	enableLeases(t, st, 30)

	now := time.Now().UTC()
	setLease(t, st, 101, now.Add(-leaseDayDuration))

	writer := failingWriter{deleteErr: errors.New("delete refused")}
	if got := vm.SweepExpiredLeases(context.Background(), leaseSweepDeps(st, idx, writer, nil), now); got != 0 {
		t.Fatalf("deleted = %d, want 0 on delete failure", got)
	}
	if !leaseExists(t, st, 101) {
		t.Fatal("a failed delete must keep the lease row for the next sweep")
	}
	if !vmPresent(t, fake, 101) {
		t.Fatal("a failed delete must leave the VM")
	}
}
