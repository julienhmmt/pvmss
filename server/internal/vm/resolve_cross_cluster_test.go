//nolint:wsl_v5 // composite identity assertions remain in one focused scenario
package vm_test

import (
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/vm"
	"testing"
)

//nolint:paralleltest,goconst // VM fixtures are shared with the package suite; names prove isolation
func TestResolveCrossCluster(t *testing.T) {
	defaultIndex := indexForVM(cluster.VM{
		VMID: 101, Name: "default-web", Node: "default-node", Pool: "default-pool", Tags: []string{"pvmss"},
	})
	secondaryIndex := indexForVM(cluster.VM{
		VMID: 101, Name: "secondary-web", Node: "secondary-node", Pool: "secondary-pool", Tags: []string{"pvmss"},
	})
	registry := inventory.NewRegistryFromIndexes(map[string]*inventory.Index{
		"default":   defaultIndex,
		"secondary": secondaryIndex,
	})
	actor := auth.Identity{Username: "admin", IsAdmin: true}

	defaultEntity, err := vm.Resolve(registry, actor, "default", 101)
	if err != nil {
		t.Fatalf("Resolve(default): %v", err)
	}
	secondaryEntity, err := vm.Resolve(registry, actor, "secondary", 101)
	if err != nil {
		t.Fatalf("Resolve(secondary): %v", err)
	}
	if defaultEntity.Name == secondaryEntity.Name || defaultEntity.Node == secondaryEntity.Node || defaultEntity.Pool == secondaryEntity.Pool {
		t.Fatalf("entities collided: default=%+v secondary=%+v", defaultEntity, secondaryEntity)
	}
	if defaultIndex.ByVMID[101].Name != "default-web" || secondaryIndex.ByVMID[101].Name != "secondary-web" {
		t.Fatal("resolving one cluster changed the other cluster index")
	}
}

func indexForVM(machine cluster.VM) *inventory.Index {
	index := inventory.BuildIndex(cluster.Snapshot{VMs: []cluster.VM{machine}})
	return &index
}
