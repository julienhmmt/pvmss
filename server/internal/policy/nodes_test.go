package policy_test

import (
	"context"
	"errors"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"slices"
	"testing"
)

func newPolicyServiceWithClient(t *testing.T, client cluster.Client) (*policy.Policy, *inventory.Projection) {
	t.Helper()

	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "policy-nodes.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	snapshot, err := client.Snapshot(context.Background())
	if err == nil {
		index := inventory.BuildIndex(snapshot)
		projection := inventory.NewProjectionFromIndex(&index)

		return policy.New(st, projection, client), projection
	}

	projection := inventory.NewProjection()

	return policy.New(st, projection, client), projection
}

func newPolicyServiceNoClientNoProjection(t *testing.T) *policy.Policy {
	t.Helper()

	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "policy-empty.db")})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	return policy.New(st, nil, nil)
}

func TestNodeCapacities_ReturnsAllDiscoveredNodesSorted(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)

	caps, err := service.NodeCapacities(context.Background(), "default")
	if err != nil {
		t.Fatalf("NodeCapacities: %v", err)
	}

	wantNodes := []string{cluster.FakeNode01, cluster.FakeNode02, cluster.FakeNode03}
	if len(caps) != len(wantNodes) {
		t.Fatalf("capacityacity count = %d, want %d", len(caps), len(wantNodes))
	}

	for i, want := range wantNodes {
		if caps[i].Node != want {
			t.Errorf("caps[%d].Node = %q, want %q", i, caps[i].Node, want)
		}
	}
}

func TestNodeCapacities_ResultsAreSortedByName(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)

	caps, err := service.NodeCapacities(context.Background(), "default")
	if err != nil {
		t.Fatalf("NodeCapacities: %v", err)
	}

	for i := 1; i < len(caps); i++ {
		if caps[i-1].Node > caps[i].Node {
			t.Fatalf("caps not sorted: %q before %q", caps[i-1].Node, caps[i].Node)
		}
	}
}

func TestNodeCapacities_PhysicalFieldsFromDiscovery(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)

	caps, err := service.NodeCapacities(context.Background(), "default")
	if err != nil {
		t.Fatalf("NodeCapacities: %v", err)
	}

	snap, snapErr := (cluster.Fake{}).Snapshot(context.Background())
	if snapErr != nil {
		t.Fatalf("fake snapshot: %v", snapErr)
	}

	nodeMap := make(map[string]cluster.Node, len(snap.Nodes))
	for _, n := range snap.Nodes {
		nodeMap[n.Name] = n
	}

	for _, capacity := range caps {
		discovered, ok := nodeMap[capacity.Node]
		if !ok {
			t.Fatalf("node %q not in snapshot", capacity.Node)
		}

		wantVCPUs := discovered.CPUCores
		if capacity.PhysicalVCPUs != wantVCPUs {
			t.Errorf("node %q PhysicalVCPUs = %d, want %d", capacity.Node, capacity.PhysicalVCPUs, wantVCPUs)
		}

		wantRAM := int(discovered.MemoryTotal / (1024 * 1024 * 1024))
		if capacity.PhysicalRAMGB != wantRAM {
			t.Errorf("node %q PhysicalRAMGB = %d, want %d", capacity.Node, capacity.PhysicalRAMGB, wantRAM)
		}
	}
}

func TestNodeCapacities_UsageMatchesNodeCapacity(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)

	caps, err := service.NodeCapacities(context.Background(), "default")
	if err != nil {
		t.Fatalf("NodeCapacities: %v", err)
	}

	for _, capacity := range caps {
		single, err := service.NodeCapacity(context.Background(), "default", capacity.Node)
		if err != nil {
			t.Fatalf("NodeCapacity(%q): %v", capacity.Node, err)
		}

		if capacity.UsedVMs != single.UsedVMs {
			t.Errorf("node %q UsedVMs: NodeCapacities=%d, NodeCapacity=%d", capacity.Node, capacity.UsedVMs, single.UsedVMs)
		}

		if capacity.UsedVCPUs != single.UsedVCPUs {
			t.Errorf("node %q UsedVCPUs: NodeCapacities=%d, NodeCapacity=%d", capacity.Node, capacity.UsedVCPUs, single.UsedVCPUs)
		}

		if capacity.UsedRAMGB != single.UsedRAMGB {
			t.Errorf("node %q UsedRAMGB: NodeCapacities=%d, NodeCapacity=%d", capacity.Node, capacity.UsedRAMGB, single.UsedRAMGB)
		}
	}
}

type failingSnapshotClient struct {
	cluster.Fake
}

func (failingSnapshotClient) Snapshot(_ context.Context) (cluster.Snapshot, error) {
	return cluster.Snapshot{}, errors.New("discovery unavailable")
}

