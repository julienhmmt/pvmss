package inventory

import (
	"context"
	"errors"
	"log/slog"
	"pvmss/server/internal/cluster"
	"slices"
	"sync"
	"testing"
	"time"
)

// incClient records which snapshot path each cycle took. While gate is set,
// SnapshotFrom blocks on it, holding an incremental cycle in flight.
type incClient struct {
	cluster.Client

	mu    sync.Mutex
	calls []string
	fail  bool
	gate  chan struct{}
}

func (c *incClient) record(kind string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.calls = append(c.calls, kind)
	if c.fail {
		return errors.New("boom")
	}

	return nil
}

func (c *incClient) Snapshot(_ context.Context) (cluster.Snapshot, error) {
	return cluster.Snapshot{ProxmoxVersion: "8"}, c.record(cycleFull)
}

func (c *incClient) SnapshotFrom(_ context.Context, _ cluster.Snapshot) (cluster.Snapshot, error) {
	if c.gate != nil {
		<-c.gate
	}

	return cluster.Snapshot{ProxmoxVersion: "8"}, c.record(cycleIncremental)
}

func (c *incClient) got() []string {
	c.mu.Lock()
	defer c.mu.Unlock()

	return slices.Clone(c.calls)
}

const (
	cycleFull        = "full"
	cycleIncremental = "incremental"
)

func newIncWorker(client *incClient, now *time.Time) *Worker {
	return NewWorker(client, NewProjection(), time.Minute, slog.New(slog.DiscardHandler),
		WithClock(func() time.Time { return *now }))
}

func TestWorker_ScheduledCyclesAreIncrementalBetweenFullResyncs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	client := &incClient{}
	w := newIncWorker(client, &now)

	_, _ = w.refreshScheduled(ctx) // nothing to build on yet: full
	_, _ = w.refreshScheduled(ctx)
	_, _ = w.Refresh(ctx) // post-write / manual: always full
	_, _ = w.refreshScheduled(ctx)

	now = now.Add(fullResyncInterval)
	_, _ = w.refreshScheduled(ctx) // resync due

	client.fail = true
	now = now.Add(time.Minute)
	_, _ = w.refreshScheduled(ctx) // incremental, fails
	client.fail = false
	_, _ = w.refreshScheduled(ctx) // after a failure: full

	want := []string{cycleFull, cycleIncremental, cycleFull, cycleIncremental, cycleFull, cycleIncremental, cycleFull}
	if got := client.got(); !slices.Equal(got, want) {
		t.Errorf("cycles = %v, want %v", got, want)
	}
}

// A post-write Refresh must never be answered by an incremental cycle that
// was already running: it waits, then runs its own full cycle.
func TestWorker_RefreshDuringIncrementalRunsItsOwnFullCycle(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	now := time.Date(2026, 11, 17, 12, 0, 0, 0, time.UTC)
	client := &incClient{}
	w := newIncWorker(client, &now)

	_, _ = w.Refresh(ctx)

	client.gate = make(chan struct{})

	scheduled := make(chan struct{})

	go func() {
		_, _ = w.refreshScheduled(ctx)

		close(scheduled)
	}()

	waitInFlight(t, w)

	refreshed := make(chan struct{})

	go func() {
		_, _ = w.Refresh(ctx)

		close(refreshed)
	}()

	close(client.gate)
	<-scheduled
	<-refreshed

	want := []string{cycleFull, cycleIncremental, cycleFull}
	if got := client.got(); !slices.Equal(got, want) {
		t.Errorf("cycles = %v, want %v", got, want)
	}
}

func waitInFlight(t *testing.T, w *Worker) {
	t.Helper()

	deadline := time.Now().Add(2 * time.Second)

	for time.Now().Before(deadline) {
		w.mu.Lock()
		busy := w.inFlight != nil
		w.mu.Unlock()

		if busy {
			return
		}

		time.Sleep(time.Millisecond)
	}

	t.Fatal("no cycle in flight")
}
