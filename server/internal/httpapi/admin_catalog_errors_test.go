package httpapi_test

import (
	"net/http"
	"testing"
)

var adminCatalogListResources = []string{"nodes", "storages", "bridges", "isos", "images", "templates", "snippet-storages"}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogLists_RejectBadClusterAndAnonymous(t *testing.T) {
	handler, authHandler := newMultiClusterAdminCatalogHandler(t)
	cookie := adminCookie(t, authHandler)

	for _, resource := range adminCatalogListResources {
		base := "/api/v1/admin/" + resource

		cases := []struct {
			name   string
			cookie *http.Cookie
			query  string
			want   int
		}{
			{caseAnonymous, nil, "?cluster=default", http.StatusUnauthorized},
			{"cluster required", cookie, "", http.StatusBadRequest},
			{"unknown cluster", cookie, "?cluster=nonexistent", http.StatusNotFound},
		}
		for _, tc := range cases {
			t.Run(resource+"/"+tc.name, func(t *testing.T) {
				rec := adminGet(t, handler, authHandler, tc.cookie, base+tc.query)
				if rec.Code != tc.want {
					t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
				}
			})
		}
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogDeletes_UnknownRowIsNotFound(t *testing.T) {
	handler, authHandler := newMultiClusterAdminCatalogHandler(t)
	cookie := adminCookie(t, authHandler)

	paths := []string{
		"/api/v1/admin/storages/default/nope/nope",
		"/api/v1/admin/bridges/default/nope/nope",
		"/api/v1/admin/isos/default/nope/nope/nope.iso",
		"/api/v1/admin/images/default/nope/nope/nope.qcow2",
	}
	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			if rec := adminDelete(t, handler, authHandler, nil, path); rec.Code != http.StatusUnauthorized {
				t.Fatalf("anonymous status = %d, want 401", rec.Code)
			}

			if rec := adminDelete(t, handler, authHandler, cookie, path); rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404: %s", rec.Code, rec.Body.String())
			}
		})
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogToggles_RejectBadBody(t *testing.T) {
	handler, authHandler := newMultiClusterAdminCatalogHandler(t)
	cookie := adminCookie(t, authHandler)

	for _, resource := range []string{"nodes", "storages", "bridges", "isos", "templates"} {
		t.Run(resource, func(t *testing.T) {
			path := "/api/v1/admin/" + resource + "/toggle"

			if rec := adminPost(t, handler, authHandler, nil, path, `{}`); rec.Code != http.StatusUnauthorized {
				t.Fatalf("anonymous status = %d, want 401", rec.Code)
			}

			if rec := adminPost(t, handler, authHandler, cookie, path, badJSONBody); rec.Code != http.StatusBadRequest {
				t.Fatalf("bad json status = %d, want 400", rec.Code)
			}

			rec := adminPost(t, handler, authHandler, cookie, path, `{"cluster":"nonexistent","node":"n","name":"x","storage":"s","file":"f","vmid":1,"enabled":true}`)
			if rec.Code != http.StatusNotFound && rec.Code != http.StatusBadRequest {
				t.Fatalf("unknown cluster status = %d, want 404 or 400: %s", rec.Code, rec.Body.String())
			}
		})
	}
}
