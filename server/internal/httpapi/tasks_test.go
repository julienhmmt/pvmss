package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/vm"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Test fixtures for the create specs used across the tasks tests (goconst).
const (
	testStorageLocalLVM = "local-lvm"
	testNICModelVirtio  = "virtio"
)

func getTask(t *testing.T, handler *httpapi.Tasks, upid string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/tasks/"+upid, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	// Path values are populated by the ServeMux; tests call the handler
	// directly, so set it manually (same convention as vm_detail_test.go).
	req.SetPathValue("upid", upid)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	return recorder
}

// TestTasks_PollTransitions — T012/SC-006: the fake's poll-count state machine
// surfaces as running, then ok across three GET calls.
//
//nolint:paralleltest // serial: shared fake task fixture
func TestTasks_PollTransitions(t *testing.T) {
	handler, authHandler, projection := newTasksHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	ctx := context.Background()

	vmid, err := (cluster.Fake{}).NextVMID(ctx)
	if err != nil {
		t.Fatalf("NextVMID: %v", err)
	}

	upid, err := (cluster.Fake{}).CreateVM(ctx, cluster.VMSpec{
		VMID:     vmid,
		Node:     cluster.FakeNode01,
		Name:     "polled-vm",
		Pool:     cluster.FakePoolAlice,
		Tags:     []string{cluster.FakeTagPvmss},
		CPUCores: 1,
		MemoryMB: 2048,
		Disk:     cluster.DiskSpec{Storage: testStorageLocalLVM, SizeGB: 20},
		Network:  cluster.NetworkSpec{{Bridge: testBridgeVmbr0, Model: testNICModelVirtio}},
	})
	if err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	for i, want := range []string{testStatusRunning, testStatusRunning, "ok"} {
		response := getTask(t, handler, upid, cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("call %d: status = %d, want 200: %s", i+1, response.Code, response.Body.String())
		}

		var body struct {
			UPID  string   `json:"upid"`
			State string   `json:"state"`
			Log   []string `json:"log"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("call %d: decode: %v", i+1, err)
		}

		if body.State != want {
			t.Fatalf("call %d: state = %q, want %q", i+1, body.State, want)
		}

		if body.UPID != upid {
			t.Errorf("call %d: upid = %q, want %q", i+1, body.UPID, upid)
		}
	}

	// FR-018: the ok observation invalidated the index — the refreshed
	// projection now contains the created VM.
	index := projection.Load()
	if index == nil {
		t.Fatalf("projection not populated after task completion")
	}

	created, ok := index.ByVMID[vmid]
	if !ok {
		t.Fatalf("created VM %d absent from projection after task ok", vmid)
	}

	if created.Name != "polled-vm" {
		t.Errorf("projection VM name = %q, want polled-vm", created.Name)
	}
}

// TestTasks_UnknownUPID — T012: an unknown upid is a 404.
//
//nolint:paralleltest // serial: shared fake task fixture
func TestTasks_UnknownUPID(t *testing.T) {
	handler, authHandler, _ := newTasksHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	response := getTask(t, handler, "UPID:pve-node-01:00000000:00000000:00000000:qmcreate:999:nobody@pve:", cookie)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusNotFound, response.Body.String())
	}

	assertAPIError(t, response.Body.Bytes(), "not_found")
}

// TestTasks_RequiresAuth — the endpoint is authenticated (T02), like every
// /api/v1 route.
//
//nolint:paralleltest // serial: shared fake task fixture
func TestTasks_RequiresAuth(t *testing.T) {
	handler, _, _ := newTasksHandler(t)

	response := getTask(t, handler, "UPID:x", nil)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}

//nolint:paralleltest // serial: shared fake task and projection fixtures
func TestTasks_RollbackInvalidatesIndex(t *testing.T) {
	handler, authHandler, projection := newTasksHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)
	ctx := context.Background()

	createUPID, err := (cluster.Fake{}).CreateSnapshot(ctx, cluster.FakeNode01, 101, "restore-point", "", false)
	if err != nil {
		t.Fatalf("CreateSnapshot: %v", err)
	}

	for range 3 {
		if _, err := (cluster.Fake{}).TaskStatus(ctx, createUPID); err != nil {
			t.Fatalf("complete snapshot: %v", err)
		}
	}

	rollbackUPID, err := (cluster.Fake{}).RollbackSnapshot(ctx, cluster.FakeNode01, 101, "restore-point")
	if err != nil {
		t.Fatalf("RollbackSnapshot: %v", err)
	}

	before := projection.Load()

	for range 3 {
		response := getTask(t, handler, rollbackUPID, cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("rollback poll status = %d: %s", response.Code, response.Body.String())
		}
	}

	if projection.Load() == before {
		t.Fatal("rollback completion did not replace the inventory projection")
	}
}

// newTasksHandlerWithRegistry builds the task-status handler with a real worker
// and a multi-cluster ClientProvider, so the per-request ?cluster= resolution
// path is exercised (the cross-cluster fix). Each named cluster maps to its
// own Fake instance so a task created on one cluster is invisible to another.
func newTasksHandlerWithRegistry(t *testing.T, clients map[string]cluster.Client, refreshers httpapi.ClusterRefresherResolver) (*httpapi.Tasks, *httpapi.Auth, *inventory.Projection) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)
	authHandler := newAuthHandler(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	projection := buildProjectionWithIndex(t, snap, time.Now())
	worker := inventory.NewWorker(cluster.Fake{}, projection, time.Hour, logger)

	provider := vmCreateClientProvider{clients: clients}

	return httpapi.NewTasksWithRegistry(authHandler, provider, cluster.Fake{}, worker, refreshers, logger), authHandler, projection
}

// getTaskWithCluster issues GET /api/v1/tasks/{upid}?cluster={cluster} against
// the handler directly, mirroring how the frontend polls after the
// cross-cluster fix.
func getTaskWithCluster(t *testing.T, handler *httpapi.Tasks, upid, clusterName string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	target := "/api/v1/tasks/" + upid + "?cluster=" + clusterName
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, target, nil)

	if cookie != nil {
		req.AddCookie(cookie)
	}

	req.SetPathValue("upid", upid)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	return recorder
}

// TestTasks_WithRegistry_PollsNamedCluster — the registry-backed handler
// resolves the ?cluster= param to that cluster's own Creator and polls the
// task against it. A task created on the "secondary" cluster is observed
// running→ok only when the poll names "secondary"; polling it through the
// default cluster's client (the pre-fix behaviour) would 404.
//
//nolint:paralleltest // serial: shared fake task fixture
func TestTasks_WithRegistry_PollsNamedCluster(t *testing.T) {
	secondary := cluster.Fake{}
	handler, authHandler, _ := newTasksHandlerWithRegistry(t, map[string]cluster.Client{
		auditTestCluster:      cluster.Fake{},
		crossSecondaryCluster: secondary,
	}, nil)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	ctx := context.Background()

	vmid, err := secondary.NextVMID(ctx)
	if err != nil {
		t.Fatalf("NextVMID: %v", err)
	}

	upid, err := secondary.CreateVM(ctx, cluster.VMSpec{
		VMID:     vmid,
		Node:     cluster.FakeNode01,
		Name:     "secondary-vm",
		Pool:     cluster.FakePoolAlice,
		Tags:     []string{cluster.FakeTagPvmss},
		CPUCores: 1,
		MemoryMB: 2048,
		Disk:     cluster.DiskSpec{Storage: testStorageLocalLVM, SizeGB: 20},
		Network:  cluster.NetworkSpec{{Bridge: testBridgeVmbr0, Model: testNICModelVirtio}},
	})
	if err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	for i, want := range []string{testStatusRunning, testStatusRunning, "ok"} {
		response := getTaskWithCluster(t, handler, upid, crossSecondaryCluster, cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("call %d: status = %d, want 200: %s", i+1, response.Code, response.Body.String())
		}

		var body struct {
			UPID  string `json:"upid"`
			State string `json:"state"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("call %d: decode: %v", i+1, err)
		}

		if body.State != want {
			t.Fatalf("call %d: state = %q, want %q", i+1, body.State, want)
		}

		if body.UPID != upid {
			t.Errorf("call %d: upid = %q, want %q", i+1, body.UPID, upid)
		}
	}
}

