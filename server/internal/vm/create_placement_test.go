package vm

import (
	"context"
	"errors"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/policy"
	"testing"
)

// Placement test fixture literals (goconst: extracted as constants so repeated
// string literals do not trigger the linter).
const (
	placementNodeA  = "pve-a"
	placementNodeB  = "pve-b"
	placementNodeC  = "pve-c"
	placementBridge = "vmbr0"
)

// placementResources builds a three-node catalog where each node has one
// storage and one bridge, so the hard filters (node has storage) always pass.
// No ISOs — these tests focus on scoring, not ISO locality.
func placementResources() catalog.Resources {
	return catalog.Resources{
		Nodes: []catalog.Node{
			{Name: placementNodeA},
			{Name: placementNodeB},
			{Name: placementNodeC},
		},
		Storages: []catalog.Storage{
			{Name: "storage-a", Node: placementNodeA},
			{Name: "storage-b", Node: placementNodeB},
			{Name: "storage-c", Node: placementNodeC},
		},
		Bridges: []catalog.Bridge{
			{Name: placementBridge, Node: placementNodeA},
			{Name: placementBridge, Node: placementNodeB},
			{Name: placementBridge, Node: placementNodeC},
		},
	}
}

// placementCapacities builds capacities where pve-b is the most free in RAM
// (the test criterion from tasks.md). pve-a is loaded, pve-b is empty, pve-c
// is half-loaded.
func placementCapacities() map[string]policy.Capacity {
	return map[string]policy.Capacity{
		placementNodeA: {Node: placementNodeA, PhysicalVCPUs: 16, PhysicalRAMGB: 64, MaxDiskGB: 1000, UsedVCPUs: 14, UsedRAMGB: 60, UsedDiskGB: 900},
		placementNodeB: {Node: placementNodeB, PhysicalVCPUs: 16, PhysicalRAMGB: 64, MaxDiskGB: 1000, UsedVCPUs: 0, UsedRAMGB: 0, UsedDiskGB: 0},
		placementNodeC: {Node: placementNodeC, PhysicalVCPUs: 16, PhysicalRAMGB: 64, MaxDiskGB: 1000, UsedVCPUs: 8, UsedRAMGB: 32, UsedDiskGB: 500},
	}
}

// TestResolveResources_PlacementSelectsMostFreeNode — three nodes, middle one
// (pve-b) most free in RAM → it is selected, not Nodes[0] (US3/issue-04).
func TestResolveResources_PlacementSelectsMostFreeNode(t *testing.T) {
	t.Parallel()

	req := CreateRequest{CPUCores: 2, MemoryMB: 2048, Disk: DiskRequest{SizeGB: 20}}
	caps := placementCapacities()

	node, _, _, err := resolveResources(req, placementResources(), caps, nil)
	if err != nil {
		t.Fatalf("resolveResources: %v", err)
	}

	if node != placementNodeB {
		t.Errorf("node = %q, want %q (most free RAM)", node, placementNodeB)
	}
}

// TestResolveResources_PlacementEqualScoreCatalogOrderBreaksTie — at equal
// score, catalog order breaks the tie (reproducible selection).
func TestResolveResources_PlacementEqualScoreCatalogOrderBreaksTie(t *testing.T) {
	t.Parallel()

	req := CreateRequest{CPUCores: 2, MemoryMB: 2048, Disk: DiskRequest{SizeGB: 20}}
	resources := placementResources()
	// All nodes identical → equal score → first in catalog order wins.
	caps := map[string]policy.Capacity{
		placementNodeA: {Node: placementNodeA, PhysicalVCPUs: 16, PhysicalRAMGB: 64, MaxDiskGB: 1000, UsedVCPUs: 8, UsedRAMGB: 32, UsedDiskGB: 500},
		placementNodeB: {Node: placementNodeB, PhysicalVCPUs: 16, PhysicalRAMGB: 64, MaxDiskGB: 1000, UsedVCPUs: 8, UsedRAMGB: 32, UsedDiskGB: 500},
		placementNodeC: {Node: placementNodeC, PhysicalVCPUs: 16, PhysicalRAMGB: 64, MaxDiskGB: 1000, UsedVCPUs: 8, UsedRAMGB: 32, UsedDiskGB: 500},
	}

	node, _, _, err := resolveResources(req, resources, caps, nil)
	if err != nil {
		t.Fatalf("resolveResources: %v", err)
	}

	if node != placementNodeA {
		t.Errorf("node = %q, want %q (catalog order tie-break)", node, placementNodeA)
	}
}

