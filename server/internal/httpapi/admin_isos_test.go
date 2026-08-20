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

type adminISODTO struct {
	Storage   string `json:"storage"`
	Node      string `json:"node"`
	File      string `json:"file"`
	SizeBytes int64  `json:"sizeBytes"`
	Enabled   bool   `json:"enabled"`
}

const (
	duplicateISOFile    = "shared.iso"
	duplicateISOStorage = "local"
	duplicateISONodeA   = "node-a"
	duplicateISONodeB   = "node-b"
)

type duplicateISOHTTPClient struct {
	cluster.Fake
}

func (duplicateISOHTTPClient) ListISOs(_ context.Context) ([]cluster.ISOImage, error) {
	return []cluster.ISOImage{
		{Storage: duplicateISOStorage, Node: duplicateISONodeA, File: duplicateISOFile, SizeBytes: 1024},
		{Storage: duplicateISOStorage, Node: duplicateISONodeB, File: duplicateISOFile, SizeBytes: 1024},
	}, nil
}

func newDuplicateISOAdminHandler(t *testing.T) (*httpapi.AdminCatalog, *httpapi.Auth) {
	t.Helper()

	authHandler := newAuthHandler(t)
	st := newAdminStore(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	handler := httpapi.NewAdminCatalog(authHandler, st, duplicateISOHTTPClient{}, nil, logger)

	return handler, authHandler
}

// TestAdminISOs_ListShowsSuperset — T016: GET /admin/isos shows the fake
// superset after the node-aware migration resets approvals.
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

	for _, i := range isos {
		if i.Enabled {
			t.Errorf("ISO %q on %q should not be enabled", i.File, i.Node)
		}
	}
}

// TestAdminISOs_Toggle — T016: toggle rocky-9 on, confirm it sticks.
//
//nolint:paralleltest,dupl // serial: shared fake dataset; intentionally parallel to TestAdminBridges_Toggle
func TestAdminISOs_Toggle(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/isos/toggle",
		`{"cluster":"default","node":"pve-node-02","storage":"local","file":"rocky-9-generic-x86_64.iso","enabled":true}`)
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
		`{"cluster":"default","node":"pve-node-01","storage":"local","file":"nope.iso","enabled":true}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestAdminISOs_ToggleMissingNodeReturns400 — the node field is now part of the
// ISO identity; omitting it is a 400, not a silent toggle of an arbitrary row.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminISOs_ToggleMissingNodeReturns400(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/isos/toggle",
		`{"cluster":"default","storage":"local","file":"debian-12-generic-amd64.iso","enabled":true}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// TestAdminISOs_SameFileOnTwoNodesTogglesIndependently — the same ISO file on
// the same storage name across two nodes must toggle independently. Before the
// node-aware identity fix, the (cluster, storage, file) PK collapsed these two
// rows into one and the toggle could not be attributed to a single node.
//
//nolint:paralleltest // serial: database-backed handler fixture
func TestAdminISOs_SameFileOnTwoNodesTogglesIndependently(t *testing.T) {
	handler, authHandler := newDuplicateISOAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	body := fmt.Sprintf(`{"cluster":"default","node":%q,"storage":%q,"file":%q,"enabled":true}`, duplicateISONodeA, duplicateISOStorage, duplicateISOFile)

	toggle := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/isos/toggle", body)
	if toggle.Code != http.StatusOK {
		t.Fatalf("toggle status = %d: %s", toggle.Code, toggle.Body.String())
	}

	var toggled adminISODTO
	if err := json.Unmarshal(toggle.Body.Bytes(), &toggled); err != nil {
		t.Fatalf("decode toggle response: %v", err)
	}

	if toggled.Node != duplicateISONodeA || toggled.Storage != duplicateISOStorage || toggled.File != duplicateISOFile || !toggled.Enabled {
		t.Fatalf("toggle response = %+v, want enabled %s on %s", toggled, duplicateISOFile, duplicateISONodeA)
	}

	list := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/isos?cluster=default")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d: %s", list.Code, list.Body.String())
	}

	var isos []adminISODTO
	if err := json.Unmarshal(list.Body.Bytes(), &isos); err != nil {
		t.Fatalf("decode isos: %v", err)
	}

	if len(isos) != 2 {
		t.Fatalf("iso count = %d, want 2", len(isos))
	}

	for _, iso := range isos {
		wantEnabled := iso.Node == duplicateISONodeA
		if iso.Enabled != wantEnabled {
			t.Errorf("%s on %s enabled = %v, want %v", iso.File, iso.Node, iso.Enabled, wantEnabled)
		}
	}
}
