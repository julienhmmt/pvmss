package apiv1

import (
	"io"
	"net/http"
	"os"
	"strconv"
	"time"

	"pvmss/database"
	"pvmss/logger"
	"pvmss/state"
)

// AdminDBHandler handles audit-log and database management endpoints.
type AdminDBHandler struct {
	state state.StateManager
	db    database.DB
}

// MakeAdminDBHandler creates a new AdminDBHandler.
func MakeAdminDBHandler(s state.StateManager, db database.DB) *AdminDBHandler {
	return &AdminDBHandler{state: s, db: db}
}

// ── Audit log ─────────────────────────────────────────────────────────────────

// AuditLogResponse is the JSON body returned by ListAuditLog.
type AuditLogResponse struct {
	Entries []database.AuditEntry `json:"entries"`
	Limit   int                   `json:"limit"`
	Offset  int                   `json:"offset"`
}

// ListAuditLog handles GET /api/v1/admin/audit.
// Optional query params: table (string), limit (int, default 50), offset (int, default 0).
func (h *AdminDBHandler) ListAuditLog(w http.ResponseWriter, r *http.Request) {
	tableFilter := r.URL.Query().Get("table")

	// Validate table filter against allowlist
	validTables := map[string]bool{
		"":                    true,
		"tags":                true,
		"vm_limits":           true,
		"node_limits":         true,
		"enabled_nodes":       true,
		"enabled_storages":    true,
		"enabled_isos":        true,
		"enabled_vmbrs":       true,
		"cloudinit_templates": true,
		"vm_profiles":         true,
		"sftp_config":         true,
		"database":            true,
	}
	if tableFilter != "" && !validTables[tableFilter] {
		errBadRequest(w, "invalid table filter")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	entries, err := h.db.ListAuditLog(tableFilter, limit, offset)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to list audit log")
		errInternal(w)
		return
	}
	writeJSON(w, AuditLogResponse{Entries: entries, Limit: limit, Offset: offset})
}

// ── DB export ─────────────────────────────────────────────────────────────────

// ExportDB handles GET /api/v1/admin/db/export.
// Creates a VACUUM INTO snapshot and streams it as a binary download.
func (h *AdminDBHandler) ExportDB(w http.ResponseWriter, r *http.Request) {
	tmpFile, err := os.CreateTemp("", "pvmss-backup-*.db")
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to create temp file for DB export")
		errInternal(w)
		return
	}
	tmpPath := tmpFile.Name()
	_ = tmpFile.Close()
	defer func() { _ = os.Remove(tmpPath) }()

	if err := h.db.Backup(tmpPath); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to backup database for export")
		errInternal(w)
		return
	}

	f, err := os.Open(tmpPath)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to open backup file for streaming")
		errInternal(w)
		return
	}
	defer func() { _ = f.Close() }()

	timestamp := time.Now().UTC().Format("2006-01-02-15-04-05")
	filename := "pvmss-backup-" + timestamp + ".db"
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	w.Header().Set("Cache-Control", "no-store")
	if _, err := io.Copy(w, f); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to stream backup file")
		// Cannot return error response since headers already sent
		return
	}
}

// ── DB import ─────────────────────────────────────────────────────────────────

// ImportDB handles POST /api/v1/admin/db/import.
// Expects a multipart form with a "db" file field.
// Validates the uploaded database, replaces the current database, then reloads
// the settings cache.
func (h *AdminDBHandler) ImportDB(w http.ResponseWriter, r *http.Request) {
	const maxUploadSize = 64 << 20 // 64 MB
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		errBadRequest(w, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("db")
	if err != nil {
		errBadRequest(w, "missing db file field")
		return
	}
	defer func() { _ = file.Close() }()

	// Validate file size before copying to temp file
	if header.Size > maxUploadSize {
		errBadRequest(w, "file too large (max 64MB)")
		return
	}

	tmpFile, err := os.CreateTemp("", "pvmss-import-*.db")
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to create temp file for DB import")
		errInternal(w)
		return
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = os.Remove(tmpPath) }()

	bytesWritten, err := io.Copy(tmpFile, file)
	if err != nil {
		_ = tmpFile.Close()
		logger.Get().Error().Err(err).Msg("Failed to write uploaded DB to temp file")
		errInternal(w)
		return
	}
	_ = tmpFile.Close()

	// Validate actual file size after copy
	if bytesWritten > maxUploadSize {
		errBadRequest(w, "file too large (max 64MB)")
		return
	}

	// RestoreFrom validates the source before making any changes.
	changedBy := usernameFromCtx(r)
	if err := h.db.RestoreFrom(tmpPath, changedBy); err != nil {
		logger.Get().Warn().Err(err).Msg("DB import rejected")
		errBadRequest(w, "invalid database file: "+err.Error())
		return
	}

	// T157: reload settings cache from the newly imported data.
	if err := h.state.LoadSettingsFromDB(); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to reload settings cache after DB import")
		errInternal(w)
		return
	}

	writeJSON(w, map[string]string{"message": "database imported successfully"})
}
