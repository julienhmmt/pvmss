//nolint:wsl_v5 // sweep test fixtures keep setup and assertions adjacent
package vm_test

import (
	"context"
	"errors"
	"log/slog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"slices"
	"strings"
	"testing"
	"time"
)

var sweepNow = time.Date(2026, 11, 1, 3, 0, 0, 0, time.UTC)

func TestIsSnapshotDue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		schedule store.SnapshotSchedule
		want     bool
	}{
		{name: "off is never due", schedule: store.SnapshotSchedule{Frequency: vm.ScheduleOff}},
		{name: "daily never run is due", schedule: store.SnapshotSchedule{Frequency: vm.ScheduleDaily}, want: true},
		{name: "daily run 25h ago is due", schedule: store.SnapshotSchedule{Frequency: vm.ScheduleDaily, LastRunAt: sweepNow.Add(-25 * time.Hour)}, want: true},
		{name: "daily run 23h ago is not due", schedule: store.SnapshotSchedule{Frequency: vm.ScheduleDaily, LastRunAt: sweepNow.Add(-23 * time.Hour)}},
		{name: "weekly run 6d ago is not due", schedule: store.SnapshotSchedule{Frequency: vm.ScheduleWeekly, LastRunAt: sweepNow.Add(-6 * 24 * time.Hour)}},
		{name: "weekly run 8d ago is due", schedule: store.SnapshotSchedule{Frequency: vm.ScheduleWeekly, LastRunAt: sweepNow.Add(-8 * 24 * time.Hour)}, want: true},
		{name: "unknown frequency is not due", schedule: store.SnapshotSchedule{Frequency: "hourly"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := vm.IsSnapshotDue(tt.schedule, sweepNow); got != tt.want {
				t.Fatalf("IsSnapshotDue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAutoSnapshotNameFitsSnapshotPolicy(t *testing.T) {
	t.Parallel()

	name := vm.AutoSnapshotName(sweepNow)
	if name != "pvmss-auto-20261101-0300" {
		t.Fatalf("AutoSnapshotName() = %q", name)
	}
	if err := vm.ValidateSnapshotName(name); err != nil {
		t.Fatalf("auto name rejected by snapshot policy: %v", err)
	}
	if !vm.IsAutoSnapshot(name) || vm.IsAutoSnapshot("before-upgrade") {
		t.Fatal("IsAutoSnapshot must match only the scheduler prefix")
	}
}

func TestAutoSnapshotsToPruneKeepsNewestAutoOnly(t *testing.T) {
	t.Parallel()

	snapshots := []vm.Snapshot{
		{Name: "pvmss-auto-c", CreatedAt: sweepNow.Add(-1 * time.Hour)},
		{Name: "manual-old", CreatedAt: sweepNow.Add(-9 * time.Hour)},
		{Name: "pvmss-auto-a", CreatedAt: sweepNow.Add(-3 * time.Hour)},
		{Name: "pvmss-auto-b", CreatedAt: sweepNow.Add(-2 * time.Hour)},
	}

	pruned := vm.AutoSnapshotsToPrune(snapshots, 1)
	names := make([]string, 0, len(pruned))
	for _, snapshot := range pruned {
		names = append(names, snapshot.Name)
	}

	if want := []string{"pvmss-auto-a", "pvmss-auto-b"}; !slices.Equal(names, want) {
		t.Fatalf("pruned = %v, want oldest first %v", names, want)
	}
	if got := vm.AutoSnapshotsToPrune(snapshots, 3); got != nil {
		t.Fatalf("keep >= auto count must prune nothing, got %v", got)
	}
	if got := vm.AutoSnapshotsToPrune(snapshots, -4); len(got) != 3 {
		t.Fatalf("negative keep clamps to zero, got %d pruned", len(got))
	}
}

func TestValidateSnapshotSchedule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		frequency string
		retain    int
		max       int
		wantErr   error
	}{
		{name: "off ignores retain", frequency: vm.ScheduleOff},
		{name: "daily within cap", frequency: vm.ScheduleDaily, retain: 3, max: 5},
		{name: "weekly without cap", frequency: vm.ScheduleWeekly, retain: 10},
		{name: "retain zero rejected", frequency: vm.ScheduleDaily, retain: 0, max: 5, wantErr: vm.ErrInvalidSnapshotSchedule},
		{name: "unknown frequency rejected", frequency: "hourly", retain: 2, max: 5, wantErr: vm.ErrInvalidSnapshotSchedule},
		{name: "retain equal to cap rejected", frequency: vm.ScheduleDaily, retain: 5, max: 5, wantErr: vm.ErrScheduleRetainExceedsMax},
		{name: "retain above cap rejected", frequency: vm.ScheduleDaily, retain: 9, max: 5, wantErr: vm.ErrScheduleRetainExceedsMax},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := vm.ValidateSnapshotSchedule(tt.frequency, tt.retain, tt.max)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}

				return
			}
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("error = %v, want %v", err, tt.wantErr)
			}
		})
	}
}

