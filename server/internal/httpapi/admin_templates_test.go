package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/store"
	"testing"
)

type adminTemplateDTO struct {
	VMID             int    `json:"vmid"`
	Node             string `json:"node"`
	Name             string `json:"name"`
	CloudInitCapable bool   `json:"cloudInitCapable"`
	DiskStorage      string `json:"diskStorage"`
	DiskSizeGB       int    `json:"diskSizeGB"`
	DiskBus          string `json:"diskBus"`
	Enabled          bool   `json:"enabled"`
}

// TestAdminTemplates_ListShowsDiscovered — GET /admin/templates returns the
// fake template discovery set, all disabled (no migration-seeded rows match
// the discovered VMIDs because the V22 seed uses the same VMIDs but the
// admin list is union of discovery + stored state).
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminTemplates_ListShowsDiscovered(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/templates?cluster=default")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var templates []adminTemplateDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &templates); err != nil {
		t.Fatalf("decode templates: %v", err)
	}

	if len(templates) != 2 {
		t.Fatalf("expected 2 templates, got %d", len(templates))
	}

	for _, tmpl := range templates {
		if tmpl.VMID == 0 || tmpl.Node == "" {
			t.Errorf("template %+v has empty vmid or node", tmpl)
		}
	}
}

// TestAdminTemplates_Toggle — toggle a discovered template on, confirm it
// sticks in the list.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminTemplates_Toggle(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/templates/toggle",
		`{"cluster":"default","vmid":9000,"enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle status = %d: %s", rec.Code, rec.Body.String())
	}

	var resp adminTemplateDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode toggle response: %v", err)
	}

	if resp.VMID != 9000 || !resp.Enabled {
		t.Fatalf("toggle response = %+v, want vmid=9000 enabled=true", resp)
	}

	list := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/templates?cluster=default")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d", list.Code)
	}

	var templates []adminTemplateDTO
	if err := json.Unmarshal(list.Body.Bytes(), &templates); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for _, tmpl := range templates {
		if tmpl.VMID == 9000 && !tmpl.Enabled {
			t.Error("template 9000 should be enabled after toggle")
		}
	}
}

// TestAdminTemplates_ToggleOffOnFirstApproval — toggling a discovered template
// to disabled when it has no stored row must insert the row with enabled=false,
// not the hardcoded enabled=1 the original InsertTemplate used (US2/issue-02).
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminTemplates_ToggleOffOnFirstApproval(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	// Template 9001 has no stored row (only 9000 is seeded by V22). Toggle
	// it to disabled — the row must be inserted with enabled=false.
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/templates/toggle",
		`{"cluster":"default","vmid":9001,"enabled":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle status = %d: %s", rec.Code, rec.Body.String())
	}

	var resp adminTemplateDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode toggle response: %v", err)
	}

	if resp.VMID != 9001 || resp.Enabled {
		t.Fatalf("toggle response = %+v, want vmid=9001 enabled=false", resp)
	}

	// The list must reflect the stored disabled state, not a hardcoded
	// enabled=true from a broken insert.
	list := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/templates?cluster=default")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d", list.Code)
	}

	var templates []adminTemplateDTO
	if err := json.Unmarshal(list.Body.Bytes(), &templates); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for _, tmpl := range templates {
		if tmpl.VMID == 9001 && tmpl.Enabled {
			t.Error("template 9001 should be disabled after toggle-off (InsertTemplate must honor enabled=false)")
		}
	}
}

// TestAdminTemplates_ListPrefersStoredValues — when a stored row exists, the
// admin list must show the stored field values (authoritative), not the
// discovered values. This keeps the admin view consistent with the clone path
// (catalog.Templates reads the same stored rows). The fake's discovered
// template 9000 is cloud-init capable on local-lvm; the test overrides the
// stored row to a different storage and verifies the list reflects it.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminTemplates_ListPrefersStoredValues(t *testing.T) {
	handler, authHandler, st := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	// Toggle 9000 on to create a stored row with discovered values, then
	// update the stored row directly to diverge from discovery.
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/templates/toggle",
		`{"cluster":"default","vmid":9000,"enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle status = %d: %s", rec.Code, rec.Body.String())
	}

	// Override the stored row's disk_storage to a value that does not match
	// discovery (fake reports local-lvm for 9000).
	if err := st.UpdateTemplate(context.Background(), "default", 9000, store.TemplateValues{
		Node: cluster.FakeNode02, Name: "debian-12-cloud", CloudInitCapable: true,
		DiskStorage: "overridden-storage", DiskSizeGB: 99, DiskBus: string(cluster.DiskBusVirtio),
	}); err != nil {
		t.Fatalf("UpdateTemplate: %v", err)
	}

	list := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/templates?cluster=default")
	if list.Code != http.StatusOK {
		t.Fatalf("list status = %d", list.Code)
	}

	var templates []adminTemplateDTO
	if err := json.Unmarshal(list.Body.Bytes(), &templates); err != nil {
		t.Fatalf("decode: %v", err)
	}

	for _, tmpl := range templates {
		if tmpl.VMID == 9000 {
			if tmpl.DiskStorage != "overridden-storage" {
				t.Errorf("template 9000 diskStorage = %q, want %q (stored value must override discovery)", tmpl.DiskStorage, "overridden-storage")
			}

			if tmpl.DiskSizeGB != 99 {
				t.Errorf("template 9000 diskSizeGB = %d, want 99 (stored value)", tmpl.DiskSizeGB)
			}

			if tmpl.DiskBus != string(cluster.DiskBusVirtio) {
				t.Errorf("template 9000 diskBus = %q, want %s (stored value)", tmpl.DiskBus, cluster.DiskBusVirtio)
			}

			return
		}
	}

	t.Fatal("template 9000 not in list")
}

// TestAdminTemplates_NonAdminReturns403 — non-admin gets 403.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminTemplates_NonAdminReturns403(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	aliceCookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	rec := adminGet(t, handler, authHandler, aliceCookie, "/api/v1/admin/templates?cluster=default")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestAdminTemplates_ToggleUnknownReturns404 — toggling a VMID not in the
// discovery set returns 404.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminTemplates_ToggleUnknownReturns404(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/templates/toggle",
		`{"cluster":"default","vmid":99999,"enabled":true}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestAdminTemplates_ToggleMissingVMIDReturns400 — a zero VMID is a 400.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminTemplates_ToggleMissingVMIDReturns400(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/templates/toggle",
		`{"cluster":"default","vmid":0,"enabled":true}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// unreachableTemplateClient wraps Fake but makes ListTemplates return
// ErrUnreachable, simulating a cluster that cannot be reached.
type unreachableTemplateClient struct {
	cluster.Fake
}

func (unreachableTemplateClient) ListTemplates(_ context.Context) ([]cluster.TemplateVM, error) {
	return nil, cluster.ErrUnreachable
}

// TestAdminTemplates_ListClusterUnreachableReturns500 — when the cluster
// client fails discovery, the list endpoint returns 500.
//
//nolint:paralleltest // serial: database-backed handler fixture
func TestAdminTemplates_ListClusterUnreachableReturns500(t *testing.T) {
	authHandler := newAuthHandler(t)
	st := newAdminStore(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	handler := httpapi.NewAdminCatalog(authHandler, st, unreachableTemplateClient{}, nil, logger)
	cookie := adminCookie(t, authHandler)

	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/templates?cluster=default")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
}
