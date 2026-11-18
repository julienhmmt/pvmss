package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
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

type dashNodeStateDTO struct {
	Name    string   `json:"name"`
	State   string   `json:"state"`
	Missing []string `json:"missing"`
	VMCount int      `json:"vmCount"`
	PVMSS   int      `json:"pvmssVMCount"`
	RAMGB   int      `json:"pvmssRamGB"`
	Loose   int      `json:"unadoptedVMCount"`
	MaxRAM  int      `json:"policyMaxRamGB"`
	MaxVMs  int      `json:"policyMaxVMs"`
}

type dashNodesDTO struct {
	Nodes              []dashNodeStateDTO  `json:"nodes"`
	NodeCount          int                 `json:"nodeCount"`
	OtherNodeCount     int                 `json:"otherNodeCount"`
	CatalogUnavailable bool                `json:"catalogUnavailable"`
	Alerts             []dashboardAlertDTO `json:"alerts"`
	Storages           []struct {
		Name string `json:"name"`
	} `json:"storages"`
}

const (
	nodeReady      = "n-ready"
	nodeIncomplete = "n-incomplete"
	nodeOffline    = "n-offline"
	nodeGone       = "n-gone"
	nodeLegacy     = "n-legacy"
	nodeLegacyDown = "n-legacy-down"
	nodeOther      = "n-other"
	dashLocal      = "local"
)

// approveComplete enables node with a storage, a bridge and an ISO.
func approveComplete(t *testing.T, st *store.Store, node string) {
	t.Helper()

	ctx := context.Background()
	for _, err := range []error{
		st.SetNodeEnabled(ctx, dashCluster, node, true),
		st.SetStorageEnabled(ctx, dashCluster, dashLocal, node, true),
		st.SetBridgeEnabled(ctx, dashCluster, node, "vmbr0", true),
		st.SetISOEnabled(ctx, dashCluster, node, dashLocal, "a.iso", true),
	} {
		if err != nil {
			t.Fatalf("approve %s: %v", node, err)
		}
	}
}

