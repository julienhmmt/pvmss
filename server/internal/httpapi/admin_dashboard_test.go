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
	Nodes          []nodeSummaryDTO  `json:"nodes"`
	NodeCount      int               `json:"nodeCount"`
	VMCount        int               `json:"vmCount"`
	VMStatusCounts vmStatusCountsDTO `json:"vmStatusCounts"`
	Version        string            `json:"version"`
	RefreshedAt    string            `json:"refreshedAt"`
}

type nodeSummaryDTO struct {
	Name             string  `json:"name"`
	Status           string  `json:"status"`
	VMCount          int     `json:"vmCount"`
	CPUCores         int     `json:"cpuCores"`
	CPUUsage         float64 `json:"cpuUsage"`
	MemoryTotalBytes int64   `json:"memoryTotalBytes"`
	MemoryUsedBytes  int64   `json:"memoryUsedBytes"`
}

type vmStatusCountsDTO struct {
	Running int `json:"running"`
	Paused  int `json:"paused"`
	Stopped int `json:"stopped"`
	Other   int `json:"other"`
}

// countNodesHostingVMs returns the number of nodes in the index that host at
// least one PVMSS-managed VM — the value the dashboard's NodeCount must match.
func countNodesHostingVMs(idx inventory.Index) int {
	count := 0

	for name := range idx.ByNode {
		if len(idx.ByNode[name]) > 0 {
			count++
		}
	}

	return count
}

// assertNodeSummariesValid verifies every returned node hosts at least one VM
// and carries non-zero CPU/RAM data.
func assertNodeSummariesValid(t *testing.T, nodes []nodeSummaryDTO) {
	t.Helper()

	for _, n := range nodes {
		if n.VMCount < 1 {
			t.Errorf("node %q returned with vmCount = %d, want >= 1", n.Name, n.VMCount)
		}

		if n.CPUCores == 0 {
			t.Errorf("node %q cpuCores = 0", n.Name)
		}

		if n.MemoryTotalBytes == 0 {
			t.Errorf("node %q memoryTotalBytes = 0", n.Name)
		}
	}
}

// TestAdminDashboard_AsAdmin_ReturnsPvmssNodesAndVmCounts — GET
// /admin/dashboard as admin returns only nodes hosting PVMSS-managed VMs,
// each with CPU/RAM usage and a VM count; the total VM count and per-status
// counts match the in-memory Index; storage is no longer surfaced.
//
//nolint:paralleltest // serial: shared fake dataset
func TestAdminDashboard_AsAdmin_ReturnsPvmssNodesAndVmCounts(t *testing.T) {
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

	fake := cluster.Fake{}
	snap, _ := fake.Snapshot(context.Background())
	idx := inventory.BuildIndex(snap)

	wantNodeCount := countNodesHostingVMs(idx)
	if dash.NodeCount != wantNodeCount {
		t.Errorf("nodeCount = %d, want %d (nodes hosting PVMSS VMs)", dash.NodeCount, wantNodeCount)
	}

	if len(dash.Nodes) != wantNodeCount {
		t.Errorf("nodes = %d, want %d", len(dash.Nodes), wantNodeCount)
	}

	assertNodeSummariesValid(t, dash.Nodes)

	if dash.VMCount != len(idx.ByVMID) {
		t.Errorf("vmCount = %d, want %d (len Index.ByVMID)", dash.VMCount, len(idx.ByVMID))
	}

	totalCounted := dash.VMStatusCounts.Running + dash.VMStatusCounts.Paused +
		dash.VMStatusCounts.Stopped + dash.VMStatusCounts.Other
	if totalCounted != dash.VMCount {
		t.Errorf("vmStatusCounts sum = %d, want %d (vmCount)", totalCounted, dash.VMCount)
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
