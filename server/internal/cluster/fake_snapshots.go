package cluster

import (
	"context"
	"fmt"
	"slices"
	"time"
)

type fakeSnapshotKey struct {
	node string
	vmid int
}

var fakeNextSnapshotTaskID uint64

// ListSnapshots returns a defensive copy of the VM's live fake snapshots.
func (Fake) ListSnapshots(_ context.Context, node string, vmid int) ([]VMSnapshot, error) {
	fakeVMMutex.RLock()
	defer fakeVMMutex.RUnlock()

	if findFakeVM(node, vmid) < 0 {
		return nil, ErrNotFound
	}

	return cloneVMSnapshots(fakeSnapshots[fakeSnapshotKey{node: node, vmid: vmid}]), nil
}

// CreateSnapshot dispatches a fake asynchronous snapshot task.
func (Fake) CreateSnapshot(_ context.Context, node string, vmid int, name, description string, vmstate bool) (string, error) {
	if err := ensureFakeVM(node, vmid); err != nil {
		return "", err
	}

	created := VMSnapshot{Name: name, Description: description, CreatedAt: time.Now().UTC(), VMState: vmstate}
	upid := newSnapshotTask(node, vmid, "qmsnapshot", func() { appendFakeSnapshot(node, vmid, created) })
	recordCall(FakeCall{Node: node, VMID: vmid, Action: "create_snapshot", Name: name})

	return upid, nil
}

// RollbackSnapshot dispatches a fake asynchronous rollback task.
func (Fake) RollbackSnapshot(_ context.Context, node string, vmid int, name string) (string, error) {
	if err := ensureFakeSnapshot(node, vmid, name); err != nil {
		return "", err
	}

	upid := newSnapshotTask(node, vmid, "qmrollback", func() { applyFakeRollback(node, vmid, name) })
	recordCall(FakeCall{Node: node, VMID: vmid, Action: "rollback_snapshot", Name: name})

	return upid, nil
}

// DeleteSnapshot dispatches a fake asynchronous delete task.
func (Fake) DeleteSnapshot(_ context.Context, node string, vmid int, name string) (string, error) {
	if err := ensureFakeSnapshot(node, vmid, name); err != nil {
		return "", err
	}

	upid := newSnapshotTask(node, vmid, "qmdelsnapshot", func() { removeFakeSnapshot(node, vmid, name) })
	recordCall(FakeCall{Node: node, VMID: vmid, Action: "delete_snapshot", Name: name})

	return upid, nil
}

func ensureFakeVM(node string, vmid int) error {
	fakeVMMutex.RLock()
	defer fakeVMMutex.RUnlock()

	if findFakeVM(node, vmid) < 0 {
		return ErrNotFound
	}

	return nil
}

func ensureFakeSnapshot(node string, vmid int, name string) error {
	fakeVMMutex.RLock()
	defer fakeVMMutex.RUnlock()

	if findFakeVM(node, vmid) < 0 {
		return ErrNotFound
	}

	if slices.ContainsFunc(fakeSnapshots[fakeSnapshotKey{node: node, vmid: vmid}], func(snapshot VMSnapshot) bool { return snapshot.Name == name }) {
		return nil
	}

	return ErrNotFound
}

func newSnapshotTask(node string, vmid int, action string, onComplete func()) string {
	fakeCreateMutex.Lock()
	fakeNextSnapshotTaskID++
	sequence := fakeNextSnapshotTaskID
	upid := fmt.Sprintf("UPID:%s:%08X:%08X:%08X:%s:%d:pvmss@pve:", node, sequence, sequence, sequence, action, vmid)

	if fakeTasks == nil {
		fakeTasks = make(map[string]*fakeTask)
	}

	fakeTasks[upid] = &fakeTask{upid: upid, onComplete: onComplete, log: []string{"starting snapshot task..."}}
	fakeCreateMutex.Unlock()

	return upid
}

func appendFakeSnapshot(node string, vmid int, snapshot VMSnapshot) {
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	key := fakeSnapshotKey{node: node, vmid: vmid}
	fakeSnapshots[key] = append(fakeSnapshots[key], snapshot)
}

func applyFakeRollback(node string, vmid int, name string) {
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	index := findFakeVM(node, vmid)
	if index < 0 {
		return
	}

	for _, snapshot := range fakeSnapshots[fakeSnapshotKey{node: node, vmid: vmid}] {
		if snapshot.Name != name {
			continue
		}

		if snapshot.VMState {
			fakeVMs[index].Status = VMRunning
			fakeVMs[index].Uptime = fakeUptimeOnStart

			return
		}

		fakeVMs[index].Status = VMStopped
		fakeVMs[index].Uptime = 0

		return
	}
}

func removeFakeSnapshot(node string, vmid int, name string) {
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	key := fakeSnapshotKey{node: node, vmid: vmid}
	snapshots := fakeSnapshots[key]

	index := slices.IndexFunc(snapshots, func(snapshot VMSnapshot) bool { return snapshot.Name == name })
	if index >= 0 {
		fakeSnapshots[key] = slices.Delete(snapshots, index, index+1)
	}
}

func cloneVMSnapshots(snapshots []VMSnapshot) []VMSnapshot {
	return append([]VMSnapshot(nil), snapshots...)
}
