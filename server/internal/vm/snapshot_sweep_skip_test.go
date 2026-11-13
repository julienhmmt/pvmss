package vm_test

import (
	"context"
	"log/slog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"slices"
	"testing"
	"time"
)

// bareClient satisfies the client provider contract but none of the snapshot
// capabilities the sweeper type-asserts on.
type bareClient struct{ cluster.Client }

type bareClients struct{}

func (bareClients) Client(string) (cluster.Client, error) { return bareClient{}, nil }
func (bareClients) List() []string                        { return []string{"c1"} }

func sweepDeps(st *sweepStore, clients cluster.ClientProvider, c sweepCase) vm.SweepDependencies {
	return vm.SweepDependencies{
		Store:   st,
		Indexes: sweepIndexes{vmids: []int{c.schedule.VMID}, fresh: c.fresh, err: c.indexErr},
		Clients: clients,
		Policy:  sweepPolicy{max: 5, err: c.policyErr},
		Log:     slog.New(slog.DiscardHandler),
		Now:     func() time.Time { return sweepNow },
	}
}

func TestSweepStopsWhenScheduleListFails(t *testing.T) {
	t.Parallel()

	st := &sweepStore{listErr: errBoom}
	vm.SweepSnapshots(context.Background(), sweepDeps(st, sweepClients{client: &sweepClient{}}, sweepCase{schedule: dueDaily(100), fresh: true}))

	if len(st.failures) != 0 || len(st.marked) != 0 {
		t.Fatalf("a failed schedule list must skip the pass, failures = %v", st.failures)
	}
}

func TestSweepSkipsWhenIndexUnavailable(t *testing.T) {
	t.Parallel()

	st, _ := runSweep(t, sweepCase{schedule: dueDaily(100), fresh: true, indexErr: errBoom})
	if len(st.failures) == 0 {
		t.Fatal("an unavailable index must mark the schedule skipped")
	}
}

func TestSweepSkipsWhenSnapshotsUnreadable(t *testing.T) {
	t.Parallel()

	st, _ := runSweep(t, sweepCase{schedule: dueDaily(100), fresh: true, client: &sweepClient{listErr: errBoom}})
	if len(st.failures) == 0 {
		t.Fatal("a failed snapshot list must mark the schedule skipped")
	}
}

func TestSweepSkipsWhenGabaritUnavailable(t *testing.T) {
	t.Parallel()

	st, _ := runSweep(t, sweepCase{schedule: dueDaily(100), fresh: true, policyErr: errBoom})
	if len(st.failures) == 0 {
		t.Fatal("an unavailable gabarit must mark the schedule skipped")
	}
}

func TestSweepSkipsWhenStatusUnreadable(t *testing.T) {
	t.Parallel()

	st, _ := runSweep(t, sweepCase{schedule: dueDaily(100), fresh: true, client: &sweepClient{statusErr: errBoom}})
	if len(st.failures) == 0 {
		t.Fatal("a failed status read must mark the schedule skipped")
	}
}

func TestSweepDefersPruneWhenRelistFails(t *testing.T) {
	t.Parallel()

	schedule := dueDaily(100)
	schedule.Retain = 1
	_, client := runSweep(t, sweepCase{schedule: schedule, fresh: true, client: &sweepClient{snapshots: autoSnapshots(), failRelist: true}})

	if len(client.created) != 1 {
		t.Fatalf("created = %v, want the due snapshot taken", client.created)
	}

	if len(client.deleted) != 0 {
		t.Fatalf("relist failure must defer pruning, deleted = %v", client.deleted)
	}
}

func TestSweepSkipsClientWithoutSnapshotCapability(t *testing.T) {
	t.Parallel()

	st := &sweepStore{schedules: []store.SnapshotSchedule{dueDaily(100)}}
	vm.SweepSnapshots(context.Background(), sweepDeps(st, bareClients{}, sweepCase{schedule: dueDaily(100), fresh: true}))

	if len(st.failures) == 0 {
		t.Fatal("a client without snapshot capability must mark the schedule skipped")
	}
}

func TestSweepPruneOrphanKeepsRowOnStoreError(t *testing.T) {
	t.Parallel()

	schedule := dueDaily(100)
	st := &sweepStore{schedules: []store.SnapshotSchedule{schedule}, deleteErr: errBoom}
	deps := sweepDeps(st, sweepClients{client: &sweepClient{}}, sweepCase{schedule: schedule, fresh: true})
	deps.Indexes = sweepIndexes{vmids: []int{999}, fresh: true}

	vm.SweepSnapshots(context.Background(), deps)

	if len(st.deleted) != 0 {
		t.Fatalf("a failed prune must keep the schedule row, deleted = %v", st.deleted)
	}
}

func TestSweepStopsOnCancelledContext(t *testing.T) {
	t.Parallel()

	st := &sweepStore{schedules: []store.SnapshotSchedule{dueDaily(100), dueDaily(101)}}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	vm.SweepSnapshots(ctx, sweepDeps(st, sweepClients{client: &sweepClient{}}, sweepCase{schedule: dueDaily(100), fresh: true}))

	if len(st.marked) != 0 || len(st.failures) != 0 {
		t.Fatal("a cancelled context must abort the pass")
	}
}

func TestSweepFailureMarkedUsesExpectedValue(t *testing.T) {
	t.Parallel()

	st, _ := runSweep(t, sweepCase{schedule: dueDaily(100), fresh: true, indexErr: errBoom})
	if !slices.Contains(st.failures, vm.ScheduleFailureUnavailable) {
		t.Fatalf("failures = %v, want %q", st.failures, vm.ScheduleFailureUnavailable)
	}
}
