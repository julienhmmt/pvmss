package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
)

const (
	imageDebian = "debian-12-generic-cloudimg-amd64.qcow2"
	imageGhost  = "ghost.qcow2"
)

type adminImageDTO struct {
	Storage string `json:"storage"`
	Node    string `json:"node"`
	File    string `json:"file"`
	Enabled bool   `json:"enabled"`
	Missing bool   `json:"missing"`
}

func listAdminImages(t *testing.T, rec interface {
	Bytes() []byte
},
) []adminImageDTO {
	t.Helper()

	var images []adminImageDTO
	if err := json.Unmarshal(rec.Bytes(), &images); err != nil {
		t.Fatalf("decode images: %v", err)
	}

	return images
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminImages_ListShowsDiscoveredSuperset(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/images?cluster=default")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	if images := listAdminImages(t, rec.Body); len(images) != 3 {
		t.Fatalf("images = %d, want the 3 fake cloud images", len(images))
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminImages_ListRejectsUnauthenticatedAndUnknownCluster(t *testing.T) {
	handler, authHandler := newMultiClusterAdminCatalogHandler(t)

	if rec := adminGet(t, handler, authHandler, nil, "/api/v1/admin/images"); rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated status = %d, want 401", rec.Code)
	}

	cookie := adminCookie(t, authHandler)

	if rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/images?cluster=nonexistent"); rec.Code != http.StatusNotFound {
		t.Fatalf("unknown cluster status = %d, want 404", rec.Code)
	}
}

func imageEnabled(t *testing.T, images []adminImageDTO, file string) bool {
	t.Helper()

	for _, image := range images {
		if image.File == file {
			return image.Enabled
		}
	}

	t.Fatalf("image %q not listed", file)

	return false
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminImages_ToggleSticks(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	listURL := "/api/v1/admin/images?cluster=default"

	before := listAdminImages(t, adminGet(t, handler, authHandler, cookie, listURL).Body)
	if imageEnabled(t, before, imageDebian) {
		t.Fatalf("%s should start disabled", imageDebian)
	}

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/images/toggle",
		`{"cluster":"default","node":"pve-node-01","storage":"local","file":"`+imageDebian+`","enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle status = %d: %s", rec.Code, rec.Body.String())
	}

	after := listAdminImages(t, adminGet(t, handler, authHandler, cookie, listURL).Body)
	if !imageEnabled(t, after, imageDebian) {
		t.Fatalf("%s should be enabled after toggle", imageDebian)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminImages_ToggleRejectsBadInput(t *testing.T) {
	handler, authHandler := newMultiClusterAdminCatalogHandler(t)
	cookie := adminCookie(t, authHandler)

	cases := []struct {
		name   string
		cookie *http.Cookie
		body   string
		want   int
	}{
		{"unauthenticated", nil, `{"cluster":"default","node":"n","storage":"s","file":"f","enabled":true}`, http.StatusUnauthorized},
		{"bad json", cookie, "{bad json", http.StatusBadRequest},
		{"missing node", cookie, `{"cluster":"default","storage":"local","file":"f","enabled":true}`, http.StatusBadRequest},
		{"unknown cluster", cookie, `{"cluster":"nonexistent","node":"pve-node-01","storage":"local","file":"f","enabled":true}`, http.StatusNotFound},
		{"image not on node", cookie, `{"cluster":"default","node":"pve-node-01","storage":"local","file":"` + imageGhost + `","enabled":true}`, http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := adminPost(t, handler, authHandler, tc.cookie, "/api/v1/admin/images/toggle", tc.body)
			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestAdminImages_DeleteOrphan(t *testing.T) {
	handler, authHandler, st := newAdminHandlerWithEmptyDiscovery(t)
	cookie := adminCookie(t, authHandler)

	if err := st.SetImageEnabled(context.Background(), "default", "pve-node-01", "local", imageGhost, 1, false); err != nil {
		t.Fatalf("seed: %v", err)
	}

	path := "/api/v1/admin/images/default/pve-node-01/local/" + imageGhost

	if rec := adminDelete(t, handler, authHandler, cookie, path); rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204: %s", rec.Code, rec.Body.String())
	}

	if rec := adminDelete(t, handler, authHandler, cookie, path); rec.Code != http.StatusNotFound {
		t.Fatalf("second delete status = %d, want 404", rec.Code)
	}
}
