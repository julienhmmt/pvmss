package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"testing"
	"time"
)

// okWorkerClient returns an empty snapshot so the background refresh succeeds.
type okWorkerClient struct{ cluster.Client }

func (okWorkerClient) Snapshot(context.Context) (cluster.Snapshot, error) {
	return cluster.Snapshot{}, nil
}

// TestClusterRefresh_LegacySingleRefresher exercises the no-registry path the
// handler keeps for direct construction.
func TestClusterRefresh_LegacySingleRefresher(t *testing.T) {
	t.Parallel()

	worker := inventory.NewWorker(okWorkerClient{}, inventory.NewProjection(), time.Hour, slog.New(slog.DiscardHandler))
	h := &ClusterRefresh{refresher: inventory.NewRefresher(worker, 5*time.Second), log: slog.New(slog.DiscardHandler)}

	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/cluster/refresh", nil))

	if w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", w.Code)
	}
}
