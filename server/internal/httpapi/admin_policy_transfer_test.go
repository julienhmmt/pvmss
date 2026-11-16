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
	"pvmss/server/internal/store"
	"strings"
	"testing"
)

const importPolicyBody = `{"clusters":["default","other"],"gabarit":{"maxSockets":2,"maxCores":4,"maxMemoryMB":8192,` +
	`"maxDiskPerVmGb":100,"maxNetworkCards":2,"maxSnapshots":3,"isolationVlanTag":0,"vmidMin":0,"vmidMax":0,"leaseDays":0},` +
	`"quota":{"maxVmPerUser":5}}`

func TestAdminPolicyImport_WritesEveryClusterAndAudits(t *testing.T) {
	t.Parallel()

	st := newAdminStore(t)
	handler, authHandler := policyHandlerWithStore(t, st)
	mux := policyMux(handler, authHandler)

	userRecorder := postJSON(t, mux, aliceCookie(t, authHandler), "/api/v1/admin/policy/import", importPolicyBody)
	if userRecorder.Code != http.StatusForbidden {
		t.Fatalf("non-admin import status = %d, want 403", userRecorder.Code)
	}

	cookie := adminCookie(t, authHandler)

	recorder := postJSON(t, mux, cookie, "/api/v1/admin/policy/import", importPolicyBody)
	if recorder.Code != http.StatusOK {
		t.Fatalf("import status = %d: %s", recorder.Code, recorder.Body.String())
	}

	if got, want := importOutcomes(t, recorder.Body.Bytes()), []string{"default:updated", "other:updated"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("outcomes = %v, want %v", got, want)
	}

	if got := countAuditRows(t, st, "admin.policy.import"); got != 1 {
		t.Fatalf("audit rows = %d, want 1", got)
	}

	for _, clusterName := range []string{auditTestCluster, "other"} {
		policyRecorder := getJSON(t, mux, cookie, "/api/v1/admin/policy?cluster="+clusterName)
		if policyRecorder.Code != http.StatusOK {
			t.Fatalf("get %s status = %d", clusterName, policyRecorder.Code)
		}

		var response struct {
			Gabarit struct {
				MaxCores int `json:"maxCores"`
			} `json:"gabarit"`
			Quota struct {
				MaxVMPerUser int `json:"maxVmPerUser"`
			} `json:"quota"`
		}
		if err := json.Unmarshal(policyRecorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode %s policy: %v", clusterName, err)
		}

		if response.Gabarit.MaxCores != 4 || response.Quota.MaxVMPerUser != 5 {
			t.Errorf("policy on %s = %+v, want maxCores 4 and maxVmPerUser 5", clusterName, response)
		}
	}

	again := postJSON(t, mux, cookie, "/api/v1/admin/policy/import", importPolicyBody)
	if got, want := importOutcomes(t, again.Body.Bytes()), []string{"default:unchanged", "other:unchanged"}; strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("second import = %v, want %v", got, want)
	}

	incomplete := postJSON(t, mux, cookie, "/api/v1/admin/policy/import",
		`{"clusters":["default"],"gabarit":{"maxCores":4},"quota":{"maxVmPerUser":5}}`)
	if incomplete.Code != http.StatusBadRequest {
		t.Fatalf("incomplete gabarit status = %d, want 400", incomplete.Code)
	}
}

