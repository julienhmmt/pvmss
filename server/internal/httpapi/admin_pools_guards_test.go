//nolint:goconst // table rows repeat request paths
package httpapi_test

import (
	"net/http"
	"testing"
)

//nolint:paralleltest // serial: shared fake and session fixtures
func TestAdminPools_GuardsAndUnknowns(t *testing.T) {
	handler, authHandler := newAdminPoolsHandler(t)
	admin := adminCookie(t, authHandler)
	alice := aliceCookie(t, authHandler)

	const (
		listPath   = "/api/v1/admin/pools"
		detailPath = "/api/v1/admin/pools/ghost-pool"
	)

	cases := []struct {
		name    string
		handler http.HandlerFunc
		method  string
		path    string
		body    string
		cookie  *http.Cookie
		want    int
	}{
		{"list anonymous", handler.ServeList, http.MethodGet, listPath, "", nil, http.StatusUnauthorized},
		{"list tenant", handler.ServeList, http.MethodGet, listPath, "", alice, http.StatusForbidden},
		{"list unknown cluster", handler.ServeList, http.MethodGet, listPath + "?cluster=nonexistent", "", admin, http.StatusNotFound},
		{"detail anonymous", handler.ServeDetail, http.MethodGet, detailPath, "", nil, http.StatusUnauthorized},
		{"detail tenant", handler.ServeDetail, http.MethodGet, detailPath, "", alice, http.StatusForbidden},
		{"detail unknown cluster", handler.ServeDetail, http.MethodGet, detailPath + "?cluster=nonexistent", "", admin, http.StatusNotFound},
		{"detail unknown pool", handler.ServeDetail, http.MethodGet, detailPath + "?cluster=default", "", admin, http.StatusNotFound},
		{"create anonymous", handler.ServeCreate, http.MethodPost, listPath, `{"name":"x"}`, nil, http.StatusUnauthorized},
		{"create tenant", handler.ServeCreate, http.MethodPost, listPath, `{"name":"x"}`, alice, http.StatusForbidden},
		{"create malformed body", handler.ServeCreate, http.MethodPost, listPath, badJSONBody, admin, http.StatusBadRequest},
		{"delete anonymous", handler.ServeDelete, http.MethodDelete, detailPath, "", nil, http.StatusUnauthorized},
		{"delete tenant", handler.ServeDelete, http.MethodDelete, detailPath, "", alice, http.StatusForbidden},
		{"delete unknown cluster", handler.ServeDelete, http.MethodDelete, detailPath + "?cluster=nonexistent", "", admin, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := adminPoolsRequest(t, tc.handler, tc.method, tc.path, tc.cookie, tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}
