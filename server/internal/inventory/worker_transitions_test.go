package inventory_test

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"strings"
	"sync"
	"testing"
	"time"
)

type syncBuf struct {
	mu sync.Mutex
	b  strings.Builder
}

func (s *syncBuf) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.b.Write(p)
}

func (s *syncBuf) lines(t *testing.T) []map[string]any {
	t.Helper()

	s.mu.Lock()
	defer s.mu.Unlock()

	var out []map[string]any

	for _, l := range strings.Split(strings.TrimSpace(s.b.String()), "\n") {
		if l == "" {
			continue
		}

		m := map[string]any{}
		if err := json.Unmarshal([]byte(l), &m); err != nil {
			t.Fatalf("bad line %q: %v", l, err)
		}

		out = append(out, m)
	}

	return out
}

func newTransitionWorker(t *testing.T, client *callCountClient, clock *time.Time) (*inventory.Worker, *syncBuf) {
	t.Helper()

	buf := &syncBuf{}
	log := slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	w := inventory.NewWorker(client, inventory.NewProjection(), time.Hour, log,
		inventory.WithClusterName("lab"), inventory.WithClock(func() time.Time { return *clock }))

	return w, buf
}

func countMsg(lines []map[string]any, msg, level string) int {
	n := 0

	for _, l := range lines {
		if l["msg"] == msg && l["level"] == level {
			n++
		}
	}

	return n
}

func TestWorker_LogsTransitionsNotRepetitions(t *testing.T) {
	t.Parallel()

	clock := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	client := &callCountClient{err: errors.New("dial tcp: refused")}
	w, buf := newTransitionWorker(t, client, &clock)
	ctx := context.Background()

	_, _ = w.Refresh(ctx) // first failure
	_, _ = w.Refresh(ctx) // still down, inside the reminder window

	lines := buf.lines(t)
	if countMsg(lines, "cluster unreachable", "ERROR") != 1 || len(lines) != 1 {
		t.Fatalf("after 2 failures want exactly one Error line, got %v", lines)
	}

	clock = clock.Add(inventory.DownReminderInterval + time.Second)
	_, _ = w.Refresh(ctx) // reminder due

	client.err = nil
	clock = clock.Add(time.Minute)
	_, _ = w.Refresh(ctx) // recovery

	lines = buf.lines(t)
	if countMsg(lines, "cluster unreachable", "ERROR") != 1 || countMsg(lines, "cluster still unreachable", "WARN") != 1 {
		t.Fatalf("want 1 Error + 1 Warn reminder, got %v", lines)
	}

	if countMsg(lines, "cluster recovered", "INFO") != 1 || countMsg(lines, "inventory refreshed", "DEBUG") != 1 {
		t.Fatalf("want 1 Info recovered + 1 Debug refreshed, got %v", lines)
	}

	for _, l := range lines {
		if l["cluster"] != "lab" {
			t.Errorf("line without cluster: %v", l)
		}

		if l["msg"] == "cluster recovered" && l["downForMs"] == nil {
			t.Errorf("recovered without downForMs: %v", l)
		}
	}
}

func TestWorker_SuccessWhileUpOnlyLogsDebugSummary(t *testing.T) {
	t.Parallel()

	clock := time.Now()
	client := &callCountClient{snapshot: cluster.Snapshot{
		Nodes: []cluster.Node{{Name: "n1"}}, VMs: []cluster.VM{{VMID: 1}, {VMID: 2}},
	}}
	w, buf := newTransitionWorker(t, client, &clock)

	_, _ = w.Refresh(context.Background())

	lines := buf.lines(t)
	if len(lines) != 1 || lines[0]["level"] != "DEBUG" || lines[0]["vms"] != float64(2) || lines[0]["nodes"] != float64(1) || lines[0]["durationMs"] == nil {
		t.Fatalf("lines = %v", lines)
	}
}
