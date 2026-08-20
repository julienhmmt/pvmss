package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

type adminBridgeDTO struct {
	Name    string `json:"name"`
	Node    string `json:"node"`
	Active  bool   `json:"active"`
	Comment string `json:"comment"`
	Enabled bool   `json:"enabled"`
}

// TestAdminBridges_ListShowsSuperset — GET /admin/bridges shows the fake
// superset with approvals reset by the node-aware migration.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminBridges_ListShowsSuperset(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/bridges?cluster=default")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var bridges []adminBridgeDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &bridges); err != nil {
		t.Fatalf("decode bridges: %v", err)
	}

	if len(bridges) != 3 {
		t.Fatalf("expected 3 bridges, got %d", len(bridges))
	}

	enabledByName := make(map[string]bool)
	for _, b := range bridges {
		enabledByName[b.Name] = b.Enabled
	}

	for name, enabled := range enabledByName {
		if enabled {
			t.Errorf("%s should not be enabled", name)
		}
	}
}

// TestAdminBridges_Toggle — T015: toggle vmbr2 on, confirm it sticks.
//
//nolint:paralleltest,dupl // serial: shared fake dataset; intentionally parallel to TestAdminISOs_Toggle
func TestAdminBridges_Toggle(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/bridges/toggle",
		`{"cluster":"default","node":"pve-node-02","name":"vmbr2","enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle status = %d: %s", rec.Code, rec.Body.String())
	}

	list := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/bridges?cluster=default")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d", list.Code)
	}

	var bridges []adminBridgeDTO
	if err := json.Unmarshal(list.Body.Bytes(), &bridges); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for _, b := range bridges {
		if b.Name == "vmbr2" && !b.Enabled {
			t.Error("vmbr2 should be enabled after toggle")
		}
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminBridges_ToggleRequiresNode(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/bridges/toggle",
		`{"cluster":"default","name":"vmbr0","enabled":true}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// TestAdminBridges_NonAdminReturns403 — T015: non-admin 403.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminBridges_NonAdminReturns403(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	aliceCookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	rec := adminGet(t, handler, authHandler, aliceCookie, "/api/v1/admin/bridges?cluster=default")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestAdminBridges_ToggleUnknownReturns404 — T015: unknown bridge 404.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminBridges_ToggleUnknownReturns404(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/bridges/toggle",
		`{"cluster":"default","node":"pve-node-01","name":"vmbr99","enabled":true}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
