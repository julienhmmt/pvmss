package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"pvmss/server/internal/catalog"
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

const (
	sevCritical = "critical"
	sevWarning  = "warning"
	sevInfo     = "info"
)

const (
	kindNodeOffline         = "node_offline"
	kindNodeOfflineDisabled = "node_offline_disabled"
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

	// n2 is offline; it must be catalog-enabled to keep the critical
	// node_offline alert (a disabled or unapproved node is informational).
	if err := st.SetNodeEnabled(context.Background(), dashCluster, "n2", true); err != nil {
		t.Fatalf("SetNodeEnabled: %v", err)
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
		{Kind: "cluster_unreachable", Severity: sevCritical, Cluster: dashWestCluster, ClusterKey: dashWestCluster},
		{Kind: kindNodeOffline, Severity: sevCritical, Cluster: clusterLabel, ClusterKey: dashCluster, Subject: "n2"},
		{Kind: "storage_full", Severity: sevCritical, Cluster: clusterLabel, ClusterKey: dashCluster, Subject: dashSharedName, Percent: 96},
		{Kind: "node_cpu", Severity: sevWarning, Cluster: clusterLabel, ClusterKey: dashCluster, Subject: "n1", Percent: 95},
		{Kind: "pool_at_quota", Severity: sevWarning, Cluster: clusterLabel, ClusterKey: dashCluster, Subject: "pvmss-p1", Percent: 100},
		{Kind: "storage_full", Severity: sevWarning, Cluster: clusterLabel, ClusterKey: dashCluster, Subject: testStorageLocalLVM, Percent: 86},
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
func TestAdminDashboard_SharedFlagDedupesNonListedPlugin(t *testing.T) {
	// A `dir` storage is not in sharedStoragePlugins; only the storage.cfg
	// `shared` flag (cluster.Storage.Shared) can mark it shared.
	authHandler := newAuthHandler(t)
	st := auditAdminStore(t)

	idx := inventory.BuildIndexForCluster(dashCluster, cluster.Snapshot{
		Nodes: []cluster.Node{
			{Name: "n1", Status: cluster.NodeOnline},
			{Name: "n2", Status: cluster.NodeOnline},
		},
		Storages: []cluster.Storage{
			{Name: "iso-share", Node: "n1", PluginType: "dir", Total: 100 * gib, Used: 10 * gib, Shared: true},
			{Name: "iso-share", Node: "n2", PluginType: "dir", Total: 100 * gib, Used: 10 * gib, Shared: true},
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

	var rows []dashboardStorageDTO

	for _, s := range dash.Storages {
		if s.Name == "iso-share" {
			rows = append(rows, s)
		}
	}

	if len(rows) != 1 {
		t.Fatalf("iso-share rows = %d, want 1 (shared flag dedupes): %+v", len(rows), rows)
	}

	if !rows[0].Shared || rows[0].Node != "" {
		t.Errorf("iso-share = %+v, want shared with no node", rows[0])
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

type offlineSeverityCase struct {
	name         string
	setCatalog   bool     // upsert a catalog_nodes row for n2
	enabled      bool     // enabled value written when setCatalog
	vmTags       []string // non-empty: a VM with these tags lives on n2
	closeStore   bool     // close the store before the request (catalog read fails)
	wantKind     string
	wantSeverity string
}

// offlineSeverityOps builds the dashboard handler for one severity case: n1
// online at 95% CPU (always a warning alert) plus n2 offline, optionally
// hosting a VM with the given tags.
func offlineSeverityOps(t *testing.T, tt offlineSeverityCase) (*httpapi.AdminOps, *httpapi.Auth) {
	t.Helper()

	authHandler := newAuthHandler(t)
	st := auditAdminStore(t)

	if tt.setCatalog {
		if err := st.SetNodeEnabled(context.Background(), dashCluster, "n2", tt.enabled); err != nil {
			t.Fatalf("SetNodeEnabled: %v", err)
		}
	}

	var vms []cluster.VM
	if len(tt.vmTags) > 0 {
		vms = append(vms, cluster.VM{VMID: 100, Name: "vm-on-n2", Node: "n2", Status: cluster.VMRunning, Tags: tt.vmTags})
	}

	idx := inventory.BuildIndexForCluster(dashCluster, cluster.Snapshot{
		Nodes: []cluster.Node{
			{Name: "n1", Status: cluster.NodeOnline, CPUCores: 8, CPUUsage: 0.95, MemoryTotal: 64 * gib, MemoryUsed: 32 * gib},
			{Name: "n2", Status: cluster.NodeOffline, CPUCores: 8, MemoryTotal: 64 * gib},
		},
		VMs: vms,
	})
	idx.RefreshedAt = time.Now()

	if tt.closeStore {
		if err := st.Close(); err != nil {
			t.Fatalf("close store: %v", err)
		}
	}

	ops := httpapi.NewAdminOps(authHandler, st, cluster.Fake{}, inventory.NewProjection(), "test", slog.New(slog.DiscardHandler))
	ops.SetInventorySource(inventory.NewRegistryFromIndexes(map[string]*inventory.Index{dashCluster: &idx}), time.Minute)

	return ops, authHandler
}

// TestAdminDashboard_OfflineNodeSeverity pins the downgrade rule: an offline
// node is informational only when it is disabled in the catalog (or never
// approved) and hosts no PVMSS-managed VM. n1 runs at 95% CPU so a warning
// alert is always present - info alerts must sort after it.
//
//nolint:paralleltest // serial: shared fake dataset
func TestAdminDashboard_OfflineNodeSeverity(t *testing.T) {
	tests := []offlineSeverityCase{
		{name: "disabled row, no PVMSS VM", setCatalog: true, wantKind: kindNodeOfflineDisabled, wantSeverity: sevInfo},
		{name: "no catalog row, no PVMSS VM", wantKind: kindNodeOfflineDisabled, wantSeverity: sevInfo},
		{name: "disabled with a PVMSS VM", setCatalog: true, vmTags: []string{catalog.ProtectedTagName}, wantKind: kindNodeOffline, wantSeverity: sevCritical},
		{name: "disabled with an untagged VM only", setCatalog: true, vmTags: []string{"legacy"}, wantKind: kindNodeOfflineDisabled, wantSeverity: sevInfo},
		{name: "enabled", setCatalog: true, enabled: true, wantKind: kindNodeOffline, wantSeverity: sevCritical},
		{name: "catalog read failure stays critical", closeStore: true, wantKind: kindNodeOffline, wantSeverity: sevCritical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assertOfflineSeverity(t, tt)
		})
	}
}

// assertOfflineSeverity GETs the dashboard for the case's cluster and checks
// the n2 offline alert kind and severity.
func assertOfflineSeverity(t *testing.T, tt offlineSeverityCase) {
	t.Helper()

	ops, authHandler := offlineSeverityOps(t, tt)

	rec := opsGet(t, ops, authHandler, adminCookie(t, authHandler), "/api/v1/admin/dashboard")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var dash attentionDashboardDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dash); err != nil {
		t.Fatalf("decode: %v", err)
	}

	// The fixture only trips node_cpu (warning, n1) and the offline
	// alert (n2): critical first, info last.
	if len(dash.Alerts) != 2 {
		t.Fatalf("alerts = %+v, want exactly 2", dash.Alerts)
	}

	offline := dash.Alerts[0]
	if tt.wantSeverity == sevInfo {
		offline = dash.Alerts[1]
	}

	if offline.Subject != "n2" || offline.Kind != tt.wantKind || offline.Severity != tt.wantSeverity {
		t.Errorf("offline alert = %+v, want n2 kind %s severity %s", offline, tt.wantKind, tt.wantSeverity)
	}
}
