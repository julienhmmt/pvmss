//nolint:wsl_v5 // fake snapshot methods keep mutation and locking adjacent
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

// ListSnapshots returns a defensive copy of the VM's live fake snapshots.
func (fake Fake) ListSnapshots(_ context.Context, node string, vmid int) ([]VMSnapshot, error) {
	state := fake.stateOrDefault()
	state.vmMu.RLock()
	defer state.vmMu.RUnlock()

	if state.findVM(node, vmid) < 0 {
		return nil, ErrNotFound
	}

	return cloneVMSnapshots(state.snapshots[fakeSnapshotKey{node: node, vmid: vmid}]), nil
}

// CreateSnapshot dispatches a fake asynchronous snapshot task.
func (fake Fake) CreateSnapshot(_ context.Context, node string, vmid int, name, description string, vmstate bool) (string, error) {
	state := fake.stateOrDefault()
	if err := state.ensureVM(node, vmid); err != nil {
		return "", err
	}

	created := VMSnapshot{Name: name, Description: description, CreatedAt: time.Now().UTC(), VMState: vmstate}
	upid := state.newSnapshotTask(node, vmid, "qmsnapshot", func() { state.appendSnapshot(node, vmid, created) })
	state.record(FakeCall{Node: node, VMID: vmid, Action: "create_snapshot", Name: name})

	return upid, nil
}

// RollbackSnapshot dispatches a fake asynchronous rollback task.
func (fake Fake) RollbackSnapshot(_ context.Context, node string, vmid int, name string) (string, error) {
	state := fake.stateOrDefault()
	if err := state.ensureSnapshot(node, vmid, name); err != nil {
		return "", err
	}

	upid := state.newSnapshotTask(node, vmid, "qmrollback", func() { state.applyRollback(node, vmid, name) })
	state.record(FakeCall{Node: node, VMID: vmid, Action: "rollback_snapshot", Name: name})

	return upid, nil
}

// DeleteSnapshot dispatches a fake asynchronous delete task.
func (fake Fake) DeleteSnapshot(_ context.Context, node string, vmid int, name string) (string, error) {
	state := fake.stateOrDefault()
	if err := state.ensureSnapshot(node, vmid, name); err != nil {
		return "", err
	}

	upid := state.newSnapshotTask(node, vmid, "qmdelsnapshot", func() { state.removeSnapshot(node, vmid, name) })
	state.record(FakeCall{Node: node, VMID: vmid, Action: "delete_snapshot", Name: name})

	return upid, nil
}

func (s *fakeState) ensureVM(node string, vmid int) error {
	s.vmMu.RLock()
	defer s.vmMu.RUnlock()

	if s.findVM(node, vmid) < 0 {
		return ErrNotFound
	}

	return nil
}

func (s *fakeState) ensureSnapshot(node string, vmid int, name string) error {
	s.vmMu.RLock()
	defer s.vmMu.RUnlock()

	if s.findVM(node, vmid) < 0 {
		return ErrNotFound
	}

	if slices.ContainsFunc(s.snapshots[fakeSnapshotKey{node: node, vmid: vmid}], func(snapshot VMSnapshot) bool { return snapshot.Name == name }) {
		return nil
	}

	return ErrNotFound
}

func (s *fakeState) newSnapshotTask(node string, vmid int, action string, onComplete func()) string {
	s.createMu.Lock()
	s.nextSnapshotTaskID++
	sequence := s.nextSnapshotTaskID
	upid := fmt.Sprintf("UPID:%s:%08X:%08X:%08X:%s:%d:pvmss@pve:", node, sequence, sequence, sequence, action, vmid)

	if s.tasks == nil {
		s.tasks = make(map[string]*fakeTask)
	}

	s.tasks[upid] = &fakeTask{upid: upid, onComplete: onComplete, log: []string{"starting snapshot task..."}}
	s.createMu.Unlock()

	return upid
}

func (s *fakeState) appendSnapshot(node string, vmid int, snapshot VMSnapshot) {
	s.vmMu.Lock()
	defer s.vmMu.Unlock()

	key := fakeSnapshotKey{node: node, vmid: vmid}
	s.snapshots[key] = append(s.snapshots[key], snapshot)
}

func (s *fakeState) applyRollback(node string, vmid int, name string) {
	s.vmMu.Lock()
	defer s.vmMu.Unlock()

	index := s.findVM(node, vmid)
	if index < 0 {
		return
	}

	for _, snapshot := range s.snapshots[fakeSnapshotKey{node: node, vmid: vmid}] {
		if snapshot.Name != name {
			continue
		}

		if snapshot.VMState {
			s.vms[index].Status = VMRunning
			s.vms[index].Uptime = fakeUptimeOnStart

			return
		}

		s.vms[index].Status = VMStopped
		s.vms[index].Uptime = 0

		return
	}
}

func (s *fakeState) removeSnapshot(node string, vmid int, name string) {
	s.vmMu.Lock()
	defer s.vmMu.Unlock()

	key := fakeSnapshotKey{node: node, vmid: vmid}
	snapshots := s.snapshots[key]

	index := slices.IndexFunc(snapshots, func(snapshot VMSnapshot) bool { return snapshot.Name == name })
	if index >= 0 {
		s.snapshots[key] = slices.Delete(snapshots, index, index+1)
	}
}

func cloneVMSnapshots(snapshots []VMSnapshot) []VMSnapshot {
	return append([]VMSnapshot(nil), snapshots...)
}
