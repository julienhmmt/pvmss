package vm_test

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
	"strings"
	"testing"
	"time"
)

type snapshotAudit struct {
	actions []string
}

func (audit *snapshotAudit) RecordAction(_ context.Context, _, _ string, _ int, action string) error {
	audit.actions = append(audit.actions, action)
	return nil
}

//nolint:wsl_v5 // snapshot tests keep fixture setup and assertions adjacent
func TestValidateSnapshotName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{name: "letters numbers hyphens underscores", input: "before-upgrade_1"},
		{name: "mixed case valid", input: "Foo-1"},
		{name: "two characters valid", input: "ab"},
		{name: "starts with digit", input: "1foo", wantErr: true},
		{name: "starts with underscore", input: "_foo", wantErr: true},
		{name: "dot is invalid", input: "before.upgrade", wantErr: true},
		{name: "single character", input: "a", wantErr: true},
		{name: "empty", input: "", wantErr: true},
		{name: "starts with hyphen", input: "-before", wantErr: true},
		{name: "invalid characters", input: "before upgrade", wantErr: true},
		{name: "too long", input: "12345678901234567890123456789012345678901", wantErr: true},
		{name: "reserved current", input: "current", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			err := vm.ValidateSnapshotName(test.input)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateSnapshotName(%q) error = %v, wantErr %t", test.input, err, test.wantErr)
			}
		})
	}
}

//nolint:paralleltest // serial: shared fake snapshot registry
func TestCreateSnapshot_GuardsRejectBeforeClusterWrite(t *testing.T) {
	tests := []struct {
		name      string
		vmid      int
		vmstate   bool
		mutate    func(*cluster.Snapshot)
		wantError error
	}{
		{
			name:      "duplicate",
			vmid:      101,
			wantError: vm.ErrDuplicateSnapshotName,
		},
		{
			name:      "max snapshots",
			vmid:      101,
			wantError: vm.ErrMaxSnapshotsReached,
		},
		{
			name:      "stopped vmstate",
			vmid:      101,
			vmstate:   true,
			wantError: vm.ErrVMStateRequiresRunning,
		},
		{
			name:    "unsupported storage",
			vmid:    101,
			vmstate: true,
			mutate: func(snap *cluster.Snapshot) {
				for index := range snap.VMs {
					if snap.VMs[index].VMID != 101 {
						continue
					}

					snap.VMs[index].Status = cluster.VMRunning
					snap.VMs[index].Disks[1].Storage = "local"
				}
			},
			wantError: vm.ErrVMStateUnsupportedStorage,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			runCreateSnapshotGuardCase(t, test.name, test.vmid, test.vmstate, test.mutate, test.wantError)
		})
	}
}

// runCreateSnapshotGuardCase runs a single CreateSnapshot guard case: seeds
// the fake snapshot registry as needed, issues the create, and asserts the
// expected rejection with no cluster write. Extracted from
// TestCreateSnapshot_GuardsRejectBeforeClusterWrite to keep its Cognitive
// Complexity under the SonarQube go:S3776 threshold.
func runCreateSnapshotGuardCase(
	t *testing.T,
	name string,
	vmid int,
	vmstate bool,
	mutate func(*cluster.Snapshot),
	wantError error,
) {
	t.Helper()

	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	if name == "duplicate" {
		seedSnapshot(t, "existing")
	}

	if name == "max snapshots" {
		for index := range 5 {
			seedSnapshot(t, "snapshot-"+string(rune('a'+index)))
		}
	}

	baselineCalls := len(cluster.FakeCallsFor(vmid))
	index := testSnapshotIndex(t, mutate)
	deps := snapshotDependencies(index, vmid, policy.DefaultGabarit())

	_, err := vm.CreateSnapshot(context.Background(), deps, "existing", "", vmstate)
	if !errors.Is(err, wantError) {
		t.Fatalf("CreateSnapshot error = %v, want %v", err, wantError)
	}

	if calls := cluster.FakeCallsFor(vmid); len(calls) != baselineCalls {
		t.Fatalf("cluster calls = %+v, want no new calls", calls)
	}
}

