package policy_test

import (
	"context"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"testing"
)

func TestPolicyCompat_SeededValuesMatchPriorTranches(t *testing.T) { //nolint:gocyclo,paralleltest // serial: shared SQLite fixture; checks every seeded invariant
	service, _ := newPolicyService(t)
	ctx := context.Background()

	gabarit, err := service.Gabarit(ctx, "default")
	if err != nil {
		t.Fatalf("Gabarit: %v", err)
	}

	if gabarit.MaxSockets != 4 || gabarit.MaxCores != 8 || gabarit.MaxMemoryMB != 16384 || gabarit.MaxDiskPerVMGB != 500 || gabarit.MaxNetworkCards != 4 || gabarit.MaxSnapshots != 5 || !gabarit.AllowCustomYaml {
		t.Fatalf("seeded gabarit = %+v", gabarit)
	}

	quota, err := service.Quota(ctx, "default", auth.Identity{Username: testUserAlice, Pool: cluster.FakePoolAlice})
	if err != nil {
		t.Fatalf("Quota: %v", err)
	}

	if quota.Allowed != -1 {
		t.Fatalf("seeded quota = %+v, want unlimited", quota)
	}

	for _, node := range []string{cluster.FakeNode01, cluster.FakeNode02} {
		capacity, capacityErr := service.NodeCapacity(ctx, "default", node)
		if capacityErr != nil {
			t.Fatalf("NodeCapacity(%q): %v", node, capacityErr)
		}

		if capacity.MaxVMs != 0 || capacity.MaxVCPUs != 0 || capacity.MaxRAMGB != 0 || capacity.MaxDiskGB != 0 {
			t.Fatalf("seeded capacity(%q) = %+v, want no caps", node, capacity)
		}
	}
}
