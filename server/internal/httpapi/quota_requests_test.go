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
	"pvmss/server/internal/policy"
	"pvmss/server/internal/store"
	"strconv"
	"strings"
	"testing"
)

type quotaFixture struct {
	handler *httpapi.QuotaRequests
	service *policy.Policy
	auth    *httpapi.Auth
	store   *store.Store
	mux     *http.ServeMux
}

// newQuotaFixture wires the quota handlers over the fake cluster and caps
// every pool at 2 VMs, so the default unlimited allowance does not block the
// request flow under test.
func newQuotaFixture(t *testing.T) quotaFixture {
	t.Helper()

	st := newAdminStore(t)
	authHandler := newAuthHandler(t)
	fake := cluster.Fake{}

	snapshot, err := fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	index := inventory.BuildIndex(snapshot)
	service := policyWithStaticProjection(st, inventory.NewProjectionFromIndex(&index), fake)

	if err := service.SetPolicy(context.Background(), "default", policy.Gabarit{}, 2); err != nil {
		t.Fatalf("SetPolicy: %v", err)
	}

	handler := httpapi.NewQuotaRequests(authHandler, service, st, slog.New(slog.DiscardHandler))
	handler.SetTrustedProxyHops(0)

	mux := http.NewServeMux()
	guard := authHandler.RequireAdmin

	mux.Handle("POST /api/v1/quota-requests", http.HandlerFunc(handler.ServeCreate))
	mux.Handle("GET /api/v1/quota-requests/mine", http.HandlerFunc(handler.ServeMine))
	mux.Handle("GET /api/v1/admin/quota-requests", guard(http.HandlerFunc(handler.ServeAdminList)))
	mux.Handle("POST /api/v1/admin/quota-requests/{id}/approve", guard(http.HandlerFunc(handler.ServeApprove)))
	mux.Handle("POST /api/v1/admin/quota-requests/{id}/deny", guard(http.HandlerFunc(handler.ServeDeny)))
	mux.Handle("GET /api/v1/admin/quota-overrides", guard(http.HandlerFunc(handler.ServeOverrideList)))
	mux.Handle("PUT /api/v1/admin/quota-overrides/{cluster}/{pool}", guard(http.HandlerFunc(handler.ServeOverrideUpsert)))
	mux.Handle("DELETE /api/v1/admin/quota-overrides/{cluster}/{pool}", guard(http.HandlerFunc(handler.ServeOverrideDelete)))

	return quotaFixture{handler: handler, service: service, auth: authHandler, store: st, mux: mux}
}

func (fixture quotaFixture) do(t *testing.T, method, path, body string, cookie *http.Cookie) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()

	request := httptest.NewRequestWithContext(context.Background(), method, path, strings.NewReader(body))
	if cookie != nil {
		request.AddCookie(cookie)
	}

	recorder := httptest.NewRecorder()
	fixture.mux.ServeHTTP(recorder, request)

	var decoded map[string]any

	_ = json.Unmarshal(recorder.Body.Bytes(), &decoded)

	return recorder, decoded
}

func TestQuotaRequest_CreateThenConflictOnSecondPending(t *testing.T) {
	t.Parallel()

	fixture := newQuotaFixture(t)
	user := aliceCookie(t, fixture.auth)

	recorder, body := fixture.do(t, http.MethodPost, "/api/v1/quota-requests",
		`{"requestedQuota":10,"reason":"  more for the lab  "}`, user)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create status = %d body %s", recorder.Code, recorder.Body.String())
	}

	if body["status"] != "pending" || body["currentQuota"] != float64(2) || body["reason"] != "more for the lab" {
		t.Fatalf("created body = %v", body)
	}

	id := body["id"]

	recorder, body = fixture.do(t, http.MethodPost, "/api/v1/quota-requests",
		`{"requestedQuota":20,"reason":"again"}`, user)
	if recorder.Code != http.StatusConflict || body["id"] != id {
		t.Fatalf("second create status = %d body %v, want 409 with id %v", recorder.Code, body, id)
	}

	recorder, body = fixture.do(t, http.MethodGet, "/api/v1/quota-requests/mine", "", user)
	if recorder.Code != http.StatusOK || body["request"] == nil {
		t.Fatalf("mine = %d %v", recorder.Code, body)
	}
}