func getNodeStatesDashboard(t *testing.T, closeStore bool) dashNodesDTO {
	t.Helper()

	ctx := context.Background()
	authHandler := newAuthHandler(t)
	st := auditAdminStore(t)

	for _, n := range []string{nodeReady, nodeOffline, nodeGone} {
		approveComplete(t, st, n)
	}

	if err := st.SetNodeEnabled(ctx, dashCluster, nodeIncomplete, true); err != nil {
		t.Fatal(err)
	}

	if err := st.SetStorageEnabled(ctx, dashCluster, dashLocal, nodeIncomplete, true); err != nil {
		t.Fatal(err)
	}

	online := func(name string) cluster.Node {
		return cluster.Node{Name: name, Status: cluster.NodeOnline, CPUCores: 8, MemoryTotal: 64 * gib}
	}

	idx := inventory.BuildIndexForCluster(dashCluster, cluster.Snapshot{
		Nodes: []cluster.Node{
			online(nodeReady), online(nodeIncomplete), online(nodeLegacy),
			// Unlisted: its CPU alert must be dropped.
			{Name: nodeOther, Status: cluster.NodeOnline, CPUCores: 8, CPUUsage: 0.95, MemoryTotal: 64 * gib},
			{Name: nodeOffline, Status: cluster.NodeOffline, CPUCores: 8, MemoryTotal: 64 * gib},
			{Name: nodeLegacyDown, Status: cluster.NodeOffline, CPUCores: 8, MemoryTotal: 64 * gib},
		},
		VMs: []cluster.VM{
			{VMID: 100, Name: "kept", Node: nodeLegacy, Status: cluster.VMRunning, MemoryTotal: 4 * gib, Tags: []string{catalog.ProtectedTagName}},
			{VMID: 101, Name: "infra", Node: nodeOther, Status: cluster.VMRunning},
			{VMID: 102, Name: "infra-2", Node: nodeLegacy, Status: cluster.VMRunning},
			{VMID: 104, Name: "adopted", Node: nodeReady, Pool: "pvmss-alice", Status: cluster.VMRunning, Tags: []string{catalog.ProtectedTagName}},
			{VMID: 103, Name: "kept-down", Node: nodeLegacyDown, Status: cluster.VMStopped, Tags: []string{catalog.ProtectedTagName}},
		},
		Storages: []cluster.Storage{
			{Name: dashLocal, Node: nodeReady, PluginType: "dir", Total: 100 * gib, Used: 10 * gib},
			{Name: dashLocal, Node: nodeOther, PluginType: "dir", Total: 100 * gib, Used: 10 * gib},
			{Name: "scratch", Node: nodeOther, PluginType: "dir", Total: 100 * gib, Used: 10 * gib},
		},
	})
	idx.RefreshedAt = time.Now()

	// 104 sits in a managed pool (adopted); 103 was hidden on /admin/recovery.
	if err := st.RegisterManagedPool(ctx, dashCluster, "pvmss-alice"); err != nil {
		t.Fatal(err)
	}

	if err := st.DismissRecoveryItems(ctx, dashCluster, store.RecoveryDismissalVM, []string{"103"}, "admin"); err != nil {
		t.Fatal(err)
	}

	if err := st.UpsertNodePolicyRow(ctx, store.NodePolicyRow{Cluster: dashCluster, Node: nodeReady, MaxVMs: 5, MaxRAMGB: 32}); err != nil {
		t.Fatal(err)
	}

	if closeStore {
		if err := st.Close(); err != nil {
			t.Fatal(err)
		}
	}

	ops := httpapi.NewAdminOps(authHandler, st, inventory.NewProjection(), "test", slog.New(slog.DiscardHandler))
	ops.SetInventorySource(inventory.NewRegistryFromIndexes(map[string]*inventory.Index{dashCluster: &idx}), time.Minute)

	rec := opsGet(t, ops, authHandler, adminCookie(t, authHandler), "/api/v1/admin/dashboard")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var dash dashNodesDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &dash); err != nil {
		t.Fatalf("decode: %v", err)
	}

	return dash
}

// The dashboard lists enabled nodes plus nodes hosting PVMSS VMs, each with
// its state, worst first; the rest only count toward otherNodeCount.
//
//nolint:paralleltest // serial: shared fake dataset
func TestAdminDashboard_NodeSetAndStates(t *testing.T) {
	dash := getNodeStatesDashboard(t, false)

	got := make([]string, 0, len(dash.Nodes))
	for _, n := range dash.Nodes {
		got = append(got, n.Name+":"+n.State)
	}

	want := []string{
		nodeGone + ":offline", nodeLegacyDown + ":offline", nodeOffline + ":offline",
		nodeIncomplete + ":incomplete",
		nodeLegacy + ":disabled_with_vms",
		nodeReady + ":ready",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("nodes = %v, want %v", got, want)
	}

	if m := dash.Nodes[3].Missing; !slices.Equal(m, []string{catalog.MissingBridge, catalog.MissingISOOrTemplate}) {
		t.Errorf("incomplete missing = %v", m)
	}

	// The disabled-with-VMs badge counts PVMSS VMs only, not the infra VM beside it.
	if legacy := dash.Nodes[4]; legacy.VMCount != 2 || legacy.PVMSS != 1 {
		t.Errorf("legacy vmCount=%d pvmssVMCount=%d, want 2 and 1", legacy.VMCount, legacy.PVMSS)
	}

	if dash.OtherNodeCount != 1 || dash.NodeCount != 6 || dash.CatalogUnavailable {
		t.Errorf("otherNodeCount=%d nodeCount=%d catalogUnavailable=%v, want 1, 6, false", dash.OtherNodeCount, dash.NodeCount, dash.CatalogUnavailable)
	}
}

