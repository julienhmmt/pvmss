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
func TestFake_NextVMID_DistinctMonotonicConcurrent(t *testing.T) {
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

	assertNextVMIDsDistinctAndAboveExisting(t, ids)
	assertNextVMIDMonotonicAfterBurst(t, client, ids)
}

// assertNextVMIDsDistinctAndAboveExisting checks that every allocated ID is
// distinct and strictly above the highest existing VMID in the default
// dataset. Extracted from TestFake_NextVMID_DistinctMonotonicConcurrent to
// keep its Cognitive Complexity under the SonarQube go:S3776 threshold.
func assertNextVMIDsDistinctAndAboveExisting(t *testing.T, ids []int) {
	t.Helper()

	seen := make(map[int]bool, len(ids))

	maxExisting := 0
	for _, vm := range defaultState().vms {
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
}

// assertNextVMIDMonotonicAfterBurst checks that a sequential NextVMID call
// after the concurrent burst returns a value strictly greater than every
// previously allocated ID. Extracted from
// TestFake_NextVMID_DistinctMonotonicConcurrent to keep its Cognitive
// Complexity under the SonarQube go:S3776 threshold.
func assertNextVMIDMonotonicAfterBurst(t *testing.T, client Fake, ids []int) {
	t.Helper()

	previous := 0
	for _, id := range ids {
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
		Tags:             []string{"team-web", FakeTagPvmss},
		CPUCores:         2,
		MemoryMB:         4096,
		Disk:             DiskSpec{Storage: FakeStorageLocalLVM, SizeGB: 40},
		Network:          NetworkSpec{{Bridge: FakeBridgeVMbr0, Model: string(DiskBusVirtio)}},
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

	assertCreatedVM(t, snap, spec, vmid)
}

// assertCreatedVM checks that the VM created from spec is present in snap with
// the expected identity fields, pvmss tag, running status, and a recorded
// create call. Extracted from TestFake_CreateVM_RecordsVMInDataset to keep
// its Cognitive Complexity under the SonarQube go:S3776 threshold.
func assertCreatedVM(t *testing.T, snap Snapshot, spec VMSpec, vmid int) {
	t.Helper()

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
		Network:  NetworkSpec{{Bridge: FakeBridgeVMbr0, Model: string(DiskBusVirtio)}},
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
		Disk:     DiskSpec{Storage: FakeStorageLocalLVM, SizeGB: 20},
		Network:  NetworkSpec{{Bridge: FakeBridgeVMbr0, Model: string(DiskBusVirtio)}},
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

// TestFake_CloneVM_RegistersTaskAndCompletes verifies the fake clone
// registers a task, reports running then ok on poll, and materializes the
// cloned VM with the correct VMID, node, pool, and disk bus.
//
//nolint:paralleltest // serial: shared mutable fake dataset
func TestFake_CloneVM_RegistersTaskAndCompletes(t *testing.T) {
	defer ResetFake()

	client := Fake{}
	ctx := context.Background()

	upid, err := client.CloneVM(ctx, CloneSpec{
		SourceVMID: 9000,
		SourceNode: FakeNode02,
		NewVMID:    10000,
		Name:       "cloned-vm",
		Full:       true,
		Storage:    FakeStorageLocalLVM,
		Pool:       FakePoolAlice,
		DiskBus:    string(DiskBusSCSI),
	})
	if err != nil {
		t.Fatalf("CloneVM: %v", err)
	}

	if upid == "" {
		t.Fatal("expected non-empty UPID")
	}

	assertTaskRunningTwice(t, ctx, client, upid)

	status := assertTaskOK(t, ctx, client, upid)
	_ = status

	snap, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v VM) bool { return v.VMID == 10000 })
	if idx < 0 {
		t.Fatalf("cloned VM 10000 not in snapshot")
	}

	cloned := snap.VMs[idx]

	if cloned.Name != "cloned-vm" || cloned.Node != FakeNode02 || cloned.Pool != FakePoolAlice {
		t.Errorf("cloned VM = %+v", cloned)
	}

	if cloned.Status != VMStopped {
		t.Errorf("cloned VM status = %q, want %q", cloned.Status, VMStopped)
	}

	if len(cloned.Disks) != 1 || cloned.Disks[0].Key != "scsi0" {
		t.Errorf("cloned VM disk = %+v, want key scsi0", cloned.Disks)
	}
}

//nolint:revive // context-as-argument: test helper signature matches caller convention
func assertTaskRunningTwice(t *testing.T, ctx context.Context, client Fake, upid string) {
	t.Helper()

	for i := range 2 {
		status, err := client.TaskStatus(ctx, upid)
		if err != nil {
			t.Fatalf("TaskStatus call %d: %v", i+1, err)
		}

		if status.State != TaskRunning {
			t.Fatalf("call %d: state = %q, want %q", i+1, status.State, TaskRunning)
		}
	}
}

//nolint:revive // context-as-argument: test helper signature matches caller convention
func assertTaskOK(t *testing.T, ctx context.Context, client Fake, upid string) TaskStatus {
	t.Helper()

	status, err := client.TaskStatus(ctx, upid)
	if err != nil {
		t.Fatalf("TaskStatus call 3: %v", err)
	}

	if status.State != TaskOK {
		t.Fatalf("call 3: state = %q, want %q", status.State, TaskOK)
	}

	return status
}

// TestFake_CloneVM_VirtioBus verifies the cloned VM's disk uses the bus
// from the CloneSpec, not a hardcoded scsi.
//
//nolint:paralleltest // serial: shared mutable fake dataset
func TestFake_CloneVM_VirtioBus(t *testing.T) {
	defer ResetFake()

	client := Fake{}
	ctx := context.Background()

	upid, err := client.CloneVM(ctx, CloneSpec{
		SourceVMID: 9001,
		SourceNode: FakeNode02,
		NewVMID:    10001,
		Name:       "virtio-clone",
		Full:       false,
		Pool:       "pool-bob",
		DiskBus:    string(DiskBusVirtio),
	})
	if err != nil {
		t.Fatalf("CloneVM: %v", err)
	}

	// Fast-forward to completion.
	for range 3 {
		if _, err := client.TaskStatus(ctx, upid); err != nil {
			t.Fatalf("TaskStatus: %v", err)
		}
	}

	snap, err := client.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	idx := slices.IndexFunc(snap.VMs, func(v VM) bool { return v.VMID == 10001 })
	if idx < 0 {
		t.Fatalf("cloned VM 10001 not in snapshot")
	}

	cloned := snap.VMs[idx]

	if len(cloned.Disks) != 1 || cloned.Disks[0].Key != "virtio0" {
		t.Errorf("cloned VM disk = %+v, want key virtio0", cloned.Disks)
	}

	if cloned.Disks[0].Bus != DiskBusVirtio {
		t.Errorf("cloned VM disk bus = %q, want %q", cloned.Disks[0].Bus, DiskBusVirtio)
	}

	if cloned.Pool != "pool-bob" {
		t.Errorf("cloned VM pool = %q, want pool-bob", cloned.Pool)
	}
}

// TestFake_CloneVM_RecordsCall verifies the fake records a clone FakeCall
// with the correct pool, storage, and full flag.
//
//nolint:paralleltest // serial: shared mutable fake dataset
func TestFake_CloneVM_RecordsCall(t *testing.T) {
	defer ResetFake()

	client := Fake{}
	ctx := context.Background()

	_, err := client.CloneVM(ctx, CloneSpec{
		SourceVMID: 9000,
		SourceNode: FakeNode02,
		NewVMID:    10002,
		Name:       "call-test",
		Full:       true,
		Storage:    FakeStorageLocal,
		Pool:       "pool-carol",
		DiskBus:    string(DiskBusSCSI),
	})
	if err != nil {
		t.Fatalf("CloneVM: %v", err)
	}

	calls := FakeCallsFor(10002)

	cloneCall := findCloneFakeCall(calls)
	if cloneCall == nil {
		t.Fatal("no clone call recorded for VMID 10002")
	}

	if cloneCall.Pool != "pool-carol" {
		t.Errorf("clone call pool = %q, want pool-carol", cloneCall.Pool)
	}

	if !cloneCall.Full {
		t.Error("clone call full = false, want true")
	}

	if cloneCall.Storage != FakeStorageLocal {
		t.Errorf("clone call storage = %q, want %q", cloneCall.Storage, FakeStorageLocal)
	}
}

// findCloneFakeCall returns the first clone FakeCall in calls, or nil.
func findCloneFakeCall(calls []FakeCall) *FakeCall {
	for i := range calls {
		if calls[i].Action == "clone" {
			return &calls[i]
		}
	}

	return nil
}
