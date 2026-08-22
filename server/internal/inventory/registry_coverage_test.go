package inventory_test

import (
	"context"
	"errors"
	"log/slog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"testing"
	"time"
)

func newTestRegistry(t *testing.T, names ...string) *inventory.Registry {
	t.Helper()
	clusters := newTestClusterRegistry(t, names...)
	return inventory.NewRegistry(clusters, time.Hour, slog.Default())
}

func newTestClusterRegistry(t *testing.T, names ...string) *cluster.Registry {
	t.Helper()
	rows := make([]store.ClusterRow, 0, len(names))
	for _, name := range names {
		rows = append(rows, store.ClusterRow{
			Name: name, URL: "https://" + name + ".invalid", TokenID: "id", TokenSecret: "secret",
		})
	}
	clusters, err := cluster.NewRegistry("fake", rows)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	return clusters
}

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_Refresh_SuccessAndError(t *testing.T) {
	registry := newTestRegistry(t, "default", "offline-demo")

	cases := []struct {
		name      string
		cluster   string
		wantErr   error
		wantIndex bool
	}{
		{name: "default succeeds", cluster: "default", wantErr: nil, wantIndex: true},
		{name: "offline-demo unreachable", cluster: "offline-demo", wantErr: cluster.ErrUnreachable, wantIndex: false},
		{name: "unknown cluster", cluster: "nonexistent", wantErr: inventory.ErrClusterNotFound, wantIndex: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := registry.Refresh(context.Background(), tc.cluster)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("Refresh(%q) error = %v, want %v", tc.cluster, err, tc.wantErr)
				}
			} else if err != nil {
				t.Fatalf("Refresh(%q): %v", tc.cluster, err)
			}

			if tc.wantIndex {
				idx, err := registry.Index(tc.cluster)
				if err != nil || idx == nil {
					t.Fatalf("Index(%q) = %v, %v", tc.cluster, idx, err)
				}
			}
		})
	}
}

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_All_MultipleClusters(t *testing.T) {
	registry := newTestRegistry(t, "default", "secondary", "offline-demo")

	if _, err := registry.Refresh(context.Background(), "default"); err != nil {
		t.Fatalf("Refresh(default): %v", err)
	}
	if _, err := registry.Refresh(context.Background(), "secondary"); err != nil {
		t.Fatalf("Refresh(secondary): %v", err)
	}

	all := registry.All()
	if len(all) != 3 {
		t.Fatalf("All() length = %d, want 3", len(all))
	}

	if all["default"] == nil {
		t.Fatal("All()[\"default\"] is nil after successful refresh")
	}
	if all["secondary"] == nil {
		t.Fatal("All()[\"secondary\"] is nil after successful refresh")
	}
	if all["offline-demo"] != nil {
		t.Fatal("All()[\"offline-demo\"] should be nil after failed refresh")
	}
}

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_All_EmptyRegistry(t *testing.T) {
	registry := inventory.NewRegistryFromIndexes(map[string]*inventory.Index{})

	all := registry.All()
	if len(all) != 0 {
		t.Fatalf("All() length = %d, want 0", len(all))
	}
}

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_Lookup_WithRefreshedCluster(t *testing.T) {
	registry := newTestRegistry(t, "default")
	if _, err := registry.Refresh(context.Background(), "default"); err != nil {
		t.Fatalf("Refresh(default): %v", err)
	}

	cases := []struct {
		name        string
		clusterName string
		vmid        int
		wantOK      bool
	}{
		{name: "found", clusterName: "default", vmid: 100, wantOK: true},
		{name: "not found vmid", clusterName: "default", vmid: 99999, wantOK: false},
		{name: "unknown cluster", clusterName: "nonexistent", vmid: 100, wantOK: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			vm, ok := registry.Lookup(tc.clusterName, tc.vmid)
			if ok != tc.wantOK {
				t.Fatalf("Lookup ok = %v, want %v", ok, tc.wantOK)
			}
			if tc.wantOK && vm.VMID != tc.vmid {
				t.Errorf("VMID = %d, want %d", vm.VMID, tc.vmid)
			}
		})
	}
}

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_Projection_FoundAndNotFound(t *testing.T) {
	registry := newTestRegistry(t, "default")

	proj, err := registry.Projection("default")
	if err != nil {
		t.Fatalf("Projection(default): %v", err)
	}
	if proj == nil {
		t.Fatal("Projection(default) is nil")
	}

	if _, err := registry.Projection("nonexistent"); !errors.Is(err, inventory.ErrClusterNotFound) {
		t.Fatalf("Projection(nonexistent) error = %v, want ErrClusterNotFound", err)
	}
}

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_Refresher_FoundAndNotFound(t *testing.T) {
	registry := newTestRegistry(t, "default")

	if _, err := registry.Refresher("nonexistent"); !errors.Is(err, inventory.ErrClusterNotFound) {
		t.Fatalf("Refresher(nonexistent) error = %v, want ErrClusterNotFound", err)
	}

	refresher, err := registry.Refresher("default")
	if err != nil {
		t.Fatalf("Refresher(default): %v", err)
	}
	if refresher == nil {
		t.Fatal("Refresher(default) is nil")
	}
}

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_Worker_FoundAndNotFound(t *testing.T) {
	registry := newTestRegistry(t, "default")

	worker, err := registry.Worker("default")
	if err != nil {
		t.Fatalf("Worker(default): %v", err)
	}
	if worker == nil {
		t.Fatal("Worker(default) is nil")
	}

	if _, err := registry.Worker("nonexistent"); !errors.Is(err, inventory.ErrClusterNotFound) {
		t.Fatalf("Worker(nonexistent) error = %v, want ErrClusterNotFound", err)
	}
}

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_AddAndRemove(t *testing.T) {
	clusters := newTestClusterRegistry(t, "default")
	registry := inventory.NewRegistry(clusters, time.Hour, slog.Default())

	if err := clusters.Add(context.Background(), store.ClusterRow{
		Name: "secondary", URL: "https://secondary.invalid", TokenID: "id", TokenSecret: "secret",
	}); err != nil {
		t.Fatalf("cluster Add(secondary): %v", err)
	}

	if err := registry.Add("secondary"); err != nil {
		t.Fatalf("Add(secondary): %v", err)
	}

	if err := registry.Add("default"); !errors.Is(err, inventory.ErrDuplicateCluster) {
		t.Fatalf("Add(default) error = %v, want ErrDuplicateCluster", err)
	}

	all := registry.All()
	if len(all) != 2 {
		t.Fatalf("All() length = %d, want 2", len(all))
	}

	registry.Remove("secondary")
	all = registry.All()
	if len(all) != 1 {
		t.Fatalf("All() after Remove = %d, want 1", len(all))
	}
	if _, ok := all["secondary"]; ok {
		t.Fatal("secondary still present after Remove")
	}
}

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_StoreSnapshot(t *testing.T) {
	registry := newTestRegistry(t, "default")

	snap, err := cluster.NewFake("default").Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if err := registry.StoreSnapshot("default", snap); err != nil {
		t.Fatalf("StoreSnapshot: %v", err)
	}

	idx, err := registry.Index("default")
	if err != nil || idx == nil {
		t.Fatalf("Index(default) = %v, %v", idx, err)
	}
	if len(idx.ByVMID) != len(snap.VMs) {
		t.Fatalf("Index ByVMID count = %d, want %d", len(idx.ByVMID), len(snap.VMs))
	}

	if err := registry.StoreSnapshot("nonexistent", snap); !errors.Is(err, inventory.ErrClusterNotFound) {
		t.Fatalf("StoreSnapshot(nonexistent) error = %v, want ErrClusterNotFound", err)
	}
}

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_SetManualRefreshMinInterval(t *testing.T) {
	registry := newTestRegistry(t, "default")

	registry.SetManualRefreshMinInterval(500 * time.Millisecond)

	refresher, err := registry.Refresher("default")
	if err != nil {
		t.Fatalf("Refresher(default): %v", err)
	}
	if got := refresher.MinInterval(); got != 500*time.Millisecond {
		t.Fatalf("MinInterval = %v, want 500ms", got)
	}
}

