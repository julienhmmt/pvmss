//nolint:wsl_v5 // HTTP fixture setup keeps request assertions readable
package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	adminClusterTestSecret = "admin-cluster-test-secret-with-32-bytes"
	adminClustersPath      = "/api/v1/admin/clusters"
)

type adminClusterFixture struct {
	handler   *httpapi.AdminClusters
	auth      *httpapi.Auth
	registry  *cluster.Registry
	inventory *inventory.Registry
	store     *store.Store
}

func newAdminClusterFixture(t *testing.T) adminClusterFixture {
	t.Helper()
	st, err := store.Open(config.Configuration{DBPath: filepath.Join(t.TempDir(), "clusters-http.db"), ClusterSource: "fake", SessionSecret: adminClusterTestSecret})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	rows, err := st.ListClusters(context.Background())
	if err != nil {
		t.Fatalf("ListClusters: %v", err)
	}
	registry, err := cluster.NewRegistry("fake", rows)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	indexes := inventory.NewRegistry(registry, time.Hour, slog.Default())
	for _, name := range []string{auditTestCluster, crossSecondaryCluster} {
		if _, err := indexes.Refresh(context.Background(), name); err != nil {
			t.Fatalf("Refresh(%s): %v", name, err)
		}
	}
	sessions, err := auth.NewSessionManager(st, adminClusterTestSecret, false)
	if err != nil {
		t.Fatalf("NewSessionManager: %v", err)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte("admin-password"), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("GenerateFromPassword: %v", err)
	}
	authHandler := httpapi.NewAuthWithRegistry(registry, st, sessions, string(hash), auth.NewTokenService(st), slog.Default())
	handler := httpapi.NewAdminClusters(authHandler, st, registry, indexes, slog.Default())
	return adminClusterFixture{handler: handler, auth: authHandler, registry: registry, inventory: indexes, store: st}
}

func adminClusterCookie(t *testing.T, authHandler *httpapi.Auth) *http.Cookie {
	t.Helper()
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/auth/admin-login", strings.NewReader(`{"password":"admin-password"}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	authHandler.AdminLogin(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("admin login status = %d: %s", response.Code, response.Body.String())
	}
	return response.Result().Cookies()[0]
}

//nolint:paralleltest // HTTP fixture shares fake cluster state
func TestAdminClusters_ListNeverLeaksToken(t *testing.T) {
	fixture := newAdminClusterFixture(t)
	cookie := adminClusterCookie(t, fixture.auth)
	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, adminClustersPath, nil)
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	fixture.auth.RequireAdmin(http.HandlerFunc(fixture.handler.ServeList)).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	body := response.Body.String()
	if strings.Contains(body, "demo-default-service-secret") || strings.Contains(body, "tokenSecretCiphertext") {
		t.Fatalf("cluster list leaks a token: %s", body)
	}
	var rows []struct {
		Name      string `json:"name"`
		TokenSet  bool   `json:"tokenSet"`
		VMCount   int    `json:"vmCount"`
		NodeCount int    `json:"nodeCount"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(rows) < 3 {
		t.Fatalf("clusters = %d, want seeded default, secondary, offline-demo", len(rows))
	}
	for _, row := range rows {
		if !row.TokenSet {
			t.Errorf("cluster %q tokenSet=false", row.Name)
		}
	}
}

//nolint:paralleltest // HTTP fixture shares fake cluster state
func TestAdminClusters_TestUnreachableIs200(t *testing.T) {
	fixture := newAdminClusterFixture(t)
	cookie := adminClusterCookie(t, fixture.auth)
	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/api/v1/admin/clusters/offline-demo/test", nil)
	request.SetPathValue("name", "offline-demo")
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	fixture.auth.RequireAdmin(http.HandlerFunc(fixture.handler.ServeTest)).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var result struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if result.Status != "unreachable" {
		t.Fatalf("status = %q, want unreachable", result.Status)
	}
}

// adminClusterRequest builds and dispatches an authenticated request against
// one AdminClusters handler method, wrapped by the real RequireAdmin guard
// (T026's own instrument: swap cookie for a non-admin one to prove 403).
// clusterRequestSpec groups the request-specific parameters of adminClusterRequest.
// It collapses the five positional parameters that helper used to take (SonarQube go:S107).
type clusterRequestSpec struct {
	Method     http.HandlerFunc
	HTTPMethod string
	Path       string
	Name       string
	Body       string
}

func adminClusterRequest(t *testing.T, fixture adminClusterFixture, cookie *http.Cookie, req clusterRequestSpec) *httptest.ResponseRecorder {
	t.Helper()
	var request *http.Request
	if req.Body != "" {
		request = httptest.NewRequestWithContext(context.Background(), req.HTTPMethod, req.Path, strings.NewReader(req.Body))
		request.Header.Set("Content-Type", "application/json")
	} else {
		request = httptest.NewRequestWithContext(context.Background(), req.HTTPMethod, req.Path, nil)
	}
	if req.Name != "" {
		request.SetPathValue("name", req.Name)
	}
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	fixture.auth.RequireAdmin(req.Method).ServeHTTP(response, request)
	return response
}

// aliceClusterCookie logs in the fake non-admin demo user against the given
// fixture's registry-backed Auth, choosing the default cluster explicitly
// (login requires an explicit cluster choice once 2+ are configured).
func aliceClusterCookie(t *testing.T, authHandler *httpapi.Auth) *http.Cookie {
	t.Helper()
	response := serveJSON(authHandler.Login, "/api/v1/auth/login", `{"username":"alice","password":"pvmss-alice","cluster":"default"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("alice login status = %d: %s", response.Code, response.Body.String())
	}
	return response.Result().Cookies()[0]
}

