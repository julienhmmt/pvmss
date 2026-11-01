//nolint:paralleltest,wsl_v5 // tests share a per-test store and index fixtures
package adoption_test

import (
	"context"
	"errors"
	"path/filepath"
	"pvmss/server/internal/adoption"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/pools"
	"pvmss/server/internal/store"
	"testing"
)

const (
	alicePool = "pvmss-alice"
	bobPool   = "pvmss-bob"
	teamXPool = "team-x"
	pvmssTag  = "pvmss"
	web01Name = "web-01"
)

// stubPoolLister reports a fixed pool list - the cluster-side truth Scan
// reads instead of cluster.Client in tests.
type stubPoolLister struct {
	pools []cluster.Pool
	err   error
}

func (s stubPoolLister) ListPools(context.Context) ([]cluster.Pool, error) {
	return s.pools, s.err
}

func newStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "adoption.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	return st
}

func projectionOf(vms ...cluster.VM) *inventory.Projection {
	index := inventory.BuildIndex(cluster.Snapshot{VMs: vms})
	return inventory.NewProjectionFromIndex(&index)
}

func TestScan_NilProjection(t *testing.T) {
	_, err := adoption.Scan(context.Background(), stubPoolLister{}, inventory.NewProjection(), newStore(t), "default")
	if !errors.Is(err, pools.ErrProjectionNotReady) {
		t.Fatalf("Scan = %v, want ErrProjectionNotReady", err)
	}
}

// TestScan_OrphanPoolsWithMembers: a pvmss-* pool unknown to managed_pools is
// reported with its member VMs; registered and foreign pools are skipped.
func TestScan_OrphanPoolsWithMembers(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	if err := st.RegisterManagedPool(ctx, "default", bobPool); err != nil {
		t.Fatalf("RegisterManagedPool: %v", err)
	}

	lister := stubPoolLister{pools: []cluster.Pool{
		{Name: alicePool, Comment: "Alice's pool"},
		{Name: bobPool},
		{Name: "pvmss-empty"},
		{Name: teamXPool, Comment: "foreign"},
	}}
	projection := projectionOf(
		cluster.VM{VMID: 100, Name: web01Name, Node: "n1", Status: cluster.VMRunning, Pool: alicePool, Tags: []string{pvmssTag}},
		cluster.VM{VMID: 101, Name: "web-02", Node: "n1", Status: cluster.VMStopped, Pool: alicePool, Tags: []string{pvmssTag}},
		cluster.VM{VMID: 102, Name: "db-01", Node: "n2", Status: cluster.VMRunning, Pool: bobPool, Tags: []string{pvmssTag}},
	)

	report, err := adoption.Scan(ctx, lister, projection, st, "default")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Pools) != 2 {
		t.Fatalf("pools = %+v, want pvmss-alice and pvmss-empty", report.Pools)
	}
	if report.Pools[0].Name != alicePool || report.Pools[0].Comment != "Alice's pool" {
		t.Fatalf("pool[0] = %+v", report.Pools[0])
	}
	if len(report.Pools[0].Members) != 2 || report.Pools[0].Members[0].VMID != 100 || report.Pools[0].Members[1].VMID != 101 {
		t.Fatalf("members = %+v, want vmids 100,101", report.Pools[0].Members)
	}
	if report.Pools[1].Name != "pvmss-empty" || len(report.Pools[1].Members) != 0 {
		t.Fatalf("pool[1] = %+v, want empty pvmss-empty", report.Pools[1])
	}
}

// TestScan_LooseVMs: pvmss-tagged VMs outside any pvmss-* pool are reported
// even though adoption writes nothing for them.
func TestScan_LooseVMs(t *testing.T) {
	ctx := context.Background()

	lister := stubPoolLister{pools: []cluster.Pool{{Name: alicePool}}}
	projection := projectionOf(
		cluster.VM{VMID: 100, Name: web01Name, Node: "n1", Status: cluster.VMRunning, Pool: alicePool, Tags: []string{pvmssTag}},
		cluster.VM{VMID: 110, Name: "solo-01", Node: "n2", Status: cluster.VMStopped, Tags: []string{pvmssTag}},
		cluster.VM{VMID: 111, Name: "team-01", Node: "n2", Status: cluster.VMRunning, Pool: teamXPool, Tags: []string{pvmssTag}},
		cluster.VM{VMID: 112, Name: "plain-01", Node: "n2", Status: cluster.VMRunning, Pool: teamXPool},
	)

	report, err := adoption.Scan(ctx, lister, projection, newStore(t), "default")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.LooseVMs) != 2 || report.LooseVMs[0].VMID != 110 || report.LooseVMs[1].VMID != 111 {
		t.Fatalf("looseVMs = %+v, want vmids 110,111", report.LooseVMs)
	}
	if report.LooseVMs[1].Pool != teamXPool {
		t.Fatalf("looseVM pool = %q, want team-x", report.LooseVMs[1].Pool)
	}
}

// TestScan_OrphanTags: tags carried by reported VMs that catalog_tags does not
// know are listed with a live count and a suggested palette color; pvmss and
// known tags are excluded.
func TestScan_OrphanTags(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)
	if err := st.InsertTag(ctx, "default", "web", "#123456", "2026-01-01T00:00:00Z"); err != nil {
		t.Fatalf("InsertTag: %v", err)
	}

	lister := stubPoolLister{pools: []cluster.Pool{{Name: alicePool}}}
	projection := projectionOf(
		cluster.VM{VMID: 100, Name: web01Name, Node: "n1", Status: cluster.VMRunning, Pool: alicePool, Tags: []string{pvmssTag, "web", "db"}},
		cluster.VM{VMID: 101, Name: "web-02", Node: "n1", Status: cluster.VMStopped, Pool: alicePool, Tags: []string{pvmssTag, "db"}},
		cluster.VM{VMID: 110, Name: "solo-01", Node: "n2", Status: cluster.VMStopped, Tags: []string{pvmssTag, "db"}},
	)

	report, err := adoption.Scan(ctx, lister, projection, st, "default")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Tags) != 1 {
		t.Fatalf("tags = %+v, want only db", report.Tags)
	}
	if report.Tags[0].Name != "db" || report.Tags[0].VMCount != 3 {
		t.Fatalf("tag = %+v, want db x3", report.Tags[0])
	}
	if report.Tags[0].Color == "" {
		t.Fatal("tag color should suggest a palette value")
	}
}