// sweepStore records every schedule write and audit row the sweeper makes.
type sweepStore struct {
	schedules []store.SnapshotSchedule
	marked    []time.Time
	deleted   []int
	actions   []string
}

func (s *sweepStore) UpsertSnapshotSchedule(context.Context, string, int, string, string, int) error {
	return nil
}

func (s *sweepStore) DeleteSnapshotSchedule(_ context.Context, _ string, vmid int) error {
	s.deleted = append(s.deleted, vmid)

	return nil
}

func (s *sweepStore) ListSnapshotSchedules(context.Context) ([]store.SnapshotSchedule, error) {
	return s.schedules, nil
}

func (s *sweepStore) MarkSnapshotScheduleRun(_ context.Context, _ string, _ int, at time.Time) error {
	s.marked = append(s.marked, at)

	return nil
}

func (s *sweepStore) RecordAction(_ context.Context, actor, _ string, _ int, action string) error {
	s.actions = append(s.actions, actor+":"+action)

	return nil
}

// sweepClient embeds the cluster client interface, so only the snapshot and
// status methods the sweeper uses are real. Calling anything else panics.
type sweepClient struct {
	cluster.Client
	snapshots []cluster.VMSnapshot
	lock      string
	listErr   error
	createErr error
	// createTaskFails makes the create task finish in error (the dispatch succeeds).
	createTaskFails bool
	// failDelete names snapshots whose delete task finishes in error; a
	// name is removed from the list only when its delete succeeds.
	failDelete map[string]bool
	created    []string
	deleted    []string
}

func (c *sweepClient) VMStatus(context.Context, string, int) (cluster.VMLiveStatus, error) {
	return cluster.VMLiveStatus{Lock: c.lock}, nil
}

func (c *sweepClient) ListSnapshots(context.Context, string, int) ([]cluster.VMSnapshot, error) {
	return append([]cluster.VMSnapshot(nil), c.snapshots...), c.listErr
}

// TaskStatus reports a task's outcome from the upid the fake handed out.
func (c *sweepClient) TaskStatus(_ context.Context, upid string) (cluster.TaskStatus, error) {
	if upid == "upid-create" && c.createTaskFails {
		return cluster.TaskStatus{State: cluster.TaskError, ExitMessage: "create failed"}, nil
	}
	if name, ok := strings.CutPrefix(upid, "upid-delete-"); ok && c.failDelete[name] {
		return cluster.TaskStatus{State: cluster.TaskError, ExitMessage: "delete failed"}, nil
	}

	return cluster.TaskStatus{State: cluster.TaskOK}, nil
}

func (c *sweepClient) CreateSnapshot(_ context.Context, _ string, _ int, name, _ string, _ bool) (string, error) {
	if c.createErr != nil {
		return "", c.createErr
	}
	c.created = append(c.created, name)
	if !c.createTaskFails {
		c.snapshots = append(c.snapshots, cluster.VMSnapshot{Name: name, CreatedAt: sweepNow})
	}

	return "upid-create", nil
}

func (c *sweepClient) DeleteSnapshot(_ context.Context, _ string, _ int, name string) (string, error) {
	c.deleted = append(c.deleted, name)
	if !c.failDelete[name] {
		c.snapshots = slices.DeleteFunc(c.snapshots, func(s cluster.VMSnapshot) bool { return s.Name == name })
	}

	return "upid-delete-" + name, nil
}

func (c *sweepClient) RollbackSnapshot(context.Context, string, int, string) (string, error) {
	return "upid", nil
}

type sweepClients struct {
	client *sweepClient
	err    error
}

func (p sweepClients) Client(string) (cluster.Client, error) {
	if p.err != nil {
		return nil, p.err
	}

	return p.client, nil
}

func (p sweepClients) List() []string { return []string{"c1"} }

type sweepIndexes struct {
	vmids    []int
	fresh    bool
	pool     string
	untagged bool
}

func (i sweepIndexes) Index(string) (*inventory.Index, error) {
	tags := []string{testPvmssTag}
	if i.untagged {
		tags = nil
	}

	index := &inventory.Index{ByVMID: map[int]cluster.VM{}}
	for _, vmid := range i.vmids {
		index.ByVMID[vmid] = cluster.VM{VMID: vmid, Node: "pve1", Cluster: "c1", Pool: i.pool, Tags: tags}
	}

	return index, nil
}

func (i sweepIndexes) IsIndexFresh(string) bool { return i.fresh }

type sweepPolicy struct{ max int }

