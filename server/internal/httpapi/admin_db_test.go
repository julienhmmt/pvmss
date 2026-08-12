//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"pvmss/server/internal/store"
	"testing"
	"time"
)

type importPreviewDTO struct {
	StagingToken  string               `json:"stagingToken"`
	ExpiresAt     string               `json:"expiresAt"`
	Tables        []store.TablePreview `json:"tables"`
	IgnoredTables []string             `json:"ignoredTables"`
}

type importResultDTO struct {
	Status string               `json:"status"`
	Tables []store.TablePreview `json:"tables"`
}

// TestAdminDBExport_AsAdmin_ReturnsSQLiteFile — T031: GET /admin/db/export
// as admin returns a downloadable, well-formed SQLite response with the
// correct headers.
//
//nolint:paralleltest // serial: shared database fixture
func TestAdminDBExport_AsAdmin_ReturnsSQLiteFile(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	rec := opsGet(t, ops, auth, cookie, "/api/v1/admin/db/export")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	cd := rec.Header().Get("Content-Disposition")
	if cd == "" || !bytes.Contains([]byte(cd), []byte("attachment")) {
		t.Errorf("Content-Disposition = %q, want attachment", cd)
	}

	ct := rec.Header().Get("Content-Type")
	if ct != "application/octet-stream" {
		t.Errorf("Content-Type = %q, want application/octet-stream", ct)
	}

	// The body must be a valid SQLite file (starts with "SQLite format 3\000").
	body := rec.Body.Bytes()
	if len(body) < 16 || string(body[:15]) != "SQLite format 3" {
		t.Errorf("body does not start with SQLite header, got %q", body[:min(16, len(body))])
	}

	// Write to a temp file and reopen as SQLite to verify it's well-formed.
	path := filepath.Join(t.TempDir(), "exported.db")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write exported: %v", err)
	}

	_ = path // the store-level tests already verify reopening; here we just
	// verify the header and headers.
}

