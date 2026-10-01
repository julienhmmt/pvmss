//nolint:goconst // table rows repeat request paths
package httpapi_test

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_RetrofitSeaBIOS_Guards(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	admin := adminCookie(t, authHandler)
	alice := aliceCookie(t, authHandler)

	cases := []struct {
		name   string
		method string
		path   string
		body   string
		cookie *http.Cookie
		want   int
	}{
		{"wrong method", http.MethodGet, "/api/v1/vms/default/101/retrofit-seabios", "", admin, http.StatusMethodNotAllowed},
		{"anonymous", http.MethodPost, "/api/v1/vms/default/101/retrofit-seabios", "", nil, http.StatusUnauthorized},
		{"tenant is refused", http.MethodPost, "/api/v1/vms/default/101/retrofit-seabios", "", alice, http.StatusForbidden},
		{"bad vmid", http.MethodPost, "/api/v1/vms/default/abc/retrofit-seabios", "", admin, http.StatusBadRequest},
		{"malformed body", http.MethodPost, "/api/v1/vms/default/101/retrofit-seabios", "{bad", admin, http.StatusBadRequest},
		{"unknown vm", http.MethodPost, "/api/v1/vms/default/999999/retrofit-seabios", "", admin, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, detailRequest(tc.method, tc.path, tc.body, tc.cookie))

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMDetail_RetrofitSeaBIOS_AdminOnExistingVM(t *testing.T) {
	handler, authHandler, _, _ := newVMDetailHandler(t)
	admin := adminCookie(t, authHandler)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPost, "/api/v1/vms/default/101/retrofit-seabios", `{"confirm":true}`, admin))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}
