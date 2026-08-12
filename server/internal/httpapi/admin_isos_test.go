package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

type adminISODTO struct {
	Storage   string `json:"storage"`
	Node      string `json:"node"`
	File      string `json:"file"`
	SizeBytes int64  `json:"sizeBytes"`
	Enabled   bool   `json:"enabled"`
}

// TestAdminISOs_ListShowsSuperset — T016: GET /admin/isos shows the fake
// superset keyed by (storage, file).
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminISOs_ListShowsSuperset(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/isos?cluster=default")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var isos []adminISODTO
	if err := json.Unmarshal(rec.Body.Bytes(), &isos); err != nil {
		t.Fatalf("decode isos: %v", err)
	}

	if len(isos) != 3 {
		t.Fatalf("expected 3 ISOs, got %d", len(isos))
	}

	enabledByKey := make(map[string]bool)
	for _, i := range isos {
		enabledByKey[i.Storage+":"+i.File] = i.Enabled
	}

	if !enabledByKey["local:debian-12-generic-amd64.iso"] {
		t.Error("debian-12 should be enabled")
	}

	if !enabledByKey["local:ubuntu-24.04-server-amd64.iso"] {
		t.Error("ubuntu-24 should be enabled")
	}

	if enabledByKey["local:rocky-9-generic-x86_64.iso"] {
		t.Error("rocky-9 should not be enabled")
	}
}

// TestAdminISOs_Toggle — T016: toggle rocky-9 on, confirm it sticks.
//
//nolint:paralleltest,dupl // serial: shared fake dataset; intentionally parallel to TestAdminBridges_Toggle
func TestAdminISOs_Toggle(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/isos/toggle",
		`{"cluster":"default","storage":"local","file":"rocky-9-generic-x86_64.iso","enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle status = %d: %s", rec.Code, rec.Body.String())
	}

	list := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/isos?cluster=default")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d", list.Code)
	}

	var isos []adminISODTO
	if err := json.Unmarshal(list.Body.Bytes(), &isos); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for _, i := range isos {
		if i.File == "rocky-9-generic-x86_64.iso" && !i.Enabled {
			t.Error("rocky-9 should be enabled after toggle")
		}
	}
}

// TestAdminISOs_NonAdminReturns403 — T016: non-admin 403.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminISOs_NonAdminReturns403(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	aliceCookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	rec := adminGet(t, handler, authHandler, aliceCookie, "/api/v1/admin/isos?cluster=default")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestAdminISOs_ToggleUnknownReturns404 — T016: unknown pair 404.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminISOs_ToggleUnknownReturns404(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/isos/toggle",
		`{"cluster":"default","storage":"local","file":"nope.iso","enabled":true}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
