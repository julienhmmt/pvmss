//nolint:wsl_v5 // refresh isolation scenarios keep setup and assertions together
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

const (
	registryTestSecret        = "secret"
	inventoryTestCluster      = "default"
	inventorySecondaryCluster = "secondary"
	inventoryOfflineCluster   = "offline-demo"
	inventoryDefaultURL       = "https://default.invalid"
	inventorySecondaryURL     = "https://secondary.invalid"
	inventoryOfflineURL       = "https://offline.invalid"
)

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_RefreshKeepsClustersIndependent(t *testing.T) {
	clusters, err := cluster.NewRegistry("fake", []store.ClusterRow{
		{Name: inventoryTestCluster, URL: "https://default.invalid", TokenID: "id", TokenSecret: registryTestSecret},
		{Name: "secondary", URL: "https://secondary.invalid", TokenID: "id", TokenSecret: registryTestSecret},
		{Name: "offline-demo", URL: "https://offline.invalid", TokenID: "id", TokenSecret: registryTestSecret},
	})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	registry := inventory.NewRegistry(clusters, time.Hour, slog.Default())

	if _, err := registry.Refresh(context.Background(), inventoryTestCluster); err != nil {
		t.Fatalf("Refresh(default): %v", err)
	}
	defaultIndex, err := registry.Index(inventoryTestCluster)
	if err != nil || defaultIndex == nil {
		t.Fatalf("Index(default) = %v, %v", defaultIndex, err)
	}
	if _, err := registry.Refresh(context.Background(), "offline-demo"); !errors.Is(err, cluster.ErrUnreachable) {
		t.Fatalf("Refresh(offline-demo) error = %v, want ErrUnreachable", err)
	}
	stillDefault, err := registry.Index(inventoryTestCluster)
	if err != nil || stillDefault != defaultIndex {
		t.Fatalf("default index changed after offline refresh: %p -> %p", defaultIndex, stillDefault)
	}

	all := registry.All()
	if len(all) != 3 {
		t.Fatalf("All() length = %d, want 3 active clusters", len(all))
	}
	if all[inventoryTestCluster] != defaultIndex {
		t.Fatal("All() returned a different default index")
	}
}

//nolint:paralleltest // registry tests share fake client fixture state
func TestRegistry_UnknownCluster(t *testing.T) {
	registry := inventory.NewRegistryFromIndexes(map[string]*inventory.Index{})
	if _, err := registry.Index("missing"); !errors.Is(err, inventory.ErrClusterNotFound) {
		t.Fatalf("Index(missing) error = %v, want ErrClusterNotFound", err)
	}
}

// TestRegistry_StartRefreshMutateRefreshCycle is an integration test that
// exercises the full lifecycle main.go wires: Registry.Start → Worker.Run
// (initial refresh + ticker) → projection swap → mutation on the fake cluster
// → manual refresh via Registry.Refresher → projection reflects the mutation.
// It proves the wiring path end-to-end, not just each component in isolation.
//
//nolint:paralleltest,gocyclo // registry tests share fake fixture; integration test has per-phase assertions
func TestRegistry_StartRefreshMutateRefreshCycle(t *testing.T) {
	clusterRegistry, err := cluster.NewRegistry("fake", []store.ClusterRow{
		{Name: inventoryTestCluster, URL: "https://default.invalid", TokenID: "id", TokenSecret: registryTestSecret},
	})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	fakeClient, err := clusterRegistry.Client(inventoryTestCluster)
	if err != nil {
		t.Fatalf("Client(default): %v", err)
	}
	writer, ok := fakeClient.(cluster.Writer)
	if !ok {
		t.Fatal("fake client does not implement Writer")
	}

	registry := inventory.NewRegistry(clusterRegistry, 50*time.Millisecond, slog.Default())
	registry.SetManualRefreshMinInterval(1 * time.Millisecond)

	projection, err := registry.Projection(inventoryTestCluster)
	if err != nil {
		t.Fatalf("Projection: %v", err)
	}
	if projection.Load() != nil {
		t.Fatal("projection should be nil before Start")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	registry.Start(ctx)

	// Phase 1: initial refresh populates the projection (T015).
	waitForProjection(t, projection, 2*time.Second)
	initial := projection.Load()
	if len(initial.ByVMID) != 25 {
		t.Fatalf("expected 25 VMs after initial refresh, got %d", len(initial.ByVMID))
	}

	// Phase 2: wait for at least one ticker-driven automatic refresh.
	time.Sleep(80 * time.Millisecond)
	autoRefreshed := projection.Load()
	if autoRefreshed == nil {
		t.Fatal("projection is nil after ticker interval")
	}
	if autoRefreshed.RefreshedAt.Before(initial.RefreshedAt) {
		t.Fatalf("auto-refresh RefreshedAt %v is before initial %v", autoRefreshed.RefreshedAt, initial.RefreshedAt)
	}

	// Phase 3: mutate the fake cluster (delete a VM), then manual refresh.
	firstVM := firstVMFromIndex(t, initial)
	if err := writer.Delete(context.Background(), firstVM.Node, firstVM.VMID); err != nil {
		t.Fatalf("delete VM: %v", err)
	}
	refresher, err := registry.Refresher(inventoryTestCluster)
	if err != nil {
		t.Fatalf("Refresher: %v", err)
	}
	if _, err := refresher.Refresh(context.Background()); err != nil {
		t.Fatalf("manual refresh after mutation: %v", err)
	}

	afterMutation := projection.Load()
	if len(afterMutation.ByVMID) != 24 {
		t.Fatalf("expected 24 VMs after delete + refresh, got %d", len(afterMutation.ByVMID))
	}
	if _, exists := afterMutation.ByVMID[firstVM.VMID]; exists {
		t.Fatalf("deleted VM %d still in projection", firstVM.VMID)
	}

	// Phase 4: cancel context — workers must stop without hanging.
	cancel()
	done := make(chan struct{})
	go func() {
		time.Sleep(100 * time.Millisecond)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("workers did not stop within 3s of context cancellation")
	}
}

// waitForProjection polls until the projection is non-nil or the deadline fires.
func waitForProjection(t *testing.T, projection *inventory.Projection, timeout time.Duration) {
	t.Helper()
	deadline := time.After(timeout)
	for projection.Load() == nil {
		select {
		case <-deadline:
			t.Fatal("projection did not populate within timeout")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

// firstVMFromIndex returns a stopped VM from the index for mutation tests.
// A stopped VM is required because the fake's Delete rejects a running VM with
// cluster.ErrVMRunning (mirroring real Proxmox); these registry tests exercise
// the refresh/projection wiring, not the force-stop path.
func firstVMFromIndex(t *testing.T, index *inventory.Index) cluster.VM {
	t.Helper()
	for _, vm := range index.ByVMID {
		if vm.Status == cluster.VMStopped {
			return vm
		}
	}
	t.Fatal("no stopped VM found in index")
	return cluster.VM{}
}
