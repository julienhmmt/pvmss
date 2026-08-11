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
			cluster.ResetFake()

			index := diskTestIndex(t, test.vmid, test.status)

			actor := auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}
			if test.name == testNonOwnerCase {
				actor = auth.Identity{Username: cluster.FakeUserBob, Pool: cluster.FakePoolBob}
			}

			deps := diskDependencies(index, actor, test.vmid)

			err := test.operation(deps)
			if test.wantErr != nil && !errors.Is(err, test.wantErr) {
				t.Fatalf("err = %v, want %v", err, test.wantErr)
			}

			if test.wantErr == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			calls := cluster.FakeCallsFor(test.vmid)
			if len(calls) != test.calls {
				t.Fatalf("fake calls = %d, want %d: %+v", len(calls), test.calls, calls)
			}

			if test.wantAction != "" && calls[0].Action != test.wantAction {
				t.Fatalf("action = %q, want %q", calls[0].Action, test.wantAction)
			}
		})
	}
}

func diskDependencies(index *inventory.Index, actor auth.Identity, vmid int) vm.DiskDependencies {
	return vm.DiskDependencies{
		Index:       index,
		Actor:       actor,
		ClusterName: testClusterName,
		VMID:        vmid,
		Writer:      cluster.Fake{},
		Resources:   catalog.Resources{Storages: []catalog.Storage{{Name: "local-lvm", Node: cluster.FakeNode01}}},
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
