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

type dashNodeStateDTO struct {
	Name    string   `json:"name"`
	State   string   `json:"state"`
	Missing []string `json:"missing"`
	VMCount int      `json:"vmCount"`
	PVMSS   int      `json:"pvmssVMCount"`
}

type dashNodesDTO struct {
	Nodes              []dashNodeStateDTO `json:"nodes"`
	NodeCount          int                `json:"nodeCount"`
	OtherNodeCount     int                `json:"otherNodeCount"`
	CatalogUnavailable bool               `json:"catalogUnavailable"`
}

const (
	nodeReady      = "n-ready"
	nodeIncomplete = "n-incomplete"
	nodeOffline    = "n-offline"
	nodeGone       = "n-gone"
	nodeLegacy     = "n-legacy"
	nodeLegacyDown = "n-legacy-down"
	nodeOther      = "n-other"
)

// approveComplete enables node with a storage, a bridge and an ISO.
func approveComplete(t *testing.T, st *store.Store, node string) {
	t.Helper()

	ctx := context.Background()
	for _, err := range []error{
		st.SetNodeEnabled(ctx, dashCluster, node, true),
		st.SetStorageEnabled(ctx, dashCluster, "local", node, true),
		st.SetBridgeEnabled(ctx, dashCluster, node, "vmbr0", true),
		st.SetISOEnabled(ctx, dashCluster, node, "local", "a.iso", true),
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

	if err := st.SetStorageEnabled(ctx, dashCluster, "local", nodeIncomplete, true); err != nil {
		t.Fatal(err)
	}

	online := func(name string) cluster.Node {
		return cluster.Node{Name: name, Status: cluster.NodeOnline, CPUCores: 8, MemoryTotal: 64 * gib}
	}

	idx := inventory.BuildIndexForCluster(dashCluster, cluster.Snapshot{
		Nodes: []cluster.Node{
			online(nodeReady), online(nodeIncomplete), online(nodeLegacy), online(nodeOther),
			{Name: nodeOffline, Status: cluster.NodeOffline, CPUCores: 8, MemoryTotal: 64 * gib},
			{Name: nodeLegacyDown, Status: cluster.NodeOffline, CPUCores: 8, MemoryTotal: 64 * gib},
		},
		VMs: []cluster.VM{
			{VMID: 100, Name: "kept", Node: nodeLegacy, Status: cluster.VMRunning, Tags: []string{catalog.ProtectedTagName}},
			{VMID: 101, Name: "infra", Node: nodeOther, Status: cluster.VMRunning},
			{VMID: 102, Name: "infra-2", Node: nodeLegacy, Status: cluster.VMRunning},
			{VMID: 103, Name: "kept-down", Node: nodeLegacyDown, Status: cluster.VMStopped, Tags: []string{catalog.ProtectedTagName}},
		},
	})
	idx.RefreshedAt = time.Now()

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

// When the catalog cannot be read, only nodes hosting PVMSS VMs are listed
// and the payload says the catalog is unavailable.
//
//nolint:paralleltest // serial: shared fake dataset
func TestAdminDashboard_NodeSetCatalogUnavailable(t *testing.T) {
	dash := getNodeStatesDashboard(t, true)

	if !dash.CatalogUnavailable {
		t.Error("catalogUnavailable = false, want true")
	}

	if len(dash.Nodes) != 2 || dash.Nodes[0].Name != nodeLegacy || dash.Nodes[1].Name != nodeLegacyDown || dash.Nodes[0].State != "" {
		t.Errorf("nodes = %+v, want only %s and %s without state", dash.Nodes, nodeLegacy, nodeLegacyDown)
	}
}
