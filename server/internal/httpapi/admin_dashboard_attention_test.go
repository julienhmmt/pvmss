package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"slices"
	"testing"
	"time"
)

const (
	gib             = int64(1) << 30
	dashCluster     = "east"
	dashWestCluster = "west"
	dashSharedName  = "ceph"
)

type dashboardAlertDTO struct {
	Kind       string `json:"kind"`
	Severity   string `json:"severity"`
	Cluster    string `json:"cluster"`
	ClusterKey string `json:"clusterKey"`
	Subject    string `json:"subject"`
	Percent    int    `json:"percent"`
}

type dashboardStorageDTO struct {
	Cluster    string `json:"cluster"`
	Name       string `json:"name"`
	Node       string `json:"node"`
	Shared     bool   `json:"shared"`
	UsedBytes  int64  `json:"usedBytes"`
	TotalBytes int64  `json:"totalBytes"`
	Percent    int    `json:"percent"`
}

type attentionDashboardDTO struct {
	Nodes []struct {
		Cluster        string `json:"cluster"`
		Name           string `json:"name"`
		VMCount        int    `json:"vmCount"`
		VMRunningCount int    `json:"vmRunningCount"`
	} `json:"nodes"`
	VMCount       int                   `json:"vmCount"`
	Alerts        []dashboardAlertDTO   `json:"alerts"`
	Storages      []dashboardStorageDTO `json:"storages"`
	RecentChanges []struct {
		Action string `json:"action"`
	} `json:"recentChanges"`
}

// attentionIndex is a small cluster built to trip every alert rule once:
// n1 CPU at 95%, n2 offline, a shared Ceph pool at 96% seen from both nodes,
// a local store at 86% on n1, and pool pvmss-p1 holding two VMs.
func attentionIndex() *inventory.Index {
	idx := inventory.BuildIndexForCluster(dashCluster, cluster.Snapshot{
		Nodes: []cluster.Node{
			{Name: "n1", Status: cluster.NodeOnline, CPUCores: 8, CPUUsage: 0.95, MemoryTotal: 64 * gib, MemoryUsed: 32 * gib},
			{Name: "n2", Status: cluster.NodeOffline, CPUCores: 8, MemoryTotal: 64 * gib},
		},
		VMs: []cluster.VM{
			{VMID: 100, Name: "a", Node: "n1", Pool: "pvmss-p1", Status: cluster.VMRunning},
			{VMID: 101, Name: "b", Node: "n1", Pool: "pvmss-p1", Status: cluster.VMStopped},
		},
		Storages: []cluster.Storage{
			{Name: dashSharedName, Node: "n1", PluginType: "rbd", Total: 100 * gib, Used: 96 * gib},
			{Name: dashSharedName, Node: "n2", PluginType: "rbd", Total: 100 * gib, Used: 96 * gib},
			{Name: testStorageLocalLVM, Node: "n1", PluginType: "lvmthin", Total: 100 * gib, Used: 86 * gib},
			{Name: testStorageLocalLVM, Node: "n2", PluginType: "lvmthin", Total: 100 * gib, Used: 10 * gib},
		},
	})
	idx.RefreshedAt = time.Now()

	return &idx
}

func getAttentionDashboard(t *testing.T) attentionDashboardDTO {
	t.Helper()

	authHandler := newAuthHandler(t)
	st := auditAdminStore(t)

	if err := st.UpsertPolicyRow(context.Background(), store.PolicyRow{Cluster: dashCluster, MaxVMPerUser: 2}); err != nil {
		t.Fatalf("UpsertPolicyRow: %v", err)
	}

	ops := httpapi.NewAdminOps(authHandler, st, cluster.Fake{}, inventory.NewProjection(), "test", slog.New(slog.DiscardHandler))
	// west never completed a refresh: it must surface as unreachable.
	ops.SetInventorySource(inventory.NewRegistryFromIndexes(map[string]*inventory.Index{dashCluster: attentionIndex(), dashWestCluster: nil}), time.Minute)

	rec := opsGet(t, ops, authHandler, adminCookie(t, authHandler), "/api/v1/admin/dashboard")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var dash attentionDashboardDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dash); err != nil {
		t.Fatalf("decode: %v", err)
	}

	return dash
}

