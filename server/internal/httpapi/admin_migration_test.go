//nolint:noctx,paralleltest // test scaffolding uses in-memory requests; shared default fake cluster forces serial tests
package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"slices"
	"strings"
	"testing"
	"time"
)

const (
	migNode01         = cluster.FakeNode01
	migNode02         = cluster.FakeNode02
	migNode03         = cluster.FakeNode03
	migRunningVM      = 100
	migStoppedVM      = 101
	migUnmanagedVM    = 109
	migWestCluster    = "west"
	migTaskPolls      = 6
	codeInvalidTarget = "invalid_target"
)

type migrationPreflightResponse struct {
	Cluster    string   `json:"cluster"`
	VMID       int      `json:"vmid"`
	Node       string   `json:"node"`
	Running    bool     `json:"running"`
	Lock       string   `json:"lock"`
	Blocked    bool     `json:"blocked"`
	Blockers   []string `json:"blockers"`
	LocalDisks []string `json:"localDisks"`
	Candidates []struct {
		Node     string   `json:"node"`
		Warnings []string `json:"warnings"`
	} `json:"candidates"`
	Excluded []struct {
		Node   string `json:"node"`
		Reason string `json:"reason"`
		Detail string `json:"detail"`
	} `json:"excluded"`
}

type migrationStartResponse struct {
	Cluster string `json:"cluster"`
	VMID    int    `json:"vmid"`
	UPID    string `json:"upid"`
	Source  string `json:"source"`
	Target  string `json:"target"`
}

type migrationTaskResponse struct {
	State       string `json:"state"`
	ExitMessage string `json:"exitMessage"`
}

type migrationFixture struct {
	mux        http.Handler
	cookie     *http.Cookie
	bob        *http.Cookie
	st         *store.Store
	projection *inventory.Projection
}

func newMigrationFixture(t *testing.T) *migrationFixture {
	t.Helper()
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	projection := buildProjectionWithIndex(t, snap, time.Now())
	authHandler := newAuthHandler(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	st := newAdminStore(t)
	worker := inventory.NewWorker(cluster.Fake{}, projection, time.Hour, logger)
	migration := httpapi.NewAdminMigrationWithRegistry(httpapi.AdminMigrationRegistryDeps{
		Projection: projection, Auth: authHandler,
		Migrator: cluster.Fake{}, Status: cluster.Fake{}, Store: st, Log: logger,
	})
	tasks := httpapi.NewTasks(authHandler, cluster.Fake{}, worker, logger)

	return &migrationFixture{
		mux:        migrationMux(authHandler, migration, tasks),
		cookie:     adminCookie(t, authHandler),
		bob:        bobCookie(t, authHandler),
		st:         st,
		projection: projection,
	}
}

func migrationMux(authHandler *httpapi.Auth, migration *httpapi.AdminMigration, tasks *httpapi.Tasks) *http.ServeMux {
	mux := http.NewServeMux()
	mux.Handle("GET /api/v1/admin/vms/{cluster}/{vmid}/migrate", authHandler.RequireAdmin(http.HandlerFunc(migration.ServePreflight)))
	mux.Handle("POST /api/v1/admin/vms/{cluster}/{vmid}/migrate", authHandler.RequireAdmin(http.HandlerFunc(migration.ServeStart)))
	mux.Handle("GET /api/v1/tasks/{upid}", tasks)

	return mux
}

func migrationDo(mux http.Handler, cookie *http.Cookie, method, path, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	return rec
}

func migrationPath(clusterName string, vmid int) string {
	return fmt.Sprintf("/api/v1/admin/vms/%s/%d/migrate", clusterName, vmid)
}

func (f *migrationFixture) preflight(t *testing.T, vmid int) migrationPreflightResponse {
	t.Helper()

	rec := migrationDo(f.mux, f.cookie, http.MethodGet, migrationPath(auditTestCluster, vmid), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("preflight status = %d, want 200; body=%s", rec.Code, rec.Body.String())
	}

	var out migrationPreflightResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode preflight: %v", err)
	}

	return out
}