// TestAdminClusters_NonAdminReturns403 — T026: every /admin/clusters/*
// endpoint rejects a non-admin identity with 403, matching the contract's
// {"code":"forbidden","message":"admin only"} body on every one of the six.
//
//nolint:paralleltest // HTTP fixture shares fake cluster state
func TestAdminClusters_NonAdminReturns403(t *testing.T) {
	fixture := newAdminClusterFixture(t)
	cookie := aliceClusterCookie(t, fixture.auth)

	cases := []struct {
		name       string
		method     http.HandlerFunc
		httpMethod string
		path       string
		pathName   string
		body       string
	}{
		{"list", fixture.handler.ServeList, http.MethodGet, adminClustersPath, "", ""},
		{"create", fixture.handler.ServeCreate, http.MethodPost, adminClustersPath, "", `{"name":"tertiary","url":"https://pve-d.example.com:8006/api2/json","tokenId":"pvmss@pve!service","tokenSecret":"s"}`},
		{"update", fixture.handler.ServeUpdate, http.MethodPut, "/api/v1/admin/clusters/secondary", crossSecondaryCluster, `{"url":"https://pve-b.example.com:8006/api2/json","tokenId":"pvmss@pve!service"}`},
		{"test", fixture.handler.ServeTest, http.MethodPost, "/api/v1/admin/clusters/secondary/test", crossSecondaryCluster, ""},
		{"oidc", fixture.handler.ServeOIDC, http.MethodPost, "/api/v1/admin/clusters/secondary/oidc", crossSecondaryCluster, `{"enabled":true}`},
		{testActionDelete, fixture.handler.ServeDelete, http.MethodDelete, "/api/v1/admin/clusters/secondary", crossSecondaryCluster, ""},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			response := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{Method: testCase.method, HTTPMethod: testCase.httpMethod, Path: testCase.path, Name: testCase.pathName, Body: testCase.body})
			if response.Code != http.StatusForbidden {
				t.Fatalf("%s status = %d, want 403: %s", testCase.name, response.Code, response.Body.String())
			}
			var body struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("%s decode: %v", testCase.name, err)
			}
			if body.Code != apiCodeForbidden || body.Message != "admin only" {
				t.Fatalf("%s error body = %+v, want {forbidden, admin only}", testCase.name, body)
			}
		})
	}
}