//nolint:paralleltest // serial: shared fake snapshot registry
func TestCreateSnapshot_VMStateAcceptedAndAudited(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	index := testSnapshotIndex(t, func(snap *cluster.Snapshot) {
		for index := range snap.VMs {
			if snap.VMs[index].VMID == 101 {
				snap.VMs[index].Status = cluster.VMRunning
			}
		}
	})
	audit := &snapshotAudit{}
	deps := snapshotDependenciesWithAudit(index, 101, policy.DefaultGabarit(), audit)

	upid, err := vm.CreateSnapshot(context.Background(), deps, "with-ram", "", true)
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	if upid == "" {
		t.Fatal("CreateSnapshot returned an empty upid")
	}

	if len(audit.actions) != 1 || audit.actions[0] != "vm_snapshot_create" {
		t.Fatalf("audit actions = %v", audit.actions)
	}
}

//nolint:paralleltest // serial: shared fake snapshot registry
func TestSnapshotOperations_ResolveExistenceAndAudit(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	seedSnapshot(t, "restore-point")
	index := testSnapshotIndex(t, nil)
	audit := &snapshotAudit{}
	deps := snapshotDependenciesWithAudit(index, 101, policy.DefaultGabarit(), audit)

	rollbackUPID, err := vm.RollbackSnapshot(context.Background(), deps, "restore-point")
	if err != nil || rollbackUPID == "" {
		t.Fatalf("RollbackSnapshot = %q, %v", rollbackUPID, err)
	}

	deleteUPID, err := vm.DeleteSnapshot(context.Background(), deps, "restore-point")
	if err != nil || deleteUPID == "" {
		t.Fatalf("DeleteSnapshot = %q, %v", deleteUPID, err)
	}

	if _, err := vm.DeleteSnapshot(context.Background(), deps, "missing"); !errors.Is(err, vm.ErrSnapshotNotFound) {
		t.Fatalf("DeleteSnapshot missing error = %v", err)
	}

	if len(audit.actions) != 2 || audit.actions[0] != "vm_snapshot_rollback" || audit.actions[1] != "vm_snapshot_delete" {
		t.Fatalf("audit actions = %v", audit.actions)
	}
}

//nolint:paralleltest // serial: shared fake snapshot registry
func TestSnapshotOperations_NonOwnerIsRejectedBeforeWriter(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	seedSnapshot(t, "restore-point")
	index := testSnapshotIndex(t, nil)
	deps := snapshotDependencies(index, 101, policy.DefaultGabarit())
	deps.Actor = auth.Identity{Username: cluster.FakeUserBob, Pool: cluster.FakePoolBob}
	baselineCalls := len(cluster.FakeCallsFor(101))

	if _, err := vm.RollbackSnapshot(context.Background(), deps, "restore-point"); !errors.Is(err, vm.ErrForbidden) {
		t.Fatalf("RollbackSnapshot error = %v, want ErrForbidden", err)
	}

	if len(cluster.FakeCallsFor(101)) != baselineCalls {
		t.Fatalf("cluster calls = %+v, want no new calls", cluster.FakeCallsFor(101))
	}
}

func snapshotDependencies(index *inventory.Index, vmid int, gabarit policy.Gabarit) vm.SnapshotDependencies {
	return snapshotDependenciesWithAudit(index, vmid, gabarit, &snapshotAudit{})
}

func snapshotDependenciesWithAudit(index *inventory.Index, vmid int, gabarit policy.Gabarit, audit vm.AuditRecorder) vm.SnapshotDependencies {
	return vm.SnapshotDependencies{
		Index: index, Actor: auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice},
		ClusterName: testClusterName, VMID: vmid, Reader: cluster.Fake{}, Writer: cluster.Fake{}, Gabarit: gabarit, Audit: audit,
	}
}

func testSnapshotIndex(t *testing.T, mutate func(*cluster.Snapshot)) *inventory.Index {
	t.Helper()

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if mutate != nil {
		mutate(&snap)
	}

	index := inventory.BuildIndex(snap)
	index.RefreshedAt = time.Now()

	return &index
}

func seedSnapshot(t *testing.T, name string) {
	t.Helper()

	upid, err := (cluster.Fake{}).CreateSnapshot(context.Background(), cluster.FakeNode01, 101, name, "", false)
	if err != nil {
		t.Fatalf("CreateSnapshot seed: %v", err)
	}

	for range 3 {
		if _, err := (cluster.Fake{}).TaskStatus(context.Background(), upid); err != nil {
			t.Fatalf("TaskStatus seed: %v", err)
		}
	}
}