func TestQuotaRequest_ValidationRules(t *testing.T) {
	t.Parallel()

	fixture := newQuotaFixture(t)
	user := aliceCookie(t, fixture.auth)
	admin := adminCookie(t, fixture.auth)

	for _, testCase := range []struct {
		name   string
		cookie *http.Cookie
		body   string
		status int
	}{
		{"admin has no pool", admin, `{"requestedQuota":10,"reason":"x"}`, http.StatusBadRequest},
		{"blank reason", user, `{"requestedQuota":10,"reason":"   "}`, http.StatusBadRequest},
		{"missing quota", user, `{"reason":"x"}`, http.StatusBadRequest},
		{"not above current", user, `{"requestedQuota":2,"reason":"x"}`, http.StatusBadRequest},
		{"above maximum", user, `{"requestedQuota":100001,"reason":"x"}`, http.StatusBadRequest},
		{"bad json", user, `{`, http.StatusBadRequest},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			recorder, _ := fixture.do(t, http.MethodPost, "/api/v1/quota-requests", testCase.body, testCase.cookie)
			if recorder.Code != testCase.status {
				t.Fatalf("status = %d body %s, want %d", recorder.Code, recorder.Body.String(), testCase.status)
			}
		})
	}
}

func TestQuotaRequest_UnlimitedCapCannotBeRaised(t *testing.T) {
	t.Parallel()

	fixture := newQuotaFixture(t)
	user := aliceCookie(t, fixture.auth)

	if err := fixture.service.SetPolicy(context.Background(), "default", policy.Gabarit{}, -1); err != nil {
		t.Fatalf("SetPolicy unlimited: %v", err)
	}

	recorder, _ := fixture.do(t, http.MethodPost, "/api/v1/quota-requests", `{"requestedQuota":10,"reason":"x"}`, user)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("unlimited user request status = %d, want 400", recorder.Code)
	}
}

func TestQuotaRequest_AdminListApproveWithOverrideAndAudit(t *testing.T) {
	t.Parallel()

	fixture := newQuotaFixture(t)
	user := aliceCookie(t, fixture.auth)
	admin := adminCookie(t, fixture.auth)

	_, created := fixture.do(t, http.MethodPost, "/api/v1/quota-requests", `{"requestedQuota":10,"reason":"lab"}`, user)
	id := requestID(t, created)

	recorder, list := fixture.do(t, http.MethodGet, "/api/v1/admin/quota-requests?status=pending", "", admin)
	if recorder.Code != http.StatusOK || requestCount(t, list) != 1 {
		t.Fatalf("admin list = %d %v", recorder.Code, list)
	}

	recorder, approved := fixture.do(t, http.MethodPost,
		"/api/v1/admin/quota-requests/"+formatID(id)+"/approve", `{"quota":6}`, admin)
	if recorder.Code != http.StatusOK || approved["status"] != "approved" || approved["decidedBy"] == "" {
		t.Fatalf("approve = %d %v", recorder.Code, approved)
	}

	requireApprovedOverride(t, fixture, 6)

	recorder, _ = fixture.do(t, http.MethodPost, "/api/v1/admin/quota-requests/"+formatID(id)+"/approve", `{}`, admin)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("second approve status = %d, want 409", recorder.Code)
	}

	entries, err := fixture.store.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	found := false

	for _, entry := range entries {
		if entry.Action == "quota.request_approved" {
			found = true
		}
	}

	if !found {
		t.Fatal("no quota.request_approved audit row")
	}
}

func TestQuotaRequest_DenyLeavesPolicyUnchanged(t *testing.T) {
	t.Parallel()

	fixture := newQuotaFixture(t)
	user := aliceCookie(t, fixture.auth)
	admin := adminCookie(t, fixture.auth)

	_, created := fixture.do(t, http.MethodPost, "/api/v1/quota-requests", `{"requestedQuota":10,"reason":"lab"}`, user)

	recorder, denied := fixture.do(t, http.MethodPost,
		"/api/v1/admin/quota-requests/"+formatID(requestID(t, created))+"/deny", "", admin)
	if recorder.Code != http.StatusOK || denied["status"] != "denied" {
		t.Fatalf("deny = %d %v", recorder.Code, denied)
	}

	settings, err := fixture.service.Settings(context.Background(), "default")
	if err != nil || settings.Allowed != 2 {
		t.Fatalf("policy allowed = %d, err %v; want unchanged 2", settings.Allowed, err)
	}

	recorder, _ = fixture.do(t, http.MethodPost, "/api/v1/admin/quota-requests/999/deny", "", admin)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("missing id status = %d, want 404", recorder.Code)
	}
}