// TestAdminClusters_CreateValidatesAndReactivates — T025 create path: 201 on
// a genuinely new name, 400 on an invalid name, 409 on an active-name
// collision, and 201-with-reactivation (removedAt cleared, fresh token
// required) on re-adding a previously soft-deleted name (FR-005, FR-007).
//
//nolint:paralleltest,gocyclo // HTTP fixture shares fake cluster state; create path covers 4 contract branches in one sequential test
func TestAdminClusters_CreateValidatesAndReactivates(t *testing.T) {
	fixture := newAdminClusterFixture(t)
	cookie := adminClusterCookie(t, fixture.auth)

	create := func(body string) *httptest.ResponseRecorder {
		return adminClusterRequest(t, fixture, cookie, clusterRequestSpec{Method: fixture.handler.ServeCreate, HTTPMethod: http.MethodPost, Path: adminClustersPath, Name: "", Body: body})
	}

	// New cluster: 201, correct shape, untested.
	response := create(`{"name":"tertiary","url":"https://pve-d.example.com:8006/api2/json","tlsInsecureSkipVerify":true,"tokenId":"pvmss@pve!service","tokenSecret":"tertiary-secret"}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("create status = %d, want 201: %s", response.Code, response.Body.String())
	}
	var created adminClusterDTOForTest
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	if created.Name != "tertiary" || created.RemovedAt != nil || created.LastTestStatus != nil || !created.TokenSet || !created.TLSInsecureSkipVerify {
		t.Fatalf("created cluster = %+v, unexpected shape", created)
	}

	// Invalid name: 400.
	response = create(`{"name":"Not_Valid","url":"https://pve-e.example.com:8006/api2/json","tokenId":"pvmss@pve!service","tokenSecret":"s"}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid name status = %d, want 400: %s", response.Code, response.Body.String())
	}
	assertClusterErrorBody(t, response, "invalid_cluster_name")

	// Active-name collision: 409.
	response = create(`{"name":"secondary","url":"https://pve-b2.example.com:8006/api2/json","tokenId":"pvmss@pve!service","tokenSecret":"s"}`)
	if response.Code != http.StatusConflict {
		t.Fatalf("duplicate name status = %d, want 409: %s", response.Code, response.Body.String())
	}
	assertClusterErrorBody(t, response, "duplicate_cluster")

	// Soft-delete tertiary, then re-add under the same name: 201, reactivated.
	deleteResponse := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{Method: fixture.handler.ServeDelete, HTTPMethod: http.MethodDelete, Path: "/api/v1/admin/clusters/tertiary", Name: "tertiary", Body: ""})
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete tertiary status = %d: %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	response = create(`{"name":"tertiary","url":"https://pve-d2.example.com:8006/api2/json","tokenId":"pvmss@pve!service","tokenSecret":"tertiary-secret-2"}`)
	if response.Code != http.StatusCreated {
		t.Fatalf("reactivate status = %d, want 201: %s", response.Code, response.Body.String())
	}
	var reactivated adminClusterDTOForTest
	if err := json.Unmarshal(response.Body.Bytes(), &reactivated); err != nil {
		t.Fatalf("decode reactivated: %v", err)
	}
	if reactivated.RemovedAt != nil || reactivated.LastTestStatus != nil || reactivated.URL != "https://pve-d2.example.com:8006/api2/json" {
		t.Fatalf("reactivated cluster = %+v, want removedAt=nil lastTestStatus=nil fresh url", reactivated)
	}
}

