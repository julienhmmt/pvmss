//nolint:noctx,paralleltest,wsl_v5 // HTTP tests use shared fake and session fixtures
package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"strings"
	"testing"
	"time"
)

type adminPoolSummary struct {
	Name    string `json:"name"`
	Comment string `json:"comment"`
	Total   int    `json:"total"`
	Running int    `json:"running"`
	Stopped int    `json:"stopped"`
	Managed bool   `json:"managed"`
}

func TestAdminPools_CreateAndListAsAdmin(t *testing.T) {
	handler, authHandler := newAdminPoolsHandler(t)
	cookie := adminCookie(t, authHandler)

	create := adminPoolsRequest(t, handler.ServeCreate, http.MethodPost, "/api/v1/admin/pools", cookie, `{"name":"newteam","comment":"","password":"S0meLongPW!"}`)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", create.Code, create.Body.String())
	}

	var created adminPoolSummary
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.Name != "newteam" || created.Total != 0 || created.Running != 0 || created.Stopped != 0 || !created.Managed {
		t.Fatalf("created = %+v", created)
	}

	list := adminPoolsRequest(t, handler.ServeList, http.MethodGet, "/api/v1/admin/pools?cluster=default&search=NEW", cookie, "")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", list.Code, list.Body.String())
	}
	var rows []adminPoolSummary
	if err := json.Unmarshal(list.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "newteam" {
		t.Fatalf("rows = %+v", rows)
	}
}

func TestAdminPools_ListCountsForEveryPool(t *testing.T) {
	handler, authHandler := newAdminPoolsHandler(t)
	recorder := adminPoolsRequest(t, handler.ServeList, http.MethodGet, "/api/v1/admin/pools?cluster=default", adminCookie(t, authHandler), "")
	if recorder.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", recorder.Code, recorder.Body.String())
	}
	var rows []adminPoolSummary
	if err := json.Unmarshal(recorder.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	for _, row := range rows {
		if row.Name == cluster.FakePoolAlice {
			if row.Total != 7 || row.Running != 3 || row.Stopped != 4 {
				t.Fatalf("alice summary = %+v", row)
			}
			return
		}
	}
	t.Fatalf("%q was not listed: %+v", cluster.FakePoolAlice, rows)
}

func TestAdminPools_DeleteZeroVmPool(t *testing.T) {
	handler, authHandler := newAdminPoolsHandler(t)
	cookie := adminCookie(t, authHandler)
	create := adminPoolsRequest(t, handler.ServeCreate, http.MethodPost, "/api/v1/admin/pools", cookie, `{"name":"newteam","password":"S0meLongPW!"}`)
	if create.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", create.Code, create.Body.String())
	}
	deleted := adminPoolsRequest(t, handler.ServeDelete, http.MethodDelete, "/api/v1/admin/pools/newteam", cookie, "")
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete status = %d: %s", deleted.Code, deleted.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(deleted.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode delete: %v", err)
	}
	status, hasStatus := result["status"].(string)
	userDeleted, hasUserDeleted := result["userDeleted"].(bool)
	if !hasStatus || !hasUserDeleted || status != testStatusDeleted || !userDeleted {
		t.Fatalf("result = %+v (want exact lowercase keys status/userDeleted)", result)
	}
	if _, wrongCase := result["UserDeleted"]; wrongCase {
		t.Fatalf("result carries uppercase UserDeleted key: %+v", result)
	}
	unknown := adminPoolsRequest(t, handler.ServeDelete, http.MethodDelete, "/api/v1/admin/pools/newteam", cookie, "")
	if unknown.Code != http.StatusNotFound {
		t.Fatalf("unknown status = %d, want 404", unknown.Code)
	}
}

func TestAdminPools_RejectsNonAdminAndInvalidRequests(t *testing.T) {
	handler, authHandler := newAdminPoolsHandler(t)
	alice := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
		want   int
	}{
		{name: "list", method: http.MethodGet, path: "/api/v1/admin/pools", want: http.StatusForbidden},
		{name: "create", method: http.MethodPost, path: "/api/v1/admin/pools", body: `{"name":"carol","password":"S0meLongPW!"}`, want: http.StatusForbidden},
		{name: testActionDelete, method: http.MethodDelete, path: "/api/v1/admin/pools/carol", want: http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rec := adminPoolsRequest(t, handlerForMethod(handler, tc.method), tc.method, tc.path, alice, tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}

	admin := adminCookie(t, authHandler)
	invalid := adminPoolsRequest(t, handler.ServeCreate, http.MethodPost, "/api/v1/admin/pools", admin, `{"name":"BAD_NAME","password":"S0meLongPW!"}`)
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, want 400", invalid.Code)
	}
	duplicate := adminPoolsRequest(t, handler.ServeCreate, http.MethodPost, "/api/v1/admin/pools", admin, `{"name":"pool-alice","password":"S0meLongPW!"}`)
	if duplicate.Code != http.StatusConflict {
		t.Fatalf("duplicate status = %d, want 409", duplicate.Code)
	}
}

