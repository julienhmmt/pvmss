//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"testing"
	"time"
)

// TestClusterRefresh_Success — POST /cluster/refresh returns 202 Accepted
// immediately; the refresh runs in the background and populates the projection.
//
//nolint:paralleltest // serial: shared inventory fixture
func TestClusterRefresh_Success(t *testing.T) {
	client := &stubClusterClient{snapshot: cluster.Snapshot{
		Nodes: []cluster.Node{{Name: cluster.FakeNode01, Status: cluster.NodeOnline}},
	}}
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(client, projection, time.Hour, slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	refresher := inventory.NewRefresher(worker, 5*time.Second)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	h := httpapi.NewClusterRefresh(refresher, logger)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/refresh", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusAccepted)
	}

	var got struct {
		RefreshedAt string `json:"refreshedAt"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.RefreshedAt == "" {
		t.Fatal("refreshedAt should not be empty")
	}

	// Wait for the background refresh to populate the projection.
	waitForProjection(t, projection, 2*time.Second)
}

// TestClusterRefresh_TooSoon — a second immediate call returns 429 with
// retryAfterSeconds (FR-006, contracts/cluster-refresh.md).
//
//nolint:paralleltest // serial: shared inventory fixture
func TestClusterRefresh_TooSoon(t *testing.T) {
	client := &stubClusterClient{snapshot: cluster.Snapshot{
		Nodes: []cluster.Node{{Name: cluster.FakeNode01, Status: cluster.NodeOnline}},
	}}
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(client, projection, time.Hour, slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	refresher := inventory.NewRefresher(worker, 5*time.Second)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	h := httpapi.NewClusterRefresh(refresher, logger)

	// First refresh returns 202 (async).
	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/refresh", nil)
	h.ServeHTTP(w1, r1)

	if w1.Code != http.StatusAccepted {
		t.Fatalf("first refresh status = %d, want %d", w1.Code, http.StatusAccepted)
	}

	// Wait for the background refresh to populate the projection so the guard
	// check on the second call sees a recent RefreshedAt.
	waitForProjection(t, projection, 2*time.Second)

	// Second immediate refresh is refused.
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/refresh", nil)
	h.ServeHTTP(w2, r2)

	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second refresh status = %d, want %d", w2.Code, http.StatusTooManyRequests)
	}

	var got struct {
		Code              string `json:"code"`
		Message           string `json:"message"`
		RetryAfterSeconds int    `json:"retryAfterSeconds"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.Code != "refresh_too_soon" {
		t.Fatalf("code = %q, want refresh_too_soon", got.Code)
	}

	if got.RetryAfterSeconds < 1 || got.RetryAfterSeconds > 5 {
		t.Fatalf("retryAfterSeconds = %d, want in (0, 5] — the remaining guard time, not more than the full interval", got.RetryAfterSeconds)
	}

	if w2.Header().Get("Retry-After") == "" {
		t.Fatal("Retry-After header should be set")
	}
}

// TestClusterRefresh_TooSoonMakesZeroClientCalls — SC-001: a 429 refusal
// never triggers a cluster client call.
//
//nolint:paralleltest // serial: shared inventory fixture
func TestClusterRefresh_TooSoonMakesZeroClientCalls(t *testing.T) {
	client := &stubClusterClient{snapshot: cluster.Snapshot{
		Nodes: []cluster.Node{{Name: cluster.FakeNode01}},
	}}
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(client, projection, time.Hour, slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	refresher := inventory.NewRefresher(worker, 5*time.Second)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	h := httpapi.NewClusterRefresh(refresher, logger)

	// First refresh (async — wait for it to complete).
	w1 := httptest.NewRecorder()
	r1 := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/refresh", nil)
	h.ServeHTTP(w1, r1)
	waitForProjection(t, projection, 2*time.Second)

	callsAfterFirst := client.calls

	// Second immediate refresh — should be refused with 0 additional calls.
	w2 := httptest.NewRecorder()
	r2 := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/refresh", nil)
	h.ServeHTTP(w2, r2)

	if client.calls != callsAfterFirst {
		t.Fatalf("429 refusal should make 0 client calls, got %d additional", client.calls-callsAfterFirst)
	}
}

// TestClusterRefresh_Unreachable — with async refresh the handler returns
// 202 Accepted immediately even when the cluster is unreachable; the refresh
// fails in the background and the client learns the outcome by re-reading the
// projection (re-loading the VM list or polling /health).
//
//nolint:paralleltest // serial: shared inventory fixture
func TestClusterRefresh_Unreachable(t *testing.T) {
	client := &stubClusterClient{err: cluster.ErrUnreachable}
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(client, projection, time.Hour, slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	refresher := inventory.NewRefresher(worker, 1*time.Millisecond)
	time.Sleep(2 * time.Millisecond)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	h := httpapi.NewClusterRefresh(refresher, logger)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/cluster/refresh", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d (async refresh returns 202 immediately)", w.Code, http.StatusAccepted)
	}

	var got struct {
		RefreshedAt string `json:"refreshedAt"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got.RefreshedAt == "" {
		t.Fatal("refreshedAt should not be empty")
	}
}

// waitForProjection polls until the projection is non-nil or the deadline fires.
func waitForProjection(t *testing.T, projection *inventory.Projection, timeout time.Duration) {
	t.Helper()

	deadline := time.After(timeout)

	for projection.Load() == nil {
		select {
		case <-deadline:
			t.Fatal("projection did not populate within timeout")
		default:
			time.Sleep(5 * time.Millisecond)
		}
	}
}

// TestClusterRefresh_MethodNotAllowed — non-POST returns 405.
//
//nolint:paralleltest // serial: shared inventory fixture
func TestClusterRefresh_MethodNotAllowed(t *testing.T) {
	client := &stubClusterClient{}
	projection := inventory.NewProjection()
	worker := inventory.NewWorker(client, projection, time.Hour, slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	refresher := inventory.NewRefresher(worker, 5*time.Second)
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	h := httpapi.NewClusterRefresh(refresher, logger)

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/api/v1/cluster/refresh", nil)
	h.ServeHTTP(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}
