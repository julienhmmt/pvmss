package httpapi_test

import (
	"context"
	"net/http"
	"testing"
)

const (
	overridePoolPath  = "/api/v1/admin/quota-overrides/" + auditTestCluster + "/pvmss-alice"
	overrideListPath  = "/api/v1/admin/quota-overrides"
	overridePoolAlice = "pvmss-alice"
)

// registerOverridePool marks the pool as a managed pool on the default cluster,
// the precondition for setting an override on it.
func registerOverridePool(t *testing.T, fixture quotaFixture) {
	t.Helper()

	if err := fixture.store.RegisterManagedPool(context.Background(), auditTestCluster, overridePoolAlice); err != nil {
		t.Fatalf("RegisterManagedPool: %v", err)
	}
}

func listedOverrides(t *testing.T, fixture quotaFixture, path string) int {
	t.Helper()

	admin := adminCookie(t, fixture.auth)

	recorder, list := fixture.do(t, http.MethodGet, path, "", admin)
	if recorder.Code != http.StatusOK {
		t.Fatalf("list %s = %d %v", path, recorder.Code, list)
	}

	items, _ := list["overrides"].([]any)

	return len(items)
}

// requireOverrideListings checks the filtered, unfiltered and other-cluster
// list views against the single override set on the default cluster.
func requireOverrideListings(t *testing.T, fixture quotaFixture) {
	t.Helper()

	if got := listedOverrides(t, fixture, overrideListPath+"?cluster="+auditTestCluster); got != 1 {
		t.Fatalf("filtered list has %d overrides, want 1", got)
	}

	if got := listedOverrides(t, fixture, overrideListPath); got != 1 {
		t.Fatalf("unfiltered list has %d overrides, want 1", got)
	}

	if got := listedOverrides(t, fixture, overrideListPath+"?cluster=elsewhere"); got != 0 {
		t.Fatalf("other cluster list has %d overrides, want 0", got)
	}
}

// requirePoolAllowance checks the pool resolves to want while the cluster-wide
// quota stays at its seeded value of 2.
func requirePoolAllowance(t *testing.T, fixture quotaFixture, want int) {
	t.Helper()

	ctx := context.Background()

	allowed, err := fixture.service.PoolAllowance(ctx, auditTestCluster, overridePoolAlice)
	if err != nil || allowed != want {
		t.Fatalf("pool allowance = %d, err %v; want %d", allowed, err, want)
	}

	settings, err := fixture.service.Settings(ctx, auditTestCluster)
	if err != nil || settings.Allowed != 2 {
		t.Fatalf("global allowed = %d, err %v; want unchanged 2", settings.Allowed, err)
	}
}

func TestQuotaOverride_SetListDelete(t *testing.T) {
	t.Parallel()

	fixture := newQuotaFixture(t)
	admin := adminCookie(t, fixture.auth)

	registerOverridePool(t, fixture)

	recorder, body := fixture.do(t, http.MethodPut, overridePoolPath, `{"maxVms":9}`, admin)
	if recorder.Code != http.StatusOK || body["cluster"] != auditTestCluster || body["pool"] != overridePoolAlice ||
		body["maxVms"] != float64(9) || body["updatedBy"] == "" || body["updatedAt"] == "" {
		t.Fatalf("put = %d %v", recorder.Code, body)
	}

	requirePoolAllowance(t, fixture, 9)
	requireOverrideListings(t, fixture)

	recorder, _ = fixture.do(t, http.MethodDelete, overridePoolPath, "", admin)
	if recorder.Code != http.StatusNoContent {
		t.Fatalf("delete = %d, want 204", recorder.Code)
	}

	recorder, _ = fixture.do(t, http.MethodDelete, overridePoolPath, "", admin)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("second delete = %d, want 404", recorder.Code)
	}

	requirePoolAllowance(t, fixture, 2)
}

func TestQuotaOverride_PutValidation(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name   string
		path   string
		body   string
		status int
	}{
		{name: "unlimited accepted", path: overridePoolPath, body: `{"maxVms":-1}`, status: http.StatusOK},
		{name: "zero accepted", path: overridePoolPath, body: `{"maxVms":0}`, status: http.StatusOK},
		{name: "below unlimited refused", path: overridePoolPath, body: `{"maxVms":-2}`, status: http.StatusBadRequest},
		{name: "above maximum refused", path: overridePoolPath, body: `{"maxVms":100001}`, status: http.StatusBadRequest},
		{name: "missing maxVms refused", path: overridePoolPath, body: `{}`, status: http.StatusBadRequest},
		{name: "malformed body refused", path: overridePoolPath, body: `{"maxVms":"x"}`, status: http.StatusBadRequest},
		{name: "unmanaged pool refused", path: "/api/v1/admin/quota-overrides/" + auditTestCluster + "/pvmss-nobody", body: `{"maxVms":5}`, status: http.StatusNotFound},
		{name: "unknown cluster refused", path: "/api/v1/admin/quota-overrides/elsewhere/" + overridePoolAlice, body: `{"maxVms":5}`, status: http.StatusNotFound},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			fixture := newQuotaFixture(t)
			admin := adminCookie(t, fixture.auth)

			registerOverridePool(t, fixture)

			recorder, _ := fixture.do(t, http.MethodPut, testCase.path, testCase.body, admin)
			if recorder.Code != testCase.status {
				t.Fatalf("status = %d body %s, want %d", recorder.Code, recorder.Body.String(), testCase.status)
			}
		})
	}
}