// TestResolveResources_PlacementExplicitNodeSkipsScoring — when the request
// specifies a node, scoring does not apply; the explicit node is used.
func TestResolveResources_PlacementExplicitNodeSkipsScoring(t *testing.T) {
	t.Parallel()

	req := CreateRequest{Node: placementNodeA, CPUCores: 2, MemoryMB: 2048, Disk: DiskRequest{SizeGB: 20}}

	node, _, _, err := resolveResources(req, placementResources(), placementCapacities(), nil)
	if err != nil {
		t.Fatalf("resolveResources: %v", err)
	}

	if node != placementNodeA {
		t.Errorf("node = %q, want %q (explicit node, scoring skipped)", node, placementNodeA)
	}
}

// TestResolveResources_PlacementNoCapacityDataSelectsFirst — when no capacity
// data is available (nil map), all nodes score 0 and catalog order breaks the
// tie. A node is still returned (bonus, not barrier).
func TestResolveResources_PlacementNoCapacityDataSelectsFirst(t *testing.T) {
	t.Parallel()

	req := CreateRequest{CPUCores: 2, MemoryMB: 2048, Disk: DiskRequest{SizeGB: 20}}

	node, _, _, err := resolveResources(req, placementResources(), nil, nil)
	if err != nil {
		t.Fatalf("resolveResources: %v", err)
	}

	if node != placementNodeA {
		t.Errorf("node = %q, want %q (first node, no capacity data)", node, placementNodeA)
	}
}

// TestResolveResources_PlacementHardFilterNoStorage — a node with no approved
// storage is filtered out of candidates.
func TestResolveResources_PlacementHardFilterNoStorage(t *testing.T) {
	t.Parallel()

	resources := catalog.Resources{
		Nodes: []catalog.Node{
			{Name: placementNodeA},
			{Name: placementNodeB},
		},
		Storages: []catalog.Storage{
			// Only pve-b has storage.
			{Name: "storage-b", Node: placementNodeB},
		},
		Bridges: []catalog.Bridge{
			{Name: "vmbr0", Node: placementNodeA},
			{Name: "vmbr0", Node: placementNodeB},
		},
	}
	req := CreateRequest{CPUCores: 2, MemoryMB: 2048, Disk: DiskRequest{SizeGB: 20}}

	node, _, _, err := resolveResources(req, resources, nil, nil)
	if err != nil {
		t.Fatalf("resolveResources: %v", err)
	}

	if node != placementNodeB {
		t.Errorf("node = %q, want %q (only node with storage)", node, placementNodeB)
	}
}

// TestBestStorageOnNode_PicksMostFree — bestStorageOnNode picks the storage
// with the most free space, not the first in catalog order.
func TestBestStorageOnNode_PicksMostFree(t *testing.T) {
	t.Parallel()

	resources := catalog.Resources{
		Storages: []catalog.Storage{
			{Name: "slow-disk", Node: placementNodeA},
			{Name: "fast-ssd", Node: placementNodeA},
		},
	}
	free := map[string]int64{
		"slow-disk": 100 * 1024 * 1024 * 1024,
		"fast-ssd":  500 * 1024 * 1024 * 1024,
	}

	got := bestStorageOnNode(resources, placementNodeA, free)
	if got != "fast-ssd" {
		t.Errorf("storage = %q, want %q (most free space)", got, "fast-ssd")
	}
}

// TestBestStorageOnNode_FallsBackToCatalogOrder — when free-space data is
// unavailable (all zero), catalog order breaks the tie.
func TestBestStorageOnNode_FallsBackToCatalogOrder(t *testing.T) {
	t.Parallel()

	resources := catalog.Resources{
		Storages: []catalog.Storage{
			{Name: "first-storage", Node: placementNodeA},
			{Name: "second-storage", Node: placementNodeA},
		},
	}

	got := bestStorageOnNode(resources, placementNodeA, nil)
	if got != "first-storage" {
		t.Errorf("storage = %q, want %q (catalog order fallback)", got, "first-storage")
	}
}