func (f *migrationFixture) start(vmid int, body string) *httptest.ResponseRecorder {
	return migrationDo(f.mux, f.cookie, http.MethodPost, migrationPath(auditTestCluster, vmid), body)
}

func (f *migrationFixture) approve(t *testing.T, node string) {
	t.Helper()

	if err := f.st.SetNodeEnabled(context.Background(), auditTestCluster, node, true); err != nil {
		t.Fatalf("SetNodeEnabled: %v", err)
	}
}

func (f *migrationFixture) migrateAudit(t *testing.T) []store.AuditEntry {
	t.Helper()

	entries, err := f.st.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	var out []store.AuditEntry

	for _, entry := range entries {
		if entry.Action == "vm.migrate" {
			out = append(out, entry)
		}
	}

	return out
}

func pollMigrationTask(t *testing.T, mux http.Handler, cookie *http.Cookie, upid, clusterName string) migrationTaskResponse {
	t.Helper()

	var task migrationTaskResponse

	for range migTaskPolls {
		rec := migrationDo(mux, cookie, http.MethodGet, "/api/v1/tasks/"+upid+"?cluster="+clusterName, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("task status = %d; body=%s", rec.Code, rec.Body.String())
		}

		if err := json.Unmarshal(rec.Body.Bytes(), &task); err != nil {
			t.Fatalf("decode task: %v", err)
		}

		if task.State != string(cluster.TaskRunning) {
			return task
		}
	}

	t.Fatalf("task %s still running after %d polls", upid, migTaskPolls)

	return task
}

func migrationNodes(response migrationPreflightResponse) []string {
	nodes := make([]string, 0, len(response.Candidates))
	for _, candidate := range response.Candidates {
		nodes = append(nodes, candidate.Node)
	}

	return nodes
}

type migrationPreflightCase struct {
	name  string
	vmid  int
	setup func(t *testing.T, f *migrationFixture)
	check func(t *testing.T, got migrationPreflightResponse)
}

