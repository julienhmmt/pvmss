//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"strings"
	"testing"
)

// TestVMBulk_MethodNotAllowed — a non-POST method → 405 with an Allow: POST
// header, before any auth or body parsing.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMBulk_MethodNotAllowed(t *testing.T) {
	handler, _ := newVMBulkHandler(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vms/bulk-action", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}

	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "POST") {
		t.Errorf("Allow header = %q, want it to contain POST", allow)
	}
}

// TestVMBulk_InvalidBody — malformed JSON → 400 invalid_request.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMBulk_InvalidBody(t *testing.T) {
	handler, authHandler := newVMBulkHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveBulkError(handler, bulkRequest(`{not json`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if env.Code != apiCodeInvalidRequest {
		t.Errorf("code = %q, want invalid_request", env.Code)
	}
}

// TestVMBulk_NilInventoryProducesErrorEntry — when the projection was never
// populated, singleClusterResolver.IndexFor returns an error and BulkAction
// records one "error" entry per target (the batch never fails as a whole).
// This covers the nil-index branch of singleClusterResolver.IndexFor.
//
//nolint:paralleltest // serial: shared fake authentication state
func TestVMBulk_NilInventoryProducesErrorEntry(t *testing.T) {
	t.Cleanup(cluster.ResetFake)

	authHandler := newAuthHandler(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	st, err := store.Open(config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "vm-bulk-nil.db"),
		LogLevel:  snapshotTestLogLevel,
		LogFormat: snapshotTestLogFormat,
		LogOutput: snapshotTestLogOutput,
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	projection := inventory.NewProjection()
	handler := httpapi.NewVMBulk(projection, authHandler, cluster.Fake{}, st, bulkNoopRefresher{}, logger)
	cookie := aliceCookie(t, authHandler)

	rec, resp := serveBulk(handler, bulkRequest(bulkBody("start", bulkTargets(101)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if len(resp.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(resp.Results))
	}

	if resp.Results[0].Status != "error" {
		t.Errorf("status = %q, want error (inventory not ready)", resp.Results[0].Status)
	}

	if !strings.Contains(resp.Results[0].Message, "inventory has not been populated") {
		t.Errorf("message = %q, want it to mention the unpopulated inventory", resp.Results[0].Message)
	}
}
