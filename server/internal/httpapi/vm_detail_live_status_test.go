package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"testing"
	"time"
)

// liveRunning reports every VM as running, while the projection still holds
// the seeded stopped web-02 (VMID 101): the state right after a start.
type liveRunning struct{}

func (liveRunning) VMStatus(context.Context, string, int) (cluster.VMLiveStatus, error) {
	return cluster.VMLiveStatus{Status: cluster.VMRunning, Uptime: 42 * time.Second}, nil
}

// The projection lags a power action by up to a refresh interval; the detail
// must report the live power state, or reloading a tab flips the page back.
//
//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_GetReportsLiveStatusOverProjection(t *testing.T) {
	t.Cleanup(cluster.ResetFake)

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	authHandler := newAuthHandler(t)
	handler := httpapi.NewVMDetailWithRegistry(httpapi.VMDetailDeps{
		Projection:   buildProjectionWithIndex(t, snap, time.Now()),
		Auth:         authHandler,
		StatusReader: liveRunning{},
		Log:          slog.New(slog.DiscardHandler),
	})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodGet, "/api/v1/vms/default/101", "", aliceCookie(t, authHandler)))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		Status        string `json:"status"`
		UptimeSeconds int64  `json:"uptimeSeconds"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if body.Status != string(cluster.VMRunning) || body.UptimeSeconds != 42 {
		t.Fatalf("detail = %+v, want the live running status with 42s uptime", body)
	}
}