// lockRetryWriter rejects the first rejections snapshot writes with a
// Proxmox-style "VM is locked (<lock>)" error, then delegates to the fake — a
// deterministic stand-in for a lock that clears (ticket 06).
type lockRetryWriter struct {
	cluster.SnapshotWriter
	rejections int
	lockName   string
}

func (w *lockRetryWriter) reject() error {
	if w.rejections > 0 {
		w.rejections--

		return fmt.Errorf("VM is locked (%s)", w.lockName)
	}

	return nil
}

func (w *lockRetryWriter) CreateSnapshot(ctx context.Context, node string, vmid int, name, description string, vmstate bool) (string, error) {
	if err := w.reject(); err != nil {
		return "", err
	}

	return w.SnapshotWriter.CreateSnapshot(ctx, node, vmid, name, description, vmstate)
}

func (w *lockRetryWriter) RollbackSnapshot(ctx context.Context, node string, vmid int, name string) (string, error) {
	if err := w.reject(); err != nil {
		return "", err
	}

	return w.SnapshotWriter.RollbackSnapshot(ctx, node, vmid, name)
}

func (w *lockRetryWriter) DeleteSnapshot(ctx context.Context, node string, vmid int, name string) (string, error) {
	if err := w.reject(); err != nil {
		return "", err
	}

	return w.SnapshotWriter.DeleteSnapshot(ctx, node, vmid, name)
}

// TestCreateSnapshot_VMStateOnDirQcow2_Allowed — ticket 07: a dir storage
// holding a qcow2 disk supports RAM-state snapshots (the old 4-entry plugin
// table refused it — false negative).
//
//nolint:paralleltest // serial: shared fake snapshot registry
func TestCreateSnapshot_VMStateOnDirQcow2_Allowed(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	index := testSnapshotIndex(t, func(snap *cluster.Snapshot) {
		for index := range snap.VMs {
			if snap.VMs[index].VMID != 101 {
				continue
			}

			snap.VMs[index].Status = cluster.VMRunning
			snap.VMs[index].Disks[1].Storage = cluster.FakeStorageLocal
			snap.VMs[index].Disks[1].Format = "qcow2"
		}
	})
	deps := snapshotDependencies(index, 101, policy.DefaultGabarit())

	upid, err := vm.CreateSnapshot(context.Background(), deps, "ram-on-dir", "", true)
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	if upid == "" {
		t.Fatal("CreateSnapshot returned an empty upid")
	}
}

// TestCreateSnapshot_RawOnDir_RejectedBeforeClusterWrite — ticket 07: a raw
// disk on a file storage cannot snapshot at all (false positive the old model
// never checked); refused before any cluster write.
//
//nolint:paralleltest // serial: shared fake snapshot registry
func TestCreateSnapshot_RawOnDir_RejectedBeforeClusterWrite(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	index := testSnapshotIndex(t, func(snap *cluster.Snapshot) {
		for index := range snap.VMs {
			if snap.VMs[index].VMID != 101 {
				continue
			}

			snap.VMs[index].Disks[1].Storage = cluster.FakeStorageLocal
			snap.VMs[index].Disks[1].Format = "raw"
		}
	})
	deps := snapshotDependencies(index, 101, policy.DefaultGabarit())
	baselineCalls := len(cluster.FakeCallsFor(101))

	_, err := vm.CreateSnapshot(context.Background(), deps, "raw-on-dir", "", false)
	if !errors.Is(err, vm.ErrSnapshotUnsupportedStorage) {
		t.Fatalf("CreateSnapshot error = %v, want ErrSnapshotUnsupportedStorage", err)
	}

	if calls := cluster.FakeCallsFor(101); len(calls) != baselineCalls {
		t.Fatalf("cluster calls = %+v, want no new calls", calls)
	}
}

