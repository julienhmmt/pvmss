package inventory

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"pvmss/server/internal/cluster"
)

// defaultRefreshTimeout caps how long a single cluster.Snapshot call may take.
// A slow or hung upstream must not block the singleflight-serialized refresh
// cycle indefinitely — every external call has a timeout (golang-design-patterns
// rule 9). Override with WithRefreshTimeout.
const defaultRefreshTimeout = 15 * time.Second

// Worker owns the refresh cycle: it calls cluster.Client.Snapshot, builds an
// Index, and atomically swaps it into the Projection on success. On failure
// it logs and keeps the previous index (FR-004). The cycle is serialized via
// a singleflight mechanism so concurrent refresh requests (automatic + manual)
// result in exactly one client call (SC-001, edge case: in-flight dedup).
type Worker struct {
	client     cluster.Client
	projection *Projection
	interval   time.Duration
	timeout    time.Duration
	log        *slog.Logger

	mu       sync.Mutex
	inFlight *inFlightRefresh
}

type inFlightRefresh struct {
	done chan struct{}
	at   time.Time
	err  error
}

// Option configures a Worker. Pass to NewWorker to override defaults.
type Option func(*Worker)

// WithRefreshTimeout sets the per-call timeout for cluster.Client.Snapshot.
// Must be positive. When unset, defaultRefreshTimeout is used.
func WithRefreshTimeout(d time.Duration) Option {
	return func(w *Worker) { w.timeout = d }
}

// NewWorker creates a worker that refreshes every interval. The per-call
// timeout defaults to defaultRefreshTimeout; override it with
// WithRefreshTimeout.
func NewWorker(client cluster.Client, projection *Projection, interval time.Duration, log *slog.Logger, opts ...Option) *Worker {
	w := &Worker{
		client:     client,
		projection: projection,
		interval:   interval,
		timeout:    defaultRefreshTimeout,
		log:        log,
	}
	for _, opt := range opts {
		opt(w)
	}
	return w
}

// refreshCycle calls the client and swaps the index on success. It is the
// single site that touches the cluster client during a refresh. The client
// call is bounded by the worker's timeout so a hung upstream cannot block
// the singleflight lock indefinitely.
func (w *Worker) refreshCycle(ctx context.Context) (time.Time, error) {
	ctx, cancel := context.WithTimeout(ctx, w.timeout)
	defer cancel()

	snap, err := w.client.Snapshot(ctx)
	if err != nil {
		w.log.Error("inventory refresh failed", "component", "inventory", "error", err)
		return time.Time{}, err
	}
	idx := BuildIndex(snap)
	idx.RefreshedAt = time.Now()
	w.projection.store(&idx)
	return idx.RefreshedAt, nil
}

// Refresh performs one refresh cycle. If a cycle is already in progress,
// waits for it and returns its result instead of starting a second call
// (edge case: manual vs. automatic in-flight dedup).
func (w *Worker) Refresh(ctx context.Context) (time.Time, error) {
	w.mu.Lock()
	if w.inFlight != nil {
		pending := w.inFlight
		w.mu.Unlock()
		select {
		case <-pending.done:
			return pending.at, pending.err
		case <-ctx.Done():
			return time.Time{}, ctx.Err()
		}
	}
	pending := &inFlightRefresh{done: make(chan struct{})}
	w.inFlight = pending
	w.mu.Unlock()

	at, err := w.refreshCycle(ctx)
	pending.at = at
	pending.err = err

	w.mu.Lock()
	w.inFlight = nil
	w.mu.Unlock()
	close(pending.done)

	return at, err
}

// Run starts the automatic ticker loop. It performs an initial refresh
// immediately, then ticks at the configured interval. Blocks until ctx is
// cancelled. Should be started before the HTTP server accepts traffic (T015).
func (w *Worker) Run(ctx context.Context) {
	_, _ = w.Refresh(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _ = w.Refresh(ctx)
		}
	}
}
