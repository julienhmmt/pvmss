package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
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
		{name: "dot is valid", input: "before.upgrade"},
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
			cluster.ResetFake()
			t.Cleanup(cluster.ResetFake)

			if test.name == "duplicate" {
				seedSnapshot(t, "existing")
			}

			if test.name == "max snapshots" {
				for index := range 5 {
					seedSnapshot(t, "snapshot-"+string(rune('a'+index)))
				}
			}

			baselineCalls := len(cluster.FakeCallsFor(test.vmid))
			index := testSnapshotIndex(t, test.mutate)
			deps := snapshotDependencies(index, test.vmid, policy.DefaultGabarit())

			_, err := vm.CreateSnapshot(context.Background(), deps, "existing", "", test.vmstate)
			if !errors.Is(err, test.wantError) {
				t.Fatalf("CreateSnapshot error = %v, want %v", err, test.wantError)
			}

			if calls := cluster.FakeCallsFor(test.vmid); len(calls) != baselineCalls {
				t.Fatalf("cluster calls = %+v, want no new calls", calls)
			}
		})
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
		ClusterName: "default", VMID: vmid, Reader: cluster.Fake{}, Writer: cluster.Fake{}, Gabarit: gabarit, Audit: audit,
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
