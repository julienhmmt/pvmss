package httpapi_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"strconv"
	"strings"
	"testing"
)

// cloneSourceVMID returns the VMID of one VM in alice's pool, read from the
// same fake snapshot the handler's inventory is built from.
func cloneSourceVMID(t *testing.T) int {
	t.Helper()

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	index := inventory.BuildIndex(snap)

	for _, candidate := range index.ByPool[cluster.FakePoolAlice] {
		if len(candidate.Disks) > 0 && len(candidate.NetworkInterfaces) > 0 {
			return candidate.VMID
		}
	}

	t.Fatal("demo fixtures have no alice VM with a disk and a NIC")

	return 0
}

// newCloneHandler is the create harness with the inventory wired, so the
// clone route can resolve its source VM.
func newCloneHandler(t *testing.T) (*httpapi.VMCreate, *httpapi.Auth) {
	t.Helper()

	handler, authHandler, _ := newVMCreateHandler(t)

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	index := inventory.BuildIndex(snap)
	handler.SetInventorySource(inventory.NewRegistryFromIndexes(map[string]*inventory.Index{auditTestCluster: &index}))

	return handler, authHandler
}

func postVMClone(t *testing.T, handler *httpapi.VMCreate, vmid, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/vms/default/"+vmid+"/clone", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.SetPathValue("cluster", "default")
	request.SetPathValue("vmid", vmid)

	if cookie != nil {
		request.AddCookie(cookie)
	}

	recorder := httptest.NewRecorder()
	handler.ServeClone(recorder, request)

	return recorder
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMClone_OwnVMReturnsAccepted(t *testing.T) {
	handler, authHandler := newCloneHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)
	vmid := strconv.Itoa(cloneSourceVMID(t))

	response := postVMClone(t, handler, vmid, `{"name":"copy-of-source"}`, cookie)

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", response.Code, response.Body.String())
	}

	if !strings.Contains(response.Body.String(), `"upid"`) || !strings.Contains(response.Body.String(), `"copy-of-source"`) {
		t.Fatalf("body missing upid or name: %s", response.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMClone_ForeignVMForbidden(t *testing.T) {
	handler, authHandler := newCloneHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"bob","password":"pvmss-bob"}`)

	response := postVMClone(t, handler, strconv.Itoa(cloneSourceVMID(t)), `{"name":"copy"}`, cookie)
	if response.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", response.Code, response.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMClone_UnknownVMNotFound(t *testing.T) {
	handler, authHandler := newCloneHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	response := postVMClone(t, handler, "999999", `{"name":"copy"}`, cookie)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404: %s", response.Code, response.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMClone_Unauthenticated(t *testing.T) {
	handler, _ := newCloneHandler(t)

	response := postVMClone(t, handler, strconv.Itoa(cloneSourceVMID(t)), `{"name":"copy"}`, nil)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401: %s", response.Code, response.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMClone_MalformedBodyRejected(t *testing.T) {
	handler, authHandler := newCloneHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	response := postVMClone(t, handler, strconv.Itoa(cloneSourceVMID(t)), `not-json`, cookie)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", response.Code, response.Body.String())
	}
}
