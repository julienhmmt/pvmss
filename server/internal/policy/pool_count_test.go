package policy

import (
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"testing"
)

func TestManagedPoolVMCountIgnoresUntaggedVMs(t *testing.T) {
	t.Parallel()

	idx := inventory.BuildIndexForCluster("c", cluster.Snapshot{VMs: []cluster.VM{
		{VMID: 100, Node: "n", Pool: "pvmss-p", Tags: []string{"pvmss"}},
		{VMID: 101, Node: "n", Pool: "pvmss-p"},
		{VMID: 102, Node: "n", Pool: "pvmss-p", Tags: []string{"web", "pvmss"}},
	}})

	if got := ManagedPoolVMCount(&idx, "pvmss-p"); got != 2 {
		t.Fatalf("ManagedPoolVMCount = %d, want 2", got)
	}
}