// TestTasks_WithRegistry_UnknownClusterReturns404 — a ?cluster= naming a
// cluster the registry does not know is a 404 cluster_not_found, not a poll
// against the default cluster's client (which would silently mislead).
//
//nolint:paralleltest // serial: shared fake task fixture
func TestTasks_WithRegistry_UnknownClusterReturns404(t *testing.T) {
	handler, authHandler, _ := newTasksHandlerWithRegistry(t, map[string]cluster.Client{
		auditTestCluster: cluster.Fake{},
	}, nil)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	response := getTaskWithCluster(t, handler, "UPID:anything", "ghost", cookie)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusNotFound, response.Body.String())
	}

	assertAPIError(t, response.Body.Bytes(), "cluster_not_found")
}

// TestTasks_WithRegistry_DefaultFallback — omitting ?cluster= against a
// single-cluster registry resolves to that one cluster (backwards compatible
// with the existing single-cluster e2e poll that sends no query param).
//
//nolint:paralleltest // serial: shared fake task fixture
func TestTasks_WithRegistry_DefaultFallback(t *testing.T) {
	handler, authHandler, _ := newTasksHandlerWithRegistry(t, map[string]cluster.Client{
		auditTestCluster: cluster.Fake{},
	}, nil)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	ctx := context.Background()

	vmid, err := (cluster.Fake{}).NextVMID(ctx)
	if err != nil {
		t.Fatalf("NextVMID: %v", err)
	}

	upid, err := (cluster.Fake{}).CreateVM(ctx, cluster.VMSpec{
		VMID: vmid, Node: cluster.FakeNode01, Name: "fallback-vm", Pool: cluster.FakePoolAlice,
		Tags: []string{cluster.FakeTagPvmss}, CPUCores: 1, MemoryMB: 2048,
		Disk:    cluster.DiskSpec{Storage: testStorageLocalLVM, SizeGB: 20},
		Network: cluster.NetworkSpec{{Bridge: testBridgeVmbr0, Model: testNICModelVirtio}},
	})
	if err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	// No ?cluster= query param — single-cluster registry resolves to "default".
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/tasks/"+upid, nil)
	req.AddCookie(cookie)

	req.SetPathValue("upid", upid)

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", recorder.Code, recorder.Body.String())
	}

	var body struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.State != testStatusRunning {
		t.Fatalf("state = %q, want running (first poll)", body.State)
	}
}