//nolint:funlen,gocyclo // scenario table
func migrationPreflightCases() []migrationPreflightCase {
	return []migrationPreflightCase{
		{
			name: "candidates are approved online nodes minus current",
			vmid: migRunningVM,
			check: func(t *testing.T, got migrationPreflightResponse) {
				t.Helper()

				if !slices.Equal(migrationNodes(got), []string{migNode02}) || got.Node != migNode01 || !got.Running || got.Blocked {
					t.Fatalf("got %+v, want only %s as candidate on running VM at %s", got, migNode02, migNode01)
				}

				if len(got.Excluded) != 0 || len(got.Blockers) != 0 || len(got.LocalDisks) != 0 {
					t.Fatalf("unapproved node must not appear: %+v", got)
				}
			},
		},
		{
			name: "approved offline node is excluded as offline",
			vmid: migRunningVM,
			setup: func(t *testing.T, f *migrationFixture) {
				t.Helper()
				f.approve(t, migNode03)
			},
			check: func(t *testing.T, got migrationPreflightResponse) {
				t.Helper()

				if len(got.Excluded) != 1 || got.Excluded[0].Node != migNode03 || got.Excluded[0].Reason != "offline" {
					t.Fatalf("excluded = %+v, want %s offline", got.Excluded, migNode03)
				}
			},
		},
		{
			name: "node refused by proxmox is excluded with its reason",
			vmid: migRunningVM,
			setup: func(_ *testing.T, _ *migrationFixture) {
				cluster.SetFakeMigrationPrecheck(migRunningVM, &cluster.MigrationPrecheck{
					NotAllowed: map[string]string{migNode02: "unavailable storages: local-lvm"},
				})
			},
			check: func(t *testing.T, got migrationPreflightResponse) {
				t.Helper()

				if len(got.Candidates) != 0 || len(got.Excluded) != 1 || got.Excluded[0].Reason != "not_allowed" || got.Excluded[0].Detail != "unavailable storages: local-lvm" {
					t.Fatalf("got %+v, want %s not_allowed with detail", got, migNode02)
				}
			},
		},
		{
			name: "local disks are reported",
			vmid: migStoppedVM,
			check: func(t *testing.T, got migrationPreflightResponse) {
				t.Helper()

				want := []string{"local-lvm:vm-101-disk-0", "local-lvm:vm-101-disk-1"}
				if !slices.Equal(got.LocalDisks, want) || got.Running {
					t.Fatalf("localDisks = %v running = %v, want %v on a stopped VM", got.LocalDisks, got.Running, want)
				}
			},
		},
		{
			name:  "locked VM is blocked with the lock name",
			vmid:  migRunningVM,
			setup: func(_ *testing.T, _ *migrationFixture) { (cluster.Fake{}).SetVMLock(migRunningVM, "backup") },
			check: func(t *testing.T, got migrationPreflightResponse) {
				t.Helper()

				if !got.Blocked || got.Lock != "backup" {
					t.Fatalf("got blocked=%v lock=%q, want blocked by backup", got.Blocked, got.Lock)
				}
			},
		},
		{
			name: "local resources block a running VM",
			vmid: migRunningVM,
			setup: func(_ *testing.T, _ *migrationFixture) {
				cluster.SetFakeMigrationPrecheck(migRunningVM, &cluster.MigrationPrecheck{
					AllowedNodes: []string{migNode02}, LocalResources: []string{"hostpci0", "usb0"},
				})
			},
			check: func(t *testing.T, got migrationPreflightResponse) {
				t.Helper()

				want := "local resources prevent live migration: hostpci0, usb0"
				if !got.Blocked || len(got.Blockers) != 1 || got.Blockers[0] != want {
					t.Fatalf("blockers = %v blocked = %v, want %q", got.Blockers, got.Blocked, want)
				}
			},
		},
		{
			name: "local resources do not block a stopped VM",
			vmid: migStoppedVM,
			setup: func(_ *testing.T, _ *migrationFixture) {
				cluster.SetFakeMigrationPrecheck(migStoppedVM, &cluster.MigrationPrecheck{
					AllowedNodes: []string{migNode02}, LocalResources: []string{"hostpci0"},
				})
			},
			check: func(t *testing.T, got migrationPreflightResponse) {
				t.Helper()

				if got.Blocked || len(got.Blockers) != 0 {
					t.Fatalf("stopped VM must not be blocked: %+v", got)
				}
			},
		},
		{
			name: "capacity overflow warns but never blocks",
			vmid: migRunningVM,
			setup: func(t *testing.T, f *migrationFixture) {
				t.Helper()

				row := store.NodePolicyRow{Cluster: auditTestCluster, Node: migNode02, MaxRAMGB: 1}
				if err := f.st.UpsertNodePolicyRow(context.Background(), row); err != nil {
					t.Fatalf("UpsertNodePolicyRow: %v", err)
				}
			},
			check: func(t *testing.T, got migrationPreflightResponse) {
				t.Helper()

				if got.Blocked || len(got.Candidates) != 1 || !slices.Contains(got.Candidates[0].Warnings, "ram") {
					t.Fatalf("got %+v, want unblocked %s candidate warning ram", got, migNode02)
				}
			},
		},
	}
}

func TestMigrationPreflight_Scenarios(t *testing.T) {
	for _, c := range migrationPreflightCases() {
		t.Run(c.name, func(t *testing.T) {
			fixture := newMigrationFixture(t)

			if c.setup != nil {
				c.setup(t, fixture)
			}

			c.check(t, fixture.preflight(t, c.vmid))
		})
	}
}

