package inventory_test

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// callCountClient wraps a cluster.Client and counts Snapshot calls — the
// instrumented counter that proves SC-001 (at most one call per refresh cycle
// regardless of concurrent readers).
type callCountClient struct {
	snapshot cluster.Snapshot
	err      error
	calls    atomic.Int64
	delay    time.Duration
}

func (c *callCountClient) Snapshot(ctx context.Context) (cluster.Snapshot, error) {
	c.calls.Add(1)

	if c.delay > 0 {
		select {
		case <-time.After(c.delay):
		case <-ctx.Done():
			return cluster.Snapshot{}, ctx.Err()
		}
	}

	if c.err != nil {
		return cluster.Snapshot{}, c.err
	}

	return c.snapshot, nil
}

func (c *callCountClient) Authenticate(_ context.Context, _, _ string) (cluster.Identity, error) {
	return cluster.Identity{}, cluster.ErrNotImplemented
}

func (c *callCountClient) ChangePassword(_ context.Context, _, _, _ string) error {
	return cluster.ErrNotImplemented
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
}

// TestWorker_SuccessfulCycleSwapsIndex — a successful refresh stores an Index
// in the projection with a non-zero RefreshedAt.
func TestWorker_SuccessfulCycleSwapsIndex(t *testing.T) {
	client := &callCountClient{snapshot: fakeSnapshot()}
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(client, projection, time.Hour, testLogger())

	if projection.Load() != nil {
		t.Fatal("projection should be nil before first refresh")
	}

	at, err := worker.Refresh(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if at.IsZero() {
		t.Fatal("refreshedAt should not be zero after successful refresh")
	}

	idx := projection.Load()
	if idx == nil {
		t.Fatal("projection should be non-nil after successful refresh")
	}

	if !idx.RefreshedAt.Equal(at) {
		t.Fatalf("projection RefreshedAt %v != returned %v", idx.RefreshedAt, at)
	}

	if len(idx.Nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(idx.Nodes))
	}

	if len(idx.ByVMID) != 25 {
		t.Fatalf("expected 25 VMs, got %d", len(idx.ByVMID))
	}
}

// TestWorker_FailingCycleLeavesPreviousIndex — FR-004: a failed refresh does
// not clear or corrupt the existing projection.
func TestWorker_FailingCycleLeavesPreviousIndex(t *testing.T) {
	client := &callCountClient{snapshot: fakeSnapshot()}
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(client, projection, time.Hour, testLogger())

	// First refresh succeeds.
	firstAt, err := worker.Refresh(context.Background())
	if err != nil {
		t.Fatalf("first refresh: %v", err)
	}

	firstIdx := projection.Load()
	if firstIdx == nil {
		t.Fatal("first refresh should have stored an index")
	}

	// Second refresh fails — the client returns an error.
	client.err = cluster.ErrUnreachable

	_, err = worker.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected error from failing refresh")
	}

	// The projection must still hold the first index, byte-for-byte unchanged.
	secondIdx := projection.Load()
	if secondIdx != firstIdx {
		t.Fatal("projection pointer changed after failed refresh — FR-004 violation")
	}

	if !secondIdx.RefreshedAt.Equal(firstAt) {
		t.Fatalf("RefreshedAt changed after failed refresh: %v vs %v", secondIdx.RefreshedAt, firstAt)
	}
}

// TestWorker_CallsClientOnceEvenUnderConcurrentReads — SC-001: the cluster
// client is called at most once per refresh cycle, regardless of how many
// concurrent readers access the projection during that cycle.
func TestWorker_CallsClientOnceEvenUnderConcurrentReads(t *testing.T) {
	client := &callCountClient{snapshot: fakeSnapshot()}
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(client, projection, time.Hour, testLogger())

	// Pre-populate so readers have something to read.
	_, _ = worker.Refresh(context.Background())
	callsBefore := client.calls.Load()

	// Start N goroutines reading the projection concurrently with a refresh.
	const N = 50

	var wg sync.WaitGroup
	wg.Add(N + 1)

	go func() {
		defer wg.Done()

		_, _ = worker.Refresh(context.Background())
	}()

	for range N {
		go func() {
			defer wg.Done()

			if idx := projection.Load(); idx != nil {
				_ = idx.Nodes
				_ = idx.ByVMID
				_ = idx.ByPool
				_ = idx.ByNode
			}
		}()
	}

	wg.Wait()

	callsAfter := client.calls.Load()

	additionalCalls := callsAfter - callsBefore
	if additionalCalls != 1 {
		t.Fatalf("expected exactly 1 additional client call, got %d (SC-001)", additionalCalls)
	}
}

// TestWorker_ConcurrentRefreshesSingleFlight — multiple concurrent refresh
// requests result in exactly one client call (in-flight dedup). The client
// has a delay so all goroutines arrive while the first is in flight.
func TestWorker_ConcurrentRefreshesSingleFlight(t *testing.T) {
	client := &callCountClient{snapshot: fakeSnapshot(), delay: 100 * time.Millisecond}
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(client, projection, time.Hour, testLogger())

	const N = 10

	var wg sync.WaitGroup
	wg.Add(N)

	for range N {
		go func() {
			defer wg.Done()

			_, _ = worker.Refresh(context.Background())
		}()
	}

	wg.Wait()

	if client.calls.Load() != 1 {
		t.Fatalf("expected exactly 1 client call for %d concurrent refreshes, got %d", N, client.calls.Load())
	}
}

