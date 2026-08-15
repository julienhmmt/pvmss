package cluster

import (
	"context"
	"fmt"
)

// Fake implementation of Creator (T06). Task completion is poll-count-based,
// not wall-clock (plan.md research decisions): a task reports running for its
// first two TaskStatus queries and ok from the third, so go test -race stays
// instant and deterministic while the browser demo still shows a visible
// in-progress state (the frontend's poll interval paces observation).

// fakeTask tracks one in-flight creation's poll count.
type fakeTask struct {
	upid       string
	polls      int
	onComplete func()
	log        []string
}

// NextVMID implements Creator. The counter starts above the dataset's highest
// VMID and only ever increases, so concurrent creations get distinct ids even
// before their CreateVM calls land (edge case: two tabs submitting at once).
func (fake Fake) NextVMID(_ context.Context) (int, error) {
	state := fake.stateOrDefault()
	state.createMu.Lock()
	defer state.createMu.Unlock()

	state.vmMu.RLock()

	floor := state.vmidFloor()

	state.vmMu.RUnlock()

	if state.nextVMID < floor {
		state.nextVMID = floor
	}

	vmid := state.nextVMID
	state.nextVMID++

	return vmid, nil
}

// CreateVM implements Creator: the VM enters the fake's mutable dataset
// immediately (running when StartAfterCreate is set — FR-022 folds the start
// into this same task) and a poll-counted task is registered under the
// returned UPID.
func (fake Fake) CreateVM(_ context.Context, spec VMSpec) (string, error) {
	state := fake.stateOrDefault()
	state.vmMu.Lock()

	status := VMStopped
	if spec.StartAfterCreate {
		status = VMRunning
	}

	var diskTotal int64
	if spec.Disk.SizeGB > 0 {
		diskTotal = int64(spec.Disk.SizeGB) * 1024 * 1024 * 1024
	}

	state.vms = append(state.vms, VM{
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
	state.vmMu.Unlock()

	upid := fmt.Sprintf("UPID:%s:%08X:%08X:%08X:qmcreate:%d:pvmss@pve:", spec.Node, spec.VMID, 0x10000000+spec.VMID, 0x20000000+spec.VMID, spec.VMID)

	state.createMu.Lock()
	if state.tasks == nil {
		state.tasks = make(map[string]*fakeTask)
	}

	state.tasks[upid] = &fakeTask{upid: upid, log: []string{"allocating disk...", "starting qmcreate..."}}
	state.createMu.Unlock()

	state.record(FakeCall{Node: spec.Node, VMID: spec.VMID, Action: "create", Name: spec.Name})

	return upid, nil
}

// TaskStatus implements Creator. Poll-count-based: running for a UPID's first
// two queries, ok from the third onward (SC-006 — deterministic, no sleeps).
func (fake Fake) TaskStatus(_ context.Context, upid string) (TaskStatus, error) {
	state := fake.stateOrDefault()
	state.createMu.Lock()

	task, ok := state.tasks[upid]
	if !ok {
		state.createMu.Unlock()
		return TaskStatus{}, ErrNotFound
	}

	task.polls++

	log := append([]string(nil), task.log...)
	if task.polls < 3 {
		state.createMu.Unlock()
		return TaskStatus{UPID: upid, State: TaskRunning, Log: log}, nil
	}

	onComplete := task.onComplete
	task.onComplete = nil
	state.createMu.Unlock()

	if onComplete != nil {
		onComplete()
	}

	log = append(log, "TASK OK")

	return TaskStatus{UPID: upid, State: TaskOK, Log: log}, nil
}
