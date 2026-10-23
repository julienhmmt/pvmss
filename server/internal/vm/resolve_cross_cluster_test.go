//nolint:wsl_v5 // composite identity assertions remain in one focused scenario
package vm_test

import (
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/vm"
	"testing"
)

//nolint:paralleltest // VM fixtures are shared with the package suite; names prove isolation
func TestResolveCrossCluster(t *testing.T) {
	defaultIndex := indexForVM(cluster.VM{
		VMID: 101, Name: "default-web", Node: "default-node", Pool: "default-pool", Tags: []string{"pvmss"},
	})
	secondaryIndex := indexForVM(cluster.VM{
		VMID: 101, Name: "secondary-web", Node: "secondary-node", Pool: "secondary-pool", Tags: []string{"pvmss"},
	})
	registry := inventory.NewRegistryFromIndexes(map[string]*inventory.Index{
		testClusterName: defaultIndex,
		"secondary":     secondaryIndex,
	})
	actor := auth.Identity{Username: "admin", IsAdmin: true}

	defaultEntity, err := vm.Resolve(registry, actor, testClusterName, 101)
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

// TestResolve_NonAdminPinnedToSignInCluster: pool names are per cluster, so a
// user signed in to one cluster must not reach a same-named pool's VM on
// another cluster.
//
//nolint:paralleltest // VM fixtures are shared with the package suite
func TestResolve_NonAdminPinnedToSignInCluster(t *testing.T) {
	registry := inventory.NewRegistryFromIndexes(map[string]*inventory.Index{
		testClusterName:          indexForVM(cluster.VM{VMID: 101, Node: crossClusterNodeA, Pool: "pvmss-x", Tags: []string{testPvmssTag}}),
		crossClusterSecondaryKey: indexForVM(cluster.VM{VMID: 102, Node: crossClusterNodeB, Pool: "pvmss-x", Tags: []string{testPvmssTag}}),
	})
	user := auth.Identity{Username: "x@pve", Pool: "pvmss-x", Cluster: testClusterName, Provider: auth.ProviderPVE}

	if _, err := vm.Resolve(registry, user, testClusterName, 101); err != nil {
		t.Fatalf("Resolve(own cluster): %v", err)
	}
	if _, err := vm.Resolve(registry, user, crossClusterSecondaryKey, 102); !errors.Is(err, vm.ErrNotFound) {
		t.Fatalf("Resolve(other cluster) err = %v, want ErrNotFound", err)
	}

	// An empty cluster name must not fall through to whatever index the
	// caller picked for it.
	if _, err := vm.Resolve(registry, user, "", 101); !errors.Is(err, vm.ErrNotFound) {
		t.Fatalf("Resolve(empty cluster) err = %v, want ErrNotFound", err)
	}

	admin := auth.Identity{Username: "root@pam", IsAdmin: true, Cluster: testClusterName}
	if _, err := vm.Resolve(registry, admin, crossClusterSecondaryKey, 102); err != nil {
		t.Fatalf("Resolve(admin, other cluster): %v", err)
	}
}

func TestTargetCluster(t *testing.T) {
	t.Parallel()

	user := auth.Identity{Username: "x@pve", Cluster: "a"}
	cases := []struct {
		name      string
		actor     auth.Identity
		requested string
		want      string
		wantOK    bool
	}{
		{"user, own cluster", user, "a", "a", true},
		{"user, empty defaults to own", user, "", "a", true},
		{"user, other cluster refused", user, "b", "b", false},
		{"admin, any cluster", auth.Identity{IsAdmin: true, Cluster: "a"}, "b", "b", true},
		{"legacy session without cluster", auth.Identity{Username: "x@pve"}, "b", "b", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := vm.TargetCluster(tc.actor, tc.requested)
			if got != tc.want || ok != tc.wantOK {
				t.Fatalf("TargetCluster = (%q, %v), want (%q, %v)", got, ok, tc.want, tc.wantOK)
			}
		})
	}
}
