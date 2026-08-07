package cluster

import (
	"context"
	"fmt"
	"sync"
)

// Fake implementation of Creator (T06). Task completion is poll-count-based,
// not wall-clock (plan.md research decisions): a task reports running for its
// first two TaskStatus queries and ok from the third, so go test -race stays
// instant and deterministic while the browser demo still shows a visible
// in-progress state (the frontend's poll interval paces observation).

// fakeTask tracks one in-flight creation's poll count.
type fakeTask struct {
	upid  string
	polls int
}

var (
	fakeCreateMutex sync.Mutex
	fakeNextVMID    int
	fakeTasks       map[string]*fakeTask
)

// fakeVMIDFloor returns one past the highest VMID currently in the dataset.
// Callers must hold fakeVMMutex (at least read-locked).
func fakeVMIDFloor() int {
	floor := 100
	for _, vm := range fakeVMs {
		if vm.VMID >= floor {
			floor = vm.VMID + 1
		}
	}
	return floor
}

// NextVMID implements Creator. The counter starts above the dataset's highest
// VMID and only ever increases, so concurrent creations get distinct ids even
// before their CreateVM calls land (edge case: two tabs submitting at once).
func (Fake) NextVMID(_ context.Context) (int, error) {
	fakeCreateMutex.Lock()
	defer fakeCreateMutex.Unlock()
	fakeVMMutex.RLock()
	floor := fakeVMIDFloor()
	fakeVMMutex.RUnlock()
	if fakeNextVMID < floor {
		fakeNextVMID = floor
	}
	vmid := fakeNextVMID
	fakeNextVMID++
	return vmid, nil
}

// CreateVM implements Creator: the VM enters the fake's mutable dataset
// immediately (running when StartAfterCreate is set — FR-022 folds the start
// into this same task) and a poll-counted task is registered under the
// returned UPID.
func (Fake) CreateVM(_ context.Context, spec VMSpec) (string, error) {
	fakeVMMutex.Lock()
	status := VMStopped
	if spec.StartAfterCreate {
		status = VMRunning
	}
	var diskTotal int64
	if spec.Disk.SizeGB > 0 {
		diskTotal = int64(spec.Disk.SizeGB) * 1024 * 1024 * 1024
	}
	fakeVMs = append(fakeVMs, VM{
		VMID:        spec.VMID,
		Name:        spec.Name,
		Node:        spec.Node,
		Status:      status,
		Pool:        spec.Pool,
		Tags:        append([]string(nil), spec.Tags...),
		CPUCores:    spec.CPUCores,
		MemoryTotal: int64(spec.MemoryMB) * 1024 * 1024,
		DiskTotal:   diskTotal,
	})
	fakeVMMutex.Unlock()

	upid := fmt.Sprintf("UPID:%s:%08X:%08X:%08X:qmcreate:%d:pvmss@pve:", spec.Node, spec.VMID, 0x10000000+spec.VMID, 0x20000000+spec.VMID, spec.VMID)

	fakeCreateMutex.Lock()
	if fakeTasks == nil {
		fakeTasks = make(map[string]*fakeTask)
	}
	fakeTasks[upid] = &fakeTask{upid: upid}
	fakeCreateMutex.Unlock()

	recordCall(FakeCall{Node: spec.Node, VMID: spec.VMID, Action: "create", Name: spec.Name})
	return upid, nil
}

// TaskStatus implements Creator. Poll-count-based: running for a UPID's first
// two queries, ok from the third onward (SC-006 — deterministic, no sleeps).
func (Fake) TaskStatus(_ context.Context, upid string) (TaskStatus, error) {
	fakeCreateMutex.Lock()
	defer fakeCreateMutex.Unlock()
	task, ok := fakeTasks[upid]
	if !ok {
		return TaskStatus{}, ErrNotFound
	}
	task.polls++
	if task.polls < 3 {
		return TaskStatus{UPID: upid, State: TaskRunning, Log: []string{"allocating disk...", "starting qmcreate..."}}, nil
	}
	return TaskStatus{UPID: upid, State: TaskOK, Log: []string{"allocating disk...", "starting qmcreate...", "TASK OK"}}, nil
}

// resetFakeCreateState clears Creator state. Called by ResetFake.
func resetFakeCreateState() {
	fakeCreateMutex.Lock()
	defer fakeCreateMutex.Unlock()
	fakeNextVMID = 0
	fakeTasks = nil
}