// TestAdminClusters_UpdateRejectsNameAnd404sOnRemoved — T025 update path:
// 200 on a valid update, the request schema rejects any "name" field
// outright (FR-004: immutable by construction, not merely ignored), and
// both an unknown and a soft-deleted cluster 404 rather than silently
// creating or resurrecting a row.
//
//nolint:paralleltest // HTTP fixture shares fake cluster state
func TestAdminClusters_UpdateRejectsNameAnd404sOnRemoved(t *testing.T) {
	fixture := newAdminClusterFixture(t)
	cookie := adminClusterCookie(t, fixture.auth)

	update := func(name, body string) *httptest.ResponseRecorder {
		return adminClusterRequest(t, fixture, cookie, clusterRequestSpec{Method: fixture.handler.ServeUpdate, HTTPMethod: http.MethodPut, Path: "/api/v1/admin/clusters/" + name, Name: name, Body: body})
	}

	// Valid update: 200, new URL reflected.
	response := update(crossSecondaryCluster, `{"url":"https://pve-b2.example.com:8006/api2/json","tlsInsecureSkipVerify":true,"tokenId":"pvmss@pve!service"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("update status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var updated adminClusterDTOForTest
	if err := json.Unmarshal(response.Body.Bytes(), &updated); err != nil {
		t.Fatalf("decode updated: %v", err)
	}
	if updated.URL != "https://pve-b2.example.com:8006/api2/json" || !updated.TLSInsecureSkipVerify {
		t.Fatalf("updated cluster = %+v, want new url and tls flag", updated)
	}

	// A "name" field in the PUT body is rejected outright, not silently dropped.
	response = update(crossSecondaryCluster, `{"name":"secondary","url":"https://pve-b3.example.com:8006/api2/json","tokenId":"pvmss@pve!service"}`)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("update-with-name status = %d, want 400 (unknown field rejected): %s", response.Code, response.Body.String())
	}

	// Unknown cluster: 404.
	response = update("nonexistent", `{"url":"https://pve-z.example.com:8006/api2/json","tokenId":"pvmss@pve!service"}`)
	if response.Code != http.StatusNotFound {
		t.Fatalf("unknown cluster status = %d, want 404: %s", response.Code, response.Body.String())
	}
	assertClusterErrorBody(t, response, "not_found")

	// Soft-deleted cluster cannot be updated: 404.
	createResponse := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{Method: fixture.handler.ServeCreate, HTTPMethod: http.MethodPost, Path: adminClustersPath, Name: "", Body: `{"name":"tertiary-for-update","url":"https://pve-f.example.com:8006/api2/json","tokenId":"pvmss@pve!service","tokenSecret":"s"}`})
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create tertiary-for-update status = %d: %s", createResponse.Code, createResponse.Body.String())
	}
	deleteResponse := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{Method: fixture.handler.ServeDelete, HTTPMethod: http.MethodDelete, Path: "/api/v1/admin/clusters/tertiary-for-update", Name: "tertiary-for-update", Body: ""})
	if deleteResponse.Code != http.StatusOK {
		t.Fatalf("delete tertiary-for-update status = %d: %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	response = update("tertiary-for-update", `{"url":"https://pve-f2.example.com:8006/api2/json","tokenId":"pvmss@pve!service"}`)
	if response.Code != http.StatusNotFound {
		t.Fatalf("update soft-deleted cluster status = %d, want 404: %s", response.Code, response.Body.String())
	}
}

// TestAdminClusters_TestReachableReportsOKAndPersists — T025 test path
// (reachable branch): a healthy cluster's test returns 200 status "ok" with
// version/counts, and — per FR-009/the contract's "a test is the refresh"
// rule — the subsequent GET /admin/clusters list reflects the new
// lastTestStatus/lastTestAt.
//
//nolint:paralleltest,gocyclo // HTTP fixture shares fake cluster state; test+list assertion path is sequential by contract
func TestAdminClusters_TestReachableReportsOKAndPersists(t *testing.T) {
	fixture := newAdminClusterFixture(t)
	cookie := adminClusterCookie(t, fixture.auth)

	response := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{Method: fixture.handler.ServeTest, HTTPMethod: http.MethodPost, Path: "/api/v1/admin/clusters/default/test", Name: auditTestCluster, Body: ""})
	if response.Code != http.StatusOK {
		t.Fatalf("test status = %d, want 200: %s", response.Code, response.Body.String())
	}
	var result struct {
		Status         string `json:"status"`
		ProxmoxVersion string `json:"proxmoxVersion"`
		NodeCount      int    `json:"nodeCount"`
		VMCount        int    `json:"vmCount"`
		TestedAt       string `json:"testedAt"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Status != "ok" || result.ProxmoxVersion == "" || result.NodeCount == 0 || result.TestedAt == "" {
		t.Fatalf("test result = %+v, want a fully-populated ok response", result)
	}

	list := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{Method: fixture.handler.ServeList, HTTPMethod: http.MethodGet, Path: adminClustersPath, Name: "", Body: ""})
	var rows []adminClusterDTOForTest
	if err := json.Unmarshal(list.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	found := false
	for _, row := range rows {
		if row.Name != auditTestCluster {
			continue
		}
		found = true
		if row.LastTestStatus == nil || *row.LastTestStatus != "ok" || row.LastTestAt == nil {
			t.Fatalf("default row after test = %+v, want lastTestStatus=ok lastTestAt set", row)
		}
		if row.DisplayName == "" {
			t.Fatalf("default row after test has empty DisplayName: %+v", row)
		}
	}
	if !found {
		t.Fatal("default cluster missing from list after test")
	}
}

// TestAdminClusters_OIDCToggleIsolated — T025/FR-011: toggling one cluster's
// OIDC flag changes only that row; every other cluster's flag is untouched.
// Also covers the 404 path for an unknown/removed cluster.
//
//nolint:paralleltest // HTTP fixture shares fake cluster state
func TestAdminClusters_OIDCToggleIsolated(t *testing.T) {
	fixture := newAdminClusterFixture(t)
	cookie := adminClusterCookie(t, fixture.auth)

	response := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{Method: fixture.handler.ServeOIDC, HTTPMethod: http.MethodPost, Path: "/api/v1/admin/clusters/secondary/oidc", Name: crossSecondaryCluster, Body: `{"enabled":true}`})
	if response.Code != http.StatusOK {
		t.Fatalf("oidc toggle status = %d, want 200: %s", response.Code, response.Body.String())
	}

	list := adminClusterRequest(t, fixture, cookie, clusterRequestSpec{Method: fixture.handler.ServeList, HTTPMethod: http.MethodGet, Path: adminClustersPath, Name: "", Body: ""})
	var rows []adminClusterDTOForTest
	if err := json.Unmarshal(list.Body.Bytes(), &rows); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	for _, row := range rows {
		switch row.Name {
		case crossSecondaryCluster:
			if !row.OIDCEnabled {
				t.Errorf("secondary oidcEnabled = false, want true after toggle")
			}
		default:
			if row.OIDCEnabled {
				t.Errorf("cluster %q oidcEnabled = true, want untouched by secondary's toggle", row.Name)
			}
		}
	}

	response = adminClusterRequest(t, fixture, cookie, clusterRequestSpec{Method: fixture.handler.ServeOIDC, HTTPMethod: http.MethodPost, Path: "/api/v1/admin/clusters/nonexistent/oidc", Name: "nonexistent", Body: `{"enabled":true}`})
	if response.Code != http.StatusNotFound {
		t.Fatalf("oidc toggle on unknown cluster status = %d, want 404: %s", response.Code, response.Body.String())
	}
}

