//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"testing"
	"time"
)

type stubClusterClient struct {
	snapshot cluster.Snapshot
	err      error
	calls    int
}

func (s *stubClusterClient) Snapshot(_ context.Context) (cluster.Snapshot, error) {
	s.calls++
	return s.snapshot, s.err
}

func (stubClusterClient) Authenticate(_ context.Context, _, _ string) (cluster.Identity, error) {
	return cluster.Identity{}, cluster.ErrNotImplemented
}

func (stubClusterClient) ChangePassword(_ context.Context, _, _, _ string) error {
	return cluster.ErrNotImplemented
}

func (stubClusterClient) ListBridges(_ context.Context) ([]cluster.Bridge, error) {
	return nil, cluster.ErrNotImplemented
}

func (stubClusterClient) ListISOs(_ context.Context) ([]cluster.ISOImage, error) {
	return nil, cluster.ErrNotImplemented
}

func (stubClusterClient) ListTemplates(_ context.Context) ([]cluster.TemplateVM, error) {
	return nil, cluster.ErrNotImplemented
}

func (stubClusterClient) ListPools(_ context.Context) ([]cluster.Pool, error) {
	return nil, cluster.ErrNotImplemented
}

func (stubClusterClient) EnsurePoolRole(_ context.Context) error {
	return cluster.ErrNotImplemented
}

func (stubClusterClient) EnsurePoolUser(_ context.Context, _, _ string) (string, error) {
	return "", cluster.ErrNotImplemented
}

func (stubClusterClient) CreatePool(_ context.Context, _, _ string) error {
	return cluster.ErrNotImplemented
}

func (stubClusterClient) SetPoolACL(_ context.Context, _, _, _ string) error {
	return cluster.ErrNotImplemented
}

func (stubClusterClient) DeletePool(_ context.Context, _ string) error {
	return cluster.ErrNotImplemented
}

func (stubClusterClient) DeleteUser(_ context.Context, _ string) error {
	return cluster.ErrNotImplemented
}

func (stubClusterClient) StorageFreeSpace(_ context.Context, _, _ string) (int64, error) {
	return 0, cluster.ErrNotImplemented
}

func (stubClusterClient) DisplayName(_ context.Context) (string, error) {
	return "", cluster.ErrNotImplemented
}

// buildProjectionWithIndex creates a Projection and populates it with an Index
// built from the given snapshot, stamped at the given refresh time.
func buildProjectionWithIndex(t *testing.T, snap cluster.Snapshot, refreshedAt time.Time) *inventory.Projection {
	t.Helper()

	idx := inventory.BuildIndex(snap)
	idx.RefreshedAt = refreshedAt

	return inventory.NewProjectionFromIndex(&idx)
}

// TestClusterNodes_Success — GET /cluster/nodes reads from the Index, includes
// vmCount and refreshedAt (contracts/cluster-refresh.md, FR-008).
//
//nolint:paralleltest // serial: shared fake cluster fixture
func TestClusterNodes_Success(t *testing.T) {
	snap := cluster.Snapshot{
		Nodes: []cluster.Node{
			{
				Name:         cluster.FakeNode01,
				Status:       cluster.NodeOnline,
				CPUCores:     32,
				CPUUsage:     0.42,
				MemoryTotal:  137438953472,
				MemoryUsed:   68719476736,
				StorageTotal: 2199023255552,
				StorageUsed:  879609302220,
			},
		},
		VMs: []cluster.VM{
			{VMID: 100, Name: "web-01", Node: cluster.FakeNode01, Pool: cluster.FakePoolAlice},
			{VMID: 101, Name: "web-02", Node: cluster.FakeNode01, Pool: cluster.FakePoolAlice},
		},
	}
	refreshedAt := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	projection := buildProjectionWithIndex(t, snap, refreshedAt)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	h := httpapi.NewClusterNodes(projection, logger)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/nodes", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var got struct {
		Nodes []struct {
			Name         string  `json:"name"`
			Status       string  `json:"status"`
			CPUCores     int     `json:"cpuCores"`
			CPUUsage     float64 `json:"cpuUsage"`
			MemoryTotal  int64   `json:"memoryTotal"`
			MemoryUsed   int64   `json:"memoryUsed"`
			StorageTotal int64   `json:"storageTotal"`
			StorageUsed  int64   `json:"storageUsed"`
			VMCount      int     `json:"vmCount"`
		} `json:"nodes"`
		RefreshedAt string `json:"refreshedAt"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if len(got.Nodes) != 1 {
		t.Fatalf("nodes count = %d, want 1", len(got.Nodes))
	}

	n := got.Nodes[0]
	if n.Name != testNodePVE01 || n.Status != "online" || n.CPUCores != 32 ||
		n.MemoryTotal != 137438953472 || n.MemoryUsed != 68719476736 ||
		n.StorageTotal != 2199023255552 || n.StorageUsed != 879609302220 {
		t.Fatalf("unexpected node shape: %+v", n)
	}

	if n.VMCount != 2 {
		t.Fatalf("vmCount = %d, want 2", n.VMCount)
	}

	if got.RefreshedAt != "2026-08-01T12:00:00Z" {
		t.Fatalf("refreshedAt = %q, want 2026-08-01T12:00:00Z", got.RefreshedAt)
	}
}

// TestClusterNodes_NotReady — before the first refresh, GET /cluster/nodes
// returns 503 inventory_not_ready, distinct from an empty list (FR-009).
//
//nolint:paralleltest // serial: shared fake cluster fixture
func TestClusterNodes_NotReady(t *testing.T) {
	projection := inventory.NewProjection()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	h := httpapi.NewClusterNodes(projection, logger)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/nodes", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusServiceUnavailable)
	}

	var got struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Code != "inventory_not_ready" {
		t.Fatalf("code = %q, want inventory_not_ready", got.Code)
	}
}

// TestClusterNodes_EmptyIsOK — an empty cluster (0 nodes) with a valid
// RefreshedAt returns 200 with an empty array, not 503.
//
//nolint:paralleltest // serial: shared fake cluster fixture
func TestClusterNodes_EmptyIsOK(t *testing.T) {
	projection := buildProjectionWithIndex(t, cluster.Snapshot{}, time.Now())
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	h := httpapi.NewClusterNodes(projection, logger)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/nodes", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusOK)
	}

	var got struct {
		Nodes       []json.RawMessage `json:"nodes"`
		RefreshedAt string            `json:"refreshedAt"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Nodes == nil || len(got.Nodes) != 0 {
		t.Fatalf("nodes = %v, want empty array not null", got.Nodes)
	}

	if got.RefreshedAt == "" {
		t.Fatal("refreshedAt should not be empty for a populated projection")
	}
}

// TestClusterNodes_MethodNotAllowed — non-GET returns 405.
//
//nolint:paralleltest // serial: shared fake cluster fixture
func TestClusterNodes_MethodNotAllowed(t *testing.T) {
	projection := inventory.NewProjection()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	h := httpapi.NewClusterNodes(projection, logger)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/nodes", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}

	if w.Header().Get("Allow") != "GET" {
		t.Fatalf("Allow header = %q, want GET", w.Header().Get("Allow"))
	}
}
