package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/httpapi"
	"testing"
)

// --- Profile DTOs for tests ---

type adminProfileDTO struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	CPUCores int    `json:"cpuCores"`
	MemoryMB int    `json:"memoryMB"`
	DiskGB   int    `json:"diskGB"`
	Bus      string `json:"bus"`
	Enabled  bool   `json:"enabled"`
}

type adminTagDTO struct {
	Name      string `json:"name"`
	Color     string `json:"color"`
	VMCount   int    `json:"vmCount"`
	Protected bool   `json:"protected"`
}

type statusDTO struct {
	Status string `json:"status"`
}

// =============================================================================
// Profile handler tests (T029)
// =============================================================================

// TestAdminProfiles_ListAsAdmin — GET /admin/profiles returns all profiles
// including the 3 seeded ones, all enabled.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminProfiles_ListAsAdmin(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/profiles?cluster=default")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var profiles []adminProfileDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &profiles); err != nil {
		t.Fatalf("decode profiles: %v", err)
	}

	if len(profiles) < 3 {
		t.Fatalf("expected at least 3 seeded profiles, got %d", len(profiles))
	}

	ids := make(map[string]bool)
	for _, p := range profiles {
		ids[p.ID] = true
		if !p.Enabled {
			t.Errorf("seeded profile %q should be enabled", p.ID)
		}
	}

	for _, expected := range []string{"small", "medium", "large"} {
		if !ids[expected] {
			t.Errorf("seeded profile %q missing", expected)
		}
	}
}

// TestAdminProfiles_ListAsNonAdmin_Returns403 — non-admin gets 403.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminProfiles_ListAsNonAdmin_Returns403(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	alice := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	rec := adminGet(t, handler, authHandler, alice, "/api/v1/admin/profiles?cluster=default")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestAdminProfiles_Create — POST creates a new profile, returns 201.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminProfiles_Create(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/profiles",
		`{"cluster":"default","label":"X-Large (8 vCPU, 16 GB, 160 GB)","cpuCores":8,"memoryMB":16384,"diskGB":160,"bus":"scsi"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var profile adminProfileDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}

	if profile.ID != "x-large-8-vcpu-16-gb-160-gb" {
		t.Errorf("profile.ID = %q, want %q", profile.ID, "x-large-8-vcpu-16-gb-160-gb")
	}

	if !profile.Enabled {
		t.Error("new profile should be enabled")
	}
}

// TestAdminProfiles_CreateDuplicate_Returns409 — slug collision returns 409.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminProfiles_CreateDuplicate_Returns409(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/profiles",
		`{"cluster":"default","label":"small","cpuCores":1,"memoryMB":2048,"diskGB":20,"bus":"scsi"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

// TestAdminProfiles_CreateInvalid_Returns400 — out-of-range fields return 400.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminProfiles_CreateInvalid_Returns400(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/profiles",
		`{"cluster":"default","label":"Bad","cpuCores":0,"memoryMB":2048,"diskGB":20,"bus":"scsi"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// TestAdminProfiles_Update — PUT updates an existing profile.
//
//nolint:paralleltest,dupl // serial: shared fake dataset; intentionally parallel to TestAdminTags_UpdateColor
func TestAdminProfiles_Update(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPut(t, handler, authHandler, cookie, "/api/v1/admin/profiles/small",
		`{"cluster":"default","label":"Small (2 vCPU, 4 GB, 40 GB)","cpuCores":2,"memoryMB":4096,"diskGB":40,"bus":"scsi"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var profile adminProfileDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &profile); err != nil {
		t.Fatalf("decode profile: %v", err)
	}

	if profile.CPUCores != 2 {
		t.Errorf("cpuCores = %d, want 2", profile.CPUCores)
	}
}

