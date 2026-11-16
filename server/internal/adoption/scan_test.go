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
	alicePool  = "pvmss-alice"
	bobPool    = "pvmss-bob"
	teamXPool  = "team-x"
	pvmssTag   = "pvmss"
	web01Name  = "web-01"
	web02Name  = "web-02"
	solo01Name = "solo-01"
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
		cluster.VM{VMID: 101, Name: web02Name, Node: "n1", Status: cluster.VMStopped, Pool: alicePool, Tags: []string{pvmssTag}},
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
		cluster.VM{VMID: 110, Name: solo01Name, Node: "n2", Status: cluster.VMStopped, Tags: []string{pvmssTag}},
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
		cluster.VM{VMID: 101, Name: web02Name, Node: "n1", Status: cluster.VMStopped, Pool: alicePool, Tags: []string{pvmssTag, "db"}},
		cluster.VM{VMID: 110, Name: solo01Name, Node: "n2", Status: cluster.VMStopped, Tags: []string{pvmssTag, "db"}},
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

// TestScan_DismissedPoolMovesToHidden: a dismissed pool leaves the
// adoptable list and lands in the hidden summary (name, comment, member
// count only); its members still feed the orphan tag counts.
func TestScan_DismissedPoolMovesToHidden(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)

	if err := st.DismissRecoveryItems(ctx, "default", store.RecoveryDismissalPool, []string{bobPool}, "admin"); err != nil {
		t.Fatalf("dismiss pool: %v", err)
	}

	lister := stubPoolLister{pools: []cluster.Pool{{Name: alicePool, Comment: "keep"}, {Name: bobPool, Comment: "hidden"}}}
	projection := projectionOf(
		cluster.VM{VMID: 100, Name: web01Name, Node: "n1", Status: cluster.VMRunning, Pool: alicePool, Tags: []string{pvmssTag, "db"}},
		cluster.VM{VMID: 102, Name: "db-01", Node: "n2", Status: cluster.VMRunning, Pool: bobPool, Tags: []string{pvmssTag, "db", "extra"}},
	)

	report, err := adoption.Scan(ctx, lister, projection, st, "default")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Pools) != 1 || report.Pools[0].Name != alicePool {
		t.Fatalf("pools = %+v, want only pvmss-alice", report.Pools)
	}
	if len(report.HiddenPools) != 1 || report.HiddenPools[0].Name != bobPool || report.HiddenPools[0].MemberCount != 1 || report.HiddenPools[0].Comment != "hidden" {
		t.Fatalf("hiddenPools = %+v, want pvmss-bob x1 member", report.HiddenPools)
	}
	// The hidden pool's member still counts toward "db" and "extra".
	if len(report.Tags) != 2 {
		t.Fatalf("tags = %+v, want db x2 and extra x1", report.Tags)
	}
}

// TestScan_DismissedTagMovesToHidden: a dismissed tag keeps its live count
// in the hidden list (suggested color preserved for later adoption) while
// other orphan tags stay visible.
func TestScan_DismissedTagMovesToHidden(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)

	if err := st.DismissRecoveryItems(ctx, "default", store.RecoveryDismissalTag, []string{"db"}, "admin"); err != nil {
		t.Fatalf("dismiss tag: %v", err)
	}

	lister := stubPoolLister{pools: []cluster.Pool{{Name: alicePool}}}
	projection := projectionOf(
		cluster.VM{VMID: 100, Name: web01Name, Node: "n1", Status: cluster.VMRunning, Pool: alicePool, Tags: []string{pvmssTag, "db"}},
		cluster.VM{VMID: 101, Name: web02Name, Node: "n1", Status: cluster.VMStopped, Pool: alicePool, Tags: []string{pvmssTag, "db", "extra"}},
	)

	report, err := adoption.Scan(ctx, lister, projection, st, "default")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.Tags) != 1 || report.Tags[0].Name != "extra" || report.Tags[0].VMCount != 1 {
		t.Fatalf("tags = %+v, want only extra x1", report.Tags)
	}
	if len(report.HiddenTags) != 1 || report.HiddenTags[0].Name != "db" || report.HiddenTags[0].VMCount != 2 {
		t.Fatalf("hiddenTags = %+v, want db x2", report.HiddenTags)
	}
}

// TestScan_DismissedLooseVMMovesToHidden: a dismissed loose VM (matched by
// vmid) leaves the visible list and lands in the hidden summary.
func TestScan_DismissedLooseVMMovesToHidden(t *testing.T) {
	ctx := context.Background()
	st := newStore(t)

	if err := st.DismissRecoveryItems(ctx, "default", store.RecoveryDismissalVM, []string{"110"}, "admin"); err != nil {
		t.Fatalf("dismiss vm: %v", err)
	}

	lister := stubPoolLister{pools: []cluster.Pool{{Name: alicePool}}}
	projection := projectionOf(
		cluster.VM{VMID: 110, Name: solo01Name, Node: "n2", Status: cluster.VMStopped, Tags: []string{pvmssTag}},
		cluster.VM{VMID: 111, Name: "team-01", Node: "n2", Status: cluster.VMRunning, Pool: teamXPool, Tags: []string{pvmssTag}},
	)

	report, err := adoption.Scan(ctx, lister, projection, st, "default")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(report.LooseVMs) != 1 || report.LooseVMs[0].VMID != 111 {
		t.Fatalf("looseVMs = %+v, want only 111", report.LooseVMs)
	}
	if len(report.HiddenLooseVMs) != 1 || report.HiddenLooseVMs[0].VMID != 110 || report.HiddenLooseVMs[0].Name != solo01Name {
		t.Fatalf("hiddenLooseVMs = %+v, want 110 solo-01", report.HiddenLooseVMs)
	}
}

// TestScan_UndismissedReportHasEmptyHidden: without dismissals the hidden
// arrays are empty (not nil) so the DTO always serializes them.
func TestScan_UndismissedReportHasEmptyHidden(t *testing.T) {
	lister := stubPoolLister{pools: []cluster.Pool{{Name: alicePool}}}
	projection := projectionOf()

	report, err := adoption.Scan(context.Background(), lister, projection, newStore(t), "default")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if report.HiddenPools == nil || report.HiddenLooseVMs == nil || report.HiddenTags == nil {
		t.Fatalf("hidden arrays must be non-nil: %+v", report)
	}
}
