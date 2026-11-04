//nolint:wsl_v5 // sweep retry tests keep fixture setup and assertions adjacent
package vm_test

import (
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"slices"
	"testing"
	"time"
)

var errBoom = errors.New("boom")

// oldestAutoSnapshot is the first of the three scheduler snapshots from autoSnapshots.
const oldestAutoSnapshot = "pvmss-auto-20261028-0300"

// autoSnapshots returns three scheduler snapshots, oldest first.
func autoSnapshots() []cluster.VMSnapshot {
	return []cluster.VMSnapshot{
		{Name: oldestAutoSnapshot, CreatedAt: sweepNow.Add(-4 * 24 * time.Hour)},
		{Name: "pvmss-auto-20261029-0300", CreatedAt: sweepNow.Add(-3 * 24 * time.Hour)},
		{Name: "pvmss-auto-20261030-0300", CreatedAt: sweepNow.Add(-2 * 24 * time.Hour)},
	}
}

func countAuto(snapshots []cluster.VMSnapshot) int {
	count := 0
	for _, snapshot := range snapshots {
		if vm.IsAutoSnapshot(snapshot.Name) {
			count++
		}
	}

	return count
}

func TestSweepPrunesNotDueSchedule(t *testing.T) {
	t.Parallel()

	// Not due (ran an hour ago) but over retain: pruning runs on every pass.
	schedule := dueDaily(100)
	schedule.Retain = 2
	schedule.LastRunAt = sweepNow.Add(-1 * time.Hour)
	_, client := runSweep(t, sweepCase{schedule: schedule, fresh: true, client: &sweepClient{snapshots: autoSnapshots()}})

	if !slices.Equal(client.deleted, []string{oldestAutoSnapshot}) {
		t.Fatalf("not-due schedule must still prune, deleted = %v", client.deleted)
	}
	if len(client.created) != 0 {
		t.Fatalf("not-due schedule must not create, created = %v", client.created)
	}
}

func TestSweepRetriesFailedPruneOnNextPass(t *testing.T) {
	t.Parallel()

	schedule := dueDaily(100)
	schedule.Retain = 2
	schedule.LastRunAt = sweepNow.Add(-1 * time.Hour)
	st := &sweepStore{schedules: []store.SnapshotSchedule{schedule}}
	client := &sweepClient{snapshots: autoSnapshots(), failDelete: map[string]bool{oldestAutoSnapshot: true}}
	c := sweepCase{schedule: schedule, fresh: true}

	sweepPass(st, client, c)
	if countAuto(client.snapshots) != 3 {
		t.Fatalf("failed delete task must leave the snapshot listed, have %d auto", countAuto(client.snapshots))
	}

	// The next 15-minute pass sees the snapshot still listed and retries.
	client.failDelete = nil
	sweepPass(st, client, c)
	if countAuto(client.snapshots) != schedule.Retain {
		t.Fatalf("retry must reach retain %d, have %d auto", schedule.Retain, countAuto(client.snapshots))
	}
}

func TestSweepFinalCountEqualsRetainAfterCreate(t *testing.T) {
	t.Parallel()

	schedule := dueDaily(100)
	schedule.Retain = 2
	_, client := runSweep(t, sweepCase{schedule: schedule, fresh: true, client: &sweepClient{snapshots: autoSnapshots()}})

	if countAuto(client.snapshots) != schedule.Retain {
		t.Fatalf("final auto count = %d, want retain %d", countAuto(client.snapshots), schedule.Retain)
	}
	if !slices.ContainsFunc(client.snapshots, func(s cluster.VMSnapshot) bool { return s.Name == "pvmss-auto-20261101-0300" }) {
		t.Fatalf("the new snapshot must be kept, list = %v", client.snapshots)
	}
}

func TestSweepFailedCreateTaskKeepsRunAndStillPrunes(t *testing.T) {
	t.Parallel()

	schedule := dueDaily(100)
	schedule.Retain = 2
	st, client := runSweep(t, sweepCase{schedule: schedule, fresh: true, client: &sweepClient{snapshots: autoSnapshots(), createTaskFails: true}})

	if len(st.marked) != 0 {
		t.Fatal("a create task that fails must not advance last_run_at")
	}
	if !slices.Equal(client.deleted, []string{oldestAutoSnapshot}) {
		t.Fatalf("prune must still run to retain, deleted = %v", client.deleted)
	}
}

func notDueOverRetain() store.SnapshotSchedule {
	schedule := dueDaily(100)
	schedule.Retain = 2
	schedule.LastRunAt = sweepNow.Add(-1 * time.Hour)

	return schedule
}

func TestSweepRecordsFailedPruneThenClearsIt(t *testing.T) {
	t.Parallel()

	schedule := notDueOverRetain()
	st := &sweepStore{schedules: []store.SnapshotSchedule{schedule}}
	client := &sweepClient{snapshots: autoSnapshots(), failDelete: map[string]bool{oldestAutoSnapshot: true}}
	sweepPass(st, client, sweepCase{schedule: schedule, fresh: true})
	if !slices.Equal(st.failures, []string{vm.ScheduleFailurePruneFailed}) {
		t.Fatalf("failed prune must be recorded: %v", st.failures)
	}

	schedule.LastFailure = vm.ScheduleFailurePruneFailed
	st.schedules = []store.SnapshotSchedule{schedule}
	client.failDelete = nil
	sweepPass(st, client, sweepCase{schedule: schedule, fresh: true})
	if st.cleared != 1 {
		t.Fatalf("successful prune must clear the failure, cleared = %d", st.cleared)
	}
}

func TestSweepRecordsSkipOnlyWhenDue(t *testing.T) {
	t.Parallel()

	st, _ := runSweep(t, sweepCase{schedule: dueDaily(100), fresh: false})
	if !slices.Equal(st.failures, []string{vm.ScheduleFailureUnavailable}) {
		t.Fatalf("due schedule on a stale cluster must record the skip: %v", st.failures)
	}

	st, _ = runSweep(t, sweepCase{schedule: notDueOverRetain(), fresh: false})
	if len(st.failures) != 0 {
		t.Fatalf("not-due schedule must raise no alert: %v", st.failures)
	}
}

func TestSweepLockedVMRecordsNoFailure(t *testing.T) {
	t.Parallel()

	st, _ := runSweep(t, sweepCase{schedule: dueDaily(100), fresh: true, client: &sweepClient{lock: "snapshot"}})
	if len(st.failures) != 0 {
		t.Fatalf("a lock is a normal retry, not a failure: %v", st.failures)
	}
}

func TestSweepFailedCreateKeepsItsFailureOverPrune(t *testing.T) {
	t.Parallel()

	schedule := dueDaily(100)
	schedule.Retain = 2
	st, _ := runSweep(t, sweepCase{schedule: schedule, fresh: true, client: &sweepClient{
		createErr: errBoom, snapshots: autoSnapshots(), failDelete: map[string]bool{oldestAutoSnapshot: true},
	}})
	if !slices.Equal(st.failures, []string{vm.ScheduleFailureCreateFailed}) {
		t.Fatalf("create failure must not be replaced: %v", st.failures)
	}
}
