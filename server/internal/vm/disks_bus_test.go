package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
	"testing"
)

//nolint:paralleltest // serial: shared fake cluster dataset
func TestAddDisk_BusFullRejectedBeforeWriter(t *testing.T) {
	cluster.ResetFake()

	index := diskTestIndex(t, 101, cluster.VMStopped)

	machine := index.ByVMID[101]
	for diskIndex := range 31 {
		machine.Disks = append(machine.Disks, cluster.Disk{
			Key:      "scsi" + string(rune('0'+diskIndex)),
			Bus:      cluster.DiskBusSCSI,
			BusIndex: diskIndex,
			SizeGB:   1,
		})
	}

	index.ByVMID[101] = machine
	deps := diskDependencies(index, aliceIdentity(), 101)

	_, err := vm.AddDisk(context.Background(), deps, cluster.DiskBusSCSI, "local-lvm", 10)
	if !errors.Is(err, vm.ErrBusFull) {
		t.Fatalf("err = %v, want ErrBusFull", err)
	}

	if calls := cluster.FakeCallsFor(101); len(calls) != 0 {
		t.Fatalf("fake calls = %+v, want none", calls)
	}
}