func (p sweepPolicy) Gabarit(context.Context, string) (policy.Gabarit, error) {
	return policy.Gabarit{MaxSnapshots: p.max}, nil
}

type sweepCase struct {
	schedule  store.SnapshotSchedule
	client    *sweepClient
	clientErr error
	vmids     []int
	fresh     bool
	max       int
	vmPool    string
	untagged  bool
}

func runSweep(t *testing.T, c sweepCase) (*sweepStore, *sweepClient) {
	t.Helper()

	st := &sweepStore{schedules: []store.SnapshotSchedule{c.schedule}}
	client := c.client
	if client == nil {
		client = &sweepClient{}
	}
	sweepPass(st, client, c)

	return st, client
}

// sweepPass runs one sweep over an existing store and client, so a test can
// run several passes in sequence.
func sweepPass(st *sweepStore, client *sweepClient, c sweepCase) {
	vmids := c.vmids
	if vmids == nil {
		vmids = []int{c.schedule.VMID}
	}
	maxSnapshots := c.max
	if maxSnapshots == 0 {
		maxSnapshots = 5
	}

	vm.SweepSnapshots(context.Background(), vm.SweepDependencies{
		Store:   st,
		Indexes: sweepIndexes{vmids: vmids, fresh: c.fresh, pool: c.vmPool, untagged: c.untagged},
		Clients: sweepClients{client: client, err: c.clientErr},
		Policy:  sweepPolicy{max: maxSnapshots},
		Log:     slog.New(slog.DiscardHandler),
		Now:     func() time.Time { return sweepNow },
	})
}

func dueDaily(vmid int) store.SnapshotSchedule {
	return store.SnapshotSchedule{Cluster: "c1", VMID: vmid, Frequency: vm.ScheduleDaily, Retain: 3}
}

func TestSweepTakesDueSnapshotAndRecordsRun(t *testing.T) {
	t.Parallel()

	st, client := runSweep(t, sweepCase{schedule: dueDaily(100), fresh: true})

	if !slices.Equal(client.created, []string{"pvmss-auto-20261101-0300"}) {
		t.Fatalf("created = %v", client.created)
	}
	if len(st.marked) != 1 || !st.marked[0].Equal(sweepNow) {
		t.Fatalf("run not recorded: %v", st.marked)
	}
	if !slices.Contains(st.actions, "system:vm_snapshot_auto") {
		t.Fatalf("missing system audit row: %v", st.actions)
	}
}

func TestSweepSnapshotsStoppedVMToo(t *testing.T) {
	t.Parallel()

	// Stopped VMs carry no lock, so the sweep snapshots them like running ones.
	_, client := runSweep(t, sweepCase{schedule: dueDaily(100), fresh: true, client: &sweepClient{lock: ""}})

	if len(client.created) != 1 {
		t.Fatalf("stopped VM should still be snapshotted, created = %v", client.created)
	}
}

func TestSweepSkipsLockedVMUntilNextPass(t *testing.T) {
	t.Parallel()

	st, client := runSweep(t, sweepCase{schedule: dueDaily(100), fresh: true, client: &sweepClient{lock: "snapshot"}})

	if len(client.created) != 0 || len(st.marked) != 0 {
		t.Fatalf("locked VM must be skipped without a run record: created=%v marked=%v", client.created, st.marked)
	}
}

func TestSweepBlocksWhenCapFull(t *testing.T) {
	t.Parallel()

	manual := make([]cluster.VMSnapshot, 0, 5)
	for i := range 5 {
		manual = append(manual, cluster.VMSnapshot{Name: "manual-" + string(rune('a'+i)), CreatedAt: sweepNow})
	}

	st, client := runSweep(t, sweepCase{schedule: dueDaily(100), fresh: true, max: 5, client: &sweepClient{snapshots: manual}})

	if len(client.created) != 0 {
		t.Fatalf("cap reached: must not create, created = %v", client.created)
	}
	if !slices.Contains(st.actions, "system:vm_snapshot_schedule_blocked") {
		t.Fatalf("blocked run must be audited: %v", st.actions)
	}
	if len(st.marked) != 0 {
		t.Fatal("blocked run must not advance last_run_at")
	}
}