func TestQuotaRequest_ApproveAfterDenyLeavesPolicyUnchanged(t *testing.T) {
	t.Parallel()

	fixture := newQuotaFixture(t)
	user := aliceCookie(t, fixture.auth)
	admin := adminCookie(t, fixture.auth)

	path := pendingAdminPath(t, fixture, user)

	if recorder, _ := fixture.do(t, http.MethodPost, path+"/deny", "", admin); recorder.Code != http.StatusOK {
		t.Fatalf("deny = %d", recorder.Code)
	}

	recorder, body := fixture.do(t, http.MethodPost, path+"/approve", `{"quota":9}`, admin)
	if recorder.Code != http.StatusConflict || body["code"] != "quota_request_decided" {
		t.Fatalf("approve after deny = %d %v, want 409 quota_request_decided", recorder.Code, body)
	}

	settings, err := fixture.service.Settings(context.Background(), "default")
	if err != nil || settings.Allowed != 2 {
		t.Fatalf("policy allowed = %d, err %v; want unchanged 2", settings.Allowed, err)
	}
}

func TestQuotaRequest_ApproveAtCurrentQuotaMarksDecided(t *testing.T) {
	t.Parallel()

	fixture := newQuotaFixture(t)
	user := aliceCookie(t, fixture.auth)
	admin := adminCookie(t, fixture.auth)

	path := pendingAdminPath(t, fixture, user)

	recorder, approved := fixture.do(t, http.MethodPost, path+"/approve", `{"quota":2}`, admin)
	if recorder.Code != http.StatusOK || approved["status"] != "approved" {
		t.Fatalf("no-change approve = %d %v, want 200 approved", recorder.Code, approved)
	}
}

func TestQuotaRequest_AdminRoutesRequireAdmin(t *testing.T) {
	t.Parallel()

	fixture := newQuotaFixture(t)
	user := aliceCookie(t, fixture.auth)

	recorder, _ := fixture.do(t, http.MethodGet, "/api/v1/admin/quota-requests", "", user)
	if recorder.Code != http.StatusForbidden {
		t.Fatalf("user admin list status = %d, want 403", recorder.Code)
	}

	recorder, _ = fixture.do(t, http.MethodGet, "/api/v1/admin/quota-requests?status=bogus", "", adminCookie(t, fixture.auth))
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("bogus status = %d, want 400", recorder.Code)
	}
}

func formatID(id float64) string {
	return strconv.FormatInt(int64(id), 10)
}

// requestID reads the id of a created request body, failing the test if absent.
func requestID(t *testing.T, body map[string]any) float64 {
	t.Helper()

	id, ok := body["id"].(float64)
	if !ok {
		t.Fatalf("body has no numeric id: %v", body)
	}

	return id
}

// requestCount reads the number of rows in an admin list body.
func requestCount(t *testing.T, body map[string]any) int {
	t.Helper()

	rows, ok := body["requests"].([]any)
	if !ok {
		t.Fatalf("body has no requests array: %v", body)
	}

	return len(rows)
}

// requireApprovedOverride checks the approved pool override took effect while
// the cluster-wide quota stayed at its seeded value of 2.
func requireApprovedOverride(t *testing.T, fixture quotaFixture, want int) {
	t.Helper()

	allowed, err := fixture.service.PoolAllowance(context.Background(), "default", cluster.FakePoolAlice)
	if err != nil || allowed != want {
		t.Fatalf("pool allowance = %d, err %v; want %d (pool override)", allowed, err, want)
	}

	settings, err := fixture.service.Settings(context.Background(), "default")
	if err != nil || settings.Allowed != 2 {
		t.Fatalf("global allowed = %d, err %v; want unchanged 2", settings.Allowed, err)
	}
}

func TestQuotaRequest_MineReturnsNullRequestBeforeAnyCreate(t *testing.T) {
	t.Parallel()

	fixture := newQuotaFixture(t)
	user := aliceCookie(t, fixture.auth)

	recorder, body := fixture.do(t, http.MethodGet, "/api/v1/quota-requests/mine", "", user)
	if recorder.Code != http.StatusOK {
		t.Fatalf("mine = %d, want 200", recorder.Code)
	}

	if value, present := body["request"]; !present || value != nil {
		t.Fatalf("request = %v, want explicit null", body["request"])
	}
}