// TestScoreNode_FitBonus — a node that fits the VM gets +1 bonus. The two
// nodes have identical free fractions so the only difference is the bonus.
func TestScoreNode_FitBonus(t *testing.T) {
	t.Parallel()

	req := CreateRequest{Sockets: 2, CPUCores: 4, MemoryMB: 4096, Disk: DiskRequest{SizeGB: 50}}

	// Both nodes: 16 physical vCPU, 8 used → 8 free. 64 GB RAM, 60 used → 4 free.
	// 1000 GB disk max, 900 used → 100 free.
	// Request needs 8 vCPU, 4 GB RAM, 50 GB disk.
	// "fits": 8 free vCPU >= 8 needed, 4 free RAM >= 4 needed, 100 free disk >= 50 needed → fits.
	// "noFit": same free amounts but we set MaxDiskGB=940 so 40 free < 50 needed → no fit.
	// To keep fractions identical, we use the same PhysicalVCPUs/PhysicalRAMGB/MaxDiskGB
	// but change only the disk fit by raising UsedDiskGB on noFit.
	fits := policy.Capacity{PhysicalVCPUs: 16, PhysicalRAMGB: 64, MaxDiskGB: 1000, UsedVCPUs: 8, UsedRAMGB: 60, UsedDiskGB: 900}
	noFit := policy.Capacity{PhysicalVCPUs: 16, PhysicalRAMGB: 64, MaxDiskGB: 1000, UsedVCPUs: 8, UsedRAMGB: 60, UsedDiskGB: 970}

	scoreFits := scoreNode(fits, req)
	scoreNoFit := scoreNode(noFit, req)

	if scoreFits <= scoreNoFit {
		t.Errorf("fitting node score (%.3f) should be higher than non-fitting (%.3f)", scoreFits, scoreNoFit)
	}

	// The difference should be the fit bonus (1.0) plus the disk fraction
	// difference. fits: 100/1000=0.1 * 0.15 = 0.015. noFit: 30/1000=0.03 * 0.15 = 0.0045.
	// diff = 1.0 + (0.015 - 0.0045) = 1.0105.
	if scoreFits-scoreNoFit < 0.99 || scoreFits-scoreNoFit > 1.02 {
		t.Errorf("score difference = %.3f, want ~1.0-1.02 (fit bonus + disk fraction diff)", scoreFits-scoreNoFit)
	}
}

// fakeFreeSpaceChecker is a minimal FreeSpaceChecker for unit-testing
// checkLiveDiskSpace without pulling in the cluster.Fake dataset.
type fakeFreeSpaceChecker struct {
	free  int64
	err   error
	calls int
}

func (f *fakeFreeSpaceChecker) StorageFreeSpace(_ context.Context, _, _ string) (int64, error) {
	f.calls++

	return f.free, f.err
}

// TestCheckLiveDiskSpace_TableDriven — exercises every branch of
// checkLiveDiskSpace (US3/issue-04 D4b): nil checker skip, zero-disk skip,
// read-error wrapping, insufficient space, exact fit, and ample space.
func TestCheckLiveDiskSpace_TableDriven(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	readErr := errors.New("boom")

	cases := []struct {
		name      string
		freeSpace FreeSpaceChecker
		diskGB    int
		wantErr   error
		wantCalls int
	}{
		{name: "nil checker skips check", freeSpace: nil, diskGB: 40, wantErr: nil, wantCalls: 0},
		{name: "zero disk skips check", freeSpace: &fakeFreeSpaceChecker{free: 0}, diskGB: 0, wantErr: nil, wantCalls: 0},
		{name: "negative disk skips check", freeSpace: &fakeFreeSpaceChecker{free: 0}, diskGB: -1, wantErr: nil, wantCalls: 0},
		{name: "read error wraps ErrClusterCreate", freeSpace: &fakeFreeSpaceChecker{err: readErr}, diskGB: 40, wantErr: ErrClusterCreate, wantCalls: 1},
		{name: "insufficient space returns ErrInsufficientDiskSpace", freeSpace: &fakeFreeSpaceChecker{free: 40 * bytesPerGB}, diskGB: 500, wantErr: ErrInsufficientDiskSpace, wantCalls: 1},
		{name: "exact fit passes", freeSpace: &fakeFreeSpaceChecker{free: 40 * bytesPerGB}, diskGB: 40, wantErr: nil, wantCalls: 1},
		{name: "ample space passes", freeSpace: &fakeFreeSpaceChecker{free: 500 * bytesPerGB}, diskGB: 40, wantErr: nil, wantCalls: 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := checkLiveDiskSpace(ctx, tc.freeSpace, "test-node", "test-storage", tc.diskGB)

			if tc.wantErr == nil {
				if err != nil {
					t.Fatalf("checkLiveDiskSpace: got %v, want nil", err)
				}
			} else if !errors.Is(err, tc.wantErr) {
				t.Fatalf("checkLiveDiskSpace: got %v, want %v", err, tc.wantErr)
			}

			if fake, ok := tc.freeSpace.(*fakeFreeSpaceChecker); ok && fake.calls != tc.wantCalls {
				t.Errorf("StorageFreeSpace calls = %d, want %d", fake.calls, tc.wantCalls)
			}
		})
	}
}