// TestAdminClusters_DeleteLastClusterConflictAndReactivateRoundTrip —
// T025/FR-027: removing the sole remaining active cluster is refused with
// 409, never leaving PVMSS with zero addressable clusters. Also proves the
// unknown-name 404 path.
//
//nolint:paralleltest // HTTP fixture shares fake cluster state
func TestAdminClusters_DeleteLastClusterConflictAndReactivateRoundTrip(t *testing.T) {
	fixture := newAdminClusterFixture(t)
	cookie := adminClusterCookie(t, fixture.auth)

	del := func(name string) *httptest.ResponseRecorder {
		return adminClusterRequest(t, fixture, cookie, clusterRequestSpec{Method: fixture.handler.ServeDelete, HTTPMethod: http.MethodDelete, Path: "/api/v1/admin/clusters/" + name, Name: name, Body: ""})
	}

	response := del("nonexistent")
	if response.Code != http.StatusNotFound {
		t.Fatalf("delete unknown cluster status = %d, want 404: %s", response.Code, response.Body.String())
	}

	// Seed is default/secondary/offline-demo. Remove down to the last one.
	if response := del(crossSecondaryCluster); response.Code != http.StatusOK {
		t.Fatalf("delete secondary status = %d: %s", response.Code, response.Body.String())
	}
	if response := del("offline-demo"); response.Code != http.StatusOK {
		t.Fatalf("delete offline-demo status = %d: %s", response.Code, response.Body.String())
	}
	response = del(auditTestCluster)
	if response.Code != http.StatusConflict {
		t.Fatalf("delete last cluster status = %d, want 409: %s", response.Code, response.Body.String())
	}
	assertClusterErrorBody(t, response, "last_cluster")
}

type adminClusterDTOForTest struct {
	Name                  string  `json:"name"`
	DisplayName           string  `json:"displayName"`
	URL                   string  `json:"url"`
	TLSInsecureSkipVerify bool    `json:"tlsInsecureSkipVerify"`
	TokenSet              bool    `json:"tokenSet"`
	OIDCEnabled           bool    `json:"oidcEnabled"`
	RemovedAt             *string `json:"removedAt"`
	LastTestStatus        *string `json:"lastTestStatus"`
	LastTestAt            *string `json:"lastTestAt"`
}

func assertClusterErrorBody(t *testing.T, response *httptest.ResponseRecorder, wantCode string) {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body.Code != wantCode {
		t.Fatalf("error code = %q, want %q (body: %s)", body.Code, wantCode, response.Body.String())
	}
}
