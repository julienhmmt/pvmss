//nolint:paralleltest,wsl_v5 // tests share a per-test store and index fixtures
package adoption_test

import (
	"context"
	"pvmss/server/internal/adoption"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"testing"
)

func adoptFixture(t *testing.T) (context.Context, stubPoolLister, *inventory.Projection, *store.Store) {
	t.Helper()
	ctx := context.Background()
	lister := stubPoolLister{pools: []cluster.Pool{
		{Name: alicePool, Comment: "Alice's pool"},
		{Name: bobPool},
		{Name: teamXPool},
	}}
	projection := projectionOf(
		cluster.VM{VMID: 100, Name: web01Name, Node: "n1", Status: cluster.VMRunning, Pool: alicePool, Tags: []string{pvmssTag, "web", "db"}},
		cluster.VM{VMID: 101, Name: "web-02", Node: "n1", Status: cluster.VMStopped, Pool: alicePool, Tags: []string{pvmssTag, "db"}},
	)
	return ctx, lister, projection, newStore(t)
}

// TestAdopt_RegistersSelectedPools: selected orphan pools land in
// managed_pools; names that are not orphans (foreign pools, unknown names)
// are ignored rather than erroring.
func TestAdopt_RegistersSelectedPools(t *testing.T) {
	ctx, lister, projection, st := adoptFixture(t)

	result, err := adoption.Adopt(ctx, lister, projection, st, "default", adoption.Selection{
		Pools: []string{alicePool, teamXPool, "pvmss-ghost"},
	})
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if len(result.Pools) != 1 || result.Pools[0] != alicePool {
		t.Fatalf("pools = %+v, want [pvmss-alice]", result.Pools)
	}

	managed, err := st.ManagedPoolNames(ctx, "default")
	if err != nil {
		t.Fatalf("ManagedPoolNames: %v", err)
	}
	if _, ok := managed[alicePool]; !ok {
		t.Fatal("pvmss-alice should be registered")
	}
	if _, ok := managed[teamXPool]; ok {
		t.Fatal("team-x must not be registered")
	}
}

// TestAdopt_CreatesSelectedTagsWithPaletteColors: adopted tags are created in
// catalog_tags with the same palette colors the report suggested.
func TestAdopt_CreatesSelectedTagsWithPaletteColors(t *testing.T) {
	ctx, lister, projection, st := adoptFixture(t)

	report, err := adoption.Scan(ctx, lister, projection, st, "default")
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	suggested := map[string]string{}
	for _, tag := range report.Tags {
		suggested[tag.Name] = tag.Color
	}

	result, err := adoption.Adopt(ctx, lister, projection, st, "default", adoption.Selection{
		Tags: []string{"db", "ghost-tag"},
	})
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if len(result.Tags) != 1 || result.Tags[0].Name != "db" || result.Tags[0].Color != suggested["db"] {
		t.Fatalf("tags = %+v, want db with suggested color %q", result.Tags, suggested["db"])
	}

	exists, err := st.TagExists(ctx, "default", "db")
	if err != nil {
		t.Fatalf("TagExists: %v", err)
	}
	if !exists {
		t.Fatal("db should exist in catalog_tags")
	}
}

// TestAdopt_Idempotent: re-running Adopt on already-registered pools and
// already-created tags reports them adopted again without error.
func TestAdopt_Idempotent(t *testing.T) {
	ctx, lister, projection, st := adoptFixture(t)
	sel := adoption.Selection{Pools: []string{alicePool}, Tags: []string{"db"}}

	if _, err := adoption.Adopt(ctx, lister, projection, st, "default", sel); err != nil {
		t.Fatalf("first Adopt: %v", err)
	}
	managed, err := st.ManagedPoolNames(ctx, "default")
	if err != nil {
		t.Fatalf("ManagedPoolNames: %v", err)
	}
	if _, ok := managed[bobPool]; ok {
		t.Fatal("fixture needs pvmss-bob unmanaged for the second adopt")
	}

	result, err := adoption.Adopt(ctx, lister, projection, st, "default", adoption.Selection{
		Pools: []string{alicePool, bobPool},
		Tags:  []string{"db"},
	})
	if err != nil {
		t.Fatalf("second Adopt: %v", err)
	}
	if len(result.Pools) != 1 || result.Pools[0] != bobPool {
		t.Fatalf("pools = %+v, want only pvmss-bob newly adopted", result.Pools)
	}
	if len(result.Tags) != 0 {
		t.Fatalf("tags = %+v, want none - db was already adopted", result.Tags)
	}
}
