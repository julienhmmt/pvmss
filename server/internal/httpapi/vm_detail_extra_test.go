//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"context"
	"errors"
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
	"time"
)

// errUnexpected is a non-sentinel error used to exercise the default (500)
// branches of writeActionError and writePatchError — it matches no sentinel
// the handlers know, so it must fall through to internal_error.
var errUnexpected = errors.New("unexpected cluster failure")

// Shared error-code / tag constants for the extra httpapi VM tests. Centralising
// them keeps the goconst linter quiet (the literal forms otherwise cross the
// package-wide 4-occurrence threshold alongside pre-existing test files).
const (
	apiCodeInvalidRequest         = "invalid_request"
	apiCodeUnauthenticated        = "unauthenticated"
	apiCodeMethodNotAllowed       = "method_not_allowed"
	apiCodeInventoryNotReady      = "inventory_not_ready"
	apiCodeClusterNotFound        = "cluster_not_found"
	apiCodeClusterError           = "cluster_error"
	apiCodeClusterUnreachable     = "cluster_unreachable"
	apiCodeInvalidStateTransition = "invalid_state_transition"
	apiCodeInternalError          = "internal_error"
	extraPvmssTag                 = "pvmss"
)

// detailFailingWriter embeds cluster.Fake (which already satisfies
// cluster.Writer) and overrides only Action / Delete / Patch so the
// cluster-write error-mapping branches of writeActionError / writePatchError
// are reachable without a live Proxmox. When the configured error is nil it
// delegates to the embedded Fake so success paths still mutate the dataset.
type detailFailingWriter struct {
	cluster.Fake
	actionErr error
	deleteErr error
	patchErr  error
}

func (w detailFailingWriter) Action(ctx context.Context, node string, vmid int, action string) error {
	if w.actionErr != nil {
		return w.actionErr
	}

	return w.Fake.Action(ctx, node, vmid, action)
}

func (w detailFailingWriter) Delete(ctx context.Context, node string, vmid int) error {
	if w.deleteErr != nil {
		return w.deleteErr
	}

	return w.Fake.Delete(ctx, node, vmid)
}

func (w detailFailingWriter) Patch(ctx context.Context, node string, vmid int, name, description string) error {
	if w.patchErr != nil {
		return w.patchErr
	}

	return w.Fake.Patch(ctx, node, vmid, name, description)
}

// newVMDetailHandlerWithWriter builds the detail handler over the fake dataset
// with a custom cluster.Writer, a real audit store, and a no-op refresher —
// used by the cluster-write error-mapping tests where the write must fail after
// Resolve succeeds.
func newVMDetailHandlerWithWriter(t *testing.T, writer cluster.Writer) (*httpapi.VMDetail, *httpapi.Auth) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	projection := buildProjectionWithIndex(t, snap, time.Now())
	authHandler := newAuthHandler(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	st, err := store.Open(config.Configuration{
		Port:      50001,
		DBPath:    filepath.Join(t.TempDir(), "vm-detail-extra.db"),
		LogLevel:  snapshotTestLogLevel,
		LogFormat: snapshotTestLogFormat,
		LogOutput: snapshotTestLogOutput,
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	handler := httpapi.NewVMDetail(projection, authHandler, writer, st, bulkNoopRefresher{}, logger)

	return handler, authHandler
}

// =============================================================================
// handleGet — registry and inventory-not-ready paths
// =============================================================================

// TestVMDetail_Get_RegistryClusterNotFound — when the handler is wired to a
// multi-cluster Registry and the path's cluster has no entry, handleGet
// returns 404 cluster_not_found (FR-015).
//
//nolint:paralleltest // serial: shared fake authentication state
func TestVMDetail_Get_RegistryClusterNotFound(t *testing.T) {
	// A registry that knows "secondary" but not "default".
	secondaryIdx := inventory.BuildIndexForCluster("secondary", cluster.Snapshot{
		VMs: []cluster.VM{{VMID: 101, Name: "secondary-web", Node: "secondary-node", Pool: cluster.FakePoolAlice, Tags: []string{extraPvmssTag}}},
	})
	registry := inventory.NewRegistryFromIndexes(map[string]*inventory.Index{"secondary": &secondaryIdx})
	projection := inventory.NewProjectionFromIndex(&secondaryIdx)
	authHandler := newAuthHandler(t)
	handler := httpapi.NewVMDetailWithRegistry(httpapi.VMDetailDeps{Source: registry, Projection: projection, Auth: authHandler, Writer: cluster.Fake{}, Store: nil, Refresher: nil, Log: slog.Default()})
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodGet, "/api/v1/vms/default/101", "", cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusNotFound, rec.Body.String())
	}

	if env.Code != apiCodeClusterNotFound {
		t.Errorf("code = %q, want cluster_not_found", env.Code)
	}
}

