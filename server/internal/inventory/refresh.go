package inventory

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

// ErrRefreshTooSoon is returned when a manual refresh is requested before the
// minimum interval has elapsed. The guard check happens before any
// client call, so a refused refresh never touches the cluster client.
var ErrRefreshTooSoon = errors.New("refresh too soon")

// ErrClusterUnreachable is returned when a manual refresh's client call fails.
// The previous projection continues to be served - only this attempt
// is reported as failed.
var ErrClusterUnreachable = errors.New("cluster unreachable")

// TooSoonError wraps ErrRefreshTooSoon with the precise remaining wait before
// the guard allows another attempt - not the full configured interval
// (retryAfterSeconds is a countdown, not a constant). errors.Is(err, ErrRefreshTooSoon) still
// matches via Unwrap.
type TooSoonError struct {
	RetryAfter time.Duration
}

func (e *TooSoonError) Error() string { return ErrRefreshTooSoon.Error() }
func (e *TooSoonError) Unwrap() error { return ErrRefreshTooSoon }

// Refresher handles refreshes of one cluster's projection. Manual refresh
// requests (RefreshAsync) are guarded by a minimum interval since the last
// successful refresh, enforced server-side, not only by disabling a button.
// Post-write invalidation (Refresh) is not guarded. It reads
// the worker's own projection directly - a Refresher is always paired with
// exactly one Worker, so there is no second projection reference a caller
// could accidentally mismatch.
type Refresher struct {
	// worker is swapped when the cluster is re-added after an admin edit.
	worker      atomic.Pointer[Worker]
	minInterval time.Duration
}

// NewRefresher creates a manual refresher with the given guard interval.
func NewRefresher(worker *Worker, minInterval time.Duration) *Refresher {
	r := &Refresher{minInterval: minInterval}
	r.worker.Store(worker)

	return r
}

// MinInterval returns the configured minimum interval between refreshes.
func (r *Refresher) MinInterval() time.Duration {
	return r.minInterval
}

// Refresh rebuilds the projection now. It is the post-write invalidation used
// by every mutation path (VM writes, task completion, create), so it is never
// guarded: a write must always be reflected before its response is built. The
// minimum interval only throttles the user-triggered manual refresh, see
// RefreshAsync. It delegates to the worker, which serializes with any
// in-flight automatic cycle.
func (r *Refresher) Refresh(ctx context.Context) (time.Time, error) {
	at, err := r.worker.Load().Refresh(ctx)
	if err != nil {
		return time.Time{}, ErrClusterUnreachable
	}

	return at, nil
}

// RefreshAsync performs the guard check synchronously and, if the guard
// passes, launches the refresh in a background goroutine using a context
// detached from the caller's request lifecycle. This lets the HTTP handler
// return 202 Accepted immediately instead of blocking for up to
// InventoryRefreshTimeout - the server's WriteTimeout would otherwise cancel
// the request before a slow or dead cluster's refresh completes.
//
// Returns nil if the refresh was started, or *TooSoonError if the guard
// refused. The refresh result is logged by the worker; callers learn the
// outcome by re-reading the projection (e.g. re-loading the VM list or
// polling /health).
func (r *Refresher) RefreshAsync(ctx context.Context) error {
	worker := r.worker.Load()
	if current := worker.projection.Load(); current != nil {
		if remaining := r.minInterval - time.Since(current.RefreshedAt); remaining > 0 {
			return &TooSoonError{RetryAfter: remaining}
		}
	}

	go func() {
		// Detach from the request's cancellation so the refresh survives the
		// HTTP response being written. The worker's own timeout
		// (InventoryRefreshTimeout) bounds the call.
		detached := context.WithoutCancel(ctx)
		if _, err := worker.Refresh(detached); err != nil {
			worker.log.ErrorContext(ctx, "async refresh failed", "component", "inventory", "error", err)
		}
	}()

	return nil
}
