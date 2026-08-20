package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"testing"
)

type adminBridgeDTO struct {
	Name    string `json:"name"`
	Node    string `json:"node"`
	Active  bool   `json:"active"`
	Comment string `json:"comment"`
	Enabled bool   `json:"enabled"`
}

const (
	duplicateBridgeName  = "shared-bridge"
	duplicateBridgeNodeA = "node-a"
	duplicateBridgeNodeB = "node-b"
)

type duplicateBridgeHTTPClient struct {
	cluster.Fake
}

func (duplicateBridgeHTTPClient) ListBridges(_ context.Context) ([]cluster.Bridge, error) {
	return []cluster.Bridge{
		{Name: duplicateBridgeName, Node: duplicateBridgeNodeA, Active: true},
		{Name: duplicateBridgeName, Node: duplicateBridgeNodeB, Active: true},
	}, nil
}

func newDuplicateBridgeAdminHandler(t *testing.T) (*httpapi.AdminCatalog, *httpapi.Auth) {
	t.Helper()

	authHandler := newAuthHandler(t)
	st := newAdminStore(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	handler := httpapi.NewAdminCatalog(authHandler, st, duplicateBridgeHTTPClient{}, nil, logger)

	return handler, authHandler
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

//nolint:paralleltest // serial: database-backed handler fixture
func TestAdminBridges_SameNameOnTwoNodesTogglesIndependently(t *testing.T) {
	handler, authHandler := newDuplicateBridgeAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	body := fmt.Sprintf(`{"cluster":"default","node":%q,"name":%q,"enabled":true}`, duplicateBridgeNodeA, duplicateBridgeName)

	toggle := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/bridges/toggle", body)
	if toggle.Code != http.StatusOK {
		t.Fatalf("toggle status = %d: %s", toggle.Code, toggle.Body.String())
	}

	var toggled adminBridgeDTO
	if err := json.Unmarshal(toggle.Body.Bytes(), &toggled); err != nil {
		t.Fatalf("decode toggle response: %v", err)
	}

	if toggled.Node != duplicateBridgeNodeA || toggled.Name != duplicateBridgeName || !toggled.Enabled {
		t.Fatalf("toggle response = %+v, want enabled %s on %s", toggled, duplicateBridgeName, duplicateBridgeNodeA)
	}

	list := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/bridges?cluster=default")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", list.Code, list.Body.String())
	}

	var bridges []adminBridgeDTO
	if err := json.Unmarshal(list.Body.Bytes(), &bridges); err != nil {
		t.Fatalf("decode bridges: %v", err)
	}

	if len(bridges) != 2 {
		t.Fatalf("bridge count = %d, want 2", len(bridges))
	}

	for _, bridge := range bridges {
		wantEnabled := bridge.Node == duplicateBridgeNodeA
		if bridge.Enabled != wantEnabled {
			t.Errorf("%s on %s enabled = %v, want %v", bridge.Name, bridge.Node, bridge.Enabled, wantEnabled)
		}
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminBridges_ToggleRequiresNode(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	tests := []struct {
		name string
		body string
	}{
		{name: "missing", body: `{"cluster":"default","name":"bridge","enabled":true}`},
		{name: "empty", body: `{"cluster":"default","node":"","name":"bridge","enabled":true}`},
		{name: "whitespace", body: `{"cluster":"default","node":"  ","name":"bridge","enabled":true}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/bridges/toggle", tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
			}
		})
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
