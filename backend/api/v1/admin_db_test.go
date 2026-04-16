package apiv1_test

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiv1 "pvmss/api/v1"
	"pvmss/database"
	"pvmss/state"
)

// newAdminDBTestHandlerAndDB builds an AdminDBHandler backed by a real in-memory
// DB with bootstrap completed.  Handlers are called directly (bypassing JWT
// middleware) so no auth context injection is required; usernameFromCtx
// gracefully returns "" when the context key is absent.
func newAdminDBTestHandlerAndDB(t *testing.T) (*apiv1.AdminDBHandler, database.DB) {
	t.Helper()
	db, err := database.OpenMemory()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.CompleteBootstrap("test"))

	sm := state.MakeAppStateWithDB(db)
	require.NoError(t, sm.LoadSettingsFromDB())
	return apiv1.MakeAdminDBHandler(sm, db), db
}

// multipartFile builds a multipart/form-data body with a single file field
// whose content is read from srcPath.  Returns the body buffer and content-type.
func multipartFile(t *testing.T, fieldName, srcPath string) (*bytes.Buffer, string) {
	t.Helper()
	data, err := os.ReadFile(srcPath) //nolint:gosec
	require.NoError(t, err)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile(fieldName, "upload.db")
	require.NoError(t, err)
	_, err = fw.Write(data)
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	return &buf, mw.FormDataContentType()
}

// ── ListAuditLog ──────────────────────────────────────────────────────────────

func TestListAuditLog_EmptyReturnsEmptySlice(t *testing.T) {
	h, _ := newAdminDBTestHandlerAndDB(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit", nil)
	w := httptest.NewRecorder()
	h.ListAuditLog(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp apiv1.AuditLogResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotNil(t, resp.Entries)
	assert.Equal(t, 50, resp.Limit)
	assert.Equal(t, 0, resp.Offset)
}

func TestListAuditLog_AfterWrite_ReturnsEntry(t *testing.T) {
	h, db := newAdminDBTestHandlerAndDB(t)
	require.NoError(t, db.SetTags([]string{"test-tag"}, "admin"))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit", nil)
	w := httptest.NewRecorder()
	h.ListAuditLog(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp apiv1.AuditLogResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp.Entries)
}

func TestListAuditLog_CustomLimitOffset(t *testing.T) {
	h, _ := newAdminDBTestHandlerAndDB(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit?limit=10&offset=5", nil)
	w := httptest.NewRecorder()
	h.ListAuditLog(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp apiv1.AuditLogResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 10, resp.Limit)
	assert.Equal(t, 5, resp.Offset)
}

func TestListAuditLog_LimitExceedingMax_ClampedTo50(t *testing.T) {
	h, _ := newAdminDBTestHandlerAndDB(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/audit?limit=9999", nil)
	w := httptest.NewRecorder()
	h.ListAuditLog(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp apiv1.AuditLogResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, 50, resp.Limit)
}

// ── ExportDB ──────────────────────────────────────────────────────────────────

func TestExportDB_ReturnsOctetStream(t *testing.T) {
	h, _ := newAdminDBTestHandlerAndDB(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/db/export", nil)
	w := httptest.NewRecorder()
	h.ExportDB(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/octet-stream", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "attachment")
	assert.Greater(t, w.Body.Len(), 0)
}

// ── ImportDB ──────────────────────────────────────────────────────────────────

func TestImportDB_ValidDB_Succeeds(t *testing.T) {
	h, srcDB := newAdminDBTestHandlerAndDB(t)

	backupPath := t.TempDir() + "/backup.db"
	require.NoError(t, srcDB.Backup(backupPath))

	body, ct := multipartFile(t, "db", backupPath)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/db/import", body)
	req.Header.Set("Content-Type", ct)
	w := httptest.NewRecorder()
	h.ImportDB(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "database imported successfully", resp["message"])
}

func TestImportDB_MissingField_ReturnsBadRequest(t *testing.T) {
	h, _ := newAdminDBTestHandlerAndDB(t)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/db/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	h.ImportDB(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestImportDB_InvalidFile_ReturnsBadRequest(t *testing.T) {
	h, _ := newAdminDBTestHandlerAndDB(t)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("db", "notadb.db")
	require.NoError(t, err)
	_, _ = fw.Write([]byte("this is not a sqlite database"))
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/db/import", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	h.ImportDB(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

// ── MigrateFromJSON ───────────────────────────────────────────────────────────

func TestMigrateFromJSON_ValidPayload_Succeeds(t *testing.T) {
	h, _ := newAdminDBTestHandlerAndDB(t)

	payload := `{
		"enabled_nodes": ["pve1"],
		"enabled_storages": ["local-lvm"],
		"isos": [],
		"vmbrs": ["vmbr0"],
		"tags": ["prod"],
		"limits": {"max_snapshots": 3},
		"max_network_cards": 2,
		"max_disk_per_vm": 4,
		"max_vm_per_user": 5,
		"allow_custom_yaml": false,
		"cloudinit_templates": [],
		"vm_profiles": [],
		"cloudinit_sftp": {"enabled": false}
	}`

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("settings", "settings.json")
	require.NoError(t, err)
	_, _ = fw.Write([]byte(payload))
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/migrate-from-json", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	h.MigrateFromJSON(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	var resp apiv1.MigrateFromJSONResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "migration completed successfully", resp.Message)
	assert.Equal(t, 1, resp.NodesCount)
	assert.Equal(t, 1, resp.TagsCount)
}

func TestMigrateFromJSON_InvalidJSON_ReturnsBadRequest(t *testing.T) {
	h, _ := newAdminDBTestHandlerAndDB(t)

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("settings", "settings.json")
	require.NoError(t, err)
	_, _ = fw.Write([]byte("{not json}"))
	_ = mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/migrate-from-json", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	w := httptest.NewRecorder()
	h.MigrateFromJSON(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}