func TestAdminPolicyNodesImport_ReportsPerNodeOutcomes(t *testing.T) {
	t.Parallel()

	st := newAdminStore(t)
	handler, authHandler := policyHandlerWithStore(t, st)
	mux := policyMux(handler, authHandler)
	cookie := adminCookie(t, authHandler)

	body := `{"cluster":"default","overwrite":false,"nodes":[` +
		`{"node":"` + cluster.FakeNode03 + `","maxVms":3,"maxVcpus":4,"maxRamGb":6,"maxDiskGb":2048},` +
		`{"node":"ghost-node","maxVms":1,"maxVcpus":1,"maxRamGb":1,"maxDiskGb":1}]}`

	recorder := postJSON(t, mux, cookie, "/api/v1/admin/policy/nodes/import", body)
	if recorder.Code != http.StatusOK {
		t.Fatalf("import status = %d: %s", recorder.Code, recorder.Body.String())
	}

	var outcomes []struct {
		Node   string `json:"node"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &outcomes); err != nil || len(outcomes) != 2 {
		t.Fatalf("decode outcomes = %v, body = %s", err, recorder.Body.String())
	}

	if outcomes[0].Node != cluster.FakeNode03 || outcomes[0].Result != "updated" {
		t.Errorf("first outcome = %+v, want an updated pve-node-03", outcomes[0])
	}

	if outcomes[1].Node != "ghost-node" || outcomes[1].Result != "not_found" {
		t.Errorf("second outcome = %+v, want ghost-node not_found", outcomes[1])
	}

	if !nodeIsCapped(t, mux, cookie, cluster.FakeNode03) {
		t.Error("imported caps not visible in the node list")
	}

	incomplete := postJSON(t, mux, cookie, "/api/v1/admin/policy/nodes/import",
		`{"cluster":"default","nodes":[{"node":"`+cluster.FakeNode03+`","maxVms":3}]}`)
	if incomplete.Code != http.StatusBadRequest {
		t.Fatalf("incomplete node row status = %d, want 400", incomplete.Code)
	}
}

// nodeIsCapped reads the node list and reports whether the node carries the
// maxVms the import wrote.
func nodeIsCapped(t *testing.T, mux *http.ServeMux, cookie *http.Cookie, node string) bool {
	t.Helper()

	recorder := getJSON(t, mux, cookie, "/api/v1/admin/policy/nodes?cluster=default")
	if recorder.Code != http.StatusOK {
		t.Fatalf("get nodes status = %d", recorder.Code)
	}

	var list struct {
		Nodes []struct {
			Node   string `json:"node"`
			MaxVMs int    `json:"maxVms"`
		} `json:"nodes"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode nodes: %v", err)
	}

	for _, item := range list.Nodes {
		if item.Node == node {
			return item.MaxVMs == 3
		}
	}

	return false
}

// policyHandlerWithStore builds the policy handler with the audit store wired
// so import actions land in audit_log.
func policyHandlerWithStore(t *testing.T, st *store.Store) (*httpapi.AdminPolicy, *httpapi.Auth) {
	t.Helper()

	authHandler := newAuthHandler(t)
	fake := cluster.Fake{}

	snapshot, err := fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	index := inventory.BuildIndex(snapshot)
	service := policyWithStaticProjection(st, inventory.NewProjectionFromIndex(&index), fake)
	handler := httpapi.NewAdminPolicyWithRegistry(authHandler, service,
		fakeClusterListerProvider{names: []string{auditTestCluster, "other"}}, slog.New(slog.DiscardHandler))
	handler.SetStore(st)

	return handler, authHandler
}

func postJSON(t *testing.T, mux *http.ServeMux, cookie *http.Cookie, path, body string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(context.Background(), http.MethodPost, path, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.AddCookie(cookie)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	return recorder
}

func getJSON(t *testing.T, mux *http.ServeMux, cookie *http.Cookie, path string) *httptest.ResponseRecorder {
	t.Helper()

	request := httptest.NewRequestWithContext(context.Background(), http.MethodGet, path, nil)
	request.AddCookie(cookie)

	recorder := httptest.NewRecorder()
	mux.ServeHTTP(recorder, request)

	return recorder
}

func importOutcomes(t *testing.T, body []byte) []string {
	t.Helper()

	var outcomes []struct {
		Cluster string `json:"cluster"`
		Result  string `json:"result"`
	}
	if err := json.Unmarshal(body, &outcomes); err != nil {
		t.Fatalf("decode outcomes: %v", err)
	}

	results := make([]string, len(outcomes))
	for i, outcome := range outcomes {
		results[i] = outcome.Cluster + ":" + outcome.Result
	}

	return results
}

func countAuditRows(t *testing.T, st *store.Store, action string) int {
	t.Helper()

	var count int
	if err := st.DB().QueryRowContext(context.Background(),
		`SELECT COUNT(*) FROM audit_log WHERE action = ?`, action).Scan(&count); err != nil {
		t.Fatalf("count audit rows: %v", err)
	}

	return count
}
