package policy_test

import (
	"context"
	"path/filepath"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"slices"
	"testing"
)

const testUserAlice = "alice"

func TestPolicyReads_SeededDefaults(t *testing.T) {
	t.Parallel()

	service, projection := newPolicyService(t)
	ctx := context.Background()

	gabarit, err := service.Gabarit(ctx, "default")
	if err != nil {
		t.Fatalf("Gabarit: %v", err)
	}

	want := policy.Gabarit{
		MaxSockets:      4,
		MaxCores:        8,
		MaxMemoryMB:     16384,
		MaxDiskPerVMGB:  500,
		MaxNetworkCards: 4,
		MaxSnapshots:    5,
		AllowCustomYAML: true,
	}
	if gabarit != want {
		t.Fatalf("gabarit = %+v, want %+v", gabarit, want)
	}

	quota, err := service.Quota(ctx, "default", auth.Identity{Username: testUserAlice, Pool: cluster.FakePoolAlice})
	if err != nil {
		t.Fatalf("Quota: %v", err)
	}

	if quota.Allowed != -1 || quota.Used != len(projection.Load().ByPool[cluster.FakePoolAlice]) {
		t.Fatalf("quota = %+v, want allowed -1 and current pool usage", quota)
	}

	capacity, err := service.NodeCapacity(ctx, "default", cluster.FakeNode01)
	if err != nil {
		t.Fatalf("NodeCapacity: %v", err)
	}

	if capacity.MaxVMs != 0 || capacity.MaxVCPUs != 0 || capacity.MaxRAMGB != 0 || capacity.MaxDiskGB != 0 {
		t.Fatalf("capacity defaults = %+v, want no caps", capacity)
	}
}

func TestNodeCapacity_AggregatesPvmssBytesAndExcludesUntagged(t *testing.T) {
	t.Parallel()

	service, projection := newPolicyService(t)
	for _, node := range []string{cluster.FakeNode01, cluster.FakeNode02} {
		t.Run(node, func(t *testing.T) {
			t.Parallel()
			runNodeCapacityCase(t, service, projection, node)
		})
	}
}

// runNodeCapacityCase computes the expected aggregated usage from the
// projection and asserts it matches the policy service's NodeCapacity result.
// Extracted from TestNodeCapacity_AggregatesPvmssBytesAndExcludesUntagged to
// keep its Cognitive Complexity under the SonarQube go:S3776 threshold.
func runNodeCapacityCase(t *testing.T, service *policy.Policy, projection *inventory.Projection, node string) {
	t.Helper()

	var (
		wantVMs, wantVCPUs int
		wantRAMBytes       int64
	)

	for _, machine := range projection.Load().ByNode[node] {
		if !slices.Contains(machine.Tags, "pvmss") {
			continue
		}

		wantVMs++

		if machine.Sockets > 0 && machine.Cores > 0 {
			wantVCPUs += machine.Sockets * machine.Cores
		} else {
			wantVCPUs += machine.CPUCores
		}

		wantRAMBytes += machine.MemoryTotal
	}

	got, err := service.NodeCapacity(context.Background(), "default", node)
	if err != nil {
		t.Fatalf("NodeCapacity: %v", err)
	}

	if got.UsedVMs != wantVMs || got.UsedVCPUs != wantVCPUs || got.UsedRAMGB != int(wantRAMBytes/(1024*1024*1024)) {
		t.Fatalf("capacity = %+v, want VMs=%d vCPUs=%d RAM=%d GB", got, wantVMs, wantVCPUs, wantRAMBytes/(1024*1024*1024))
	}
}

func newPolicyService(t *testing.T) (*policy.Policy, *inventory.Projection) {
	t.Helper()

	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "policy.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	fake := cluster.Fake{}

	snapshot, err := fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("fake snapshot: %v", err)
	}

	index := inventory.BuildIndex(snapshot)
	projection := inventory.NewProjectionFromIndex(&index)

	return policy.New(st, projection, fake), projection
}