// TestAdminProfiles_UpdateNotFound_Returns404 — updating a non-existent
// profile returns 404.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminProfiles_UpdateNotFound_Returns404(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPut(t, handler, authHandler, cookie, "/api/v1/admin/profiles/nonexistent",
		`{"cluster":"default","label":"Test","cpuCores":1,"memoryMB":2048,"diskGB":20,"bus":"scsi"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestAdminProfiles_Delete — DELETE removes a profile.
//
//nolint:paralleltest,dupl // serial: shared fake dataset; intentionally parallel to TestAdminTags_DeleteSuccess
func TestAdminProfiles_Delete(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	// Create a profile to delete.
	adminPost(t, handler, authHandler, cookie, "/api/v1/admin/profiles",
		`{"cluster":"default","label":"Temp","cpuCores":1,"memoryMB":2048,"diskGB":20,"bus":"scsi"}`)

	rec := adminDelete(t, handler, authHandler, cookie, "/api/v1/admin/profiles/temp?cluster=default")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var status statusDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}

	if status.Status != "deleted" {
		t.Errorf("status = %q, want deleted", status.Status)
	}
}

// TestAdminProfiles_Toggle — disabling a profile and re-enabling it.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminProfiles_Toggle(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	// Disable.
	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/profiles/small/toggle",
		`{"cluster":"default","enabled":false}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle off: status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	// Verify it's disabled in the list.
	rec = adminGet(t, handler, authHandler, cookie, "/api/v1/admin/profiles?cluster=default")

	var profiles []adminProfileDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &profiles); err != nil {
		t.Fatalf("decode profiles: %v", err)
	}

	for _, p := range profiles {
		if p.ID == "small" && p.Enabled {
			t.Error("profile should be disabled after toggle")
		}
	}

	// Re-enable.
	rec = adminPost(t, handler, authHandler, cookie, "/api/v1/admin/profiles/small/toggle",
		`{"cluster":"default","enabled":true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("toggle on: status = %d, want %d", rec.Code, http.StatusOK)
	}
}

// TestAdminProfiles_NonAdminAll_Returns403 — every profile endpoint returns
// 403 for non-admin.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminProfiles_NonAdminAll_Returns403(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	alice := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	checks := []adminEndpointCheck{
		{http.MethodGet, "/api/v1/admin/profiles", ""},
		{http.MethodPost, "/api/v1/admin/profiles", `{"cluster":"default","label":"X","cpuCores":1,"memoryMB":2048,"diskGB":20,"bus":"scsi"}`},
		{http.MethodPut, "/api/v1/admin/profiles/small", `{"cluster":"default","label":"X","cpuCores":1,"memoryMB":2048,"diskGB":20,"bus":"scsi"}`},
		{http.MethodDelete, "/api/v1/admin/profiles/small?cluster=default", ""},
		{http.MethodPost, "/api/v1/admin/profiles/small/toggle", `{"cluster":"default","enabled":false}`},
	}
	assertAllAdminReturn403(t, handler, authHandler, alice, checks)
}

// =============================================================================
// Tag handler tests (T036)
// =============================================================================

// TestAdminTags_ListAsAdmin — GET /admin/tags returns the seeded pvmss tag
// with a live VM count > 0 and protected = true.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminTags_ListAsAdmin(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminGet(t, handler, authHandler, cookie, "/api/v1/admin/tags?cluster=default")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var tags []adminTagDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &tags); err != nil {
		t.Fatalf("decode tags: %v", err)
	}

	found := false

	for _, tag := range tags {
		if tag.Name == "pvmss" {
			found = true

			if !tag.Protected {
				t.Error("pvmss tag should be protected")
			}

			if tag.VMCount == 0 {
				t.Error("pvmss tag should have VM count > 0")
			}
		}
	}

	if !found {
		t.Error("pvmss tag not found in list")
	}
}

// TestAdminTags_ListAsNonAdmin_Returns403 — non-admin gets 403.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminTags_ListAsNonAdmin_Returns403(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	alice := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	rec := adminGet(t, handler, authHandler, alice, "/api/v1/admin/tags?cluster=default")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestAdminTags_Create — POST creates a new tag.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminTags_Create(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/tags",
		`{"cluster":"default","name":"teamweb","color":"#16a34a"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var tag adminTagDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &tag); err != nil {
		t.Fatalf("decode tag: %v", err)
	}

	if tag.Name != "teamweb" {
		t.Errorf("tag.Name = %q, want teamweb", tag.Name)
	}

	if tag.Protected {
		t.Error("new tag should not be protected")
	}
}