// TestVMDetail_Get_InventoryNotReady — a projection that was never populated
// returns 503 inventory_not_ready rather than panicking.
//
//nolint:paralleltest // serial: shared fake authentication state
func TestVMDetail_Get_InventoryNotReady(t *testing.T) {
	authHandler := newAuthHandler(t)
	projection := inventory.NewProjection()
	handler := httpapi.NewVMDetail(projection, authHandler, cluster.Fake{}, nil, bulkNoopRefresher{}, slog.Default())
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodGet, "/api/v1/vms/default/100", "", cookie))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	if env.Code != apiCodeInventoryNotReady {
		t.Errorf("code = %q, want inventory_not_ready", env.Code)
	}
}

// =============================================================================
// parsePath — invalid vmid segment
// =============================================================================

// TestVMDetail_Get_InvalidVMIDPath — a non-numeric or non-positive vmid
// segment is rejected with 400 invalid_request before any Resolve call.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Get_InvalidVMIDPath(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	for _, vmid := range []string{"abc", "0"} {
		t.Run(vmid, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/vms/default/"+vmid, nil)
			req.SetPathValue("cluster", "default")
			req.SetPathValue("vmid", vmid)
			req.AddCookie(cookie)

			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusBadRequest, rec.Body.String())
			}

			assertAPIError(t, rec.Body.Bytes(), apiCodeInvalidRequest)
		})
	}
}

// =============================================================================
// ServeHTTP — method not allowed on the base VM path
// =============================================================================

// TestVMDetail_MethodNotAllowed — a PUT to the base VM path (no disk/cdrom/
// network/hardware suffix) is rejected with 405 and an Allow header listing
// the four accepted verbs.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_MethodNotAllowed(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodPut, "/api/v1/vms/default/100", "", cookie))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}

	if env.Code != apiCodeMethodNotAllowed {
		t.Errorf("code = %q, want method_not_allowed", env.Code)
	}

	if allow := rec.Header().Get("Allow"); !strings.Contains(allow, "GET") || !strings.Contains(allow, "POST") {
		t.Errorf("Allow header = %q, want it to list the accepted verbs", allow)
	}
}

// =============================================================================
// handleDelete — inventory not ready
// =============================================================================

// TestVMDetail_Delete_InventoryNotReady — DELETE against a never-populated
// projection returns 503 inventory_not_ready.
//
//nolint:paralleltest // serial: shared fake authentication state
func TestVMDetail_Delete_InventoryNotReady(t *testing.T) {
	authHandler := newAuthHandler(t)
	projection := inventory.NewProjection()
	handler := httpapi.NewVMDetail(projection, authHandler, cluster.Fake{}, nil, bulkNoopRefresher{}, slog.Default())
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodDelete, "/api/v1/vms/default/100", "", cookie))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}

	if env.Code != apiCodeInventoryNotReady {
		t.Errorf("code = %q, want inventory_not_ready", env.Code)
	}
}

// =============================================================================
// writeActionError — cluster-write error mapping (action + delete share it)
// =============================================================================

// TestVMDetail_Action_ClusterNotFoundMapped — a cluster.ErrNotFound from the
// writer after Resolve succeeds maps to 502 cluster_error.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Action_ClusterNotFoundMapped(t *testing.T) {
	handler, authHandler := newVMDetailHandlerWithWriter(t, detailFailingWriter{actionErr: cluster.ErrNotFound})
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/101/actions", `{"action":"start"}`, cookie))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}

	if env.Code != apiCodeClusterError {
		t.Errorf("code = %q, want cluster_error", env.Code)
	}
}