func TestMigrationPreflight_Rejections(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		cookie func(f *migrationFixture) *http.Cookie
		want   int
	}{
		{"non-admin is forbidden", migrationPath(auditTestCluster, migRunningVM), func(f *migrationFixture) *http.Cookie { return f.bob }, http.StatusForbidden},
		{"unknown VM", migrationPath(auditTestCluster, 9999), func(f *migrationFixture) *http.Cookie { return f.cookie }, http.StatusNotFound},
		{"VM without the pvmss tag", migrationPath(auditTestCluster, migUnmanagedVM), func(f *migrationFixture) *http.Cookie { return f.cookie }, http.StatusNotFound},
		{"invalid vmid", "/api/v1/admin/vms/default/abc/migrate", func(f *migrationFixture) *http.Cookie { return f.cookie }, http.StatusBadRequest},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fixture := newMigrationFixture(t)

			rec := migrationDo(fixture.mux, c.cookie(fixture), http.MethodGet, c.path, "")
			if rec.Code != c.want {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, c.want, rec.Body.String())
			}
		})
	}
}

//nolint:gocyclo // one end-to-end flow with sequential assertions
func TestMigrationStart_MovesVMAndFollowsTask(t *testing.T) {
	cases := []struct {
		name       string
		vmid       int
		wantOnline bool
	}{
		{"running VM migrates online", migRunningVM, true},
		{"stopped VM migrates offline", migStoppedVM, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fixture := newMigrationFixture(t)

			rec := fixture.start(c.vmid, `{"target":"`+migNode02+`"}`)
			if rec.Code != http.StatusAccepted {
				t.Fatalf("status = %d, want 202; body=%s", rec.Code, rec.Body.String())
			}

			var started migrationStartResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &started); err != nil {
				t.Fatalf("decode: %v", err)
			}

			if started.Source != migNode01 || started.Target != migNode02 || started.VMID != c.vmid || started.Cluster != auditTestCluster || started.UPID == "" {
				t.Fatalf("response = %+v", started)
			}

			calls := cluster.FakeCallsFor(c.vmid)
			if len(calls) != 1 || calls[0].Action != "migrate" || calls[0].Node != migNode01 || calls[0].Name != migNode02 || calls[0].Online != c.wantOnline {
				t.Fatalf("fake calls = %+v, want one migrate to %s online=%v", calls, migNode02, c.wantOnline)
			}

			if locked := fixture.preflight(t, c.vmid); locked.Lock != "migrate" || !locked.Blocked {
				t.Fatalf("in-flight VM lock = %q blocked = %v, want migrate lock", locked.Lock, locked.Blocked)
			}

			if task := pollMigrationTask(t, fixture.mux, fixture.cookie, started.UPID, auditTestCluster); task.State != string(cluster.TaskOK) {
				t.Fatalf("task state = %q, want ok", task.State)
			}

			if got := fixture.projection.Load().ByVMID[c.vmid].Node; got != migNode02 {
				t.Fatalf("projection node = %q, want %q", got, migNode02)
			}

			after := fixture.preflight(t, c.vmid)
			if after.Node != migNode02 || after.Lock != "" || !slices.Contains(migrationNodes(after), migNode01) {
				t.Fatalf("after migration: %+v", after)
			}

			audit := fixture.migrateAudit(t)
			if len(audit) != 1 {
				t.Fatalf("audit rows = %d, want 1", len(audit))
			}

			row := audit[0]
			if row.Cluster != auditTestCluster || row.VMID == nil || *row.VMID != c.vmid || !strings.Contains(row.Detail.String, migNode01) || !strings.Contains(row.Detail.String, migNode02) {
				t.Fatalf("audit row = %+v", row)
			}
		})
	}
}

func approveNode03(t *testing.T, f *migrationFixture) {
	t.Helper()
	f.approve(t, migNode03)
}

