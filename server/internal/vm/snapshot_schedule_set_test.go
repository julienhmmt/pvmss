//nolint:wsl_v5 // schedule fixtures keep setup and assertions adjacent
package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
	"testing"
)

// scheduleIndex is the single-VM index the schedule writes resolve against:
// alice owns VM 100 through her pool, everyone else is refused by Resolve.
func scheduleIndex() *inventory.Index {
	return &inventory.Index{ByVMID: map[int]cluster.VM{
		100: {VMID: 100, Name: "web-sched", Node: cluster.FakeNode01, Cluster: testClusterName, Pool: cluster.FakePoolAlice, Tags: []string{testPvmssTag}},
	}}
}

func scheduleDeps(fixture createFixture) vm.ScheduleDependencies {
	return vm.ScheduleDependencies{
		Index:       scheduleIndex(),
		Actor:       aliceIdentity(),
		ClusterName: testClusterName,
		VMID:        100,
		Policy:      mustPolicyService(fixture.store),
		Store:       fixture.store,
	}
}

func scheduleAuditActions(t *testing.T, fixture createFixture) []string {
	t.Helper()

	entries, err := fixture.store.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	actions := make([]string, 0, len(entries))
	for _, entry := range entries {
		actions = append(actions, entry.Action)
	}

	return actions
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestSetSnapshotSchedule_WritesRowAndAudit(t *testing.T) {
	fixture := newCreateFixture(t)

	if err := vm.SetSnapshotSchedule(context.Background(), scheduleDeps(fixture), vm.ScheduleDaily, 3); err != nil {
		t.Fatalf("SetSnapshotSchedule: %v", err)
	}

	schedule, found, err := fixture.store.GetSnapshotSchedule(context.Background(), testClusterName, 100)
	if err != nil || !found {
		t.Fatalf("GetSnapshotSchedule = %+v found=%v err=%v", schedule, found, err)
	}
	if schedule.Frequency != vm.ScheduleDaily || schedule.Retain != 3 || schedule.Pool != cluster.FakePoolAlice {
		t.Fatalf("schedule = %+v, want daily retain 3 pool %s", schedule, cluster.FakePoolAlice)
	}
	if actions := scheduleAuditActions(t, fixture); len(actions) != 1 || actions[0] != vm.AuditActionScheduleSet {
		t.Fatalf("audit = %v, want [%s]", actions, vm.AuditActionScheduleSet)
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestSetSnapshotSchedule_OffStoresRetainZero(t *testing.T) {
	fixture := newCreateFixture(t)

	if err := vm.SetSnapshotSchedule(context.Background(), scheduleDeps(fixture), vm.ScheduleOff, 9); err != nil {
		t.Fatalf("SetSnapshotSchedule: %v", err)
	}

	schedule, found, err := fixture.store.GetSnapshotSchedule(context.Background(), testClusterName, 100)
	if err != nil || !found {
		t.Fatalf("GetSnapshotSchedule = %+v found=%v err=%v", schedule, found, err)
	}
	if schedule.Frequency != vm.ScheduleOff || schedule.Retain != 0 {
		t.Fatalf("schedule = %+v, want off retain 0", schedule)
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestSetSnapshotSchedule_ValidationAndDependencies(t *testing.T) {
	fixture := newCreateFixture(t)
	svc := mustPolicyService(fixture.store)
	if err := svc.SetPolicy(context.Background(), testClusterName, policy.Gabarit{MaxSnapshots: 2}, -1); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	for _, testCase := range []struct {
		name      string
		mutate    func(vm.ScheduleDependencies) vm.ScheduleDependencies
		frequency string
		retain    int
		wantErr   error
	}{
		{name: "no index", mutate: func(d vm.ScheduleDependencies) vm.ScheduleDependencies { d.Index = nil; return d }, frequency: vm.ScheduleDaily, retain: 1, wantErr: vm.ErrNotFound},
		{name: "vm outside scope", mutate: func(d vm.ScheduleDependencies) vm.ScheduleDependencies { d.VMID = 999; return d }, frequency: vm.ScheduleDaily, retain: 1, wantErr: vm.ErrNotFound},
		{name: "no policy", mutate: func(d vm.ScheduleDependencies) vm.ScheduleDependencies { d.Policy = nil; return d }, frequency: vm.ScheduleDaily, retain: 1, wantErr: policy.ErrUnavailable},
		{name: "unknown frequency", frequency: "hourly", retain: 1, wantErr: vm.ErrInvalidSnapshotSchedule},
		{name: "retain reaches cap", frequency: vm.ScheduleDaily, retain: 2, wantErr: vm.ErrScheduleRetainExceedsMax},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			deps := scheduleDeps(fixture)
			if testCase.mutate != nil {
				deps = testCase.mutate(deps)
			}
			if err := vm.SetSnapshotSchedule(context.Background(), deps, testCase.frequency, testCase.retain); !errors.Is(err, testCase.wantErr) {
				t.Fatalf("SetSnapshotSchedule = %v, want %v", err, testCase.wantErr)
			}
		})
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestClearSnapshotSchedule_RemovesRowAndAudits(t *testing.T) {
	fixture := newCreateFixture(t)
	deps := scheduleDeps(fixture)

	if err := vm.SetSnapshotSchedule(context.Background(), deps, vm.ScheduleWeekly, 2); err != nil {
		t.Fatalf("SetSnapshotSchedule: %v", err)
	}
	if err := vm.ClearSnapshotSchedule(context.Background(), deps); err != nil {
		t.Fatalf("ClearSnapshotSchedule: %v", err)
	}

	if _, found, err := fixture.store.GetSnapshotSchedule(context.Background(), testClusterName, 100); err != nil || found {
		t.Fatalf("schedule after clear found=%v err=%v, want gone", found, err)
	}

	want := []string{vm.AuditActionScheduleSet, vm.AuditActionScheduleClear}
	if actions := scheduleAuditActions(t, fixture); len(actions) != 2 || actions[0] != want[0] || actions[1] != want[1] {
		t.Fatalf("audit = %v, want %v", actions, want)
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestClearSnapshotSchedule_TargetMustResolve(t *testing.T) {
	fixture := newCreateFixture(t)
	deps := scheduleDeps(fixture)
	deps.Index = nil

	if err := vm.ClearSnapshotSchedule(context.Background(), deps); !errors.Is(err, vm.ErrNotFound) {
		t.Fatalf("ClearSnapshotSchedule = %v, want %v", err, vm.ErrNotFound)
	}
}
