package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"sync"
	"sync/atomic"
	"testing"
)

var errBuildFailed = errors.New("inventory not ready")

func bodyHandler(body string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(body))
	})
}

func serveBody(h http.Handler) string {
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/v1/ping", nil)

	h.ServeHTTP(rec, req)

	return rec.Body.String()
}

func newSwapFixture(t *testing.T, build func() (http.Handler, error)) *setupRouterSwap {
	t.Helper()

	registry, err := cluster.NewRegistry("fake", []store.ClusterRow{{Name: "c1"}})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}

	return &setupRouterSwap{
		setup:    bodyHandler("setup"),
		registry: registry,
		log:      slog.New(slog.DiscardHandler),
		build:    build,
	}
}

// failFirstBuild returns a build that fails on its first call and succeeds after.
func failFirstBuild(calls *atomic.Int32) func() (http.Handler, error) {
	return func() (http.Handler, error) {
		if calls.Add(1) == 1 {
			return nil, errBuildFailed
		}

		return bodyHandler("full"), nil
	}
}

func TestSetupRouterSwap_RetriesAfterFailedBuild(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	swap := newSwapFixture(t, failFirstBuild(&calls))

	if got := serveBody(swap); got != "setup" {
		t.Fatalf("first request after failed build = %q, want setup fallback", got)
	}

	if got := serveBody(swap); got != "full" {
		t.Fatalf("second request = %q, want full router after retry", got)
	}

	if got := serveBody(swap); got != "full" {
		t.Fatalf("third request = %q, want cached full router", got)
	}

	if n := calls.Load(); n != 2 {
		t.Fatalf("build calls = %d, want 2 (one failure, one success)", n)
	}
}

func TestSetupRouterSwap_ConcurrentRequestsBuildOnce(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	swap := newSwapFixture(t, failFirstBuild(&calls))

	const workers = 32

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			_ = serveBody(swap)
		})
	}

	wg.Wait()

	if got := serveBody(swap); got != "full" {
		t.Fatalf("after concurrent burst = %q, want full router", got)
	}

	if n := calls.Load(); n != 2 {
		t.Fatalf("build calls = %d, want 2 (failures serialized, success cached)", n)
	}
}

func TestSetupRouterSwap_NoClusterAlwaysSetup(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32

	swap := newSwapFixture(t, func() (http.Handler, error) {
		calls.Add(1)

		return bodyHandler("full"), nil
	})
	swap.registry.Remove("c1")

	if got := serveBody(swap); got != "setup" {
		t.Fatalf("no cluster = %q, want setup", got)
	}

	if n := calls.Load(); n != 0 {
		t.Fatalf("build calls = %d, want 0 without a cluster", n)
	}
}