func TestMigrationStart_Rejections(t *testing.T) {
	cases := []struct {
		name     string
		vmid     int
		body     string
		setup    func(t *testing.T, f *migrationFixture)
		wantCode int
		wantErr  string
	}{
		{"unapproved target", migRunningVM, `{"target":"` + migNode03 + `"}`, nil, http.StatusBadRequest, codeInvalidTarget},
		{"unknown target", migRunningVM, `{"target":"nowhere"}`, nil, http.StatusBadRequest, codeInvalidTarget},
		{"same node", migRunningVM, `{"target":"` + migNode01 + `"}`, nil, http.StatusBadRequest, codeInvalidTarget},
		{"offline node", migRunningVM, `{"target":"` + migNode03 + `"}`, approveNode03, http.StatusBadRequest, codeInvalidTarget},
		{"locked VM", migRunningVM, `{"target":"` + migNode02 + `"}`, func(_ *testing.T, _ *migrationFixture) { (cluster.Fake{}).SetVMLock(migRunningVM, "backup") }, http.StatusConflict, "vm_locked"},
		{
			"live migration blocker", migRunningVM, `{"target":"` + migNode02 + `"}`,
			func(_ *testing.T, _ *migrationFixture) {
				cluster.SetFakeMigrationPrecheck(migRunningVM, &cluster.MigrationPrecheck{AllowedNodes: []string{migNode02}, LocalResources: []string{"hostpci0"}})
			},
			http.StatusConflict, "migration_blocked",
		},
		{"unknown VM", 9999, `{"target":"` + migNode02 + `"}`, nil, http.StatusNotFound, "not_found"},
		{"malformed body", migRunningVM, `{`, nil, http.StatusBadRequest, "invalid_request"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fixture := newMigrationFixture(t)

			if c.setup != nil {
				c.setup(t, fixture)
			}

			rec := fixture.start(c.vmid, c.body)
			if rec.Code != c.wantCode {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, c.wantCode, rec.Body.String())
			}

			var env apiErrorEnvelope

			_ = json.Unmarshal(rec.Body.Bytes(), &env)

			if env.Code != c.wantErr {
				t.Fatalf("code = %q, want %q", env.Code, c.wantErr)
			}

			if calls := cluster.FakeCallsFor(c.vmid); len(calls) != 0 {
				t.Fatalf("fake received %+v, want no dispatch", calls)
			}

			if audit := fixture.migrateAudit(t); len(audit) != 0 {
				t.Fatalf("audit rows = %d, want none", len(audit))
			}
		})
	}
}

func TestMigrationStart_NonAdminForbidden(t *testing.T) {
	fixture := newMigrationFixture(t)

	rec := migrationDo(fixture.mux, fixture.bob, http.MethodPost, migrationPath(auditTestCluster, migRunningVM), `{"target":"`+migNode02+`"}`)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", rec.Code)
	}

	if calls := cluster.FakeCallsFor(migRunningVM); len(calls) != 0 {
		t.Fatalf("fake received %+v, want no dispatch", calls)
	}
}

