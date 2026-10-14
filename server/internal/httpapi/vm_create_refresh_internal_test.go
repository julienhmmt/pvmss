package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
	"sync"
	"testing"
	"time"
)

// evRefresh is the event recorded for each projection rebuild.
const evRefresh = "refresh"

// scriptedCreator reports TaskRunning for the first running polls, then the
// given final state. Every TaskStatus call is recorded as an event.
type scriptedCreator struct {
	cluster.Creator

	mu      sync.Mutex
	running int
	final   cluster.TaskState
	events  *[]string
}

func (c *scriptedCreator) TaskStatus(_ context.Context, _ string) (cluster.TaskStatus, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.running > 0 {
		c.running--

		return cluster.TaskStatus{State: cluster.TaskRunning}, nil
	}

	*c.events = append(*c.events, "task:"+string(c.final))

	return cluster.TaskStatus{State: c.final, ExitMessage: "boom"}, nil
}

// recordingRefresher counts projection refreshes and records them in order.
type recordingRefresher struct {
	mu     sync.Mutex
	events *[]string
	err    error
}

func (r *recordingRefresher) Refresh(_ context.Context) (time.Time, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	*r.events = append(*r.events, evRefresh)

	return time.Now(), r.err
}

// withFastCreateRefresh shortens the task poll and the refresh backoff for the
// duration of one test, restoring the package values on cleanup.
func withFastCreateRefresh(t *testing.T, backoff []time.Duration) {
	t.Helper()

	origPoll, origBackoff := vm.CreateTaskPoll, createRefreshBackoff
	vm.CreateTaskPoll = time.Millisecond
	createRefreshBackoff = backoff

	t.Cleanup(func() {
		vm.CreateTaskPoll = origPoll
		createRefreshBackoff = origBackoff
	})
}

func newQuietCreateLog() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// The projection must be rebuilt only once the create task reports OK, and
// again on the backoff schedule so a lagging /cluster/resources still lands.
//
//nolint:paralleltest // serial: swaps package-level poll and backoff vars
func TestAwaitCreateThenRefresh_RefreshesAfterTaskOK(t *testing.T) {
	withFastCreateRefresh(t, []time.Duration{0, time.Millisecond, time.Millisecond})

	var events []string

	creator := &scriptedCreator{running: 2, final: cluster.TaskOK, events: &events}
	refresher := &recordingRefresher{events: &events}
	h := &VMCreate{log: newQuietCreateLog()}

	h.awaitCreateThenRefresh(context.Background(), creator, refresher, "c", "upid")

	want := []string{"task:" + string(cluster.TaskOK), evRefresh, evRefresh, evRefresh}
	if len(events) != len(want) {
		t.Fatalf("events = %v, want %v", events, want)
	}

	for i := range want {
		if events[i] != want[i] {
			t.Fatalf("events = %v, want %v", events, want)
		}
	}
}

// A failed task must never trigger a projection rebuild.
//
//nolint:paralleltest // serial: swaps package-level poll and backoff vars
func TestAwaitCreateThenRefresh_SkipsRefreshOnTaskError(t *testing.T) {
	withFastCreateRefresh(t, []time.Duration{0})

	var events []string

	creator := &scriptedCreator{final: cluster.TaskError, events: &events}
	refresher := &recordingRefresher{events: &events}
	h := &VMCreate{log: newQuietCreateLog()}

	h.awaitCreateThenRefresh(context.Background(), creator, refresher, "c", "upid")

	for _, e := range events {
		if e == evRefresh {
			t.Fatalf("refresh ran for a failed task: %v", events)
		}
	}
}

// A refresh error is logged and the remaining attempts still run.
//
//nolint:paralleltest // serial: swaps package-level poll and backoff vars
func TestAwaitCreateThenRefresh_ContinuesOnRefreshError(t *testing.T) {
	withFastCreateRefresh(t, []time.Duration{0, time.Millisecond})

	var events []string

	creator := &scriptedCreator{final: cluster.TaskOK, events: &events}
	refresher := &recordingRefresher{events: &events, err: errors.New("unreachable")}
	h := &VMCreate{log: newQuietCreateLog()}

	h.awaitCreateThenRefresh(context.Background(), creator, refresher, "c", "upid")

	refreshes := 0

	for _, e := range events {
		if e == evRefresh {
			refreshes++
		}
	}

	if refreshes != 2 {
		t.Fatalf("refreshes = %d, want 2 (events %v)", refreshes, events)
	}
}
