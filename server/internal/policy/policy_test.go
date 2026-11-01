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

	return policyWithStaticProjection(st, projection, fake), projection
}

func TestPolicyReads_MissingRow_ReturnsDefaults(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)
	ctx := context.Background()

	// "ghost" has no vm_limits row: reads fall back to the shipped defaults
	// instead of failing the whole request.
	gabarit, err := service.Gabarit(ctx, "ghost")
	if err != nil {
		t.Fatalf("Gabarit: %v", err)
	}

	if gabarit != policy.DefaultGabarit() {
		t.Fatalf("gabarit = %+v, want %+v", gabarit, policy.DefaultGabarit())
	}
}

const (
	dimVMs   = "vms"
	dimVCPUs = "vcpus"
	dimRAM   = "ram"
	dimDisk  = "disk"
)

// NodeOverflow is the pure node-cap predicate used by placement: it must
// report exactly the dimensions the new VM would overflow, counting only
// pvmss-tagged usage, and nothing when every cap is zero.
func TestNodeOverflow(t *testing.T) {
	t.Parallel()

	gb := int64(1024 * 1024 * 1024)
	pvmssVM := cluster.VM{Node: "n1", Sockets: 1, Cores: 2, MemoryTotal: gb, DiskTotal: 2 * gb, Tags: []string{"pvmss"}}
	foreignVM := cluster.VM{Node: "n1", Sockets: 4, Cores: 8, MemoryTotal: 32 * gb, DiskTotal: 200 * gb}

	cases := []struct {
		name    string
		byNode  []cluster.VM
		caps    store.NodePolicyRow
		machine cluster.VM
		want    []string
	}{
		{name: "no caps never overflows", byNode: []cluster.VM{pvmssVM}, caps: store.NodePolicyRow{}, machine: pvmssVM, want: nil},
		{name: "vm count only", byNode: []cluster.VM{pvmssVM}, caps: store.NodePolicyRow{MaxVMs: 1}, machine: pvmssVM, want: []string{dimVMs}},
		{name: "untagged usage is ignored", byNode: []cluster.VM{foreignVM}, caps: store.NodePolicyRow{MaxVMs: 2}, machine: pvmssVM, want: nil},
		{name: "vcpu cap", byNode: []cluster.VM{pvmssVM}, caps: store.NodePolicyRow{MaxVCPUs: 3}, machine: pvmssVM, want: []string{dimVCPUs}},
		{name: "ram cap", byNode: []cluster.VM{pvmssVM}, caps: store.NodePolicyRow{MaxRAMGB: 1}, machine: pvmssVM, want: []string{dimRAM}},
		{name: "disk cap", byNode: []cluster.VM{pvmssVM}, caps: store.NodePolicyRow{MaxDiskGB: 3}, machine: pvmssVM, want: []string{dimDisk}},
		{name: "all dimensions at once", byNode: []cluster.VM{pvmssVM}, caps: store.NodePolicyRow{MaxVMs: 1, MaxVCPUs: 1, MaxRAMGB: 1, MaxDiskGB: 1}, machine: pvmssVM, want: []string{dimVMs, dimVCPUs, dimRAM, dimDisk}},
		{name: "boundary is not overflow", byNode: []cluster.VM{pvmssVM}, caps: store.NodePolicyRow{MaxVMs: 2, MaxVCPUs: 4, MaxRAMGB: 2, MaxDiskGB: 4}, machine: pvmssVM, want: nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			index := &inventory.Index{ByNode: map[string][]cluster.VM{"n1": tc.byNode}}

			if got := policy.NodeOverflow(index, tc.caps, "n1", tc.machine); !slices.Equal(got, tc.want) {
				t.Errorf("NodeOverflow = %v, want %v", got, tc.want)
			}
		})
	}
}
