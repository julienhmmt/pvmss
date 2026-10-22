package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
	"testing"
	"time"
)

//nolint:funlen,paralleltest // table covers the disk guard matrix; fake dataset is shared
func TestDiskOperations_GuardsAndWrites(t *testing.T) {
	tests := []struct {
		name       string
		operation  func(vm.DiskDependencies) error
		wantErr    error
		wantAction string
		vmid       int
		status     cluster.VMStatus
		calls      int
	}{
		{
			name: "add disk on stopped VM",
			operation: func(deps vm.DiskDependencies) error {
				_, err := vm.AddDisk(context.Background(), deps, cluster.DiskBusSCSI, "local-lvm", 10)
				return err
			},
			wantAction: "add_disk",
			vmid:       101,
			status:     cluster.VMStopped,
			calls:      1,
		},
		{
			name: "add disk on running VM",
			operation: func(deps vm.DiskDependencies) error {
				_, err := vm.AddDisk(context.Background(), deps, cluster.DiskBusSCSI, "local-lvm", 10)
				return err
			},
			wantErr: vm.ErrVMNotStopped,
			vmid:    101,
			status:  cluster.VMRunning,
		},
		{
			name: "reject unapproved storage",
			operation: func(deps vm.DiskDependencies) error {
				_, err := vm.AddDisk(context.Background(), deps, cluster.DiskBusSCSI, "not-approved", 10)
				return err
			},
			wantErr: vm.ErrDiskStorageNotApproved,
			vmid:    101,
			status:  cluster.VMStopped,
		},
		{
			name: "reject disk over gabarit",
			operation: func(deps vm.DiskDependencies) error {
				_, err := vm.AddDisk(context.Background(), deps, cluster.DiskBusSCSI, "local-lvm", 501)
				return err
			},
			wantErr: vm.ErrDiskSizeExceedsLimit,
			vmid:    101,
			status:  cluster.VMStopped,
		},
		{
			name: "resize disk while running",
			operation: func(deps vm.DiskDependencies) error {
				return vm.ResizeDisk(context.Background(), deps, "scsi1", 20)
			},
			wantAction: "resize_disk",
			vmid:       101,
			status:     cluster.VMRunning,
			calls:      1,
		},
		{
			name: "reject shrinking disk",
			operation: func(deps vm.DiskDependencies) error {
				return vm.ResizeDisk(context.Background(), deps, "scsi1", 5)
			},
			wantErr: vm.ErrDiskSizeNotGreater,
			vmid:    101,
			status:  cluster.VMRunning,
		},
		{
			name: "reject delete while running",
			operation: func(deps vm.DiskDependencies) error {
				return vm.DeleteDisk(context.Background(), deps, "scsi1")
			},
			wantErr: vm.ErrVMNotStopped,
			vmid:    101,
			status:  cluster.VMRunning,
		},
		{
			name: "reject resize over gabarit",
			operation: func(deps vm.DiskDependencies) error {
				return vm.ResizeDisk(context.Background(), deps, "scsi1", 501)
			},
			wantErr: vm.ErrDiskSizeExceedsLimit,
			vmid:    101,
			status:  cluster.VMRunning,
		},
		{
			name: "protect boot disk",
			operation: func(deps vm.DiskDependencies) error {
				return vm.DeleteDisk(context.Background(), deps, "scsi0")
			},
			wantErr: vm.ErrBootDiskProtected,
			vmid:    101,
			status:  cluster.VMStopped,
		},
		{
			name: "delete non-boot disk",
			operation: func(deps vm.DiskDependencies) error {
				return vm.DeleteDisk(context.Background(), deps, "scsi1")
			},
			wantAction: "delete_disk",
			vmid:       101,
			status:     cluster.VMStopped,
			calls:      1,
		},
		{
			name: testNonOwnerCase,
			operation: func(deps vm.DiskDependencies) error {
				return vm.DeleteDisk(context.Background(), deps, "scsi1")
			},
			wantErr: vm.ErrForbidden,
			vmid:    101,
			status:  cluster.VMStopped,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runDiskOperationCase(t, diskOperationCase{
				name:       test.name,
				operation:  test.operation,
				wantErr:    test.wantErr,
				wantAction: test.wantAction,
				vmid:       test.vmid,
				status:     test.status,
				wantCalls:  test.calls,
			})
		})
	}
}

// diskOperationCase groups the parameters for runDiskOperationCase so the
// helper stays under the SonarQube go:S107 parameter-count threshold (7).
type diskOperationCase struct {
	name       string
	operation  func(vm.DiskDependencies) error
	wantErr    error
	wantAction string
	vmid       int
	status     cluster.VMStatus
	wantCalls  int
}

