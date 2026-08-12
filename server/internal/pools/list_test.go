//nolint:paralleltest,wsl_v5 // tests use shared fake fixtures and table assertions
package pools_test

import (
	"context"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/pools"
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
