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
		t.Fatalf("capacity count = %d, want %d", len(caps), len(wantNodes))
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

	for _, cap := range caps {
		discovered, ok := nodeMap[cap.Node]
		if !ok {
			t.Fatalf("node %q not in snapshot", cap.Node)
		}

		wantVCPUs := discovered.CPUCores
		if cap.PhysicalVCPUs != wantVCPUs {
			t.Errorf("node %q PhysicalVCPUs = %d, want %d", cap.Node, cap.PhysicalVCPUs, wantVCPUs)
		}

		wantRAM := int(discovered.MemoryTotal / (1024 * 1024 * 1024))
		if cap.PhysicalRAMGB != wantRAM {
			t.Errorf("node %q PhysicalRAMGB = %d, want %d", cap.Node, cap.PhysicalRAMGB, wantRAM)
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

	for _, cap := range caps {
		single, err := service.NodeCapacity(context.Background(), "default", cap.Node)
		if err != nil {
			t.Fatalf("NodeCapacity(%q): %v", cap.Node, err)
		}

		if cap.UsedVMs != single.UsedVMs {
			t.Errorf("node %q UsedVMs: NodeCapacities=%d, NodeCapacity=%d", cap.Node, cap.UsedVMs, single.UsedVMs)
		}

		if cap.UsedVCPUs != single.UsedVCPUs {
			t.Errorf("node %q UsedVCPUs: NodeCapacities=%d, NodeCapacity=%d", cap.Node, cap.UsedVCPUs, single.UsedVCPUs)
		}

		if cap.UsedRAMGB != single.UsedRAMGB {
			t.Errorf("node %q UsedRAMGB: NodeCapacities=%d, NodeCapacity=%d", cap.Node, cap.UsedRAMGB, single.UsedRAMGB)
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
		t.Fatalf("capacity count = %d, want 0", len(caps))
	}
}

func TestNodeCapacity_TableDriven(t *testing.T) {
	t.Parallel()

	service, projection := newPolicyService(t)

	snap, _ := (cluster.Fake{}).Snapshot(context.Background())
	nodeMap := make(map[string]cluster.Node, len(snap.Nodes))
	for _, n := range snap.Nodes {
		nodeMap[n.Name] = n
	}

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

			cap, err := service.NodeCapacity(context.Background(), "default", tc.node)
			if err != nil {
				t.Fatalf("NodeCapacity: %v", err)
			}

			if cap.Node != tc.node {
				t.Fatalf("Node = %q, want %q", cap.Node, tc.node)
			}

			var wantVMs, wantVCPUs int
			var wantRAMBytes int64
			for _, machine := range projection.Load().ByNode[tc.node] {
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

			if cap.UsedVMs != wantVMs {
				t.Errorf("UsedVMs = %d, want %d", cap.UsedVMs, wantVMs)
			}
			if cap.UsedVCPUs != wantVCPUs {
				t.Errorf("UsedVCPUs = %d, want %d", cap.UsedVCPUs, wantVCPUs)
			}
			wantRAMGB := int(wantRAMBytes / (1024 * 1024 * 1024))
			if cap.UsedRAMGB != wantRAMGB {
				t.Errorf("UsedRAMGB = %d, want %d", cap.UsedRAMGB, wantRAMGB)
			}

			discovered := nodeMap[tc.node]
			if cap.PhysicalVCPUs != discovered.CPUCores {
				t.Errorf("PhysicalVCPUs = %d, want %d", cap.PhysicalVCPUs, discovered.CPUCores)
			}
			wantPhysRAM := int(discovered.MemoryTotal / (1024 * 1024 * 1024))
			if cap.PhysicalRAMGB != wantPhysRAM {
				t.Errorf("PhysicalRAMGB = %d, want %d", cap.PhysicalRAMGB, wantPhysRAM)
			}
		})
	}
}

func TestNodeCapacity_UnknownNodeReturnsZeroUsage(t *testing.T) {
	t.Parallel()

	service, _ := newPolicyService(t)

	cap, err := service.NodeCapacity(context.Background(), "default", "nonexistent-node")
	if err != nil {
		t.Fatalf("NodeCapacity: %v", err)
	}

	if cap.UsedVMs != 0 || cap.UsedVCPUs != 0 || cap.UsedRAMGB != 0 {
		t.Fatalf("usage for unknown node = %+v, want all zero", cap)
	}

	if cap.PhysicalVCPUs != 0 || cap.PhysicalRAMGB != 0 {
		t.Fatalf("physical for unknown node = %+v, want all zero", cap)
	}
}
