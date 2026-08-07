//nolint:noctx // test scaffolding does not need real context
//nolint:goconst // test fixture strings
package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"slices"
	"testing"
	"time"
)

const (
	testMaxListPageSize = 100
	testDefaultQuota    = -1
)

type vmListItem struct {
	VMID        int      `json:"vmid"`
	Name        string   `json:"name"`
	Node        string   `json:"node"`
	Status      string   `json:"status"`
	Pool        string   `json:"pool"`
	Tags        []string `json:"tags"`
	CPUCores    int      `json:"cpuCores"`
	MemoryTotal int64    `json:"memoryTotal"`
}

type vmListQuota struct {
	Used    int `json:"used"`
	Allowed int `json:"allowed"`
}

type vmListResponse struct {
	Items          []vmListItem `json:"items"`
	Total          int          `json:"total"`
	Page           int          `json:"page"`
	PageSize       int          `json:"pageSize"`
	AvailableNodes []string     `json:"availableNodes"`
	EmptyReason    string       `json:"emptyReason"`
	Quota          *vmListQuota `json:"quota"`
}

// newVMsHandler builds the handler over the T01/T02 fake dataset — alice owns
// pool-alice (VMIDs 100, 101, 102, 114, 115), bob owns pool-bob.
func newVMsHandler(t *testing.T) (*httpapi.VMs, *httpapi.Auth) {
	t.Helper()

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	projection := buildProjectionWithIndex(t, snap, time.Now())
	authHandler := newAuthHandler(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	return httpapi.NewVMs(projection, authHandler, testMaxListPageSize, testDefaultQuota, logger), authHandler
}

func loginCookie(t *testing.T, authHandler *httpapi.Auth, body string) *http.Cookie {
	t.Helper()

	response := serveJSON(authHandler.Login, "/api/v1/auth/login", body)
	if response.Code != http.StatusOK {
		t.Fatalf("login status = %d, want %d", response.Code, http.StatusOK)
	}

	return response.Result().Cookies()[0]
}

func getVMList(t *testing.T, handler *httpapi.VMs, cookie *http.Cookie, query string) (*httptest.ResponseRecorder, vmListResponse) {
	t.Helper()

	request := httptest.NewRequest(http.MethodGet, "/api/v1/vms?"+query, nil)
	request.AddCookie(cookie)

	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	var list vmListResponse
	if response.Code == http.StatusOK {
		if err := json.Unmarshal(response.Body.Bytes(), &list); err != nil {
			t.Fatalf("decode response: %v", err)
		}
	}

	return response, list
}

func sortedVMIDs(list vmListResponse) []int {
	ids := make([]int, len(list.Items))
	for i, item := range list.Items {
		ids[i] = item.VMID
	}

	slices.Sort(ids)

	return ids
}

// TestVMs_DefaultScopeReturnsOnlyCallerPool — T005: GET /vms with no scope
// parameter returns exactly the caller's pool.
func TestVMs_DefaultScopeReturnsOnlyCallerPool(t *testing.T) {
	handler, authHandler := newVMsHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	response, list := getVMList(t, handler, cookie, "")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	if got, want := sortedVMIDs(list), []int{100, 101, 102, 114, 115, 123, 124}; !slices.Equal(got, want) {
		t.Errorf("vmids = %v, want %v", got, want)
	}

	for _, item := range list.Items {
		if item.Pool != "pool-alice" {
			t.Errorf("item %+v leaks another pool", item)
		}
	}

	if list.Total != 7 {
		t.Errorf("total = %d, want 7", list.Total)
	}
}

// TestVMs_NonAdminScopeAllSilentlyOverridden — T006: scope=all from a
// non-admin returns the same result as no scope at all (SC-005).
func TestVMs_NonAdminScopeAllSilentlyOverridden(t *testing.T) {
	handler, authHandler := newVMsHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	_, scoped := getVMList(t, handler, cookie, "scope=all")

	_, unscoped := getVMList(t, handler, cookie, "")
	if got, want := sortedVMIDs(scoped), sortedVMIDs(unscoped); !slices.Equal(got, want) {
		t.Errorf("scope=all vmids = %v, want %v", got, want)
	}
}

// TestVMs_AdminScopeAllReturnsAcrossPools — T007: an admin asking for
// scope=all sees VMs across pools.
func TestVMs_AdminScopeAllReturnsAcrossPools(t *testing.T) {
	handler, authHandler := newVMsHandler(t)
	response := serveJSON(authHandler.AdminLogin, "/api/v1/auth/admin-login", `{"password":"pvmss-local-admin"}`)
	cookie := response.Result().Cookies()[0]

	httpResponse, list := getVMList(t, handler, cookie, "scope=all&pageSize=100")
	if httpResponse.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", httpResponse.Code, http.StatusOK)
	}

	pools := make(map[string]bool)
	for _, item := range list.Items {
		pools[item.Pool] = true
	}

	if len(pools) < 3 {
		t.Errorf("pools = %v, want VMs across at least 3 pools", pools)
	}

	if list.Total != 25 {
		t.Errorf("total = %d, want 25", list.Total)
	}
}