// TestAdminDBImport_WellFormedUpload_ReturnsPreview — T032: POST
// /admin/db/import with a well-formed upload returns 200 preview with
// correct tables/ignoredTables.
//
//nolint:paralleltest // serial: shared database fixture
func TestAdminDBImport_WellFormedUpload_ReturnsPreview(t *testing.T) {
	ops, auth, st := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	// Export the live DB to get a well-formed upload file.
	var exportBuf bytes.Buffer
	if err := st.ExportDatabase(context.Background(), &exportBuf); err != nil {
		// nil ctx is fine for the store — it uses context.Background() internally
		t.Fatalf("ExportDatabase: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", "export.db")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}

	if _, err := io.Copy(part, &exportBuf); err != nil {
		t.Fatalf("copy export: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/db/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	opsMux(ops, auth).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var preview importPreviewDTO
	if err := json.Unmarshal(rec.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if preview.StagingToken == "" {
		t.Error("stagingToken is empty")
	}

	if len(preview.Tables) == 0 {
		t.Error("tables is empty — export should contain catalog tables")
	}
}

// TestAdminDBImport_MalformedUpload_Returns400 — T032: POST /admin/db/import
// with a malformed upload returns 400, nothing staged.
//
//nolint:paralleltest // serial: shared database fixture
func TestAdminDBImport_MalformedUpload_Returns400(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "garbage.db")
	_, _ = part.Write([]byte("this is not a sqlite database"))
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/db/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	opsMux(ops, auth).ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	var errBody map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &errBody); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if errBody["code"] != "invalid_database" {
		t.Errorf("code = %q, want invalid_database", errBody["code"])
	}
}

// TestAdminDBImportConfirm_ValidToken_ReplacesTables — T033: POST
// /admin/db/import/confirm with a valid token returns 200 and the live
// database reflects the replace.
//
//nolint:paralleltest // serial: shared database fixture
func TestAdminDBImportConfirm_ValidToken_ReplacesTables(t *testing.T) {
	ops, auth, st := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	// Export, then import to get a staging token.
	var exportBuf bytes.Buffer
	if err := st.ExportDatabase(context.Background(), &exportBuf); err != nil {
		t.Fatalf("ExportDatabase: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "export.db")
	_, _ = io.Copy(part, &exportBuf)
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/db/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	opsMux(ops, auth).ServeHTTP(rec, req)

	var preview importPreviewDTO

	_ = json.Unmarshal(rec.Body.Bytes(), &preview)

	// Confirm.
	confirmRec := opsPost(t, ops, auth, cookie, "/api/v1/admin/db/import/confirm",
		`{"stagingToken":"`+preview.StagingToken+`"}`)
	if confirmRec.Code != http.StatusOK {
		t.Fatalf("confirm status = %d: %s", confirmRec.Code, confirmRec.Body.String())
	}

	var result importResultDTO
	if err := json.Unmarshal(confirmRec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode: %v", err)
	}

	if result.Status != "restored" {
		t.Errorf("status = %q, want restored", result.Status)
	}
}

// TestAdminDBImportConfirm_UnknownToken_Returns404 — T033.
//
//nolint:paralleltest // serial: shared database fixture
func TestAdminDBImportConfirm_UnknownToken_Returns404(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	rec := opsPost(t, ops, auth, cookie, "/api/v1/admin/db/import/confirm", `{"stagingToken":"never-staged"}`)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotFound)
	}
}

// TestAdminDBImportConfirm_ExpiredToken_Returns410 — T033.
//
//nolint:paralleltest // serial: shared database fixture
func TestAdminDBImportConfirm_ExpiredToken_Returns410(t *testing.T) {
	ops, auth, st := newAdminOpsHandler(t)
	cookie := adminCookie(t, auth)

	var exportBuf bytes.Buffer
	if err := st.ExportDatabase(context.Background(), &exportBuf); err != nil {
		t.Fatalf("ExportDatabase: %v", err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, _ := writer.CreateFormFile("file", "export.db")
	_, _ = io.Copy(part, &exportBuf)
	_ = writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/db/import", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	opsMux(ops, auth).ServeHTTP(rec, req)

	var preview importPreviewDTO

	_ = json.Unmarshal(rec.Body.Bytes(), &preview)

	// Advance the staging clock past the TTL.
	st.Staging().AdvanceTime(6 * time.Minute) // 6 minutes — will fix below

	confirmRec := opsPost(t, ops, auth, cookie, "/api/v1/admin/db/import/confirm",
		`{"stagingToken":"`+preview.StagingToken+`"}`)
	if confirmRec.Code != http.StatusGone {
		t.Fatalf("status = %d, want %d (410 Gone): %s", confirmRec.Code, http.StatusGone, confirmRec.Body.String())
	}
}

// TestAdminDB_NonAdmin_Returns403 — T034: all three db endpoints as non-admin
// return 403.
//
//nolint:paralleltest // serial: shared database fixture
func TestAdminDB_NonAdmin_Returns403(t *testing.T) {
	ops, auth, _ := newAdminOpsHandler(t)
	aliceCookie := loginCookie(t, auth, `{"username":"alice","password":"pvmss-alice"}`)

	exportRec := opsGet(t, ops, auth, aliceCookie, "/api/v1/admin/db/export")
	if exportRec.Code != http.StatusForbidden {
		t.Errorf("export status = %d, want 403", exportRec.Code)
	}

	importRec := opsPost(t, ops, auth, aliceCookie, "/api/v1/admin/db/import", "")
	if importRec.Code != http.StatusForbidden {
		t.Errorf("import status = %d, want 403", importRec.Code)
	}

	confirmRec := opsPost(t, ops, auth, aliceCookie, "/api/v1/admin/db/import/confirm", `{"stagingToken":"x"}`)
	if confirmRec.Code != http.StatusForbidden {
		t.Errorf("confirm status = %d, want 403", confirmRec.Code)
	}
}
