//nolint:wsl_v5 // cross-cluster query assertions remain table-oriented
package vm_test

import (
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/vm"
	"slices"
	"testing"
)

const (
	crossClusterDefaultName   = "default-web"
	crossClusterSecondaryName = "secondary-web"
	crossClusterSecondaryKey  = "secondary"
	crossClusterPoolAlice     = "pool-alice"
)

//nolint:paralleltest // VM fixtures are shared with the package suite; names prove isolation
func TestList_CrossClusterPoolMerge(t *testing.T) {
	defaultIndex := inventory.BuildIndexForCluster("default", cluster.Snapshot{VMs: []cluster.VM{
		{VMID: 101, Name: crossClusterDefaultName, Node: "node-a", Pool: crossClusterPoolAlice, Tags: []string{testPvmssTag}},
	}})
	secondaryIndex := inventory.BuildIndexForCluster(crossClusterSecondaryKey, cluster.Snapshot{VMs: []cluster.VM{
		{VMID: 101, Name: crossClusterSecondaryName, Node: "node-b", Pool: crossClusterPoolAlice, Tags: []string{testPvmssTag}},
	}})
	thirdIndex := inventory.BuildIndexForCluster("third", cluster.Snapshot{VMs: []cluster.VM{
		{VMID: 202, Name: "other-web", Node: "node-c", Pool: "pool-bob", Tags: []string{testPvmssTag}},
	}})
	registry := inventory.NewRegistryFromIndexes(map[string]*inventory.Index{
		"default": &defaultIndex, crossClusterSecondaryKey: &secondaryIndex, "third": &thirdIndex,
	})

	result, err := vm.List(registry, vm.ListQuery{}, auth.Identity{Username: "alice", Pool: crossClusterPoolAlice}, -1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if got := names(result); !slices.Equal(got, []string{crossClusterDefaultName, crossClusterSecondaryName}) {
		t.Fatalf("merged names = %v, want both Alice VMs", got)
	}
	if result.Items[0].Cluster == "" || result.Items[0].Cluster == result.Items[1].Cluster {
		t.Fatalf("cluster labels = %+v, want distinct labels", result.Items)
	}

	filtered, err := vm.List(registry, vm.ListQuery{Cluster: crossClusterSecondaryKey}, auth.Identity{Username: "alice", Pool: crossClusterPoolAlice}, -1)
	if err != nil {
		t.Fatalf("List(cluster): %v", err)
	}
	if len(filtered.Items) != 1 || filtered.Items[0].Cluster != crossClusterSecondaryKey {
		t.Fatalf("filtered items = %+v, want secondary only", filtered.Items)
	}
}

// TestList_AdminScopeAllSpansEveryCluster — T017/spec.md User Story 1
// acceptance scenario 5: an admin with scope=all and no cluster filter sees
// every VM from every configured cluster, not only the pools their own
// identity happens to match — proves the adminAll branch merges across
// inventory.Registry.All() the same way the mine-scope branch already does.
//
//nolint:paralleltest // VM fixtures are shared with the package suite; names prove isolation
func TestList_AdminScopeAllSpansEveryCluster(t *testing.T) {
	defaultIndex := inventory.BuildIndexForCluster("default", cluster.Snapshot{VMs: []cluster.VM{
		{VMID: 101, Name: crossClusterDefaultName, Node: "node-a", Pool: crossClusterPoolAlice, Tags: []string{testPvmssTag}},
	}})
	secondaryIndex := inventory.BuildIndexForCluster(crossClusterSecondaryKey, cluster.Snapshot{VMs: []cluster.VM{
		{VMID: 101, Name: crossClusterSecondaryName, Node: "node-b", Pool: "pool-bob", Tags: []string{testPvmssTag}},
	}})
	thirdIndex := inventory.BuildIndexForCluster("third", cluster.Snapshot{VMs: []cluster.VM{
		{VMID: 202, Name: "third-web", Node: "node-c", Pool: "pool-carol", Tags: []string{testPvmssTag}},
	}})
	registry := inventory.NewRegistryFromIndexes(map[string]*inventory.Index{
		"default": &defaultIndex, crossClusterSecondaryKey: &secondaryIndex, "third": &thirdIndex,
	})

	// The admin's own identity matches none of these pools — scope=all must
	// still return every VM from every cluster, proving admin visibility is
	// not accidentally routed through the same pool-membership merge mine
	// scope uses.
	admin := auth.Identity{Username: "root", Pool: "no-such-pool", IsAdmin: true}
	result, err := vm.List(registry, vm.ListQuery{Scope: vm.ScopeAll}, admin, -1)
	if err != nil {
		t.Fatalf("List(scope=all): %v", err)
	}
	if got := names(result); !slices.Equal(got, []string{crossClusterDefaultName, crossClusterSecondaryName, "third-web"}) {
		t.Fatalf("admin scope=all names = %v, want every cluster's VM", got)
	}
	clusters := make(map[string]bool, len(result.Items))
	for _, item := range result.Items {
		clusters[item.Cluster] = true
	}
	if len(clusters) != 3 || !clusters["default"] || !clusters[crossClusterSecondaryKey] || !clusters["third"] {
		t.Fatalf("admin scope=all cluster labels = %v, want default+secondary+third all present", clusters)
	}
}

func names(result vm.ListResult) []string {
	out := make([]string, 0, len(result.Items))
	for _, machine := range result.Items {
		out = append(out, machine.Name)
	}
	return out
}