func TestQuotaRequest_MineUnauthenticated(t *testing.T) {
	t.Parallel()

	fixture := newQuotaFixture(t)

	recorder, _ := fixture.do(t, http.MethodGet, "/api/v1/quota-requests/mine", "", nil)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("mine unauthenticated = %d, want 401", recorder.Code)
	}
}

func TestQuotaRequest_CreateReasonTooLong(t *testing.T) {
	t.Parallel()

	fixture := newQuotaFixture(t)
	user := aliceCookie(t, fixture.auth)

	recorder, _ := fixture.do(t, http.MethodPost, "/api/v1/quota-requests",
		`{"requestedQuota":10,"reason":"`+strings.Repeat("x", 501)+`"}`, user)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("long reason status = %d, want 400", recorder.Code)
	}
}

func TestQuotaRequest_AdminListFiltersDecidedRows(t *testing.T) {
	t.Parallel()

	fixture := newQuotaFixture(t)
	user := aliceCookie(t, fixture.auth)
	admin := adminCookie(t, fixture.auth)

	path := pendingAdminPath(t, fixture, user)

	if recorder, _ := fixture.do(t, http.MethodPost, path+"/deny", "", admin); recorder.Code != http.StatusOK {
		t.Fatalf("deny = %d", recorder.Code)
	}

	for _, testCase := range []struct {
		status string
		want   int
	}{
		{"pending", 0},
		{"denied", 1},
		{"all", 1},
	} {
		recorder, list := fixture.do(t, http.MethodGet, "/api/v1/admin/quota-requests?status="+testCase.status, "", admin)
		if recorder.Code != http.StatusOK || requestCount(t, list) != testCase.want {
			t.Fatalf("status=%s: %d rows %v, want %d", testCase.status, recorder.Code, list, testCase.want)
		}
	}
}

func TestQuotaRequest_AdminDecisionValidation(t *testing.T) {
	t.Parallel()

	fixture := newQuotaFixture(t)
	admin := adminCookie(t, fixture.auth)

	for _, testCase := range []struct {
		name   string
		path   string
		body   string
		status int
	}{
		{"approve non-numeric id", "/api/v1/admin/quota-requests/abc/approve", `{}`, http.StatusBadRequest},
		{"approve zero id", "/api/v1/admin/quota-requests/0/approve", `{}`, http.StatusBadRequest},
		{"approve missing id", "/api/v1/admin/quota-requests/999/approve", `{}`, http.StatusNotFound},
		{"deny non-numeric id", "/api/v1/admin/quota-requests/abc/deny", ``, http.StatusBadRequest},
		{"deny negative id", "/api/v1/admin/quota-requests/-1/deny", ``, http.StatusBadRequest},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			recorder, _ := fixture.do(t, http.MethodPost, testCase.path, testCase.body, admin)
			if recorder.Code != testCase.status {
				t.Fatalf("status = %d, want %d", recorder.Code, testCase.status)
			}
		})
	}
}

func TestQuotaRequest_ApproveBadJSON(t *testing.T) {
	t.Parallel()

	fixture := newQuotaFixture(t)
	user := aliceCookie(t, fixture.auth)
	admin := adminCookie(t, fixture.auth)

	path := pendingAdminPath(t, fixture, user)

	recorder, _ := fixture.do(t, http.MethodPost, path+"/approve", `{`, admin)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("approve bad json = %d, want 400", recorder.Code)
	}
}

// An approve body whose quota is out of the policy range maps to 400
// invalid_policy, not a generic failure.
func TestQuotaRequest_ApproveQuotaOutOfRange(t *testing.T) {
	t.Parallel()

	fixture := newQuotaFixture(t)
	user := aliceCookie(t, fixture.auth)
	admin := adminCookie(t, fixture.auth)

	path := pendingAdminPath(t, fixture, user)

	recorder, body := fixture.do(t, http.MethodPost, path+"/approve", `{"quota":-5}`, admin)
	if recorder.Code != http.StatusBadRequest || body["code"] != "invalid_policy" {
		t.Fatalf("approve out of range = %d %v, want 400 invalid_policy", recorder.Code, body)
	}
}

// pendingAdminPath posts a pending request as the user and returns its admin
// decision path.
func pendingAdminPath(t *testing.T, fixture quotaFixture, user *http.Cookie) string {
	t.Helper()

	_, created := fixture.do(t, http.MethodPost, "/api/v1/quota-requests", `{"requestedQuota":10,"reason":"lab"}`, user)

	return "/api/v1/admin/quota-requests/" + formatID(requestID(t, created))
}
