package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"pvmss/server/internal/catalog"
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
// fake template discovery set, all disabled (the schema ships no approval
// rows; the admin list is the union of discovery + stored state).
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

	// Template 9001 has no stored row yet. Toggle it to disabled — the row
	// must be inserted with enabled=false.
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

// TestAdminTemplates_ListDiscoveryWinsOnValues — when a stored row's values
// drift from discovery (template resized/migrated/renamed in Proxmox after
// approval), the list shows the discovered values and the stored row is
// reconciled (issue 02). The stored enabled flag stays authoritative.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminTemplates_ListDiscoveryWinsOnValues(t *testing.T) {
	handler, authHandler, st := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	// Toggle 9000 on to create a stored row with discovered values, then
	// update the stored row directly to diverge from discovery.
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/templates/toggle",
		`{"cluster":"default","vmid":9000,"enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle status = %d: %s", rec.Code, rec.Body.String())
	}

	// Override the stored row's values so they drift from discovery (fake
	// reports local-lvm / 8 GB / scsi for 9000).
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
			assertDiscoveredTemplateValues(t, tmpl)
			return
		}
	}

	t.Fatal("template 9000 not in list")
}

// assertDiscoveredTemplateValues verifies that template 9000's list entry
// carries the fake's discovered values, not the drifted stored ones.
func assertDiscoveredTemplateValues(t *testing.T, tmpl adminTemplateDTO) {
	t.Helper()

	if tmpl.DiskStorage != "local-lvm" {
		t.Errorf("template 9000 diskStorage = %q, want %q (discovery must win on values)", tmpl.DiskStorage, "local-lvm")
	}

	if tmpl.DiskSizeGB != 8 {
		t.Errorf("template 9000 diskSizeGB = %d, want 8 (discovered value)", tmpl.DiskSizeGB)
	}

	if tmpl.DiskBus != string(cluster.DiskBusSCSI) {
		t.Errorf("template 9000 diskBus = %q, want %s (discovered value)", tmpl.DiskBus, cluster.DiskBusSCSI)
	}

	if !tmpl.Enabled {
		t.Error("template 9000 enabled must come from the stored row (true)")
	}
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

// TestAdminTemplates_DeleteRemovesOrphanApproval — DELETE
// /admin/templates/{cluster}/{vmid} removes the approval row (204); the row
// disappears from catalog.Templates; an unknown vmid or cluster is a 404;
// non-admins get 403.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminTemplates_DeleteRemovesOrphanApproval(t *testing.T) {
	handler, authHandler, st := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)
	ctx := context.Background()

	// Ship an approval for 9000, then delete it.
	if err := st.InsertTemplate(ctx, "default", 9000, store.TemplateValues{
		Node: cluster.FakeNode02, Name: "debian-12-cloud", CloudInitCapable: true,
		DiskStorage: "local-lvm", DiskSizeGB: 8, DiskBus: string(cluster.DiskBusSCSI),
	}, true); err != nil {
		t.Fatalf("InsertTemplate: %v", err)
	}

	rec := adminDelete(t, handler, authHandler, cookie, "/api/v1/admin/templates/default/9000")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d: %s", rec.Code, rec.Body.String())
	}

	templates, err := catalog.Templates(ctx, st, "default")
	if err != nil {
		t.Fatalf("catalog.Templates: %v", err)
	}

	for _, tmpl := range templates {
		if tmpl.VMID == 9000 {
			t.Error("template 9000 should be gone from the catalog after delete")
		}
	}

	// Unknown vmid → 404.
	rec = adminDelete(t, handler, authHandler, cookie, "/api/v1/admin/templates/default/99999")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown vmid status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	// Unknown cluster → 404 (no rows to delete there).
	rec = adminDelete(t, handler, authHandler, cookie, "/api/v1/admin/templates/no-such-cluster/9001")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown cluster status = %d, want %d", rec.Code, http.StatusNotFound)
	}

	// Non-admin → 403.
	aliceCookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	rec = adminDelete(t, handler, authHandler, aliceCookie, "/api/v1/admin/templates/default/9001")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("non-admin status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// unreadableTemplateHTTPClient serves one unreadable template (issue 03).
type unreadableTemplateHTTPClient struct {
	cluster.Fake
}

func (unreadableTemplateHTTPClient) TemplateByVMID(_ context.Context, vmid int) (cluster.TemplateVM, error) {
	return cluster.TemplateVM{VMID: vmid, Node: cluster.FakeNode02, Name: "unreadable", DiskUnreadable: true}, nil
}

// TestAdminTemplates_ToggleUnreadable — approving an unreadable template is a
// 400 (the row would carry empty disk fields); disabling stays possible.
//
//nolint:paralleltest // serial: database-backed handler fixture
func TestAdminTemplates_ToggleUnreadable(t *testing.T) {
	authHandler := newAuthHandler(t)
	st := newAdminStore(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	handler := httpapi.NewAdminCatalog(authHandler, st, unreadableTemplateHTTPClient{}, nil, logger)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/templates/toggle",
		`{"cluster":"default","vmid":9000,"enabled":true}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("approve status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	rec = adminPost(t, handler, authHandler, cookie, "/api/v1/admin/templates/toggle",
		`{"cluster":"default","vmid":9000,"enabled":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("disable status = %d, want %d", rec.Code, http.StatusOK)
	}
}