func TestProjection_Load_EmptyReturnsNil(t *testing.T) {
	t.Parallel()

	projection := inventory.NewProjection()
	if projection.Load() != nil {
		t.Fatal("Load() should return nil for empty projection")
	}
}

func TestProjection_Load_PrePopulatedReturnsIndex(t *testing.T) {
	t.Parallel()

	idx := inventory.BuildIndex(fakeSnapshot())
	projection := inventory.NewProjectionFromIndex(&idx)

	loaded := projection.Load()
	if loaded == nil {
		t.Fatal("Load() should return the pre-populated index")
	}
	if loaded.ByVMID[100].VMID != 100 {
		t.Errorf("Load() ByVMID[100].VMID = %d, want 100", loaded.ByVMID[100].VMID)
	}
}

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_Start_Idempotent(t *testing.T) {
	registry := newTestRegistry(t, "default")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry.Start(ctx)
	registry.Start(ctx)

	cancel()
}

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_Add_AfterStart(t *testing.T) {
	clusters := newTestClusterRegistry(t, "default")
	registry := inventory.NewRegistry(clusters, time.Hour, slog.Default())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry.Start(ctx)

	if err := clusters.Add(context.Background(), store.ClusterRow{
		Name: "secondary", URL: "https://secondary.invalid", TokenID: "id", TokenSecret: "secret",
	}); err != nil {
		t.Fatalf("cluster Add(secondary): %v", err)
	}

	if err := registry.Add("secondary"); err != nil {
		t.Fatalf("Add(secondary) after Start: %v", err)
	}

	all := registry.All()
	if len(all) != 2 {
		t.Fatalf("All() length = %d, want 2", len(all))
	}

	cancel()
}

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_Refresh_NoWorkerReturnsNotFound(t *testing.T) {
	idx := inventory.BuildIndex(fakeSnapshot())
	registry := inventory.NewRegistryFromIndexes(map[string]*inventory.Index{"default": &idx})

	if _, err := registry.Refresh(context.Background(), "default"); !errors.Is(err, inventory.ErrClusterNotFound) {
		t.Fatalf("Refresh on registry without worker error = %v, want ErrClusterNotFound", err)
	}
}
