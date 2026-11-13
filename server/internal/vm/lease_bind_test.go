package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
	"testing"
	"time"
)

type configReaderStub struct {
	cfg map[string]string
	err error
}

func (stub configReaderStub) SnapshotConfig(_ context.Context, _ string, _ int, _ string) (map[string]string, error) {
	return stub.cfg, stub.err
}

//nolint:paralleltest // serial: shared fake fixtures
func TestBindLeaseIdentity_NoLeaseNoOp(t *testing.T) {
	t.Cleanup(cluster.ResetFake)
	st := bulkTestStore(t)

	if err := vm.BindLeaseIdentity(context.Background(), st, cluster.Fake{}, testClusterName, cluster.FakeNode01, 101); err != nil {
		t.Fatalf("BindLeaseIdentity: %v", err)
	}
}

//nolint:paralleltest // serial: shared fake fixtures
func TestBindLeaseIdentity_ReadError(t *testing.T) {
	t.Cleanup(cluster.ResetFake)
	st := bulkTestStore(t)
	setLease(t, st, 101, time.Now().Add(leaseDayDuration))

	reader := configReaderStub{err: errors.New("proxmox down")}
	if err := vm.BindLeaseIdentity(context.Background(), st, reader, testClusterName, cluster.FakeNode01, 101); err == nil {
		t.Fatal("expected read error, got nil")
	}
}

//nolint:paralleltest // serial: shared fake fixtures
func TestBindLeaseIdentity_NoIdentity(t *testing.T) {
	t.Cleanup(cluster.ResetFake)
	st := bulkTestStore(t)
	setLease(t, st, 101, time.Now().Add(leaseDayDuration))

	reader := configReaderStub{cfg: map[string]string{"name": "web-02"}}
	if err := vm.BindLeaseIdentity(context.Background(), st, reader, testClusterName, cluster.FakeNode01, 101); !errors.Is(err, vm.ErrNoVMIdentity) {
		t.Fatalf("err = %v, want ErrNoVMIdentity", err)
	}
}

//nolint:paralleltest // serial: shared fake fixtures
func TestBindLeaseIdentity_Success(t *testing.T) {
	t.Cleanup(cluster.ResetFake)
	st := bulkTestStore(t)
	setLease(t, st, 101, time.Now().Add(leaseDayDuration))

	if err := vm.BindLeaseIdentity(context.Background(), st, cluster.Fake{}, testClusterName, cluster.FakeNode01, 101); err != nil {
		t.Fatalf("BindLeaseIdentity: %v", err)
	}

	lease, err := st.GetVMLease(context.Background(), testClusterName, 101)
	if err != nil {
		t.Fatalf("GetVMLease: %v", err)
	}

	if lease.Identity != cluster.FakeCreationTime {
		t.Fatalf("identity = %q, want %q", lease.Identity, cluster.FakeCreationTime)
	}
}
