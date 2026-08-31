package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/vm"
	"slices"
	"testing"
	"time"
)

const (
	testCdromKey    = "ide2"
	testBootDiskKey = "scsi0"
)

// bootTestIndex builds an Index from the default fake dataset with one VM's
// status and boot order overridden.
func bootTestIndex(t *testing.T, vmid int, status cluster.VMStatus, bootOrder []string) *inventory.Index {
	t.Helper()

	snapshot, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for index := range snapshot.VMs {
		if snapshot.VMs[index].VMID == vmid {
			snapshot.VMs[index].Status = status
			snapshot.VMs[index].BootOrder = bootOrder
		}
	}

	built := inventory.BuildIndex(snapshot)

	return &built
}

// bootWriter wraps cluster.Fake and records SetBootOrder calls and actions.
type bootWriter struct {
	cluster.Fake
	orders    [][]string
	actions   []string
	failStart bool
}

func (w *bootWriter) Action(ctx context.Context, node string, vmid int, action string) error {
	w.actions = append(w.actions, action)
	if w.failStart && action == actionStart {
		return errors.New("start rejected")
	}

	return w.Fake.Action(ctx, node, vmid, action)
}

func (w *bootWriter) SetBootOrder(ctx context.Context, node string, vmid int, order []string) error {
	w.orders = append(w.orders, append([]string(nil), order...))

	return w.Fake.SetBootOrder(ctx, node, vmid, order)
}

//nolint:paralleltest // serial: shared fake dataset + global var mutation
func TestBootFromCDROM_Success(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	originalPoll := vm.BootPollInterval
	vm.BootPollInterval = time.Millisecond

	t.Cleanup(func() { vm.BootPollInterval = originalPoll })

	// VM 101: stopped, alice's, CDROM mounted with an ISO, no recorded boot order.
	idx := bootTestIndex(t, 101, cluster.VMStopped, nil)
	writer := &bootWriter{Fake: cluster.Fake{}}

	err := vm.BootFromCDROM(context.Background(), vm.BootDependencies{
		Index: idx, Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 101,
		Writer: writer, Audit: noopAudit{}, Refresher: noopRefresher{}, StatusReader: cluster.Fake{},
	})
	if err != nil {
		t.Fatalf("BootFromCDROM: %v", err)
	}

	// CD-first order set, start sent, original order restored.
	if len(writer.orders) != 2 || !slices.Equal(writer.orders[0], []string{testCdromKey, testBootDiskKey}) || len(writer.orders[1]) != 0 {
		t.Fatalf("boot orders = %v, want [ide2 scsi0] then restore", writer.orders)
	}

	if len(writer.actions) != 1 || writer.actions[0] != actionStart {
		t.Errorf("actions = %v, want [start]", writer.actions)
	}

	snap, snapErr := (cluster.Fake{}).Snapshot(context.Background())
	if snapErr != nil {
		t.Fatalf("Snapshot: %v", snapErr)
	}

	for _, testVM := range snap.VMs {
		if testVM.VMID == 101 {
			if testVM.Status != cluster.VMRunning {
				t.Errorf("status = %q, want running", testVM.Status)
			}

			if len(testVM.BootOrder) != 0 {
				t.Errorf("BootOrder = %v, want restored to empty", testVM.BootOrder)
			}
		}
	}
}

//nolint:paralleltest // serial: shared fake dataset
func TestBootFromCDROM_Rejections(t *testing.T) {
	cases := []struct {
		name    string
		vmid    int
		actor   auth.Identity
		mountOn int
		wantErr error
	}{
		{name: "no iso mounted", vmid: 124, actor: aliceIdentity(), wantErr: vm.ErrCDROMNotMounted},
		{name: "running vm rejected", vmid: 100, actor: aliceIdentity(), mountOn: 100, wantErr: cluster.ErrVMRunning},
		{name: "non-owner rejected", vmid: 101, actor: bobIdentity(), wantErr: vm.ErrForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cluster.ResetFake()
			t.Cleanup(cluster.ResetFake)

			if tc.mountOn != 0 {
				if err := (cluster.Fake{}).SetCDROM(context.Background(), cluster.FakeNode01, tc.mountOn, cluster.CDROMState{State: cluster.CDROMMounted, ISOVolID: "local:iso/debian-12-generic-amd64.iso"}); err != nil {
					t.Fatalf("SetCDROM: %v", err)
				}
			}

			err := vm.BootFromCDROM(context.Background(), vm.BootDependencies{
				Index: buildResolveIndex(t), Actor: tc.actor, ClusterName: testClusterName, VMID: tc.vmid,
				Writer: cluster.Fake{}, Audit: noopAudit{}, Refresher: noopRefresher{}, StatusReader: cluster.Fake{},
			})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("err = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

//nolint:paralleltest // serial: shared fake dataset
func TestBootFromCDROM_StartFails_RestoresOrder(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	original := vm.BootPollInterval
	vm.BootPollInterval = time.Millisecond

	t.Cleanup(func() { vm.BootPollInterval = original })

	writer := &bootWriter{Fake: cluster.Fake{}, failStart: true}

	err := vm.BootFromCDROM(context.Background(), vm.BootDependencies{
		Index: buildResolveIndex(t), Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 101,
		Writer: writer, Audit: noopAudit{}, Refresher: noopRefresher{}, StatusReader: cluster.Fake{},
	})
	if err == nil {
		t.Fatal("expected start failure")
	}

	// The deferred restore must have put the original (empty) order back.
	if len(writer.orders) != 2 || len(writer.orders[1]) != 0 {
		t.Fatalf("boot orders = %v, want CD-first then restore", writer.orders)
	}
}

func TestBootOrderCDFirst(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		entity vm.Entity
		want   []string
	}{
		{name: "disk first cd second", entity: vm.Entity{BootOrder: []string{"scsi0", "ide2", "net0"}}, want: []string{testCdromKey, testBootDiskKey, "net0"}},
		{name: "no recorded order falls back to first disk", entity: vm.Entity{Disks: []cluster.Disk{{Key: "virtio0"}, {Key: "virtio1"}}}, want: []string{testCdromKey, "virtio0"}},
		{name: "no disks keeps cd only", entity: vm.Entity{}, want: []string{testCdromKey}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := vm.BootOrderCDFirst(tc.entity)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("BootOrderCDFirst = %v, want %v", got, tc.want)
			}
		})
	}
}
