package httpapi_test

import (
	"encoding/json"
	"net/http"
	"pvmss/server/internal/cluster"
	"testing"
)

type adminStorageDTO struct {
	Name    string `json:"name"`
	Node    string `json:"node"`
	Type    string `json:"type"`
	Total   int64  `json:"totalBytes"`
	Used    int64  `json:"usedBytes"`
	Enabled bool   `json:"enabled"`
}

// TestAdminStorages_ListShowsAllWithCorrectEnabled — T014: GET /admin/storages
// shows all VM-capable fake storages with correct per-(name,node) enabled state.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminStorages_ListShowsAllWithCorrectEnabled(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/storages?cluster=default")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var storages []adminStorageDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &storages); err != nil {
		t.Fatalf("decode storages: %v", err)
	}

	if len(storages) != 4 {
		t.Fatalf("expected 4 VM-capable storages, got %d", len(storages))
	}

	enabledByKey := make(map[string]bool)
	for _, s := range storages {
		enabledByKey[s.Name+"@"+s.Node] = s.Enabled
	}

	if !enabledByKey["local-lvm@pve-node-01"] {
		t.Error("local-lvm@pve-node-01 should be enabled")
	}

	if !enabledByKey["local@pve-node-02"] {
		t.Error("local@pve-node-02 should be enabled")
	}

	if !enabledByKey["ceph-data@pve-node-02"] {
		t.Error("ceph-data@pve-node-02 should be enabled")
	}

	if enabledByKey["local@pve-node-01"] {
		t.Error("local@pve-node-01 should not be enabled")
	}

	for _, name := range []string{cluster.FakeStorageBackupNFS, cluster.FakeStoragePBS} {
		if _, exists := enabledByKey[name+"@"+cluster.FakeNode03]; exists {
			t.Errorf("ineligible storage %q should be absent", name)
		}
	}
}

// TestAdminStorages_ToggleOnePairLeavesSameNamedPairUntouched — T014: toggling
// one storage+node pair does not affect a same-named pair on another node.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminStorages_ToggleOnePairLeavesSameNamedPairUntouched(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	// Toggle local@pve-node-02 off.
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/storages/toggle",
		`{"cluster":"default","name":"local","node":"pve-node-02","enabled":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle status = %d: %s", rec.Code, rec.Body.String())
	}

	// local@pve-node-01 is unaffected (still disabled — its own state).
	list := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/storages?cluster=default")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d", list.Code)
	}

	var storages []adminStorageDTO
	if err := json.Unmarshal(list.Body.Bytes(), &storages); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for _, s := range storages {
		if s.Name == "local" && s.Node == "pve-node-02" && s.Enabled {
			t.Error("local@pve-node-02 should be disabled after toggle")
		}

		if s.Name == "local" && s.Node == "pve-node-01" && s.Enabled {
			t.Error("local@pve-node-01 should still be disabled (unaffected)")
		}
	}
}

// TestAdminStorages_NonAdminReturns403 — T014: non-admin gets 403.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminStorages_NonAdminReturns403(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	aliceCookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	rec := adminGet(t, handler, authHandler, aliceCookie, "/api/v1/admin/storages?cluster=default")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestAdminStorages_ToggleUnknownPairReturns404 — T014: unknown pair 404.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminStorages_ToggleUnknownPairReturns404(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/storages/toggle",
		`{"cluster":"default","name":"nope","node":"pve-node-01","enabled":true}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
