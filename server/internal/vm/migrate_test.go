package vm_test

import (
	"context"
	"database/sql"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strings"
	"testing"
	"time"
)

type stubMigrationStore struct {
	nodes      []store.CatalogNodeEnabled
	nodesErr   error
	policy     store.NodePolicyRow
	policyErr  error
	auditErr   error
	lastAction string
	lastDetail string
}

func (s *stubMigrationStore) CatalogNodesEnabled(_ context.Context, _ string) ([]store.CatalogNodeEnabled, error) {
	if s.nodesErr != nil {
		return nil, s.nodesErr
	}

	return s.nodes, nil
}

func (s *stubMigrationStore) NodePolicyRow(_ context.Context, _, _ string) (store.NodePolicyRow, error) {
	if s.policyErr != nil {
		return store.NodePolicyRow{}, s.policyErr
	}

	return s.policy, nil
}

func (s *stubMigrationStore) RecordVMActionDetail(_ context.Context, _, _ string, _ int, action, detail string) error {
	s.lastAction = action
	s.lastDetail = detail

	return s.auditErr
}

type stubMigrator struct {
	precheck    cluster.MigrationPrecheck
	precheckErr error
	upid        string
	migrateErr  error
	lastSpec    cluster.MigrateSpec
}

func (m *stubMigrator) MigrationPrecheck(_ context.Context, _ string, _ int) (cluster.MigrationPrecheck, error) {
	return m.precheck, m.precheckErr
}

func (m *stubMigrator) Migrate(_ context.Context, _ string, _ int, spec cluster.MigrateSpec) (string, error) {
	m.lastSpec = spec

	return m.upid, m.migrateErr
}

type stubStatusReader struct {
	status cluster.VMLiveStatus
	err    error
}

func (r stubStatusReader) VMStatus(_ context.Context, _ string, _ int) (cluster.VMLiveStatus, error) {
	return r.status, r.err
}

const testDisallowedNode = "pve-node-04"

func newMigrationTestDeps(t *testing.T) (vm.MigrationDependencies, *stubMigrator, *stubMigrationStore) {
	t.Helper()

	fake := cluster.NewFake("test-vm-migrate")

	snap, err := fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	snap.Nodes = append(snap.Nodes, cluster.Node{Name: testDisallowedNode, Status: cluster.NodeOnline})
	idx := inventory.BuildIndex(snap)
	idx.RefreshedAt = time.Now()

	st := &stubMigrationStore{
		nodes: []store.CatalogNodeEnabled{
			{Name: cluster.FakeNode01, Enabled: true},
			{Name: cluster.FakeNode02, Enabled: true},
			{Name: cluster.FakeNode03, Enabled: true},
			{Name: testDisallowedNode, Enabled: true},
		},
		policyErr: sql.ErrNoRows,
	}
	migrator := &stubMigrator{
		precheck: cluster.MigrationPrecheck{
			AllowedNodes: []string{cluster.FakeNode02},
			NotAllowed:   map[string]string{testDisallowedNode: "unavailable storage"},
			LocalDisks:   []string{"local-lvm:vm-100-disk-0"},
		},
		upid: "UPID:pve-node-01:0001:qmigrate:100:admin@pve:",
	}
	deps := vm.MigrationDependencies{
		Index:       &idx,
		Actor:       auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice},
		ClusterName: "default",
		VMID:        100,
		Migrator:    migrator,
		Status:      stubStatusReader{status: cluster.VMLiveStatus{Status: cluster.VMRunning}},
		Store:       st,
	}

	return deps, migrator, st
}

func TestPreflightMigration_CandidatesExclusionsAndBlockers(t *testing.T) {
	t.Parallel()

	deps, migrator, _ := newMigrationTestDeps(t)
	migrator.precheck.LocalResources = []string{"hostpci0"}

	preflight, err := vm.PreflightMigration(context.Background(), deps)
	if err != nil {
		t.Fatalf("PreflightMigration: %v", err)
	}

	if !preflight.Blocked() || len(preflight.Blockers) != 1 {
		t.Fatalf("preflight blockers = %v, want 1 local-resource blocker", preflight.Blockers)
	}

	if len(preflight.Candidates) != 1 || preflight.Candidates[0].Node != cluster.FakeNode02 {
		t.Fatalf("candidates = %+v, want [%s]", preflight.Candidates, cluster.FakeNode02)
	}

	if len(preflight.Excluded) != 2 {
		t.Fatalf("excluded = %+v, want 2 (not_allowed + offline)", preflight.Excluded)
	}
}