// TestVMDetail_Action_ClusterUnreachableMapped — a cluster.ErrUnreachable from
// the writer maps to 502 cluster_unreachable.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Action_ClusterUnreachableMapped(t *testing.T) {
	handler, authHandler := newVMDetailHandlerWithWriter(t, detailFailingWriter{actionErr: cluster.ErrUnreachable})
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/101/actions", `{"action":"start"}`, cookie))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}

	if env.Code != apiCodeClusterUnreachable {
		t.Errorf("code = %q, want cluster_unreachable", env.Code)
	}
}

// TestVMDetail_Action_InvalidStateTransitionMapped — a status-incompatible
// transition (start on a running VM) maps to 409 invalid_state_transition.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Action_InvalidStateTransitionMapped(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	// VM 100 is running; start is incompatible → ErrInvalidStateTransition.
	rec, env := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/100/actions", `{"action":"start"}`, cookie))
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusConflict, rec.Body.String())
	}

	if env.Code != apiCodeInvalidStateTransition {
		t.Errorf("code = %q, want invalid_state_transition", env.Code)
	}
}

// TestVMDetail_Action_DefaultErrorMapped — a non-sentinel writer error maps to
// 500 internal_error (the writeActionError default branch).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Action_DefaultErrorMapped(t *testing.T) {
	handler, authHandler := newVMDetailHandlerWithWriter(t, detailFailingWriter{actionErr: errUnexpected})
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodPost, "/api/v1/vms/default/101/actions", `{"action":"start"}`, cookie))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	if env.Code != apiCodeInternalError {
		t.Errorf("code = %q, want internal_error", env.Code)
	}
}

// TestVMDetail_Delete_ClusterNotFoundMapped — DELETE shares writeActionError;
// a cluster.ErrNotFound from the writer maps to 502 cluster_error.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Delete_ClusterNotFoundMapped(t *testing.T) {
	handler, authHandler := newVMDetailHandlerWithWriter(t, detailFailingWriter{deleteErr: cluster.ErrNotFound})
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodDelete, "/api/v1/vms/default/101", "", cookie))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}

	if env.Code != apiCodeClusterError {
		t.Errorf("code = %q, want cluster_error", env.Code)
	}
}

// =============================================================================
// writePatchError — cluster-write and validation error mapping
// =============================================================================

// TestVMDetail_Patch_DescriptionTooLong — a description over the max length
// maps to 400 invalid_request (writePatchError's ErrDescriptionTooLong branch).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Patch_DescriptionTooLong(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	long := strings.Repeat("a", 600)

	rec, env := serveDetailError(handler, detailRequest(http.MethodPatch, "/api/v1/vms/default/100", `{"description":"`+long+`"}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if env.Code != apiCodeInvalidRequest {
		t.Errorf("code = %q, want invalid_request", env.Code)
	}
}

// TestVMDetail_Patch_InvalidBody — malformed JSON maps to 400 invalid_request
// (the decodeJSON branch of handlePatch).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Patch_InvalidBody(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodPatch, "/api/v1/vms/default/100", `{not json`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if env.Code != apiCodeInvalidRequest {
		t.Errorf("code = %q, want invalid_request", env.Code)
	}
}

// TestVMDetail_Patch_ClusterNotFoundMapped — a cluster.ErrNotFound from the
// patch writer maps to 502 cluster_error (writePatchError's ErrNotFound branch).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Patch_ClusterNotFoundMapped(t *testing.T) {
	handler, authHandler := newVMDetailHandlerWithWriter(t, detailFailingWriter{patchErr: cluster.ErrNotFound})
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodPatch, "/api/v1/vms/default/100", `{"name":"web-new"}`, cookie))
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}

	if env.Code != apiCodeClusterError {
		t.Errorf("code = %q, want cluster_error", env.Code)
	}
}

// TestVMDetail_Patch_DefaultErrorMapped — a non-sentinel patch writer error
// maps to 500 internal_error (the writePatchError default branch).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_Patch_DefaultErrorMapped(t *testing.T) {
	handler, authHandler := newVMDetailHandlerWithWriter(t, detailFailingWriter{patchErr: errUnexpected})
	cookie := aliceCookie(t, authHandler)

	rec, env := serveDetailError(handler, detailRequest(http.MethodPatch, "/api/v1/vms/default/100", `{"name":"web-new"}`, cookie))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}

	if env.Code != apiCodeInternalError {
		t.Errorf("code = %q, want internal_error", env.Code)
	}
}