// TestSnapshotCapability_Reasons — ticket 07: ComputeSnapshotCapability
// reports canSnapshot/canVMState with the reasons the dialog shows.
//
//nolint:paralleltest // serial: shared fake snapshot registry
func TestSnapshotCapability_Reasons(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	index := testSnapshotIndex(t, nil)

	capability := vm.ComputeSnapshotCapability(mustResolveVM(t, index, 101), index)

	if !capability.CanSnapshot {
		t.Errorf("canSnapshot = false, want true (lvmthin disks)")
	}

	if capability.CanVMState {
		t.Errorf("canVMState = true, want false (VM stopped)")
	}

	if len(capability.Warnings) == 0 {
		t.Error("warnings empty, want the running-state reason")
	}
}

func mustResolveVM(t *testing.T, index *inventory.Index, vmid int) vm.Entity {
	t.Helper()

	entity, err := vm.Resolve(index, auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}, testClusterName, vmid)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	return entity
}

// TestCreateSnapshot_RetriesOnLockUntilItClears — ticket 06: a snapshot
// create rejected while the VM is locked is retried until the lock clears.
//
//nolint:paralleltest // serial: shared fake snapshot registry
func TestCreateSnapshot_RetriesOnLockUntilItClears(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	index := testSnapshotIndex(t, nil)
	deps := snapshotDependencies(index, 101, policy.DefaultGabarit())
	deps.Writer = &lockRetryWriter{SnapshotWriter: cluster.Fake{}, rejections: 2, lockName: "backup"}

	upid, err := vm.CreateSnapshot(context.Background(), deps, "locked-then-ok", "", false)
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	if upid == "" {
		t.Fatal("CreateSnapshot returned an empty upid")
	}
}

// TestSnapshotWrite_LockRetryExpiryNamesLock — the retry budget expiring
// reports vm.ErrVMLocked naming the lock, not a generic failure.
//
//nolint:paralleltest // serial: shared fake snapshot registry
func TestSnapshotWrite_LockRetryExpiryNamesLock(t *testing.T) {
	origPoll, origWait := vm.LockRetryPollInterval, vm.MaxLockRetryWait
	vm.LockRetryPollInterval = time.Millisecond
	vm.MaxLockRetryWait = 30 * time.Millisecond

	t.Cleanup(func() { vm.LockRetryPollInterval, vm.MaxLockRetryWait = origPoll, origWait })

	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	index := testSnapshotIndex(t, nil)
	deps := snapshotDependencies(index, 101, policy.DefaultGabarit())
	deps.Writer = &lockRetryWriter{SnapshotWriter: cluster.Fake{}, rejections: 100, lockName: "backup"}

	_, err := vm.CreateSnapshot(context.Background(), deps, "never-unlocked", "", false)
	if !errors.Is(err, vm.ErrVMLocked) {
		t.Fatalf("error = %v, want ErrVMLocked", err)
	}

	if !strings.Contains(err.Error(), "backup") {
		t.Errorf("error %q should name the lock", err.Error())
	}
}

// TestDeleteSnapshot_LockSnapshotDeleteCarriesUnlockCommand — a VM stuck at
// lock=snapshot-delete (NFS ESTALE, pegaprox incident #422) cannot clear
// itself by waiting; the expiry error carries the operator command.
//
//nolint:paralleltest // serial: shared fake snapshot registry
func TestDeleteSnapshot_LockSnapshotDeleteCarriesUnlockCommand(t *testing.T) {
	origPoll, origWait := vm.LockRetryPollInterval, vm.MaxLockRetryWait
	vm.LockRetryPollInterval = time.Millisecond
	vm.MaxLockRetryWait = 30 * time.Millisecond

	t.Cleanup(func() { vm.LockRetryPollInterval, vm.MaxLockRetryWait = origPoll, origWait })

	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	seedSnapshot(t, "stuck")
	index := testSnapshotIndex(t, nil)
	deps := snapshotDependencies(index, 101, policy.DefaultGabarit())
	deps.Writer = &lockRetryWriter{SnapshotWriter: cluster.Fake{}, rejections: 100, lockName: "snapshot-delete"}

	_, err := vm.DeleteSnapshot(context.Background(), deps, "stuck")
	if !errors.Is(err, vm.ErrVMLocked) {
		t.Fatalf("error = %v, want ErrVMLocked", err)
	}

	if !strings.Contains(err.Error(), "qm unlock 101") {
		t.Errorf("error %q should carry the qm unlock command", err.Error())
	}
}