// TestWorker_RunDoesInitialRefresh — Run performs an initial refresh before
// starting the ticker, so the projection is populated before the HTTP server
// accepts traffic (T015).
func TestWorker_RunDoesInitialRefresh(t *testing.T) {
	client := &callCountClient{snapshot: fakeSnapshot()}
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(client, projection, time.Hour, testLogger())

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		worker.Run(ctx)
		close(done)
	}()

	// Wait for the initial refresh to populate the projection.
	deadline := time.After(2 * time.Second)

	for projection.Load() == nil {
		select {
		case <-deadline:
			t.Fatal("Run did not perform initial refresh within 2s")
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	cancel()
	<-done
}

// TestWorker_FailingFirstRefreshLeavesNil — edge case: if the very first
// refresh fails, the projection remains nil (FR-009: never-refreshed is
// distinct from empty).
func TestWorker_FailingFirstRefreshLeavesNil(t *testing.T) {
	client := &callCountClient{err: cluster.ErrUnreachable}
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(client, projection, time.Hour, testLogger())

	_, err := worker.Refresh(context.Background())
	if err == nil {
		t.Fatal("expected error from failing first refresh")
	}

	if projection.Load() != nil {
		t.Fatal("projection should be nil after failed first refresh (FR-009)")
	}
}

// TestWorker_ShrinkingDataset — edge case: if the dataset shrinks between
// refreshes (a node disappears), the projection reflects the new, smaller set.
func TestWorker_ShrinkingDataset(t *testing.T) {
	client := &callCountClient{snapshot: fakeSnapshot()}
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(client, projection, time.Hour, testLogger())

	_, _ = worker.Refresh(context.Background())

	if len(projection.Load().Nodes) != 3 {
		t.Fatalf("expected 3 nodes initially, got %d", len(projection.Load().Nodes))
	}

	// Shrink the dataset to 1 node, 0 VMs, 0 storages.
	client.snapshot = cluster.Snapshot{
		Nodes:    []cluster.Node{{Name: "pve-node-01", Status: cluster.NodeOnline}},
		VMs:      nil,
		Storages: nil,
	}
	_, _ = worker.Refresh(context.Background())

	idx := projection.Load()
	if len(idx.Nodes) != 1 {
		t.Fatalf("expected 1 node after shrink, got %d", len(idx.Nodes))
	}

	if len(idx.ByVMID) != 0 {
		t.Fatalf("expected 0 VMs after shrink, got %d", len(idx.ByVMID))
	}
}

// Ensure errors.Is works for common sentinel checks.
func TestWorker_ErrorWrapping(t *testing.T) {
	client := &callCountClient{err: cluster.ErrUnreachable}
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(client, projection, time.Hour, testLogger())

	_, err := worker.Refresh(context.Background())
	if !errors.Is(err, cluster.ErrUnreachable) {
		t.Fatalf("expected ErrUnreachable, got %v", err)
	}
}

// TestWorker_TimeoutCancelsHungClient — a Snapshot call that blocks forever
// is cancelled by the worker's per-call timeout, so the singleflight lock is
// released and a later refresh can proceed (golang-design-patterns rule 9:
// every external call has a timeout).
func TestWorker_TimeoutCancelsHungClient(t *testing.T) {
	hung := &hungClient{}
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(
		hung,
		projection,
		time.Hour,
		testLogger(),
		inventory.WithRefreshTimeout(50*time.Millisecond),
	)

	start := time.Now()
	_, err := worker.Refresh(context.Background())
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error from hung client, got nil")
	}

	if elapsed > time.Second {
		t.Fatalf("refresh took %v, should have timed out near 50ms", elapsed)
	}

	// The singleflight lock must be released — a second refresh with a
	// working client must complete, not hang waiting for the first.
	working := &callCountClient{snapshot: fakeSnapshot()}
	worker2 := inventory.NewWorker(
		working,
		projection,
		time.Hour,
		testLogger(),
		inventory.WithRefreshTimeout(2*time.Second),
	)
	done := make(chan struct{})

	go func() {
		_, _ = worker2.Refresh(context.Background())

		close(done)
	}()

	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("second refresh hung — singleflight lock was not released after timeout")
	}

	if projection.Load() == nil {
		t.Fatal("projection should be populated after the second, successful refresh")
	}
}

// hungClient blocks on Snapshot until the context is cancelled.
type hungClient struct{}

func (hungClient) Snapshot(ctx context.Context) (cluster.Snapshot, error) {
	<-ctx.Done()
	return cluster.Snapshot{}, ctx.Err()
}

func (hungClient) Authenticate(_ context.Context, _, _ string) (cluster.Identity, error) {
	return cluster.Identity{}, cluster.ErrNotImplemented
}

func (hungClient) ChangePassword(_ context.Context, _, _, _ string) error {
	return cluster.ErrNotImplemented
}
