package httpapi

import (
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"slices"
	"testing"
)

// A node with no approved storage or bridge cannot host a new VM; the
// create catalog does not offer it.
func TestCatalogNodeNames_DropsNodesThatCannotHostAVM(t *testing.T) {
	t.Parallel()

	const big = "big"

	resources := catalog.Resources{
		Nodes:    []catalog.Node{{Name: big}, {Name: "nostorage"}, {Name: "nobridge"}},
		Storages: []catalog.Storage{{Name: "local", Node: big}, {Name: "local", Node: "nobridge"}},
		Bridges:  []catalog.Bridge{{Name: "vmbr0", Node: big}, {Name: "vmbr0", Node: "nostorage"}},
	}
	snap := cluster.Snapshot{Nodes: []cluster.Node{{Name: big}, {Name: "nostorage"}, {Name: "nobridge"}}}

	if got := catalogNodeNames(resources, snap); !slices.Equal(got, []string{big}) {
		t.Errorf("nodes = %v, want [big]", got)
	}
}
