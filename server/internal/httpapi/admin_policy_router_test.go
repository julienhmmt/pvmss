package httpapi_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/policy"
	"testing"
)

func TestAdminPolicyRoutes_RequireAdminThroughProductionRouter(t *testing.T) {
	st := newAdminStore(t)
	authHandler := newAuthHandler(t)
	fake := cluster.Fake{}
	snapshot, err := fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}
	index := inventory.BuildIndex(snapshot)
	service := policy.New(st, inventory.NewProjectionFromIndex(&index), fake)
	adminPolicy := httpapi.NewAdminPolicy(authHandler, service, slog.New(slog.DiscardHandler))
	noop := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	mux := httpapi.NewRouter(httpapi.RouterConfig{
		Health:         noop,
		ClusterNodes:   noop,
		ClusterRefresh: noop,
		VMs:            noop,
		VMDetail:       noop,
		Auth:           authHandler,
		Log:            slog.New(slog.DiscardHandler),
		AdminPolicy:    adminPolicy,
	})
	cookie := aliceCookie(t, authHandler)

	for _, testCase := range []struct {
		method string
		path   string
	}{
		{method: http.MethodGet, path: "/api/v1/admin/policy"},
		{method: http.MethodPut, path: "/api/v1/admin/policy"},
		{method: http.MethodGet, path: "/api/v1/admin/policy/nodes"},
		{method: http.MethodPut, path: "/api/v1/admin/policy/nodes/pve-node-01"},
	} {
		t.Run(testCase.method+" "+testCase.path, func(t *testing.T) {
			request := httptest.NewRequest(testCase.method, testCase.path, nil)
			request.AddCookie(cookie)
			recorder := httptest.NewRecorder()
			mux.ServeHTTP(recorder, request)
			if recorder.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want 403: %s", recorder.Code, recorder.Body.String())
			}
		})
	}
}