// runDiskOperationCase runs a single disk operation case: builds the deps,
// invokes the operation, and asserts the expected error, call count, and
// action. Extracted from TestDiskOperations_GuardsAndWrites to keep its
// Cognitive Complexity under the SonarQube go:S3776 threshold.
func runDiskOperationCase(t *testing.T, tc diskOperationCase) {
	t.Helper()

	cluster.ResetFake()

	index := diskTestIndex(t, tc.vmid, tc.status)

	actor := auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}
	if tc.name == testNonOwnerCase {
		actor = auth.Identity{Username: cluster.FakeUserBob, Pool: cluster.FakePoolBob}
	}

	deps := diskDependencies(index, actor, tc.vmid)

	err := tc.operation(deps)
	if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
		t.Fatalf("err = %v, want %v", err, tc.wantErr)
	}

	if tc.wantErr == nil && err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	calls := cluster.FakeCallsFor(tc.vmid)
	if len(calls) != tc.wantCalls {
		t.Fatalf("fake calls = %d, want %d: %+v", len(calls), tc.wantCalls, calls)
	}

	if tc.wantAction != "" && calls[0].Action != tc.wantAction {
		t.Fatalf("action = %q, want %q", calls[0].Action, tc.wantAction)
	}
}

func diskDependencies(index *inventory.Index, actor auth.Identity, vmid int) vm.DiskDependencies {
	return vm.DiskDependencies{
		Index:       index,
		Actor:       actor,
		ClusterName: testClusterName,
		VMID:        vmid,
		Writer:      cluster.Fake{},
		Resources:   catalog.Resources{Storages: []catalog.Storage{{Name: testDiskStorage, Node: cluster.FakeNode01}}},
		Gabarit:     policy.DefaultGabarit(),
		Audit:       noopAudit{},
		Refresher:   noopRefresher{},
	}
}

func diskTestIndex(t *testing.T, vmid int, status cluster.VMStatus) *inventory.Index {
	t.Helper()

	snapshot, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for index := range snapshot.VMs {
		if snapshot.VMs[index].VMID == vmid {
			snapshot.VMs[index].Status = status
		}
	}

	built := inventory.BuildIndex(snapshot)

	return &built
}

type noopAudit struct{}

func (noopAudit) RecordAction(context.Context, string, string, int, string) error { return nil }

type noopRefresher struct{}

func (noopRefresher) Refresh(context.Context) (time.Time, error) { return time.Time{}, nil }

// testDiskStorage is the approved storage diskDependencies offers.
const testDiskStorage = "local-lvm"

// diskIndexWithDisks is diskTestIndex with VM vmid's disks replaced, so a
// case controls the sizes the per-VM total is computed from.
func diskIndexWithDisks(t *testing.T, vmid int, disks []cluster.Disk) *inventory.Index {
	t.Helper()

	snapshot, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for index := range snapshot.VMs {
		if snapshot.VMs[index].VMID == vmid {
			snapshot.VMs[index].Status = cluster.VMStopped
			snapshot.VMs[index].Disks = disks
		}
	}

	built := inventory.BuildIndex(snapshot)

	return &built
}

// The gabarit's MaxDiskPerVMGB caps the VM's total disk ("Maximum disk per
// VM"), not each disk: adding or growing disks must not push the sum past it.
//
//nolint:paralleltest // fake dataset is shared
func TestDiskOperations_LimitIsPerVMTotal(t *testing.T) {
	disks := []cluster.Disk{
		{Key: testBootDiskKey, Bus: cluster.DiskBusSCSI, BusIndex: 0, Storage: testDiskStorage, SizeGB: 300},
		{Key: "scsi1", Bus: cluster.DiskBusSCSI, BusIndex: 1, Storage: testDiskStorage, SizeGB: 150},
	}
	gabarit := policy.DefaultGabarit()
	gabarit.MaxDiskPerVMGB = 500

	tests := []struct {
		name      string
		operation func(vm.DiskDependencies) error
		wantErr   error
	}{
		{"add within total", func(deps vm.DiskDependencies) error {
			_, err := vm.AddDisk(context.Background(), deps, cluster.DiskBusSCSI, testDiskStorage, 50)
			return err
		}, nil},
		{"add past total", func(deps vm.DiskDependencies) error {
			_, err := vm.AddDisk(context.Background(), deps, cluster.DiskBusSCSI, testDiskStorage, 51)
			return err
		}, vm.ErrDiskSizeExceedsLimit},
		{"resize within total", func(deps vm.DiskDependencies) error {
			return vm.ResizeDisk(context.Background(), deps, "scsi1", 200)
		}, nil},
		{"resize past total", func(deps vm.DiskDependencies) error {
			return vm.ResizeDisk(context.Background(), deps, "scsi1", 201)
		}, vm.ErrDiskSizeExceedsLimit},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			cluster.ResetFake()

			deps := diskDependencies(diskIndexWithDisks(t, 101, disks), auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}, 101)
			deps.Gabarit = gabarit

			if err := test.operation(deps); !errors.Is(err, test.wantErr) {
				t.Fatalf("err = %v, want %v", err, test.wantErr)
			}
		})
	}
}
