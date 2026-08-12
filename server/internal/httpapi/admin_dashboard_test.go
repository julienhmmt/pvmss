package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"testing"
)

type dashboardDTO struct {
	Nodes             []nodeSummaryDTO    `json:"nodes"`
	NodeCount         int                 `json:"nodeCount"`
	VMCount           int                 `json:"vmCount"`
	Storages          []storageSummaryDTO `json:"storages"`
	StorageTotalBytes int64               `json:"storageTotalBytes"`
	StorageUsedBytes  int64               `json:"storageUsedBytes"`
	Version           string              `json:"version"`
	RefreshedAt       string              `json:"refreshedAt"`
}

type nodeSummaryDTO struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

type storageSummaryDTO struct {
	Name       string `json:"name"`
	Node       string `json:"node"`
	Type       string `json:"type"`
	TotalBytes int64  `json:"totalBytes"`
	UsedBytes  int64  `json:"usedBytes"`
}

// TestAdminDashboard_AsAdmin_ReturnsCountsAndStorages — T023: GET
// /admin/dashboard as admin returns correct node list/count, VM count,
// storage list/aggregate, and version.
//
//nolint:paralleltest // serial: shared fake dataset
func TestAdminDashboard_AsAdmin_ReturnsCountsAndStorages(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	rec := opsGet(t, ops, auth, cookie, "/api/v1/admin/dashboard")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var dash dashboardDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dash); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if dash.NodeCount != 3 {
		t.Errorf("nodeCount = %d, want 3", dash.NodeCount)
	}

	if len(dash.Nodes) != 3 {
		t.Errorf("nodes = %d, want 3", len(dash.Nodes))
	}

	// VM count must match len(Index.ByVMID) computed independently.
	fake := cluster.Fake{}
	snap, _ := fake.Snapshot(context.Background())

	idx := inventory.BuildIndex(snap)
	if dash.VMCount != len(idx.ByVMID) {
		t.Errorf("vmCount = %d, want %d (len Index.ByVMID)", dash.VMCount, len(idx.ByVMID))
	}

	if len(dash.Storages) == 0 {
		t.Error("storages is empty — Index.StoragesByNode not read")
	}

	if dash.Version == "" {
		t.Error("version is empty")
	}
}

// TestAdminDashboard_AsNonAdmin_Returns403 — T024: GET /admin/dashboard as
// non-admin returns 403.
//
//nolint:paralleltest // serial: shared fake dataset
func TestAdminDashboard_AsNonAdmin_Returns403(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	aliceCookie := loginCookie(t, auth, `{"username":"alice","password":"pvmss-alice"}`)

	rec := opsGet(t, ops, auth, aliceCookie, "/api/v1/admin/dashboard")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestAdminDashboard_Sc003_NoClusterClientCallForVMCount — T025/SC-003: a
// dashboard read makes zero cluster.Client calls — VM count comes from
// len(Index.ByVMID) and storage occupancy from Index.StoragesByNode, both
// in-memory. Uses a call-counting fake cluster.Client.
//
//nolint:paralleltest // serial: shared fake dataset
func TestAdminDashboard_Sc003_NoClusterClientCallForVMCount(t *testing.T) {
	authHandler := newAuthHandler(t)
	st := auditAdminStore(t)

	// Use a call-counting fake client. The dashboard should never call it.
	countingClient := &callCountingClient{}

	fake := cluster.Fake{}
	snap, _ := fake.Snapshot(context.Background())
	idx := inventory.BuildIndex(snap)
	projection := inventory.NewProjectionFromIndex(&idx)
	ops := httpapi.NewAdminOps(authHandler, st, countingClient, projection, "0.4.0-test", slog.New(slog.DiscardHandler))
	cookie := adminCookie(t, authHandler)

	rec := opsGet(t, ops, authHandler, cookie, "/api/v1/admin/dashboard")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var dash dashboardDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dash); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// SC-003: zero cluster.Client calls — the dashboard reads entirely from
	// the in-memory Index.
	if countingClient.snapshotCalls != 0 {
		t.Errorf("cluster.Client.Snapshot called %d times, want 0", countingClient.snapshotCalls)
	}

	// VM count equals len(Index.ByVMID), not derived from any cluster.Client
	// call that returns a full VM list.
	if dash.VMCount != len(idx.ByVMID) {
		t.Errorf("vmCount = %d, want %d", dash.VMCount, len(idx.ByVMID))
	}
}

// callCountingClient wraps a cluster.Fake and counts Snapshot calls. All
// other interface methods are delegated to the embedded Fake.
type callCountingClient struct {
	cluster.Fake
	snapshotCalls int
}

func (c *callCountingClient) Snapshot(ctx context.Context) (cluster.Snapshot, error) {
	c.snapshotCalls++
	return c.Fake.Snapshot(ctx)
}