func TestSweepPrunesAutoToRetainAndKeepsManual(t *testing.T) {
	t.Parallel()

	snapshots := []cluster.VMSnapshot{
		{Name: oldestAutoSnapshot, CreatedAt: sweepNow.Add(-4 * 24 * time.Hour)},
		{Name: "pvmss-auto-20261029-0300", CreatedAt: sweepNow.Add(-3 * 24 * time.Hour)},
		{Name: "pvmss-auto-20261030-0300", CreatedAt: sweepNow.Add(-2 * 24 * time.Hour)},
		{Name: "before-upgrade", CreatedAt: sweepNow.Add(-10 * 24 * time.Hour)},
	}
	schedule := store.SnapshotSchedule{Cluster: "c1", VMID: 100, Frequency: vm.ScheduleDaily, Retain: 2}

	_, client := runSweep(t, sweepCase{schedule: schedule, fresh: true, client: &sweepClient{snapshots: snapshots}})

	// Retain 2: keep the newest auto (one slot stays free for the new run),
	// so the two oldest auto snapshots go. The manual one is never touched.
	if want := []string{oldestAutoSnapshot, "pvmss-auto-20261029-0300"}; !slices.Equal(client.deleted, want) {
		t.Fatalf("deleted = %v, want %v", client.deleted, want)
	}
	if len(client.created) != 1 {
		t.Fatalf("expected one new auto snapshot, created = %v", client.created)
	}
}

func TestSweepSkipsNotDueSchedule(t *testing.T) {
	t.Parallel()

	schedule := dueDaily(100)
	schedule.LastRunAt = sweepNow.Add(-2 * time.Hour)
	st, client := runSweep(t, sweepCase{schedule: schedule, fresh: true})

	if len(client.created) != 0 || len(st.marked) != 0 {
		t.Fatalf("not-due schedule must not run: created=%v", client.created)
	}
}

func TestSweepPrunesScheduleWhenVMGone(t *testing.T) {
	t.Parallel()

	st, client := runSweep(t, sweepCase{schedule: dueDaily(100), fresh: true, vmids: []int{7}})

	if !slices.Equal(st.deleted, []int{100}) {
		t.Fatalf("orphan schedule not pruned: %v", st.deleted)
	}
	if len(client.created) != 0 {
		t.Fatal("missing VM must not be snapshotted")
	}
}

func TestSweepKeepsScheduleWhenInventoryNotFresh(t *testing.T) {
	t.Parallel()

	// A never-refreshed index reports no VMs. That must not read as "VM gone".
	st, client := runSweep(t, sweepCase{schedule: dueDaily(100), fresh: false, vmids: []int{}})

	if len(st.deleted) != 0 || len(client.created) != 0 {
		t.Fatalf("stale inventory must skip, not prune: deleted=%v created=%v", st.deleted, client.created)
	}
}

func TestSweepUnreachableClusterSkipsWithoutRecord(t *testing.T) {
	t.Parallel()

	st, client := runSweep(t, sweepCase{schedule: dueDaily(100), fresh: true, clientErr: cluster.ErrClusterNotFound})

	if len(client.created) != 0 || len(st.marked) != 0 || len(st.deleted) != 0 {
		t.Fatalf("unreachable cluster must only skip: marked=%v deleted=%v", st.marked, st.deleted)
	}
}

func TestSweepDropsScheduleWhenVMIDReusedInOtherPool(t *testing.T) {
	t.Parallel()

	// The schedule was set on a VM in tenant-a. The VMID now belongs to a
	// tagged VM in tenant-b: no snapshot, no prune, schedule dropped, even
	// though the schedule is not due yet.
	schedule := dueDaily(100)
	schedule.Pool = "tenant-a"
	schedule.LastRunAt = sweepNow.Add(-1 * time.Hour)
	st, client := runSweep(t, sweepCase{schedule: schedule, fresh: true, vmPool: "tenant-b", client: &sweepClient{
		snapshots: []cluster.VMSnapshot{{Name: "pvmss-auto-20261001-0300", CreatedAt: sweepNow.Add(-40 * 24 * time.Hour)}},
	}})

	if len(client.created) != 0 || len(client.deleted) != 0 || len(st.marked) != 0 {
		t.Fatalf("reused VMID must get no snapshot or prune: created=%v deleted=%v marked=%v", client.created, client.deleted, st.marked)
	}
	if !slices.Equal(st.deleted, []int{100}) {
		t.Fatalf("schedule not dropped: %v", st.deleted)
	}
}

func TestSweepSkipsUntaggedVMWithoutDropping(t *testing.T) {
	t.Parallel()

	st, client := runSweep(t, sweepCase{schedule: dueDaily(100), fresh: true, untagged: true})

	if len(client.created) != 0 || len(client.deleted) != 0 {
		t.Fatalf("untagged VM must not be snapshotted or pruned: created=%v deleted=%v", client.created, client.deleted)
	}
	if len(st.deleted) != 0 {
		t.Fatalf("untagged VM is skipped, its schedule is kept: deleted=%v", st.deleted)
	}
}

func TestSweepCreateFailureRetriesNextPass(t *testing.T) {
	t.Parallel()

	st, _ := runSweep(t, sweepCase{schedule: dueDaily(100), fresh: true, client: &sweepClient{createErr: errors.New("boom")}})

	if len(st.marked) != 0 {
		t.Fatal("failed create must not advance last_run_at, so the next pass retries")
	}
}