// TestAdminTags_CreateDuplicate_Returns409 — duplicate name returns 409.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminTags_CreateDuplicate_Returns409(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/tags",
		`{"cluster":"default","name":"pvmss","color":"#000000"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusConflict)
	}
}

// TestAdminTags_CreateInvalidName_Returns400 — non-alphanumeric name returns
// 400.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminTags_CreateInvalidName_Returns400(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPost(t, handler, authHandler, cookie, "/api/v1/admin/tags",
		`{"cluster":"default","name":"team web","color":"#000000"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
}

// TestAdminTags_UpdateColor — PUT changes the color, even for pvmss.
//
//nolint:paralleltest,dupl // serial: shared fake dataset; intentionally parallel to TestAdminProfiles_Update
func TestAdminTags_UpdateColor(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminPut(t, handler, authHandler, cookie, "/api/v1/admin/tags/pvmss/color",
		`{"cluster":"default","color":"#dc2626"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var tag adminTagDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &tag); err != nil {
		t.Fatalf("decode tag: %v", err)
	}

	if tag.Color != "#dc2626" {
		t.Errorf("tag.Color = %q, want #dc2626", tag.Color)
	}
}

// TestAdminTags_DeletePvmss_Returns403 — deleting pvmss returns 403.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminTags_DeletePvmss_Returns403(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	rec := adminDelete(t, handler, authHandler, cookie, "/api/v1/admin/tags/pvmss?cluster=default")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

// TestAdminTags_DeleteSuccess — deleting a non-pvmss tag succeeds.
//
//nolint:paralleltest,dupl // serial: shared fake dataset; intentionally parallel to TestAdminProfiles_Delete
func TestAdminTags_DeleteSuccess(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	cookie := adminCookie(t, authHandler)

	// Create a tag to delete.
	adminPost(t, handler, authHandler, cookie, "/api/v1/admin/tags",
		`{"cluster":"default","name":"temp","color":"#000000"}`)

	rec := adminDelete(t, handler, authHandler, cookie, "/api/v1/admin/tags/temp?cluster=default")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var status statusDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode status: %v", err)
	}

	if status.Status != "deleted" {
		t.Errorf("status = %q, want deleted", status.Status)
	}
}

// TestAdminTags_NonAdminAll_Returns403 — every tag endpoint returns 403 for
// non-admin.
//
//nolint:paralleltest // serial: shared fake dataset and database fixture
func TestAdminTags_NonAdminAll_Returns403(t *testing.T) {
	handler, authHandler, _ := newAdminHandler(t)
	alice := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	checks := []adminEndpointCheck{
		{http.MethodGet, "/api/v1/admin/tags", ""},
		{http.MethodPost, "/api/v1/admin/tags", `{"cluster":"default","name":"x","color":"#000"}`},
		{http.MethodPut, "/api/v1/admin/tags/pvmss/color", `{"cluster":"default","color":"#000"}`},
		{http.MethodDelete, "/api/v1/admin/tags/pvmss?cluster=default", ""},
	}
	assertAllAdminReturn403(t, handler, authHandler, alice, checks)
}

// adminEndpointCheck describes a single admin endpoint invocation for the 403
// test matrix.
type adminEndpointCheck struct {
	method string
	path   string
	body   string
}

// assertAllAdminReturn403 dispatches each check through the matching admin
// helper and asserts every response is 403 Forbidden. Shared by the profile
// and tag non-admin test matrices.
func assertAllAdminReturn403(
	t *testing.T,
	handler *httpapi.AdminCatalog,
	authHandler *httpapi.Auth,
	cookie *http.Cookie,
	checks []adminEndpointCheck,
) {
	t.Helper()

	for _, c := range checks {
		var rec *httptest.ResponseRecorder

		switch c.method {
		case http.MethodGet:
			rec = adminGet(t, handler, authHandler, cookie, c.path)
		case http.MethodPost:
			rec = adminPost(t, handler, authHandler, cookie, c.path, c.body)
		case http.MethodPut:
			rec = adminPut(t, handler, authHandler, cookie, c.path, c.body)
		case http.MethodDelete:
			rec = adminDelete(t, handler, authHandler, cookie, c.path)
		}

		if rec.Code != http.StatusForbidden {
			t.Errorf("%s %s: status = %d, want %d", c.method, c.path, rec.Code, http.StatusForbidden)
		}
	}
}