// TestAdminPools_DeleteUnmanagedProxmoxPoolIsRejected verifies the API refuses
// to cascade-delete a Proxmox pool that PVMSS did not provision.
//
//nolint:paralleltest // serial: shared fake fixtures
func TestAdminPools_DeleteUnmanagedProxmoxPoolIsRejected(t *testing.T) {
	handler, authHandler := newAdminPoolsHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPoolsRequest(t, handler.ServeDelete, http.MethodDelete, "/api/v1/admin/pools/"+cluster.FakePoolAlice, cookie, "")
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if code, _ := body["code"].(string); code != "not_managed" {
		t.Fatalf("code = %q, want not_managed", code)
	}
	remaining, err := cluster.Fake{}.ListPools(context.Background())
	if err != nil {
		t.Fatalf("ListPools: %v", err)
	}
	for _, pool := range remaining {
		if pool.Name == cluster.FakePoolAlice {
			return
		}
	}
	t.Fatalf("unmanaged pool was deleted: %+v", remaining)
}

// TestAdminPools_ListExposesManagedFlag verifies the list payload carries the
// managed flag and that a freshly created pool is reported as managed.
//
//nolint:paralleltest // serial: shared fake fixtures
func TestAdminPools_ListExposesManagedFlag(t *testing.T) {
	handler, authHandler := newAdminPoolsHandler(t)
	cookie := adminCookie(t, authHandler)

	if rec := adminPoolsRequest(t, handler.ServeCreate, http.MethodPost, "/api/v1/admin/pools", cookie, `{"name":"team-z","password":"S0meLongPW!"}`); rec.Code != http.StatusCreated {
		t.Fatalf("create status = %d: %s", rec.Code, rec.Body.String())
	}

	list := adminPoolsRequest(t, handler.ServeList, http.MethodGet, "/api/v1/admin/pools?cluster=default&search=team", cookie, "")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", list.Code, list.Body.String())
	}
	var rows []adminPoolSummary
	if err := json.Unmarshal(list.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(rows) != 1 || rows[0].Name != "team-z" || !rows[0].Managed {
		t.Fatalf("rows = %+v, want one managed team-z", rows)
	}

	all := adminPoolsRequest(t, handler.ServeList, http.MethodGet, "/api/v1/admin/pools?cluster=default", cookie, "")
	if all.Code != http.StatusOK {
		t.Fatalf("list all status = %d: %s", all.Code, all.Body.String())
	}
	var allRows []adminPoolSummary
	if err := json.Unmarshal(all.Body.Bytes(), &allRows); err != nil {
		t.Fatalf("decode all: %v", err)
	}
	for _, row := range allRows {
		if row.Name == cluster.FakePoolAlice && row.Managed {
			t.Fatalf("alice should not be managed: %+v", row)
		}
		if row.Name == "team-z" && !row.Managed {
			t.Fatalf("team-z should be managed: %+v", row)
		}
	}
}

func newAdminPoolsHandler(t *testing.T) (*httpapi.AdminPools, *httpapi.Auth) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)
	authHandler := newAuthHandler(t)
	client := cluster.Fake{}
	snapshot, err := client.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	index := inventory.BuildIndex(snapshot)
	projection := inventory.NewProjectionFromIndex(&index)
	worker := inventory.NewWorker(client, projection, time.Hour, slog.Default())
	st := newAdminStore(t)
	handler := httpapi.NewAdminPools(authHandler, client, projection, client, st, worker, st, slog.Default())
	return handler, authHandler
}

func adminPoolsRequest(t *testing.T, handler http.HandlerFunc, method, path string, cookie *http.Cookie, body string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		request.Header.Set("Content-Type", "application/json")
	}
	if method == http.MethodDelete {
		parts := strings.Split(strings.Trim(path, "/"), "/")
		request.SetPathValue("name", parts[len(parts)-1])
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	recorder := httptest.NewRecorder()
	handler(recorder, request)
	return recorder
}

func handlerForMethod(handler *httpapi.AdminPools, method string) http.HandlerFunc {
	switch method {
	case http.MethodGet:
		return handler.ServeList
	case http.MethodPost:
		return handler.ServeCreate
	default:
		return handler.ServeDelete
	}
}
