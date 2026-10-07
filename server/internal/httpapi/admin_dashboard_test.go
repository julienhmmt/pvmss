package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"path/filepath"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"slices"
	"testing"
	"time"
)

type dashboardDTO struct {
	Nodes          []nodeSummaryDTO  `json:"nodes"`
	NodeCount      int               `json:"nodeCount"`
	VMCount        int               `json:"vmCount"`
	VMStatusCounts vmStatusCountsDTO `json:"vmStatusCounts"`
	Version        string            `json:"version"`
	RefreshedAt    string            `json:"refreshedAt"`
}

const (
	nodeSummaryTestCluster = "node-summary"
	dashClusterLabel       = "East Campus"
)

type nodeSummaryDTO struct {
	ClusterKey       string  `json:"clusterKey"`
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
// least one PVMSS-managed VM - the value the dashboard's NodeCount must match.
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

// TestAdminDashboard_AsAdmin_ReturnsPvmssNodesAndVmCounts - GET
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

//nolint:wsl_v5 // focused response assertions stay together
func TestAdminDashboard_NodeSummariesIncludeClusterKey(t *testing.T) {
	t.Parallel()
	authHandler := newAuthHandler(t)
	st := auditAdminStore(t)
	fake := cluster.NewFake(nodeSummaryTestCluster)
	snapshot, _ := fake.Snapshot(context.Background())
	index := inventory.BuildIndexForCluster(nodeSummaryTestCluster, snapshot)
	projection := inventory.NewProjectionFromIndex(&index)
	ops := httpapi.NewAdminOps(authHandler, st, projection, "0.4.0-test", slog.New(slog.DiscardHandler))
	ops.SetInventorySource(inventory.NewRegistryFromIndexes(map[string]*inventory.Index{nodeSummaryTestCluster: &index}), 0)
	cookie := adminCookie(t, authHandler)
	rec := opsGet(t, ops, authHandler, cookie, "/api/v1/admin/dashboard")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var dash dashboardDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dash); err != nil {
		t.Fatalf("decode dashboard: %v", err)
	}
	if len(dash.Nodes) == 0 {
		t.Fatal("dashboard has no node summaries")
	}
	for _, node := range dash.Nodes {
		if node.ClusterKey != nodeSummaryTestCluster {
			t.Errorf("node %q clusterKey = %q, want %s", node.Name, node.ClusterKey, nodeSummaryTestCluster)
		}
	}
}

// TestAdminDashboard_AlertsCarryClusterKey - every alert exposes the cluster
// registry key in clusterKey so the UI can deep-link into
// /admin/nodes/{clusterKey}/{node}; cluster keeps the administrator-facing
// display label, which differs from the key here on purpose.
//
//nolint:paralleltest // serial: shared fake dataset
func TestAdminDashboard_AlertsCarryClusterKey(t *testing.T) {
	authHandler := newAuthHandler(t)
	ctx := context.Background()

	st, err := store.Open(config.Configuration{
		DBPath:        filepath.Join(t.TempDir(), "dashboard-alerts.db"),
		SessionSecret: "a-session-secret-with-at-least-thirty-two-bytes",
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	//nolint:gosec // deterministic test secret
	if err := st.CreateCluster(ctx, store.ClusterRow{
		Name: dashCluster, DisplayName: dashClusterLabel,
		URL: "https://pve-east.example.com:8006/api2/json", TokenID: "pvmss@pve!test", TokenSecret: "dashboard-test-token",
	}); err != nil {
		t.Fatalf("CreateCluster: %v", err)
	}

	if err := st.UpsertPolicyRow(ctx, store.PolicyRow{Cluster: dashCluster, MaxVMPerUser: 2}); err != nil {
		t.Fatalf("UpsertPolicyRow: %v", err)
	}

	// n2 is offline; it must be catalog-enabled to keep the critical
	// node_offline alert (a disabled or unapproved node is informational).
	if err := st.SetNodeEnabled(ctx, dashCluster, "n2", true); err != nil {
		t.Fatalf("SetNodeEnabled: %v", err)
	}

	ops := httpapi.NewAdminOps(authHandler, st, inventory.NewProjection(), "0.4.0-test", slog.New(slog.DiscardHandler))
	ops.SetInventorySource(inventory.NewRegistryFromIndexes(map[string]*inventory.Index{dashCluster: attentionIndex(), dashWestCluster: nil}), time.Minute)

	rec := opsGet(t, ops, authHandler, adminCookie(t, authHandler), "/api/v1/admin/dashboard")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var dash attentionDashboardDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dash); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Critical first, then by kind, cluster and subject - same order as
	// TestAdminDashboard_AlertsCoverEveryRule, with cluster now the label.
	want := wantAttentionAlerts(dashClusterLabel)
	if !slices.Equal(dash.Alerts, want) {
		t.Errorf("alerts =\n%+v\nwant\n%+v", dash.Alerts, want)
	}
}

func TestAdminDashboardVMCounts_SplitsPvmssAndOtherVMs(t *testing.T) {
	t.Parallel()
	auth := newAuthHandler(t)
	st := auditAdminStore(t)
	snap := cluster.Snapshot{VMs: []cluster.VM{
		{VMID: 100, Status: cluster.VMRunning, Tags: []string{catalog.ProtectedTagName}},
		{VMID: 101, Status: cluster.VMStopped, Tags: []string{catalog.ProtectedTagName}},
		{VMID: 102, Status: cluster.VMPaused, Tags: []string{"legacy"}},
		{VMID: 103, Status: cluster.VMStopped},
	}}
	idx := inventory.BuildIndex(snap)
	projection := inventory.NewProjectionFromIndex(&idx)
	ops := httpapi.NewAdminOps(auth, st, projection, "0.4.0-test", slog.New(slog.DiscardHandler))
	cookie := adminCookie(t, auth)

	rec := opsGet(t, ops, auth, cookie, "/api/v1/admin/dashboard")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var dash struct {
		VMCount             int               `json:"vmCount"`
		PVMSSVMCount        int               `json:"pvmssVMCount"`
		PVMSSVMStatusCounts vmStatusCountsDTO `json:"pvmssVMStatusCounts"`
		OtherVMCount        int               `json:"otherVMCount"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dash); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if dash.VMCount != 4 || dash.PVMSSVMCount != 2 || dash.OtherVMCount != 2 {
		t.Errorf("VM counts = all:%d PVMSS:%d other:%d, want all:4 PVMSS:2 other:2", dash.VMCount, dash.PVMSSVMCount, dash.OtherVMCount)
	}

	wantStatuses := vmStatusCountsDTO{Running: 1, Stopped: 1}
	if dash.PVMSSVMStatusCounts != wantStatuses {
		t.Errorf("PVMSS status counts = %+v, want %+v", dash.PVMSSVMStatusCounts, wantStatuses)
	}
}

// TestAdminDashboard_AsNonAdmin_Returns403 - GET /admin/dashboard as
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

// TestAdminDashboard_Sc003_NoClusterClientCallForVMCount - a
// dashboard read makes zero cluster.Client calls - VM count comes from
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
	ops := httpapi.NewAdminOps(authHandler, st, projection, "0.4.0-test", slog.New(slog.DiscardHandler))
	cookie := adminCookie(t, authHandler)

	rec := opsGet(t, ops, authHandler, cookie, "/api/v1/admin/dashboard")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var dash dashboardDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dash); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// Zero cluster.Client calls - the dashboard reads entirely from
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
