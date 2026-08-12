package cluster

import (
	"context"
	"testing"
)

//nolint:paralleltest // serial: shared fake snapshot registry
func TestFakeSnapshotLifecycle_IsTaskVisibleOnlyAfterCompletion(t *testing.T) {
	ResetFake()
	t.Cleanup(ResetFake)

	client := Fake{}
	ctx := context.Background()

	upid, err := client.CreateSnapshot(ctx, FakeNode01, 101, "before-upgrade", "pre-migration", false)
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	before, err := client.ListSnapshots(ctx, FakeNode01, 101)
	if err != nil {
		t.Fatalf("ListSnapshots before completion: %v", err)
	}

	if len(before) != 0 {
		t.Fatalf("snapshots before completion = %+v, want empty", before)
	}

	for range 3 {
		if _, err := client.TaskStatus(ctx, upid); err != nil {
			t.Fatalf("TaskStatus: %v", err)
		}
	}

	after, err := client.ListSnapshots(ctx, FakeNode01, 101)
	if err != nil {
		t.Fatalf("ListSnapshots after completion: %v", err)
	}

	if len(after) != 1 || after[0].Name != "before-upgrade" {
		t.Fatalf("snapshots after completion = %+v", after)
	}

	if after[0].Description != "pre-migration" || after[0].VMState {
		t.Fatalf("snapshot details = %+v", after[0])
	}
}

//nolint:paralleltest // serial: shared fake snapshot registry
func TestFakeSnapshotLifecycle_RollbackAndDeleteCompleteAsTasks(t *testing.T) {
	ResetFake()
	t.Cleanup(ResetFake)

	client := Fake{}
	ctx := context.Background()

	createUPID, err := client.CreateSnapshot(ctx, FakeNode01, 101, "restore-point", "", true)
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	completeFakeTask(t, client, createUPID)

	rollbackUPID, err := client.RollbackSnapshot(ctx, FakeNode01, 101, "restore-point")
	if err != nil {
		t.Fatalf("RollbackSnapshot: %v", err)
	}

	completeFakeTask(t, client, rollbackUPID)

	deleteUPID, err := client.DeleteSnapshot(ctx, FakeNode01, 101, "restore-point")
	if err != nil {
		t.Fatalf("DeleteSnapshot: %v", err)
	}

	completeFakeTask(t, client, deleteUPID)

	snapshots, err := client.ListSnapshots(ctx, FakeNode01, 101)
	if err != nil {
		t.Fatalf("ListSnapshots: %v", err)
	}

	if len(snapshots) != 0 {
		t.Fatalf("snapshots after delete = %+v, want empty", snapshots)
	}
}

func completeFakeTask(t *testing.T, client Fake, upid string) {
	t.Helper()

	for range 3 {
		if _, err := client.TaskStatus(context.Background(), upid); err != nil {
			t.Fatalf("TaskStatus: %v", err)
		}
	}
}