func TestNodeCapacities_ClientSnapshotErrorPropagates(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyServiceWithClient(t, failingSnapshotClient{})

	_, err := service.NodeCapacities(context.Background(), "default")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestNodeCapacities_NilClientAndNilProjectionReturnsEmpty(t *testing.T) {
	t.Parallel()

	service := newPolicyServiceNoClientNoProjection(t)

	caps, err := service.NodeCapacities(context.Background(), "default")
	if err != nil {
		t.Fatalf("NodeCapacities: %v", err)
	}

	if len(caps) != 0 {
		t.Fatalf("capacityacity count = %d, want 0", len(caps))
	}
}

func TestNodeCapacity_TableDriven(t *testing.T) {
	t.Parallel()

	service, projection := newPolicyService(t)

	nodeMap := buildNodeMapFromFakeSnapshot()

	cases := []struct {
		name string
		node string
	}{
		{"node01", cluster.FakeNode01},
		{"node02", cluster.FakeNode02},
		{"node03", cluster.FakeNode03},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertNodeCapacityForNode(t, service, projection, nodeMap, tc.node)
		})
	}
}

func buildNodeMapFromFakeSnapshot() map[string]cluster.Node {
	snap, _ := (cluster.Fake{}).Snapshot(context.Background())

	nodeMap := make(map[string]cluster.Node, len(snap.Nodes))
	for _, n := range snap.Nodes {
		nodeMap[n.Name] = n
	}

	return nodeMap
}

func assertNodeCapacityForNode(t *testing.T, service *policy.Policy, projection *inventory.Projection, nodeMap map[string]cluster.Node, node string) {
	t.Helper()

	capacity, err := service.NodeCapacity(context.Background(), "default", node)
	if err != nil {
		t.Fatalf("NodeCapacity: %v", err)
	}

	if capacity.Node != node {
		t.Fatalf("Node = %q, want %q", capacity.Node, node)
	}

	want := computeExpectedNodeUsage(projection.Load().ByNode[node])
	assertNodeUsageMatches(t, capacity, want)
	assertNodePhysicalMatches(t, capacity, nodeMap[node])
}

type expectedNodeUsage struct {
	vms      int
	vcpus    int
	ramBytes int64
}

func computeExpectedNodeUsage(machines []cluster.VM) expectedNodeUsage {
	var u expectedNodeUsage

	for _, machine := range machines {
		if !slices.Contains(machine.Tags, "pvmss") {
			continue
		}

		u.vms++
		u.vcpus += machineVCPUs(machine)
		u.ramBytes += machine.MemoryTotal
	}

	return u
}

func machineVCPUs(machine cluster.VM) int {
	if machine.Sockets > 0 && machine.Cores > 0 {
		return machine.Sockets * machine.Cores
	}

	return machine.CPUCores
}

func assertNodeUsageMatches(t *testing.T, capacity policy.Capacity, want expectedNodeUsage) {
	t.Helper()

	if capacity.UsedVMs != want.vms {
		t.Errorf("UsedVMs = %d, want %d", capacity.UsedVMs, want.vms)
	}

	if capacity.UsedVCPUs != want.vcpus {
		t.Errorf("UsedVCPUs = %d, want %d", capacity.UsedVCPUs, want.vcpus)
	}

	wantRAMGB := int(want.ramBytes / (1024 * 1024 * 1024))
	if capacity.UsedRAMGB != wantRAMGB {
		t.Errorf("UsedRAMGB = %d, want %d", capacity.UsedRAMGB, wantRAMGB)
	}
}

func assertNodePhysicalMatches(t *testing.T, capacity policy.Capacity, discovered cluster.Node) {
	t.Helper()

	if capacity.PhysicalVCPUs != discovered.CPUCores {
		t.Errorf("PhysicalVCPUs = %d, want %d", capacity.PhysicalVCPUs, discovered.CPUCores)
	}

	wantPhysRAM := int(discovered.MemoryTotal / (1024 * 1024 * 1024))
	if capacity.PhysicalRAMGB != wantPhysRAM {
		t.Errorf("PhysicalRAMGB = %d, want %d", capacity.PhysicalRAMGB, wantPhysRAM)
	}
}

func TestNodeCapacity_UnknownNodeReturnsZeroUsage(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)

	capacity, err := service.NodeCapacity(context.Background(), "default", "nonexistent-node")
	if err != nil {
		t.Fatalf("NodeCapacity: %v", err)
	}

	if capacity.UsedVMs != 0 || capacity.UsedVCPUs != 0 || capacity.UsedRAMGB != 0 {
		t.Fatalf("usage for unknown node = %+v, want all zero", capacity)
	}

	if capacity.PhysicalVCPUs != 0 || capacity.PhysicalRAMGB != 0 {
		t.Fatalf("physical for unknown node = %+v, want all zero", capacity)
	}
}
