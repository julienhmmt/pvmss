//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"pvmss/server/internal/httpapi"
	"testing"
)

type adminToggleResp struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_Nodes_Unauthenticated(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	rec := adminGet(t, handler, authHandler, nil, "/api/v1/admin/nodes?cluster=default")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_NodeToggle_Unauthenticated(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	rec := adminPost(t, handler, authHandler, nil, "/api/v1/admin/nodes/toggle",
		`{"cluster":"default","name":"pve-node-01","enabled":false}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_NodeToggle_InvalidBody(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/nodes/toggle", "{bad json")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_NodeToggle_UnknownCluster(t *testing.T) {
	handler, authHandler := newMultiClusterAdminCatalogHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/nodes/toggle",
		`{"cluster":"nonexistent","name":"pve-node-01","enabled":false}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_Storages_Unauthenticated(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	rec := adminGet(t, handler, authHandler, nil, "/api/v1/admin/storages?cluster=default")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_StorageToggle_Unauthenticated(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	rec := adminPost(t, handler, authHandler, nil, "/api/v1/admin/storages/toggle",
		`{"cluster":"default","name":"local","node":"pve-node-01","enabled":false}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_StorageToggle_InvalidBody(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/storages/toggle", "{bad json")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_StorageToggle_UnknownCluster(t *testing.T) {
	handler, authHandler := newMultiClusterAdminCatalogHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/storages/toggle",
		`{"cluster":"nonexistent","name":"local","node":"pve-node-01","enabled":true}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_Bridges_Unauthenticated(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	rec := adminGet(t, handler, authHandler, nil, "/api/v1/admin/bridges?cluster=default")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_BridgeToggle_Unauthenticated(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	rec := adminPost(t, handler, authHandler, nil, "/api/v1/admin/bridges/toggle",
		`{"cluster":"default","node":"pve-node-01","name":"vmbr0","enabled":true}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_BridgeToggle_InvalidBody(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/bridges/toggle", "{bad json")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_BridgeToggle_UnknownCluster(t *testing.T) {
	handler, authHandler := newMultiClusterAdminCatalogHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/bridges/toggle",
		`{"cluster":"nonexistent","node":"pve-node-01","name":"vmbr0","enabled":true}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_ISOs_Unauthenticated(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	rec := adminGet(t, handler, authHandler, nil, "/api/v1/admin/isos?cluster=default")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_ISOToggle_Unauthenticated(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	rec := adminPost(t, handler, authHandler, nil, "/api/v1/admin/isos/toggle",
		`{"cluster":"default","node":"pve-node-01","storage":"local","file":"test.iso","enabled":true}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_ISOToggle_InvalidBody(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/isos/toggle", "{bad json")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_ISOToggle_UnknownCluster(t *testing.T) {
	handler, authHandler := newMultiClusterAdminCatalogHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/isos/toggle",
		`{"cluster":"nonexistent","node":"pve-node-01","storage":"local","file":"test.iso","enabled":true}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_NodeToggle_DisableApprovedNode(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/nodes/toggle",
		`{"cluster":"default","name":"pve-node-01","enabled":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	var resp adminToggleResp
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp.Name != "pve-node-01" || resp.Enabled {
		t.Fatalf("resp = %+v, want name=pve-node-01 enabled=false", resp)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_StorageToggle_EnableStorage(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/storages/toggle",
		`{"cluster":"default","name":"local","node":"pve-node-01","enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_BridgeToggle_EnableBridge(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/bridges/toggle",
		`{"cluster":"default","node":"pve-node-01","name":"vmbr0","enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_ISOToggle_EnableISO(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/isos/toggle",
		`{"cluster":"default","node":"pve-node-01","storage":"local","file":"debian-12-generic-amd64.iso","enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_Nodes_DefaultClusterParam(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/nodes")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_Storages_DefaultClusterParam(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/storages")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_Bridges_DefaultClusterParam(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/bridges")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_ISOs_DefaultClusterParam(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/isos")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_NodeToggle_UnknownFieldRejected(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/nodes/toggle",
		`{"cluster":"default","name":"pve-node-01","enabled":false,"extra":"field"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (unknown field rejected)", rec.Code, http.StatusBadRequest)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_StorageToggle_UnknownFieldRejected(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/storages/toggle",
		`{"cluster":"default","name":"local","node":"pve-node-01","enabled":true,"extra":"field"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (unknown field rejected)", rec.Code, http.StatusBadRequest)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_BridgeToggle_UnknownFieldRejected(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/bridges/toggle",
		`{"cluster":"default","node":"pve-node-01","name":"vmbr0","enabled":true,"extra":"field"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (unknown field rejected)", rec.Code, http.StatusBadRequest)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_ISOToggle_UnknownFieldRejected(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/isos/toggle",
		`{"cluster":"default","node":"pve-node-01","storage":"local","file":"test.iso","enabled":true,"extra":"field"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d (unknown field rejected)", rec.Code, http.StatusBadRequest)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_NilClientReturns404(t *testing.T) {
	authHandler := newAuthHandler(t)
	st := newAdminStore(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	handler := httpapi.NewAdminCatalog(authHandler, st, nil, nil, logger)
	cookie := adminCookie(t, authHandler)
	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/nodes?cluster=default")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_NilClientStorageToggleReturns404(t *testing.T) {
	authHandler := newAuthHandler(t)
	st := newAdminStore(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	handler := httpapi.NewAdminCatalog(authHandler, st, nil, nil, logger)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/storages/toggle",
		`{"cluster":"default","name":"local","node":"pve-node-01","enabled":true}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_NilClientBridgeToggleReturns404(t *testing.T) {
	authHandler := newAuthHandler(t)
	st := newAdminStore(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	handler := httpapi.NewAdminCatalog(authHandler, st, nil, nil, logger)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/bridges/toggle",
		`{"cluster":"default","node":"pve-node-01","name":"vmbr0","enabled":true}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_NilClientISOToggleReturns404(t *testing.T) {
	authHandler := newAuthHandler(t)
	st := newAdminStore(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	handler := httpapi.NewAdminCatalog(authHandler, st, nil, nil, logger)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/isos/toggle",
		`{"cluster":"default","node":"pve-node-01","storage":"local","file":"test.iso","enabled":true}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_StorageToggle_NodeNotFound(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/storages/toggle",
		`{"cluster":"default","name":"local","node":"pve-node-99","enabled":true}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_BridgeToggle_NodeNotFound(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/bridges/toggle",
		`{"cluster":"default","node":"pve-node-99","name":"vmbr0","enabled":true}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminCatalogCoverage_ISOToggle_NodeNotFound(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/isos/toggle",
		`{"cluster":"default","node":"pve-node-99","storage":"local","file":"test.iso","enabled":true}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}