// recordedInvalidator is a TaskInvalidator test double that counts refreshes.
type recordedInvalidator struct {
	label     string
	refreshed atomic.Int32
}

func (r *recordedInvalidator) Refresh(context.Context) (time.Time, error) {
	r.refreshed.Add(1)

	return time.Now(), nil
}

// taskRefresherRecorder is a ClusterRefresherResolver test double that records
// every cluster it was asked to resolve.
type taskRefresherRecorder struct {
	mu        sync.Mutex
	asked     []string
	byCluster map[string]*recordedInvalidator
}

func (m *taskRefresherRecorder) RefresherFor(clusterName string) (vm.IndexRefresher, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.asked = append(m.asked, clusterName)

	invalidator, ok := m.byCluster[clusterName]
	if !ok {
		return nil, fmt.Errorf("no invalidator for cluster %q", clusterName)
	}

	return invalidator, nil
}

// TestTasks_WithRegistry_InvalidatesNamedClusterProjection — ticket 05: the
// post-task invalidation resolves the ?cluster= param to that cluster's own
// invalidator instead of the startup default. Fails before the fix (the
// default worker was refreshed for every cluster).
//
//nolint:paralleltest // serial: shared fake task fixture
func TestTasks_WithRegistry_InvalidatesNamedClusterProjection(t *testing.T) {
	secondary := cluster.Fake{}
	alphaInvalidator := &recordedInvalidator{label: "alpha"}
	betaInvalidator := &recordedInvalidator{label: "beta"}
	resolver := &taskRefresherRecorder{byCluster: map[string]*recordedInvalidator{
		auditTestCluster:      alphaInvalidator,
		crossSecondaryCluster: betaInvalidator,
	}}

	handler, authHandler, _ := newTasksHandlerWithRegistry(t, map[string]cluster.Client{
		auditTestCluster:      cluster.Fake{},
		crossSecondaryCluster: secondary,
	}, resolver)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	ctx := context.Background()

	vmid, err := secondary.NextVMID(ctx)
	if err != nil {
		t.Fatalf("NextVMID: %v", err)
	}

	upid, err := secondary.CreateVM(ctx, cluster.VMSpec{
		VMID: vmid, Node: cluster.FakeNode01, Name: "secondary-vm", Pool: cluster.FakePoolAlice,
		Tags: []string{cluster.FakeTagPvmss}, CPUCores: 1, MemoryMB: 2048,
		Disk:    cluster.DiskSpec{Storage: testStorageLocalLVM, SizeGB: 20},
		Network: cluster.NetworkSpec{{Bridge: testBridgeVmbr0, Model: testNICModelVirtio}},
	})
	if err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	// Poll through the third call, when the fake task reaches ok and the
	// invalidation fires.
	for range 3 {
		response := getTaskWithCluster(t, handler, upid, crossSecondaryCluster, cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("poll status = %d, want 200: %s", response.Code, response.Body.String())
		}
	}

	if betaInvalidator.refreshed.Load() != 1 {
		t.Errorf("secondary invalidations = %d, want 1", betaInvalidator.refreshed.Load())
	}

	if alphaInvalidator.refreshed.Load() != 0 {
		t.Errorf("default invalidations = %d, want 0", alphaInvalidator.refreshed.Load())
	}
}
