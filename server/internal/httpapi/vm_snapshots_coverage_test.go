//nolint:noctx // test scaffolding uses in-memory requests
package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"testing"
)

// completeSnapshotTask polls the fake's TaskStatus 3 times so the async
// snapshot creation completes and the snapshot becomes visible in ListSnapshots.
func completeSnapshotTask(t *testing.T, upid string) {
	t.Helper()
	for range 3 {
		if _, err := (cluster.Fake{}).TaskStatus(context.Background(), upid); err != nil {
			t.Fatalf("TaskStatus(%q): %v", upid, err)
		}
	}
}

// snapshotNameRequest builds a request with the snapshot name path value set
// correctly (the shared snapshotRequest helper sets segments[5] which is
// "snapshots" not the actual name).
func snapshotNameRequest(method, path, name, body string, cookie *http.Cookie) *http.Request {
	req := snapshotRequest(method, path, body, cookie)
	req.SetPathValue("name", name)
	return req
}

// createSnapshotAndWait creates a snapshot and completes the async task so the
// snapshot is immediately visible in subsequent List/Delete/Rollback calls.
func createSnapshotAndWait(t *testing.T, handler *httpapi.VMSnapshots, cookie *http.Cookie, path, body string) {
	t.Helper()
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodPost, path, body, cookie))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("create status = %d, want %d: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
	var task snapshotTaskResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &task); err != nil {
		t.Fatalf("decode task: %v", err)
	}
	completeSnapshotTask(t, task.UPID)
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_List_Unauthenticated(t *testing.T) {
	handler, _ := newVMSnapshotsHandler(t)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodGet, "/api/v1/vms/default/101/snapshots", "", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertAPIError(t, rec.Body.Bytes(), "unauthenticated")
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_List_NonOwnerForbidden(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := bobCookie(t, authHandler)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodGet, "/api/v1/vms/default/101/snapshots", "", cookie))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	assertAPIError(t, rec.Body.Bytes(), apiCodeForbidden)
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_List_UntaggedNotFound(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodGet, "/api/v1/vms/default/109/snapshots", "", cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	assertAPIError(t, rec.Body.Bytes(), "not_found")
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_List_NonexistentVM(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodGet, "/api/v1/vms/default/999/snapshots", "", cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	assertAPIError(t, rec.Body.Bytes(), "not_found")
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_List_AdminSeesAnyTaggedVM(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodGet, "/api/v1/vms/default/103/snapshots", "", cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_Create_Unauthenticated(t *testing.T) {
	handler, _ := newVMSnapshotsHandler(t)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodPost, "/api/v1/vms/default/101/snapshots", `{"name":"snap1"}`, nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertAPIError(t, rec.Body.Bytes(), "unauthenticated")
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_Create_InvalidJSON(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodPost, "/api/v1/vms/default/101/snapshots", "{bad json", cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertAPIError(t, rec.Body.Bytes(), "invalid_request")
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_Create_EmptyName(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodPost, "/api/v1/vms/default/101/snapshots", `{"name":""}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	assertAPIError(t, rec.Body.Bytes(), "invalid_name")
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_Create_ReservedNameCurrent(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodPost, "/api/v1/vms/default/101/snapshots", `{"name":"current"}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertAPIError(t, rec.Body.Bytes(), "invalid_name")
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_Create_DuplicateName(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)

	createSnapshotAndWait(t, handler, cookie, "/api/v1/vms/default/101/snapshots", `{"name":"dup-snap"}`)

	create2 := httptest.NewRecorder()
	handler.ServeHTTP(create2, snapshotRequest(http.MethodPost, "/api/v1/vms/default/101/snapshots", `{"name":"dup-snap"}`, cookie))
	if create2.Code != http.StatusBadRequest {
		t.Fatalf("second create status = %d, want %d: %s", create2.Code, http.StatusBadRequest, create2.Body.String())
	}
	assertAPIError(t, create2.Body.Bytes(), "duplicate_name")
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_Create_VMStateOnStoppedVM(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodPost, "/api/v1/vms/default/101/snapshots", `{"name":"with-ram","vmstate":true}`, cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	assertAPIError(t, rec.Body.Bytes(), "vmstate_requires_running")
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_Create_AdminOnAnyTaggedVM(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodPost, "/api/v1/vms/default/103/snapshots", `{"name":"admin-snap"}`, cookie))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_Delete_Unauthenticated(t *testing.T) {
	handler, _ := newVMSnapshotsHandler(t)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodDelete, "/api/v1/vms/default/101/snapshots/missing", "", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertAPIError(t, rec.Body.Bytes(), "unauthenticated")
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_Delete_NonOwnerForbidden(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := bobCookie(t, authHandler)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodDelete, "/api/v1/vms/default/101/snapshots/missing", "", cookie))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	assertAPIError(t, rec.Body.Bytes(), apiCodeForbidden)
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_Delete_UntaggedNotFound(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodDelete, "/api/v1/vms/default/109/snapshots/missing", "", cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	assertAPIError(t, rec.Body.Bytes(), "not_found")
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_Rollback_Unauthenticated(t *testing.T) {
	handler, _ := newVMSnapshotsHandler(t)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodPost, "/api/v1/vms/default/101/snapshots/missing/rollback", "", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
	assertAPIError(t, rec.Body.Bytes(), "unauthenticated")
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_Rollback_NonOwnerForbidden(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := bobCookie(t, authHandler)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodPost, "/api/v1/vms/default/101/snapshots/missing/rollback", "", cookie))
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
	assertAPIError(t, rec.Body.Bytes(), apiCodeForbidden)
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_Rollback_UntaggedNotFound(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodPost, "/api/v1/vms/default/109/snapshots/missing/rollback", "", cookie))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
	assertAPIError(t, rec.Body.Bytes(), "not_found")
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_MethodNotAllowed(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodPut, "/api/v1/vms/default/101/snapshots", "", cookie))
	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_Create_ThenDelete_Success(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)

	createSnapshotAndWait(t, handler, cookie, "/api/v1/vms/default/101/snapshots", `{"name":"to-delete"}`)

	deleteRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteRec, snapshotNameRequest(http.MethodDelete, "/api/v1/vms/default/101/snapshots/to-delete", "to-delete", "", cookie))
	if deleteRec.Code != http.StatusAccepted {
		t.Fatalf("delete status = %d, want %d: %s", deleteRec.Code, http.StatusAccepted, deleteRec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_Create_ThenRollback_Success(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)

	createSnapshotAndWait(t, handler, cookie, "/api/v1/vms/default/101/snapshots", `{"name":"to-rollback"}`)

	rollbackRec := httptest.NewRecorder()
	handler.ServeHTTP(rollbackRec, snapshotNameRequest(http.MethodPost, "/api/v1/vms/default/101/snapshots/to-rollback/rollback", "to-rollback", "", cookie))
	if rollbackRec.Code != http.StatusAccepted {
		t.Fatalf("rollback status = %d, want %d: %s", rollbackRec.Code, http.StatusAccepted, rollbackRec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_List_AfterCreateShowsSnapshot(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)

	createSnapshotAndWait(t, handler, cookie, "/api/v1/vms/default/101/snapshots", `{"name":"visible-snap","description":"test desc"}`)

	listRec := httptest.NewRecorder()
	handler.ServeHTTP(listRec, snapshotRequest(http.MethodGet, "/api/v1/vms/default/101/snapshots", "", cookie))
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", listRec.Code, listRec.Body.String())
	}

	var list snapshotListResponse
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	found := false
	for _, s := range list.Snapshots {
		if s.Name == "visible-snap" {
			found = true
		}
	}
	if !found {
		t.Errorf("snapshot 'visible-snap' not found in list: %+v", list.Snapshots)
	}
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_Create_InvalidVMPath(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/vms/default/0/snapshots", nil)
	req.SetPathValue("cluster", "default")
	req.SetPathValue("vmid", "0")
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	assertAPIError(t, rec.Body.Bytes(), "invalid_request")
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_Create_WithDescriptionAndVMState(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, snapshotRequest(http.MethodPost, "/api/v1/vms/default/100/snapshots", `{"name":"running-ram-snap","description":"with RAM","vmstate":true}`, cookie))
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake and SQLite fixtures
func TestVMSnapshotsCoverage_Delete_AdminOnAnyTaggedVM(t *testing.T) {
	handler, authHandler := newVMSnapshotsHandler(t)
	cookie := adminCookie(t, authHandler)

	createSnapshotAndWait(t, handler, cookie, "/api/v1/vms/default/103/snapshots", `{"name":"admin-del"}`)

	deleteRec := httptest.NewRecorder()
	handler.ServeHTTP(deleteRec, snapshotNameRequest(http.MethodDelete, "/api/v1/vms/default/103/snapshots/admin-del", "admin-del", "", cookie))
	if deleteRec.Code != http.StatusAccepted {
		t.Fatalf("delete status = %d, want %d: %s", deleteRec.Code, http.StatusAccepted, deleteRec.Body.String())
	}
}