func TestMigrationStart_FailedTaskLeavesVMOnSource(t *testing.T) {
	fixture := newMigrationFixture(t)

	rec := fixture.start(migRunningVM, `{"target":"`+migNode02+`"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}

	var started migrationStartResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &started); err != nil {
		t.Fatalf("decode: %v", err)
	}

	cluster.SetFakeTaskError("migration aborted: target unreachable")

	task := pollMigrationTask(t, fixture.mux, fixture.cookie, started.UPID, auditTestCluster)
	if task.State != string(cluster.TaskError) || task.ExitMessage != "migration aborted: target unreachable" {
		t.Fatalf("task = %+v, want error with exit message", task)
	}

	after := fixture.preflight(t, migRunningVM)
	if after.Node != migNode01 || after.Lock != "" || after.Blocked {
		t.Fatalf("after failure: node=%q lock=%q blocked=%v, want %s unlocked", after.Node, after.Lock, after.Blocked, migNode01)
	}
}

func TestMigrationStart_DispatchRejectionSurfacesProxmoxMessage(t *testing.T) {
	fixture := newMigrationFixture(t)

	cluster.SetFakeMigrateError(&cluster.RejectionError{Status: http.StatusInternalServerError, Message: "cannot migrate: target storage offline", Method: http.MethodPost, Path: "/migrate"})

	rec := fixture.start(migRunningVM, `{"target":"`+migNode02+`"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", rec.Code, rec.Body.String())
	}

	var env apiErrorEnvelope

	_ = json.Unmarshal(rec.Body.Bytes(), &env)

	if env.Message != "cannot migrate: target storage offline" {
		t.Fatalf("message = %q, want Proxmox message", env.Message)
	}

	if audit := fixture.migrateAudit(t); len(audit) != 0 {
		t.Fatalf("audit rows = %d, want none after rejected dispatch", len(audit))
	}
}

func TestMigrationStart_DispatchNotFoundMapsToClusterError(t *testing.T) {
	fixture := newMigrationFixture(t)

	cluster.SetFakeMigrateError(cluster.ErrNotFound)

	rec := fixture.start(migRunningVM, `{"target":"`+migNode02+`"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502; body=%s", rec.Code, rec.Body.String())
	}
}

func TestMigrationStart_UnexpectedDispatchErrorIs500(t *testing.T) {
	fixture := newMigrationFixture(t)

	cluster.SetFakeMigrateError(errors.New("boom"))

	rec := fixture.start(migRunningVM, `{"target":"`+migNode02+`"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
}

func TestMigration_MultiClusterUsesTheVMsOwnCluster(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	registry, err := cluster.NewRegistry("fake", []store.ClusterRow{{Name: auditTestCluster}, {Name: migWestCluster}})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	indexes := inventory.NewRegistry(registry, time.Hour, logger)

	for _, name := range []string{auditTestCluster, migWestCluster} {
		if _, err := indexes.Refresh(context.Background(), name); err != nil {
			t.Fatalf("Refresh(%s): %v", name, err)
		}
	}

	st := newAdminStore(t)
	if err := st.SetNodeEnabled(context.Background(), migWestCluster, migNode02, true); err != nil {
		t.Fatalf("SetNodeEnabled: %v", err)
	}

	authHandler := newAuthHandler(t)
	migration := httpapi.NewAdminMigrationWithRegistry(httpapi.AdminMigrationRegistryDeps{
		Source: indexes, Auth: authHandler, Migrator: cluster.Fake{}, Status: cluster.Fake{}, Clients: registry, Store: st, Log: logger,
	})
	tasks := httpapi.NewTasksWithRegistry(authHandler, registry, cluster.Fake{}, nil, httpapi.NewRegistryRefresherResolver(indexes), logger)
	mux := migrationMux(authHandler, migration, tasks)
	cookie := adminCookie(t, authHandler)

	rec := migrationDo(mux, cookie, http.MethodPost, migrationPath(migWestCluster, migRunningVM), `{"target":"`+migNode02+`"}`)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}

	var started migrationStartResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &started); err != nil {
		t.Fatalf("decode: %v", err)
	}

	wrong := migrationDo(mux, cookie, http.MethodGet, "/api/v1/tasks/"+started.UPID+"?cluster="+auditTestCluster, "")
	if wrong.Code != http.StatusNotFound {
		t.Fatalf("polling the default cluster status = %d, want 404 for a foreign UPID", wrong.Code)
	}

	if task := pollMigrationTask(t, mux, cookie, started.UPID, migWestCluster); task.State != string(cluster.TaskOK) {
		t.Fatalf("task state = %q, want ok", task.State)
	}

	west, _ := indexes.Index(migWestCluster)
	def, _ := indexes.Index(auditTestCluster)

	if west.ByVMID[migRunningVM].Node != migNode02 || def.ByVMID[migRunningVM].Node != migNode01 {
		t.Fatalf("west node = %q default node = %q, want only west moved", west.ByVMID[migRunningVM].Node, def.ByVMID[migRunningVM].Node)
	}

	unknown := migrationDo(mux, cookie, http.MethodGet, migrationPath("nowhere", migRunningVM), "")
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown cluster status = %d, want 404", unknown.Code)
	}
}
