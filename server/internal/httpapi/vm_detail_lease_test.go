//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"strings"
	"testing"
	"time"
)

func newLeaseDetail(t *testing.T) (*httpapi.VMDetail, *httpapi.Auth, *store.Store) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)

	fake := cluster.NewFake(auditTestCluster)

	snap, err := fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	projection := buildProjectionWithIndex(t, snap, time.Now())
	authHandler := newAuthHandler(t)
	logger := slog.New(slog.DiscardHandler)
	st := newAdminStore(t)
	worker := inventory.NewWorker(fake, projection, time.Hour, logger)

	return httpapi.NewVMDetailWithRegistry(httpapi.VMDetailDeps{
		Source: projectionRegistry(projection), Projection: projection, Auth: authHandler,
		Writer: fake, Store: st, Refresher: worker, Log: logger,
	}, testPolicyService(st, projection)), authHandler, st
}

func doLease(t *testing.T, handler *httpapi.VMDetail, method, vmidPath, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	var reader *strings.Reader
	if body == "" {
		reader = strings.NewReader("")
	} else {
		reader = strings.NewReader(body)
	}

	req := httptest.NewRequest(method, "/api/v1/vms/default/"+vmidPath+"/lease", reader)
	req.SetPathValue("cluster", "default")
	req.SetPathValue("vmid", vmidPath)

	if cookie != nil {
		req.AddCookie(cookie)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	return recorder
}

//nolint:paralleltest // serial: shared fake fixtures
func TestLeaseExtend_Success(t *testing.T) {
	handler, authHandler, st := newLeaseDetail(t)
	seedLeasePolicy(t, st, 30)
	seedVMLease(t, st, 101)

	response := doLease(t, handler, http.MethodPut, "101", `{"days":5}`, aliceCookie(t, authHandler))
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s, want 200", response.Code, response.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake fixtures
func TestLeaseExtend_Rejections(t *testing.T) {
	handler, authHandler, st := newLeaseDetail(t)
	seedLeasePolicy(t, st, 30)

	alice := aliceCookie(t, authHandler)

	for _, testCase := range []struct {
		name, method, path, body string
		cookie                   *http.Cookie
		want                     int
	}{
		{"unauthenticated", http.MethodPut, "101", `{"days":5}`, nil, http.StatusUnauthorized},
		{"bad path", http.MethodPut, badVMIDPath, `{"days":5}`, alice, http.StatusBadRequest},
		{"bad json", http.MethodPut, "101", `{`, alice, http.StatusBadRequest},
		{"zero days", http.MethodPut, "101", `{"days":0}`, alice, http.StatusBadRequest},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			response := doLease(t, handler, testCase.method, testCase.path, testCase.body, testCase.cookie)
			if response.Code != testCase.want {
				t.Fatalf("status = %d body = %s, want %d", response.Code, response.Body.String(), testCase.want)
			}
		})
	}
}

//nolint:paralleltest // serial: shared fake fixtures
func TestLeaseExtend_LeasesDisabled(t *testing.T) {
	handler, authHandler, st := newLeaseDetail(t)
	seedLeasePolicy(t, st, 0)

	response := doLease(t, handler, http.MethodPut, "101", `{"days":5}`, aliceCookie(t, authHandler))
	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d body = %s, want 409", response.Code, response.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake fixtures
func TestLeaseRemove_GuardsAndSuccess(t *testing.T) {
	handler, authHandler, st := newLeaseDetail(t)
	seedLeasePolicy(t, st, 30)

	admin := adminCookie(t, authHandler)
	alice := aliceCookie(t, authHandler)

	if got := doLease(t, handler, http.MethodDelete, "101", "", nil).Code; got != http.StatusUnauthorized {
		t.Fatalf("unauthenticated = %d, want 401", got)
	}

	if got := doLease(t, handler, http.MethodDelete, "101", "", alice).Code; got != http.StatusForbidden {
		t.Fatalf("non-admin = %d, want 403", got)
	}

	if got := doLease(t, handler, http.MethodDelete, badVMIDPath, "", admin).Code; got != http.StatusBadRequest {
		t.Fatalf("bad path = %d, want 400", got)
	}
	// No lease row yet -> lease_not_found.
	if got := doLease(t, handler, http.MethodDelete, "101", "", admin).Code; got != http.StatusNotFound {
		t.Fatalf("no lease = %d, want 404", got)
	}

	seedVMLease(t, st, 101)

	if got := doLease(t, handler, http.MethodDelete, "101", "", admin).Code; got != http.StatusOK {
		t.Fatalf("remove = %d, want 200", got)
	}
}

//nolint:paralleltest // serial: shared fake fixtures
func TestLease_MethodNotAllowed(t *testing.T) {
	handler, _, _ := newLeaseDetail(t)

	if got := doLease(t, handler, http.MethodGet, "101", "", nil).Code; got != http.StatusMethodNotAllowed {
		t.Fatalf("GET /lease = %d, want 405", got)
	}
}

const badVMIDPath = "abc"

func seedVMLease(t *testing.T, st *store.Store, vmid int) {
	t.Helper()

	lease := store.VMLease{Cluster: auditTestCluster, VMID: vmid, ExpiresAt: time.Now().Add(24 * time.Hour)}
	if err := st.SetVMLease(context.Background(), lease); err != nil {
		t.Fatalf("SetVMLease: %v", err)
	}
}

func seedLeasePolicy(t *testing.T, st *store.Store, days int) {
	t.Helper()

	row := store.PolicyRow{Cluster: auditTestCluster, MaxSockets: 4, MaxCores: 8, MaxMemoryMB: 16384, MaxDiskPerVMGB: 500, MaxNetworkCards: 4, MaxSnapshots: 5, MaxVMPerUser: -1, LeaseDays: days}
	if err := st.UpsertPolicyRow(context.Background(), row); err != nil {
		t.Fatalf("UpsertPolicyRow: %v", err)
	}
}