// TestVMs_NoVMsOwnedEmptyReason — T008: a caller whose pool has no VMs gets
// emptyReason no_vms_owned, not an error.
func TestVMs_NoVMsOwnedEmptyReason(t *testing.T) {
	handler, authHandler := newVMsHandler(t)
	response := serveJSON(authHandler.AdminLogin, "/api/v1/auth/admin-login", `{"password":"pvmss-local-admin"}`)
	cookie := response.Result().Cookies()[0]

	httpResponse, list := getVMList(t, handler, cookie, "")
	if httpResponse.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", httpResponse.Code, http.StatusOK)
	}

	if len(list.Items) != 0 {
		t.Fatalf("items = %v, want none", sortedVMIDs(list))
	}

	if list.EmptyReason != "no_vms_owned" {
		t.Errorf("emptyReason = %q, want no_vms_owned", list.EmptyReason)
	}
}

// TestVMs_SearchByName — T014.
func TestVMs_SearchByName(t *testing.T) {
	handler, authHandler := newVMsHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	_, list := getVMList(t, handler, cookie, "search=web")
	if got, want := sortedVMIDs(list), []int{100, 101}; !slices.Equal(got, want) {
		t.Errorf("vmids = %v, want %v", got, want)
	}
}

// TestVMs_SearchByTag — T015.
func TestVMs_SearchByTag(t *testing.T) {
	handler, authHandler := newVMsHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	// "db" matches db-01 by tag AND sandbox-01/sandbox-02 by name substring —
	// one input, both match kinds, union (FR-002).
	_, list := getVMList(t, handler, cookie, "search=db")
	if got, want := sortedVMIDs(list), []int{102, 114, 115}; !slices.Equal(got, want) {
		t.Errorf("vmids = %v, want %v", got, want)
	}
}

// TestVMs_SearchByNumericID — T016.
func TestVMs_SearchByNumericID(t *testing.T) {
	handler, authHandler := newVMsHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	_, list := getVMList(t, handler, cookie, "search=114")
	if got, want := sortedVMIDs(list), []int{114}; !slices.Equal(got, want) {
		t.Errorf("vmids = %v, want %v", got, want)
	}
}

// TestVMs_SearchNoMatch — T017.
func TestVMs_SearchNoMatch(t *testing.T) {
	handler, authHandler := newVMsHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	_, list := getVMList(t, handler, cookie, "search=does-not-exist")
	if len(list.Items) != 0 {
		t.Fatalf("items = %v, want none", sortedVMIDs(list))
	}

	if list.EmptyReason != "no_match" {
		t.Errorf("emptyReason = %q, want no_match", list.EmptyReason)
	}
}

// TestVMs_StatusFilterCombinedWithSearch — T020.
func TestVMs_StatusFilterCombinedWithSearch(t *testing.T) {
	handler, authHandler := newVMsHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	_, list := getVMList(t, handler, cookie, "search=web&status=stopped")
	if got, want := sortedVMIDs(list), []int{101}; !slices.Equal(got, want) {
		t.Errorf("vmids = %v, want %v", got, want)
	}
}

// TestVMs_NodeFilterKeepsFacet — T021: the node filter narrows results but
// availableNodes still lists every node in the scoped set (facet pre-filter).
func TestVMs_NodeFilterKeepsFacet(t *testing.T) {
	handler, authHandler := newVMsHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	_, list := getVMList(t, handler, cookie, "node=pve-node-02")
	if got, want := sortedVMIDs(list), []int{114, 115}; !slices.Equal(got, want) {
		t.Errorf("vmids = %v, want %v", got, want)
	}

	if got, want := list.AvailableNodes, []string{"pve-node-01", "pve-node-02"}; !slices.Equal(got, want) {
		t.Errorf("availableNodes = %v, want %v", got, want)
	}
}