// wantAttentionAlerts is the alert list the attentionIndex fixture produces,
// in dashboard order: critical first, then by kind, cluster and subject.
// clusterLabel is the admin-facing cluster label - the registry key when the
// cluster has no store row, its DisplayName when it has one.
func wantAttentionAlerts(clusterLabel string) []dashboardAlertDTO {
	return []dashboardAlertDTO{
		{Kind: "cluster_unreachable", Severity: "critical", Cluster: dashWestCluster, ClusterKey: dashWestCluster},
		{Kind: "node_offline", Severity: "critical", Cluster: clusterLabel, ClusterKey: dashCluster, Subject: "n2"},
		{Kind: "storage_full", Severity: "critical", Cluster: clusterLabel, ClusterKey: dashCluster, Subject: dashSharedName, Percent: 96},
		{Kind: "node_cpu", Severity: "warning", Cluster: clusterLabel, ClusterKey: dashCluster, Subject: "n1", Percent: 95},
		{Kind: "pool_at_quota", Severity: "warning", Cluster: clusterLabel, ClusterKey: dashCluster, Subject: "pvmss-p1", Percent: 100},
		{Kind: "storage_full", Severity: "warning", Cluster: clusterLabel, ClusterKey: dashCluster, Subject: testStorageLocalLVM, Percent: 86},
	}
}

//nolint:paralleltest // serial: shared fake dataset
func TestAdminDashboard_AlertsCoverEveryRule(t *testing.T) {
	dash := getAttentionDashboard(t)

	want := wantAttentionAlerts(dashCluster)
	if !slices.Equal(dash.Alerts, want) {
		t.Errorf("alerts =\n%+v\nwant\n%+v", dash.Alerts, want)
	}
}

//nolint:paralleltest // serial: shared fake dataset
func TestAdminDashboard_PoolQuotaIgnoresNonPVMSSPools(t *testing.T) {
	authHandler := newAuthHandler(t)
	st := auditAdminStore(t)

	if err := st.UpsertPolicyRow(context.Background(), store.PolicyRow{Cluster: dashCluster, MaxVMPerUser: 2}); err != nil {
		t.Fatalf("UpsertPolicyRow: %v", err)
	}

	idx := inventory.BuildIndexForCluster(dashCluster, cluster.Snapshot{
		Nodes: []cluster.Node{
			{Name: "n1", Status: cluster.NodeOnline, CPUCores: 8, MemoryTotal: 64 * gib},
		},
		VMs: []cluster.VM{
			{VMID: 100, Name: "a", Node: "n1", Pool: "prod", Status: cluster.VMRunning},
			{VMID: 101, Name: "b", Node: "n1", Pool: "prod", Status: cluster.VMRunning},
		},
	})
	idx.RefreshedAt = time.Now()

	ops := httpapi.NewAdminOps(authHandler, st, cluster.Fake{}, inventory.NewProjection(), "test", slog.New(slog.DiscardHandler))
	ops.SetInventorySource(inventory.NewRegistryFromIndexes(map[string]*inventory.Index{dashCluster: &idx}), time.Minute)

	rec := opsGet(t, ops, authHandler, adminCookie(t, authHandler), "/api/v1/admin/dashboard")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var dash attentionDashboardDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dash); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for _, a := range dash.Alerts {
		if a.Kind == "pool_at_quota" {
			t.Errorf("pool_at_quota alert for non-PVMSS pool: %+v", a)
		}
	}
}

//nolint:paralleltest // serial: shared fake dataset
func TestAdminDashboard_SharedStorageCountedOnce(t *testing.T) {
	dash := getAttentionDashboard(t)

	var ceph, local int

	for _, s := range dash.Storages {
		switch s.Name {
		case dashSharedName:
			ceph++

			if !s.Shared || s.Node != "" {
				t.Errorf("ceph = %+v, want shared with no node", s)
			}
		case testStorageLocalLVM:
			local++
		}
	}

	if ceph != 1 || local != 2 {
		t.Errorf("storages ceph=%d local=%d, want 1 and 2 (per-node local-lvm)", ceph, local)
	}
}

//nolint:paralleltest // serial: shared fake dataset
func TestAdminDashboard_NodesCarryRunningCountAndRecentChanges(t *testing.T) {
	dash := getAttentionDashboard(t)

	if len(dash.Nodes) != 1 || dash.Nodes[0].Cluster != dashCluster || dash.Nodes[0].VMCount != 2 || dash.Nodes[0].VMRunningCount != 1 {
		t.Errorf("nodes = %+v, want east/n1 with 1 of 2 running", dash.Nodes)
	}

	if dash.VMCount != 2 {
		t.Errorf("vmCount = %d, want 2", dash.VMCount)
	}

	if len(dash.RecentChanges) == 0 || len(dash.RecentChanges) > 8 {
		t.Errorf("recentChanges = %d entries, want 1..8", len(dash.RecentChanges))
	}
}
