package httpapi_test

import (
	"encoding/json"
	"net/http"
	"testing"
)

type adminNodeDTO struct {
	Name         string  `json:"name"`
	Status       string  `json:"status"`
	CPUCores     int     `json:"cpuCores"`
	CPUUsage     float64 `json:"cpuUsage"`
	MemoryTotal  int64   `json:"memoryTotal"`
	MemoryUsed   int64   `json:"memoryUsed"`
	StorageTotal int64   `json:"storageTotal"`
	StorageUsed  int64   `json:"storageUsed"`
	VMCount      int     `json:"vmCount"`
	Enabled      bool    `json:"enabled"`
}

type adminToggleResponse struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

// TestAdminNodes_ListAsAdmin_ReturnsAllNodes — T010: GET /admin/nodes as admin
// returns every fake node (3), with correct enabled per T06's seed.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminNodes_ListAsAdmin_ReturnsAllNodes(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/nodes?cluster=default")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var nodes []adminNodeDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &nodes); err != nil {
		t.Fatalf("decode nodes: %v", err)
	}

	if len(nodes) != 3 {
		t.Fatalf("expected 3 nodes, got %d", len(nodes))
	}

	enabledByName := make(map[string]bool, len(nodes))
	for _, n := range nodes {
		enabledByName[n.Name] = n.Enabled
	}

	if !enabledByName["pve-node-01"] {
		t.Error("pve-node-01 should be enabled")
	}

	if !enabledByName["pve-node-02"] {
		t.Error("pve-node-02 should be enabled")
	}

	if enabledByName["pve-node-03"] {
		t.Error("pve-node-03 should not be enabled")
	}
}

// TestAdminNodes_ListAsNonAdmin_Returns403 — T011: GET /admin/nodes as a
// non-admin identity returns 403 (FR-008).
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminNodes_ListAsNonAdmin_Returns403(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	aliceCookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	rec := adminGet(t, handler, authHandler, aliceCookie, "/api/v1/admin/nodes?cluster=default")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestAdminNodes_ToggleUnapprovedNode — T012: POST /admin/nodes/toggle on the
// unapproved node returns 200, and a subsequent GET reflects it.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminNodes_ToggleUnapprovedNode(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/nodes/toggle",
		`{"cluster":"default","name":"pve-node-03","enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var toggleResp adminToggleResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &toggleResp); err != nil {
		t.Fatalf("decode toggle response: %v", err)
	}

	if toggleResp.Name != "pve-node-03" || !toggleResp.Enabled {
		t.Fatalf("toggle response = %+v", toggleResp)
	}

	// Subsequent GET reflects the change.
	list := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/nodes?cluster=default")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d", list.Code)
	}

	var nodes []adminNodeDTO
	if err := json.Unmarshal(list.Body.Bytes(), &nodes); err != nil {
		t.Fatalf("decode nodes: %v", err)
	}

	for _, n := range nodes {
		if n.Name == "pve-node-03" && !n.Enabled {
			t.Error("pve-node-03 should be enabled after toggle")
		}
	}
}

// TestAdminNodes_ToggleUnknownNode_Returns404 — T013: toggling a node not in
// the discovery set returns 404.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminNodes_ToggleUnknownNode_Returns404(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/nodes/toggle",
		`{"cluster":"default","name":"pve-node-99","enabled":true}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}