// TestVMs_SortColumns — T022: every supported column, both directions.
func TestVMs_SortColumns(t *testing.T) {
	handler, authHandler := newVMsHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	tests := []struct {
		query   string
		wantIDs []int
	}{
		{query: "sortBy=vmid&sortDir=asc", wantIDs: []int{100, 101, 102, 114, 115, 123, 124}},
		{query: "sortBy=vmid&sortDir=desc", wantIDs: []int{124, 123, 115, 114, 102, 101, 100}},
		{query: "sortBy=name&sortDir=asc", wantIDs: []int{102, 123, 124, 114, 115, 100, 101}},
		{query: "sortBy=name&sortDir=desc", wantIDs: []int{101, 100, 115, 114, 124, 123, 102}},
		{query: "sortBy=node&sortDir=asc", wantIDs: []int{100, 101, 102, 123, 124, 114, 115}},
		{query: "sortBy=status&sortDir=asc", wantIDs: []int{100, 102, 123, 101, 114, 115, 124}},
		{query: "sortBy=cpu&sortDir=desc", wantIDs: []int{102, 124, 123, 101, 100, 115, 114}},
		{query: "sortBy=memory&sortDir=asc", wantIDs: []int{114, 115, 100, 101, 123, 124, 102}},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			_, list := getVMList(t, handler, cookie, tt.query)
			if got := sortedVMIDsOrderPreserved(list); !slices.Equal(got, tt.wantIDs) {
				t.Errorf("vmids = %v, want %v", got, tt.wantIDs)
			}
		})
	}
}

func sortedVMIDsOrderPreserved(list vmListResponse) []int {
	ids := make([]int, len(list.Items))
	for i, item := range list.Items {
		ids[i] = item.VMID
	}

	return ids
}

// TestVMs_InvalidSortColumnRejected — T022: unsupported sort column → 400,
// never silently defaulted (FR-005).
func TestVMs_InvalidSortColumnRejected(t *testing.T) {
	handler, authHandler := newVMsHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	response, _ := getVMList(t, handler, cookie, "sortBy=unknownColumn")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var envelope struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if envelope.Code != "invalid_sort_column" {
		t.Errorf("code = %q, want invalid_sort_column", envelope.Code)
	}
}

// TestVMs_PageBeyondRangeClamps — T026: a page past the end returns the
// nearest valid page, not an error or an empty result (FR-006).
func TestVMs_PageBeyondRangeClamps(t *testing.T) {
	handler, authHandler := newVMsHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	response, list := getVMList(t, handler, cookie, "page=99&pageSize=2")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusOK)
	}

	if list.Page != 4 {
		t.Errorf("page = %d, want clamped to 4", list.Page)
	}

	if len(list.Items) != 1 {
		t.Errorf("items = %d, want last page's 1 item", len(list.Items))
	}
}

// TestVMs_PageSizeOverMaximumRejected — T027.
func TestVMs_PageSizeOverMaximumRejected(t *testing.T) {
	handler, authHandler := newVMsHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	response, _ := getVMList(t, handler, cookie, "pageSize=101")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusBadRequest)
	}

	var envelope struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if envelope.Code != "page_size_too_large" {
		t.Errorf("code = %q, want page_size_too_large", envelope.Code)
	}
}

// TestVMs_QuotaReported — T030: a non-admin default-scope request carries
// quota; an admin scope=all request omits it entirely (spec Assumptions 5.3).
func TestVMs_QuotaReported(t *testing.T) {
	handler, authHandler := newVMsHandler(t)

	aliceCookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	_, userList := getVMList(t, handler, aliceCookie, "")
	if userList.Quota == nil {
		t.Fatal("quota missing for non-admin default scope")
	}

	if userList.Quota.Used != 7 || userList.Quota.Allowed != -1 {
		t.Errorf("quota = %+v, want {used:7 allowed:-1}", userList.Quota)
	}

	adminResponse := serveJSON(authHandler.AdminLogin, "/api/v1/auth/admin-login", `{"password":"pvmss-local-admin"}`)
	adminCookie := adminResponse.Result().Cookies()[0]

	_, adminList := getVMList(t, handler, adminCookie, "scope=all&pageSize=100")
	if adminList.Quota != nil {
		t.Errorf("quota = %+v, want absent for admin scope=all", adminList.Quota)
	}
}

// TestVMs_Unauthenticated — the endpoint requires a resolved identity (T02).
func TestVMs_Unauthenticated(t *testing.T) {
	handler, _ := newVMsHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/vms", nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)

	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