func TestStartMigration_SuccessAndAudit(t *testing.T) {
	t.Parallel()

	deps, migrator, st := newMigrationTestDeps(t)

	started, err := vm.StartMigration(context.Background(), deps, cluster.FakeNode02)
	if err != nil {
		t.Fatalf("StartMigration: %v", err)
	}

	if started.Source != cluster.FakeNode01 || started.Target != cluster.FakeNode02 || started.UPID == "" {
		t.Fatalf("started = %+v", started)
	}

	if !migrator.lastSpec.Online || !migrator.lastSpec.WithLocalDisks {
		t.Fatalf("lastSpec = %+v, want online with local disks", migrator.lastSpec)
	}

	if st.lastAction != "vm.migrate" || !strings.Contains(st.lastDetail, cluster.FakeNode02) {
		t.Fatalf("audit = %q %q", st.lastAction, st.lastDetail)
	}
}

func TestStartMigration_Rejections(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		target  string
		mutate  func(*vm.MigrationDependencies, *stubMigrator, *stubMigrationStore)
		wantErr error
	}{
		{
			name:   "locked VM",
			target: cluster.FakeNode02,
			mutate: func(deps *vm.MigrationDependencies, _ *stubMigrator, _ *stubMigrationStore) {
				deps.Status = stubStatusReader{status: cluster.VMLiveStatus{Status: cluster.VMRunning, Lock: "backup"}}
			},
			wantErr: vm.ErrVMLocked,
		},
		{
			name:   "blocked by local resources",
			target: cluster.FakeNode02,
			mutate: func(_ *vm.MigrationDependencies, m *stubMigrator, _ *stubMigrationStore) {
				m.precheck.LocalResources = []string{"hostpci0"}
			},
			wantErr: vm.ErrMigrationBlocked,
		},
		{
			name:    "target is current node",
			target:  cluster.FakeNode01,
			mutate:  func(*vm.MigrationDependencies, *stubMigrator, *stubMigrationStore) {},
			wantErr: vm.ErrInvalidMigrationTarget,
		},
		{
			name:    "target is excluded not_allowed",
			target:  testDisallowedNode,
			mutate:  func(*vm.MigrationDependencies, *stubMigrator, *stubMigrationStore) {},
			wantErr: vm.ErrInvalidMigrationTarget,
		},
		{
			name:    "target is excluded offline",
			target:  cluster.FakeNode03,
			mutate:  func(*vm.MigrationDependencies, *stubMigrator, *stubMigrationStore) {},
			wantErr: vm.ErrInvalidMigrationTarget,
		},
		{
			name:    "target is unknown node",
			target:  "pve-node-99",
			mutate:  func(*vm.MigrationDependencies, *stubMigrator, *stubMigrationStore) {},
			wantErr: vm.ErrInvalidMigrationTarget,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps, migrator, st := newMigrationTestDeps(t)
			tc.mutate(&deps, migrator, st)

			if _, err := vm.StartMigration(context.Background(), deps, tc.target); !errors.Is(err, tc.wantErr) {
				t.Fatalf("StartMigration(%s) error = %v, want %v", tc.name, err, tc.wantErr)
			}
		})
	}
}

func TestMigration_DependencyErrors(t *testing.T) {
	t.Parallel()

	boom := errors.New("dependency failure")

	cases := []struct {
		name   string
		mutate func(*vm.MigrationDependencies, *stubMigrator, *stubMigrationStore)
	}{
		{
			name: "status reader error",
			mutate: func(deps *vm.MigrationDependencies, _ *stubMigrator, _ *stubMigrationStore) {
				deps.Status = stubStatusReader{err: boom}
			},
		},
		{
			name: "precheck error",
			mutate: func(_ *vm.MigrationDependencies, m *stubMigrator, _ *stubMigrationStore) {
				m.precheckErr = boom
			},
		},
		{
			name: "approved nodes error",
			mutate: func(_ *vm.MigrationDependencies, _ *stubMigrator, st *stubMigrationStore) {
				st.nodesErr = boom
			},
		},
		{
			name: "node policy error",
			mutate: func(_ *vm.MigrationDependencies, _ *stubMigrator, st *stubMigrationStore) {
				st.policyErr = boom
			},
		},
		{
			name: "migrate dispatch error",
			mutate: func(_ *vm.MigrationDependencies, m *stubMigrator, _ *stubMigrationStore) {
				m.migrateErr = boom
			},
		},
		{
			name: "audit error",
			mutate: func(_ *vm.MigrationDependencies, _ *stubMigrator, st *stubMigrationStore) {
				st.auditErr = boom
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			deps, migrator, st := newMigrationTestDeps(t)
			tc.mutate(&deps, migrator, st)

			if _, err := vm.StartMigration(context.Background(), deps, cluster.FakeNode02); !errors.Is(err, boom) {
				t.Fatalf("StartMigration(%s) error = %v, want %v", tc.name, err, boom)
			}
		})
	}
}
