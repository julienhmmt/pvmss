//nolint:paralleltest,wsl_v5 // tests use shared fake fixtures and table assertions
package pools_test

import (
	"context"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/pools"
	"pvmss/server/internal/store"
	"testing"
)

func TestList_RunningStoppedBreakdown(t *testing.T) {
	client := cluster.Fake{}
	snapshot, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	index := inventory.BuildIndex(snapshot)
	projection := inventory.NewProjectionFromIndex(&index)

	items, err := pools.List(context.Background(), client, projection, "ALICE")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("got %d rows, want 1", len(items))
	}
	item := items[0]
	if item.Name != cluster.FakePoolAlice {
		t.Fatalf("name = %q, want %q", item.Name, cluster.FakePoolAlice)
	}
	if item.Total != 7 || item.Running != 3 || item.Stopped != 4 {
		t.Fatalf("summary = %+v, want total=7 running=3 stopped=4", item)
	}
	if item.Running+item.Stopped != item.Total {
		t.Fatalf("running + stopped != total: %+v", item)
	}
	if item.Managed {
		t.Fatalf("legacy List should not mark pools managed: %+v", item)
	}

	items, err = pools.List(context.Background(), client, projection, "does-not-exist")
	if err != nil {
		t.Fatalf("List no match: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("no-match rows = %d, want 0", len(items))
	}
}

func TestList_EmptyPoolHasZeroCounts(t *testing.T) {
	client := cluster.Fake{}
	if err := client.CreatePool(context.Background(), "empty", ""); err != nil {
		t.Fatalf("CreatePool: %v", err)
	}
	projection := inventory.NewProjectionFromIndex(&inventory.Index{ByPool: map[string][]cluster.VM{}})

	items, err := pools.List(context.Background(), client, projection, "empty")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(items) != 1 || items[0].Total != 0 || items[0].Running != 0 || items[0].Stopped != 0 {
		t.Fatalf("rows = %+v, want one zero summary", items)
	}
}

// TestList_WithManagedStoreFlagsManagedPools verifies the Managed flag is
// populated from the store for managed pools and false for unmanaged ones.
//
//nolint:paralleltest // serial: shared fake fixtures
func TestList_WithManagedCheckerFlagsManagedPools(t *testing.T) {
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)
	client := cluster.Fake{}
	snapshot, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	index := inventory.BuildIndex(snapshot)
	projection := inventory.NewProjectionFromIndex(&index)

	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "pools-list-managed.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	if err := st.RegisterManagedPool(context.Background(), "default", cluster.FakePoolAlice); err != nil {
		t.Fatalf("RegisterManagedPool: %v", err)
	}

	items, err := pools.ListWithManaged(context.Background(), client, projection, st, "default", "")
	if err != nil {
		t.Fatalf("ListWithManaged: %v", err)
	}
	managedCount := 0
	for _, item := range items {
		if item.Name == cluster.FakePoolAlice && !item.Managed {
			t.Fatalf("alice should be managed: %+v", item)
		}
		if item.Name == cluster.FakePoolBob && item.Managed {
			t.Fatalf("bob should not be managed: %+v", item)
		}
		if item.Managed {
			managedCount++
		}
	}
	if managedCount != 1 {
		t.Fatalf("managed count = %d, want 1", managedCount)
	}
}
