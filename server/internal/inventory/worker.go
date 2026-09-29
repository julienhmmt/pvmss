package inventory

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"pvmss/server/internal/cluster"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// defaultRefreshTimeout caps how long a single cluster.Snapshot call may take.
// A slow or hung upstream must not block the singleflight-serialized refresh
// cycle indefinitely - every external call has a timeout (golang-design-patterns
// rule 9). Override with WithRefreshTimeout.
//
// It must be at least as long as the Proxmox HTTP client's 20s per-attempt
// timeout plus a small margin, otherwise the worker cancels before the client
// can return its own network error and the refresh logs a generic
// "context deadline exceeded" instead of "cluster unreachable".
const defaultRefreshTimeout = 25 * time.Second

// Worker owns the refresh cycle: it calls cluster.Client.Snapshot, builds an
// Index, and atomically swaps it into the Projection on success. On failure
// it logs and keeps the previous index. The cycle is serialized via
// a singleflight mechanism so concurrent refresh requests (automatic + manual)
// result in exactly one client call (edge case: in-flight dedup).
type Worker struct {
	client     cluster.Client
	projection *Projection
	cluster    string
	interval   time.Duration
	timeout    time.Duration
	log        *slog.Logger
	now        func() time.Time

	// Health transition state. Only touched inside refreshCycle, which the
	// singleflight in Refresh serialises, so it needs no lock of its own.
	down         bool
	downSince    time.Time
	lastReminder time.Time

	mu       sync.Mutex
	inFlight *inFlightRefresh
}

// DownReminderInterval is how often a cluster that stays down is re-announced
// at Warn. The first failure is Error, recovery is Info; nothing in between.
const DownReminderInterval = 5 * time.Minute

// WithClock replaces time.Now, so tests can drive the down/reminder timeline.
func WithClock(now func() time.Time) Option {
	return func(w *Worker) { w.now = now }
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

// WithClusterName stamps refreshed VMs with their owning registry key.
func WithClusterName(name string) Option {
	return func(w *Worker) { w.cluster = name }
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
		now:        time.Now,
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

	// Parent span for the whole cycle: the Proxmox calls made by Snapshot
	// become its children. The status carries no error text (it can embed
	// hostnames and Proxmox messages); the logs hold the detail.
	ctx, span := otel.Tracer("pvmss/inventory").Start(ctx, "inventory.refresh", trace.WithAttributes(attribute.String("cluster", w.cluster)))
	defer span.End()

	started := w.now()

	snap, err := w.client.Snapshot(ctx)
	if err != nil {
		span.SetStatus(codes.Error, "refresh failed")

		// A deadline exceeded from the worker's own timeout is still a cluster
		// unreachability signal: the cluster did not respond before we gave up.
		// Wrap it so downstream code and logs see cluster.ErrUnreachable.
		if errors.Is(err, context.DeadlineExceeded) {
			err = fmt.Errorf("%w: %w", cluster.ErrUnreachable, err)
		}

		w.noteFailure(ctx, err)

		return time.Time{}, err
	}

	idx := BuildIndexForCluster(w.cluster, snap)
	idx.RefreshedAt = time.Now()
	w.projection.store(&idx)
	w.noteSuccess(ctx, started, snap)

	return idx.RefreshedAt, nil
}

// noteFailure logs health transitions, not repetitions: Error on the first
// failure, then a Warn reminder every DownReminderInterval while it stays down.
func (w *Worker) noteFailure(ctx context.Context, err error) {
	now := w.now()

	if !w.down {
		w.down, w.downSince, w.lastReminder = true, now, now
		w.log.ErrorContext(ctx, "cluster unreachable", "component", "inventory", "cluster", w.cluster, "error", err)

		return
	}

	if now.Sub(w.lastReminder) >= DownReminderInterval {
		w.lastReminder = now
		w.log.WarnContext(ctx, "cluster still unreachable", "component", "inventory", "cluster", w.cluster,
			"downForMs", now.Sub(w.downSince).Milliseconds(), "error", err)
	}
}

// noteSuccess announces recovery once, then keeps per-refresh detail at Debug.
func (w *Worker) noteSuccess(ctx context.Context, started time.Time, snap cluster.Snapshot) {
	now := w.now()

	if w.down {
		w.log.InfoContext(ctx, "cluster recovered", "component", "inventory", "cluster", w.cluster,
			"downForMs", now.Sub(w.downSince).Milliseconds())
		w.down = false
	}

	w.log.DebugContext(ctx, "inventory refreshed", "component", "inventory", "cluster", w.cluster,
		"durationMs", now.Sub(started).Milliseconds(), "vms", len(snap.VMs), "nodes", len(snap.Nodes))
}

// Refresh performs one refresh cycle. If a cycle is already in progress,
// waits for it and returns its result instead of starting a second call
// (edge case: manual vs. automatic in-flight dedup). A panic in refreshCycle
// is recovered so the inFlight state is always cleared and future refreshes
// are not permanently blocked.
func (w *Worker) Refresh(ctx context.Context) (at time.Time, err error) {
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

	defer func() {
		if rec := recover(); rec != nil {
			w.log.ErrorContext(ctx, "inventory refresh panic recovered", "component", "inventory", "cluster", w.cluster, "panic", rec)
			err = fmt.Errorf("inventory refresh panic: %v", rec)
		}
		// Always clear inFlight and close the done channel, even on panic,
		// so waiting callers and future refresh cycles are not blocked.
		w.mu.Lock()
		w.inFlight = nil
		w.mu.Unlock()
		close(pending.done)
	}()

	at, err = w.refreshCycle(ctx)
	pending.at = at
	pending.err = err

	return at, err
}

// Run starts the automatic ticker loop. It performs an initial refresh
// immediately, then ticks at the configured interval. Blocks until ctx is
// cancelled. Should be started before the HTTP server accepts traffic.
// A panic in a single refresh cycle is recovered inside Refresh; the ticker
// loop itself also has a recovery guard so the worker never silently dies.
func (w *Worker) Run(ctx context.Context) {
	defer func() {
		if rec := recover(); rec != nil {
			w.log.ErrorContext(ctx, "inventory worker loop panic recovered", "component", "inventory", "cluster", w.cluster, "panic", rec)
		}
	}()

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
