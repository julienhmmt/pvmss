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

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
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

	for l := range strings.SplitSeq(strings.TrimSpace(s.b.String()), "\n") {
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

// One parent span per refresh cycle, named inventory.refresh with the cluster;
// failed cycles are marked as errors without the raw error text.
//
//nolint:paralleltest // serial: swaps the global tracer provider
func TestWorker_RefreshSpan(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exp)))
	prev := otel.GetTracerProvider()

	otel.SetTracerProvider(tp)
	t.Cleanup(func() { otel.SetTracerProvider(prev); _ = tp.Shutdown(context.Background()) })

	clock := time.Now()
	client := &callCountClient{err: errors.New("dial tcp 10.0.0.9: refused")}
	w, _ := newTransitionWorker(t, client, &clock)

	_, _ = w.Refresh(context.Background())

	client.err = nil
	_, _ = w.Refresh(context.Background())

	spans := exp.GetSpans()
	if len(spans) != 2 || spans[0].Name != "inventory.refresh" {
		t.Fatalf("want 2 inventory.refresh spans, got %v", spans)
	}

	if spans[0].Status.Code != codes.Error || strings.Contains(spans[0].Status.Description, "10.0.0.9") {
		t.Errorf("failed cycle status = %+v", spans[0].Status)
	}

	if spans[1].Status.Code == codes.Error {
		t.Errorf("successful cycle marked as error")
	}

	found := false

	for _, a := range spans[0].Attributes {
		found = found || (a.Key == "cluster" && a.Value.AsString() == "lab")
	}

	if !found {
		t.Errorf("cluster attribute missing: %v", spans[0].Attributes)
	}
}
