package inventory_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"testing"
	"time"
)

const (
	retryGuardInterval    = 2 * time.Second
	retryObservationDelay = 100 * time.Millisecond
	retryMaximumAfterWait = 1900 * time.Millisecond
)

// TestRefresh_OutsideGuardSucceeds — a manual refresh outside the guard
// interval succeeds and calls the client.
//
//nolint:paralleltest // serial: shared refresh fixture
func TestRefresh_OutsideGuardSucceeds(t *testing.T) {
	client := &callCountClient{snapshot: fakeSnapshot()}
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(client, projection, time.Hour, testLogger())

	// Populate the projection first.
	_, _ = worker.Refresh(context.Background())
	callsBefore := client.calls.Load()

	// Wait long enough to be outside the guard.
	refresher := inventory.NewRefresher(worker, 50*time.Millisecond)
	time.Sleep(60 * time.Millisecond)

	at, err := refresher.Refresh(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if at.IsZero() {
		t.Fatal("refreshedAt should not be zero")
	}

	if client.calls.Load() != callsBefore+1 {
		t.Fatalf("expected 1 additional client call, got %d", client.calls.Load()-callsBefore)
	}
}

// TestRefresh_InsideGuardRefusedWithZeroCalls — FR-006, SC-001: a manual
// refresh within the guard interval is refused with ErrRefreshTooSoon and
// makes zero client calls.
//
//nolint:paralleltest // serial: shared refresh fixture
func TestRefresh_InsideGuardRefusedWithZeroCalls(t *testing.T) {
	client := &callCountClient{snapshot: fakeSnapshot()}
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(client, projection, time.Hour, testLogger())

	// Populate the projection.
	_, _ = worker.Refresh(context.Background())
	callsBefore := client.calls.Load()

	// Immediately attempt a manual refresh — should be refused.
	refresher := inventory.NewRefresher(worker, 5*time.Second)

	_, err := refresher.Refresh(context.Background())
	if !errors.Is(err, inventory.ErrRefreshTooSoon) {
		t.Fatalf("expected ErrRefreshTooSoon, got %v", err)
	}

	if client.calls.Load() != callsBefore {
		t.Fatalf("guard refusal should make 0 client calls, got %d additional", client.calls.Load()-callsBefore)
	}

	var tooSoon *inventory.TooSoonError
	if !errors.As(err, &tooSoon) {
		t.Fatalf("expected *TooSoonError, got %T", err)
	}

	if tooSoon.RetryAfter <= 0 || tooSoon.RetryAfter > 5*time.Second {
		t.Fatalf("RetryAfter = %v, want (0, 5s]", tooSoon.RetryAfter)
	}
}

// TestRefresh_RetryAfterCountsDownNotFullInterval — the remaining wait
// reported shrinks as time passes, it is not the full guard interval on
// every refusal (contracts/cluster-refresh.md: retryAfterSeconds is how long
// is left, not a constant).
//
//nolint:paralleltest // serial: shared refresh fixture
func TestRefresh_RetryAfterCountsDownNotFullInterval(t *testing.T) {
	client := &callCountClient{snapshot: fakeSnapshot()}
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(client, projection, time.Hour, testLogger())
	_, _ = worker.Refresh(context.Background())

	refresher := inventory.NewRefresher(worker, retryGuardInterval)

	time.Sleep(retryObservationDelay)

	_, err := refresher.Refresh(context.Background())

	var tooSoon *inventory.TooSoonError
	if !errors.As(err, &tooSoon) {
		t.Fatalf("expected *TooSoonError, got %v", err)
	}

	if tooSoon.RetryAfter >= retryMaximumAfterWait {
		t.Fatalf("RetryAfter = %v, expected less than %v after the observation delay", tooSoon.RetryAfter, retryMaximumAfterWait)
	}
}

// TestRefresh_FirstRefreshAllowedWhenProjectionEmpty — a manual refresh before
// the first automatic cycle is allowed (the guard only applies after a
// successful refresh).
//
//nolint:paralleltest // serial: shared refresh fixture
func TestRefresh_FirstRefreshAllowedWhenProjectionEmpty(t *testing.T) {
	client := &callCountClient{snapshot: fakeSnapshot()}
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(client, projection, time.Hour, testLogger())

	refresher := inventory.NewRefresher(worker, 5*time.Second)

	at, err := refresher.Refresh(context.Background())
	if err != nil {
		t.Fatalf("first manual refresh should succeed, got %v", err)
	}

	if at.IsZero() {
		t.Fatal("refreshedAt should not be zero")
	}

	if client.calls.Load() != 1 {
		t.Fatalf("expected 1 client call, got %d", client.calls.Load())
	}
}

// TestRefresh_FailingClientReturnsUnreachable — a manual refresh whose client
// call fails returns ErrClusterUnreachable; the previous projection is
// still served (FR-004).
//
//nolint:paralleltest // serial: shared refresh fixture
func TestRefresh_FailingClientReturnsUnreachable(t *testing.T) {
	client := &callCountClient{snapshot: fakeSnapshot()}
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(client, projection, time.Hour, testLogger())

	// Populate.
	_, _ = worker.Refresh(context.Background())
	before := projection.Load()

	// Make the client fail.
	client.err = cluster.ErrUnreachable
	refresher := inventory.NewRefresher(worker, 1*time.Millisecond)
	time.Sleep(2 * time.Millisecond)

	_, err := refresher.Refresh(context.Background())
	if !errors.Is(err, inventory.ErrClusterUnreachable) {
		t.Fatalf("expected ErrClusterUnreachable, got %v", err)
	}

	if projection.Load() != before {
		t.Fatal("projection should be unchanged after failed manual refresh (FR-004)")
	}
}

// TestRefresh_MinInterval returns the configured guard.
//
//nolint:paralleltest // serial: shared refresh fixture
func TestRefresh_MinInterval(t *testing.T) {
	worker := inventory.NewWorker(&callCountClient{}, inventory.NewProjection(), time.Hour, testLogger())

	refresher := inventory.NewRefresher(worker, 7*time.Second)
	if got := refresher.MinInterval(); got != 7*time.Second {
		t.Fatalf("MinInterval = %v, want 7s", got)
	}
}
