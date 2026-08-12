package cluster

import (
	"context"
	"slices"
	"sync"
	"testing"
)

// T006 fake-client contract tests (T06 data-model.md): NextVMID allocates,
// CreateVM mutates the dataset, TaskStatus is poll-count-based. Run with
// -race: NextVMID concurrency is the point of the first test.

//nolint:paralleltest // serial: shared mutable fake dataset
func TestFake_NextVMID_DistinctMonotonicConcurrent(t *testing.T) { //nolint:gocyclo // concurrency test checks multiple synchronization invariants
	defer ResetFake()

	client := Fake{}

	const (
		callers   = 8
		perCaller = 8
	)

	results := make(chan int, callers*perCaller)

	var wg sync.WaitGroup
	for range callers {
		wg.Go(func() {
			for range perCaller {
				vmid, err := client.NextVMID(context.Background())
				if err != nil {
					t.Errorf("NextVMID: %v", err)
					return
				}

				results <- vmid
			}
		})
	}

	wg.Wait()
	close(results)

	var ids []int
	for id := range results {
		ids = append(ids, id)
	}

	if len(ids) != callers*perCaller {
		t.Fatalf("got %d ids, want %d", len(ids), callers*perCaller)
	}

	seen := make(map[int]bool, len(ids))

	maxExisting := 0
	for _, vm := range fakeVMs {
		if vm.VMID > maxExisting {
			maxExisting = vm.VMID
		}
	}

	for _, id := range ids {
		if seen[id] {
			t.Fatalf("duplicate VMID allocated: %d", id)
		}

		seen[id] = true
		if id <= maxExisting {
			t.Fatalf("allocated VMID %d collides with existing dataset (max %d)", id, maxExisting)
		}
	}

	// Sequential calls after the concurrent burst must keep increasing.
	previous := 0
	for id := range seen {
		if id > previous {
			previous = id
		}
	}

	next, err := client.NextVMID(context.Background())
	if err != nil {
		t.Fatalf("NextVMID: %v", err)
	}

	if next <= previous {
		t.Fatalf("NextVMID not monotonic: got %d after %d", next, previous)
	}
}

//nolint:paralleltest // serial: shared mutable fake dataset
func TestFake_CreateVM_RecordsVMInDataset(t *testing.T) {
	defer ResetFake()

	client := Fake{}
	ctx := context.Background()

	vmid, err := client.NextVMID(ctx)
	if err != nil {
		t.Fatalf("NextVMID: %v", err)
	}

	spec := VMSpec{
		VMID:             vmid,
		Node:             "pve-node-01",
		Name:             "web-test",
		Pool:             "pool-alice",
		Tags:             []string{"team-web", "pvmss"},
		CPUCores:         2,
		MemoryMB:         4096,
		Disk:             DiskSpec{Storage: FakeStorageLocalLVM, SizeGB: 40},
		Network:          NetworkSpec{Bridge: FakeBridgeVMbr0, Model: string(DiskBusVirtio)},
		StartAfterCreate: true,
	}

	upid, err := client.CreateVM(ctx, spec)
	if err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	if upid == "" {
		t.Fatalf("CreateVM returned empty upid")
	}

	snap, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v VM) bool { return v.VMID == vmid })
	if idx < 0 {
		t.Fatalf("created VM %d not in snapshot", vmid)
	}

	created := snap.VMs[idx]
	if created.Name != spec.Name || created.Pool != spec.Pool || created.Node != spec.Node {
		t.Errorf("created VM mismatch: %+v", created)
	}

	if !slices.Contains(created.Tags, "pvmss") {
		t.Errorf("created VM missing pvmss tag: %v", created.Tags)
	}
	// StartAfterCreate folds the start into the creation task (FR-022): the
	// VM is running once the fake has recorded it.
	if created.Status != VMRunning {
		t.Errorf("status = %q, want %q (startAfterCreate)", created.Status, VMRunning)
	}

	if !slices.ContainsFunc(FakeCallsFor(vmid), func(c FakeCall) bool { return c.Action == "create" }) {
		t.Errorf("no create call recorded for VMID %d", vmid)
	}
}

//nolint:paralleltest // serial: shared mutable fake dataset
func TestFake_CreateVM_NoStartLeavesVMStopped(t *testing.T) {
	defer ResetFake()

	client := Fake{}
	ctx := context.Background()

	vmid, err := client.NextVMID(ctx)
	if err != nil {
		t.Fatalf("NextVMID: %v", err)
	}

	spec := VMSpec{
		VMID:     vmid,
		Node:     "pve-node-02",
		Name:     "stopped-vm",
		Pool:     "pool-bob",
		Tags:     []string{"pvmss"},
		CPUCores: 1,
		MemoryMB: 2048,
		Disk:     DiskSpec{Storage: FakeStorageLocal, SizeGB: 20},
		Network:  NetworkSpec{Bridge: "vmbr0", Model: "virtio"},
	}
	if _, err := client.CreateVM(ctx, spec); err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	snap, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v VM) bool { return v.VMID == vmid })
	if idx < 0 {
		t.Fatalf("created VM %d not in snapshot", vmid)
	}

	if snap.VMs[idx].Status != VMStopped {
		t.Errorf("status = %q, want %q (no startAfterCreate)", snap.VMs[idx].Status, VMStopped)
	}
}

// TestFake_TaskStatus_PollCount — SC-006: the first two queries for a upid
// return running, the third and later return ok. No wall-clock dependency.
//
//nolint:paralleltest // serial: shared mutable fake dataset
func TestFake_TaskStatus_PollCount(t *testing.T) {
	defer ResetFake()

	client := Fake{}
	ctx := context.Background()

	vmid, err := client.NextVMID(ctx)
	if err != nil {
		t.Fatalf("NextVMID: %v", err)
	}

	upid, err := client.CreateVM(ctx, VMSpec{
		VMID:     vmid,
		Node:     "pve-node-01",
		Name:     "task-vm",
		Pool:     "pool-alice",
		Tags:     []string{"pvmss"},
		CPUCores: 1,
		MemoryMB: 2048,
		Disk:     DiskSpec{Storage: "local-lvm", SizeGB: 20},
		Network:  NetworkSpec{Bridge: "vmbr0", Model: "virtio"},
	})
	if err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	for i, want := range []TaskState{TaskRunning, TaskRunning, TaskOK, TaskOK} {
		status, err := client.TaskStatus(ctx, upid)
		if err != nil {
			t.Fatalf("TaskStatus call %d: %v", i+1, err)
		}

		if status.UPID != upid {
			t.Errorf("call %d: UPID = %q, want %q", i+1, status.UPID, upid)
		}

		if status.State != want {
			t.Fatalf("call %d: state = %q, want %q", i+1, status.State, want)
		}
	}

	final, err := client.TaskStatus(ctx, upid)
	if err != nil {
		t.Fatalf("TaskStatus: %v", err)
	}

	if len(final.Log) == 0 || final.Log[len(final.Log)-1] != "TASK OK" {
		t.Errorf("terminal log = %v, want last line %q", final.Log, "TASK OK")
	}
}

//nolint:paralleltest // serial: shared mutable fake dataset
func TestFake_TaskStatus_UnknownUPID(t *testing.T) {
	defer ResetFake()

	_, err := Fake{}.TaskStatus(context.Background(), "UPID:pve-node-01:00000000:00000000:00000000:qmcreate:999:nobody@pve:")
	if err == nil {
		t.Fatalf("expected error for unknown upid, got nil")
	}
}
