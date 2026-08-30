package inventory

import (
	"context"
	"errors"
	"time"
)

// ErrRefreshTooSoon is returned when a manual refresh is requested before the
// minimum interval has elapsed (FR-006). The guard check happens before any
// client call, so a refused refresh never touches the cluster client (SC-001).
var ErrRefreshTooSoon = errors.New("refresh too soon")

// ErrClusterUnreachable is returned when a manual refresh's client call fails.
// The previous projection continues to be served (FR-004) — only this attempt
// is reported as failed.
var ErrClusterUnreachable = errors.New("cluster unreachable")

// TooSoonError wraps ErrRefreshTooSoon with the precise remaining wait before
// the guard allows another attempt — not the full configured interval
// (contracts/cluster-refresh.md: retryAfterSeconds is a countdown, not a
// constant). errors.Is(err, ErrRefreshTooSoon) still matches via Unwrap.
type TooSoonError struct {
	RetryAfter time.Duration
}

func (e *TooSoonError) Error() string { return ErrRefreshTooSoon.Error() }
func (e *TooSoonError) Unwrap() error { return ErrRefreshTooSoon }

// Refresher handles manual refresh requests, guarded by a minimum interval
// since the last successful refresh (FR-005, FR-006). The guard is enforced
// server-side (constitution VI), not only by disabling a button. It reads
// the worker's own projection directly — a Refresher is always paired with
// exactly one Worker, so there is no second projection reference a caller
// could accidentally mismatch.
type Refresher struct {
	worker      *Worker
	minInterval time.Duration
}

// NewRefresher creates a manual refresher with the given guard interval.
func NewRefresher(worker *Worker, minInterval time.Duration) *Refresher {
	return &Refresher{
		worker:      worker,
		minInterval: minInterval,
	}
}

// MinInterval returns the configured minimum interval between refreshes.
func (r *Refresher) MinInterval() time.Duration {
	return r.minInterval
}

// Refresh attempts a manual refresh. If the minimum interval has not elapsed
// since the last successful refresh, it returns a *TooSoonError carrying the
// precise remaining wait, without calling the cluster client (FR-006,
// SC-001). Otherwise it delegates to the worker, which serializes with any
// in-flight automatic cycle.
func (r *Refresher) Refresh(ctx context.Context) (time.Time, error) {
	if current := r.worker.projection.Load(); current != nil {
		if remaining := r.minInterval - time.Since(current.RefreshedAt); remaining > 0 {
			return time.Time{}, &TooSoonError{RetryAfter: remaining}
		}
	}

	at, err := r.worker.Refresh(ctx)
	if err != nil {
		return time.Time{}, ErrClusterUnreachable
	}

	return at, nil
}

// RefreshAsync performs the guard check synchronously and, if the guard
// passes, launches the refresh in a background goroutine using a context
// detached from the caller's request lifecycle. This lets the HTTP handler
// return 202 Accepted immediately instead of blocking for up to
// InventoryRefreshTimeout — the server's WriteTimeout would otherwise cancel
// the request before a slow or dead cluster's refresh completes.
//
// Returns nil if the refresh was started, or *TooSoonError if the guard
// refused. The refresh result is logged by the worker; callers learn the
// outcome by re-reading the projection (e.g. re-loading the VM list or
// polling /health).
func (r *Refresher) RefreshAsync(ctx context.Context) error {
	if current := r.worker.projection.Load(); current != nil {
		if remaining := r.minInterval - time.Since(current.RefreshedAt); remaining > 0 {
			return &TooSoonError{RetryAfter: remaining}
		}
	}

	go func() {
		// Detach from the request's cancellation so the refresh survives the
		// HTTP response being written. The worker's own timeout
		// (InventoryRefreshTimeout) bounds the call.
		detached := context.WithoutCancel(ctx)
		if _, err := r.worker.Refresh(detached); err != nil {
			r.worker.log.Error("async refresh failed", "component", "inventory", "error", err)
		}
	}()

	return nil
}
