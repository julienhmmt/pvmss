package cluster

import (
	"context"
	"fmt"
	"slices"
)

const fakeMigrateLock = "migrate"

// MigrationPrecheck implements Migrator over the fake dataset: every other
// online node is allowed and disks on non-shared storage are local.
func (fake Fake) MigrationPrecheck(_ context.Context, node string, vmid int) (MigrationPrecheck, error) {
	state := fake.stateOrDefault()

	state.vmMu.RLock()
	defer state.vmMu.RUnlock()

	index := state.findVM(node, vmid)
	if index < 0 {
		return MigrationPrecheck{}, ErrNotFound
	}

	if override, ok := state.migrationPrechecks[vmid]; ok && override != nil {
		return cloneMigrationPrecheck(*override), nil
	}

	precheck := MigrationPrecheck{}

	for _, candidate := range state.nodes {
		if candidate.Name != node && candidate.Status == NodeOnline {
			precheck.AllowedNodes = append(precheck.AllowedNodes, candidate.Name)
		}
	}

	for position, disk := range state.vms[index].Disks {
		if !state.storageShared(node, disk.Storage) {
			precheck.LocalDisks = append(precheck.LocalDisks, fmt.Sprintf("%s:vm-%d-disk-%d", disk.Storage, vmid, position))
		}
	}

	return precheck, nil
}

// Migrate implements Migrator: it locks the VM and moves it to the target
// when the returned task completes.
func (fake Fake) Migrate(_ context.Context, node string, vmid int, spec MigrateSpec) (string, error) {
	state := fake.stateOrDefault()
	if err := state.migrateError(); err != nil {
		return "", err
	}

	if err := state.ensureVM(node, vmid); err != nil {
		return "", err
	}

	if err := state.lockForMigration(vmid); err != nil {
		return "", err
	}

	upid := state.newSnapshotTask(node, vmid, "qmigrate", func() { state.applyMigration(node, spec.Target, vmid) })
	state.setTaskOnError(upid, func() { state.clearVMLock(vmid) })
	state.record(FakeCall{Node: node, VMID: vmid, Action: "migrate", Name: spec.Target, Online: spec.Online})

	return upid, nil
}

// SetFakeMigrationPrecheck replaces the computed precheck answer for vmid on
// the default fake. A nil precheck clears the override.
func SetFakeMigrationPrecheck(vmid int, precheck *MigrationPrecheck) {
	state := defaultState()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	if precheck == nil {
		delete(state.migrationPrechecks, vmid)
		return
	}

	if state.migrationPrechecks == nil {
		state.migrationPrechecks = make(map[int]*MigrationPrecheck)
	}

	state.migrationPrechecks[vmid] = precheck
}

// SetFakeMigrateError makes every Migrate dispatch on the default fake fail
// with err. A nil error clears it.
func SetFakeMigrateError(err error) {
	state := defaultState()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	state.migrateErr = err
}

func (s *fakeState) migrateError() error {
	s.vmMu.RLock()
	defer s.vmMu.RUnlock()

	return s.migrateErr
}

func (s *fakeState) storageShared(node, name string) bool {
	return slices.ContainsFunc(s.storages, func(storage Storage) bool {
		return storage.Name == name && storage.Node == node && storage.Shared
	})
}

func (s *fakeState) lockForMigration(vmid int) error {
	s.vmMu.Lock()
	defer s.vmMu.Unlock()

	if lock := s.vmLocks[vmid]; lock != "" {
		return fmt.Errorf("VM is locked (%s)", lock)
	}

	if s.vmLocks == nil {
		s.vmLocks = make(map[int]string)
	}

	s.vmLocks[vmid] = fakeMigrateLock

	return nil
}

func (s *fakeState) clearVMLock(vmid int) {
	s.vmMu.Lock()
	defer s.vmMu.Unlock()

	delete(s.vmLocks, vmid)
}

func (s *fakeState) setTaskOnError(upid string, onError func()) {
	s.createMu.Lock()
	defer s.createMu.Unlock()

	if task, ok := s.tasks[upid]; ok {
		task.onError = onError
	}
}

func (s *fakeState) applyMigration(source, target string, vmid int) {
	s.vmMu.Lock()
	defer s.vmMu.Unlock()

	if index := s.findVM(source, vmid); index >= 0 {
		s.vms[index].Node = target
		moveKeyed(s.snapshots, fakeSnapshotKey{node: source, vmid: vmid}, fakeSnapshotKey{node: target, vmid: vmid})
		moveKeyed(s.cloudInitConfigs, fakeCloudInitKey{node: source, vmid: vmid}, fakeCloudInitKey{node: target, vmid: vmid})
		moveKeyed(s.cloudInitDrives, fakeCloudInitKey{node: source, vmid: vmid}, fakeCloudInitKey{node: target, vmid: vmid})
	}

	delete(s.vmLocks, vmid)
}

func moveKeyed[K comparable, V any](m map[K]V, from, to K) {
	if value, ok := m[from]; ok {
		m[to] = value
		delete(m, from)
	}
}

func cloneMigrationPrecheck(precheck MigrationPrecheck) MigrationPrecheck {
	precheck.AllowedNodes = slices.Clone(precheck.AllowedNodes)
	precheck.LocalDisks = slices.Clone(precheck.LocalDisks)
	precheck.LocalResources = slices.Clone(precheck.LocalResources)

	return precheck
}