// Each card carries the PVMSS RAM share and the node's policy limits; only
// storages approved in the catalog (or holding PVMSS disks) are listed.
//
//nolint:paralleltest // serial: shared fake dataset
func TestAdminDashboard_NodeCapacityAndStorages(t *testing.T) {
	dash := getNodeStatesDashboard(t, false)

	if legacy := dash.Nodes[4]; legacy.RAMGB != 4 {
		t.Errorf("legacy pvmssRamGB = %d, want 4", legacy.RAMGB)
	}

	if ready := dash.Nodes[5]; ready.MaxRAM != 32 || ready.MaxVMs != 5 {
		t.Errorf("ready policy caps = %d GB / %d VMs, want 32 / 5", ready.MaxRAM, ready.MaxVMs)
	}

	if len(dash.Storages) != 1 || dash.Storages[0].Name != dashLocal {
		t.Errorf("storages = %+v, want only local", dash.Storages)
	}
}

// When the catalog cannot be read, only nodes hosting PVMSS VMs are listed
// and the payload says the catalog is unavailable.
//
//nolint:paralleltest // serial: shared fake dataset
func TestAdminDashboard_NodeSetCatalogUnavailable(t *testing.T) {
	dash := getNodeStatesDashboard(t, true)

	if !dash.CatalogUnavailable {
		t.Error("catalogUnavailable = false, want true")
	}

	names := make([]string, 0, len(dash.Nodes))
	for _, n := range dash.Nodes {
		names = append(names, n.Name+":"+n.State)
	}

	if want := []string{nodeLegacy + ":", nodeLegacyDown + ":", nodeReady + ":"}; !slices.Equal(names, want) {
		t.Errorf("nodes = %v, want %v (PVMSS nodes only, no state)", names, want)
	}
}

// Node alerts cover listed nodes only: offline (including enabled nodes gone
// from inventory), incomplete with its first missing item, and unadopted
// PVMSS VMs (outside managed pools, not hidden).
//
//nolint:paralleltest // serial: shared fake dataset
func TestAdminDashboard_NodeAlerts(t *testing.T) {
	dash := getNodeStatesDashboard(t, false)

	got := make([]string, 0, len(dash.Alerts))
	for _, a := range dash.Alerts {
		got = append(got, fmt.Sprintf("%s/%s/%s/%s/%d", a.Kind, a.Severity, a.Subject, a.Detail, a.Used))
	}

	slices.Sort(got)

	want := []string{
		"node_incomplete/warning/" + nodeIncomplete + "/" + catalog.MissingBridge + "/0",
		"node_offline/critical/" + nodeGone + "//0",
		"node_offline/critical/" + nodeLegacyDown + "//0",
		"node_offline/critical/" + nodeOffline + "//0",
		"node_unadopted_vms/warning/" + nodeLegacy + "//1",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("alerts =\n%v\nwant\n%v", got, want)
	}

	counts := map[string]int{}
	for _, n := range dash.Nodes {
		counts[n.Name] = n.Loose
	}

	if counts[nodeLegacy] != 1 || counts[nodeLegacyDown] != 0 || counts[nodeReady] != 0 {
		t.Errorf("unadopted counts = %v, want legacy 1, legacy-down 0 (hidden), ready 0 (managed pool)", counts)
	}
}

// With the catalog unreadable, no readiness alert is raised and offline
// nodes keep their critical alert: an unknown state must not silence it.
//
//nolint:paralleltest // serial: shared fake dataset
func TestAdminDashboard_NodeAlertsCatalogUnavailable(t *testing.T) {
	dash := getNodeStatesDashboard(t, true)

	for _, a := range dash.Alerts {
		if a.Kind == "node_incomplete" {
			t.Errorf("unexpected readiness alert %+v", a)
		}
	}

	if !slices.ContainsFunc(dash.Alerts, func(a dashboardAlertDTO) bool {
		return a.Kind == "node_offline" && a.Severity == "critical" && a.Subject == nodeOffline
	}) {
		t.Errorf("alerts = %+v, want critical node_offline for %s", dash.Alerts, nodeOffline)
	}
}
