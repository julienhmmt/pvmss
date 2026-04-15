// Package database white-box tests. These tests use package database (not
// database_test) to access unexported functions and trigger error paths that
// cannot be exercised through the exported DB interface alone.
package database

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// openRaw opens an in-memory *sql.DB with pragmas already applied.
func openRaw(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	require.NoError(t, applyPragmas(db))
	t.Cleanup(func() { _ = db.Close() })
	return db
}

// ── applyPragmas ──────────────────────────────────────────────────────────────

func TestApplyPragmas_ClosedDB_ReturnsError(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	_ = db.Close()
	err = applyPragmas(db)
	assert.Error(t, err)
}

// ── migrations ────────────────────────────────────────────────────────────────

func TestApplyMigration_UnknownVersion_ReturnsError(t *testing.T) {
	db := openRaw(t)
	require.NoError(t, ensureMigrationsTable(db))
	err := applyMigration(db, 999)
	assert.ErrorContains(t, err, "no DDL registered for version 999")
}

func TestRunMigrations_ClosedDB_ReturnsError(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	_ = db.Close()
	err = RunMigrations(db)
	assert.Error(t, err)
}

func TestEnsureMigrationsTable_Idempotent(t *testing.T) {
	db := openRaw(t)
	require.NoError(t, ensureMigrationsTable(db))
	require.NoError(t, ensureMigrationsTable(db), "second call must not error")
}

func TestAppliedVersions_Empty(t *testing.T) {
	db := openRaw(t)
	require.NoError(t, ensureMigrationsTable(db))
	m, err := appliedVersions(db)
	require.NoError(t, err)
	assert.Empty(t, m)
}

func TestAppliedVersions_AfterMigration(t *testing.T) {
	db := openRaw(t)
	require.NoError(t, RunMigrations(db))
	m, err := appliedVersions(db)
	require.NoError(t, err)
	assert.True(t, m[1])
}

// ── schema constants ──────────────────────────────────────────────────────────

func TestSchemaV1_IsNotEmpty(t *testing.T) {
	assert.NotEmpty(t, schemaV1)
}

// ── appendAudit error path ────────────────────────────────────────────────────

func TestAppendAudit_ClosedTx_ReturnsError(t *testing.T) {
	db := openRaw(t)
	require.NoError(t, RunMigrations(db))

	tx, err := db.Begin()
	require.NoError(t, err)
	require.NoError(t, tx.Rollback()) // commit a rollback to close the transaction

	err = appendAudit(tx, "vm_limits", "1", "update", "{}", "{}", "admin")
	assert.Error(t, err)
}

// ── nullableString ────────────────────────────────────────────────────────────

func TestNullableString_EmptyReturnsNil(t *testing.T) {
	assert.Nil(t, nullableString(""))
}

func TestNullableString_NonEmptyReturnsString(t *testing.T) {
	assert.Equal(t, "hello", nullableString("hello"))
}

// ── boolToInt ─────────────────────────────────────────────────────────────────

func TestBoolToInt(t *testing.T) {
	assert.Equal(t, 1, boolToInt(true))
	assert.Equal(t, 0, boolToInt(false))
}

// ── Open error path ───────────────────────────────────────────────────────────

func TestOpen_InvalidPath_ReturnsError(t *testing.T) {
	_, err := Open(filepath.Join("/nonexistent-directory", "pvmss.db"))
	assert.Error(t, err)
}

// ── auditAction ───────────────────────────────────────────────────────────────

func TestAuditAction_ExistingKey_ReturnsUpdate(t *testing.T) {
	m := map[string]int{"pve1": 5}
	assert.Equal(t, "update", auditAction(m, "pve1"))
}

func TestAuditAction_MissingKey_ReturnsCreate(t *testing.T) {
	m := map[string]int{}
	assert.Equal(t, "create", auditAction(m, "pve1"))
}

// ── write error paths (closed DB) ─────────────────────────────────────────────

// closedDB returns a *sql.DB that has already been closed (to force query errors).
func closedDB(t *testing.T) *sqliteDB {
	t.Helper()
	raw, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	_ = raw.Close()
	return &sqliteDB{db: raw}
}

func TestGetVMLimits_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	_, err := s.GetVMLimits()
	assert.Error(t, err)
}

func TestSetVMLimits_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	err := s.SetVMLimits(&VMLimits{MaxVMs: 1}, "admin")
	assert.Error(t, err)
}

func TestGetNodeLimits_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	_, err := s.GetNodeLimits()
	assert.Error(t, err)
}

func TestGetSFTPConfig_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	_, err := s.GetSFTPConfig()
	assert.Error(t, err)
}

func TestSetSFTPConfig_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	err := s.SetSFTPConfig(&SFTPConfig{Port: 22}, "admin")
	assert.Error(t, err)
}

func TestGetEnabledNodes_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	_, err := s.GetEnabledNodes()
	assert.Error(t, err)
}

func TestListCloudInitTemplates_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	_, err := s.ListCloudInitTemplates()
	assert.Error(t, err)
}

func TestListVMProfiles_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	_, err := s.ListVMProfiles()
	assert.Error(t, err)
}

func TestLoadAppSettings_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	_, err := s.LoadAppSettings()
	assert.Error(t, err)
}

func TestIsBootstrapComplete_ClosedDB_HandledGracefully(t *testing.T) {
	s := closedDB(t)
	// closed DB → QueryRow returns error at Scan → should NOT be treated as ErrNoRows
	_, err := s.IsBootstrapComplete()
	assert.Error(t, err)
}

func TestListAuditLog_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	_, err := s.ListAuditLog("", 10, 0)
	assert.Error(t, err)
}

func TestSetNodeLimit_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	err := s.SetNodeLimit("pve1", 5, "admin")
	assert.Error(t, err)
}

func TestDeleteNodeLimit_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	err := s.DeleteNodeLimit("pve1", "admin")
	assert.Error(t, err)
}

func TestSetEnabledNodes_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	err := s.SetEnabledNodes([]string{"pve1"}, "admin")
	assert.Error(t, err)
}

func TestGetCloudInitTemplate_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	_, err := s.GetCloudInitTemplate("id")
	assert.Error(t, err)
}

func TestGetVMProfile_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	_, err := s.GetVMProfile("id")
	assert.Error(t, err)
}

func TestCreateCloudInitTemplate_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	err := s.CreateCloudInitTemplate(&CloudInitTemplate{
		ID: "x", Name: "x", YAMLContent: "#cloud-config",
	}, "admin")
	assert.Error(t, err)
}

func TestCreateVMProfile_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	err := s.CreateVMProfile(&VMProfile{
		ID: "x", Name: "x", Config: "{}",
	}, "admin")
	assert.Error(t, err)
}

func TestUpdateCloudInitTemplate_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	err := s.UpdateCloudInitTemplate(&CloudInitTemplate{
		ID: "x", Name: "x", YAMLContent: "#cloud-config",
	}, "admin")
	assert.Error(t, err)
}

func TestDeleteCloudInitTemplate_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	err := s.DeleteCloudInitTemplate("x", "admin")
	assert.Error(t, err)
}

func TestUpdateVMProfile_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	err := s.UpdateVMProfile(&VMProfile{
		ID: "x", Name: "x", Config: "{}",
	}, "admin")
	assert.Error(t, err)
}

func TestDeleteVMProfile_ClosedDB_ReturnsError(t *testing.T) {
	s := closedDB(t)
	err := s.DeleteVMProfile("x", "admin")
	assert.Error(t, err)
}
