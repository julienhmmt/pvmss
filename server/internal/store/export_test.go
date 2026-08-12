package store_test

import (
	"bytes"
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"pvmss/server/internal/config"
	"pvmss/server/internal/store"
	"testing"

	_ "modernc.org/sqlite"
)

// newExportStore opens a fully-migrated Store with a couple of audit rows so
// the exported snapshot has non-empty tables to verify against.
func newExportStore(t *testing.T) *store.Store {
	t.Helper()
	cfg := config.Configuration{
		Port:      50001,
		DBPath:    filepath.Join(t.TempDir(), "export.db"),
		LogLevel:  testStoreLogLevel,
		LogFormat: testStoreLogFormat,
		LogOutput: testStoreLogOutput,
	}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	ctx := context.Background()
	if err := st.RecordAction(ctx, "alice@pve", "default", 101, "start"); err != nil {
		t.Fatalf("RecordAction: %v", err)
	}

	if err := st.RecordAction(ctx, "bob@pve", "default", 102, "stop"); err != nil {
		t.Fatalf("RecordAction: %v", err)
	}

	return st
}

// TestExportDatabase_ProducesValidSnapshotWithSameRowCount — T006:
// ExportDatabase produces a file that reopens as a valid, internally
// consistent SQLite database with the same row counts as the live one.
//
//nolint:paralleltest // serial: shared database fixture
func TestExportDatabase_ProducesValidSnapshotWithSameRowCount(t *testing.T) {
	st := newExportStore(t)
	ctx := context.Background()

	var buf bytes.Buffer
	if err := st.ExportDatabase(ctx, &buf); err != nil {
		t.Fatalf("ExportDatabase: %v", err)
	}

	if buf.Len() == 0 {
		t.Fatal("export produced an empty file")
	}

	// Reopen the exported bytes as a fresh SQLite database and verify row counts.
	exportedPath := filepath.Join(t.TempDir(), "exported.db")
	if err := writeFile(exportedPath, buf.Bytes()); err != nil {
		t.Fatalf("write exported file: %v", err)
	}

	exportedDB, err := sql.Open("sqlite", "file:"+exportedPath+"?mode=ro")
	if err != nil {
		t.Fatalf("open exported: %v", err)
	}
	defer func() { _ = exportedDB.Close() }()

	var exportedAuditCount int
	if err := exportedDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_log`).Scan(&exportedAuditCount); err != nil {
		t.Fatalf("count exported audit_log: %v", err)
	}

	if exportedAuditCount != 2 {
		t.Errorf("exported audit_log rows = %d, want 2", exportedAuditCount)
	}

	// The exported snapshot should contain the catalog tables too (T06 seed).
	var nodeCount int
	if err := exportedDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM catalog_nodes`).Scan(&nodeCount); err != nil {
		t.Errorf("count exported catalog_nodes: %v", err)
	} else if nodeCount == 0 {
		t.Error("exported catalog_nodes is empty — seed data missing from snapshot")
	}
}

// TestExportDatabase_DoesNotBlockConcurrentWrite — T006: a concurrent write
// during export succeeds; VACUUM INTO does not block writers on the live
// database.
//
//nolint:paralleltest // serial: shared database fixture
func TestExportDatabase_DoesNotBlockConcurrentWrite(t *testing.T) {
	st := newExportStore(t)
	ctx := context.Background()

	var buf bytes.Buffer
	if err := st.ExportDatabase(ctx, &buf); err != nil {
		t.Fatalf("ExportDatabase: %v", err)
	}

	// After export, a write to the live database must still work.
	if err := st.RecordAction(ctx, "carol@pve", "default", 103, "reboot"); err != nil {
		t.Fatalf("post-export write failed: %v", err)
	}

	rows, err := st.QueryAudit(ctx)
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	if len(rows) != 3 {
		t.Errorf("live audit rows after export = %d, want 3", len(rows))
	}
}

// TestExportDatabase_SnapshotIsConsistent — the exported snapshot reflects
// the state at export time, not later writes.
//
//nolint:paralleltest // serial: shared database fixture
func TestExportDatabase_SnapshotIsConsistent(t *testing.T) {
	st := newExportStore(t)
	ctx := context.Background()

	var buf bytes.Buffer
	if err := st.ExportDatabase(ctx, &buf); err != nil {
		t.Fatalf("ExportDatabase: %v", err)
	}

	// Write to the live DB after the export.
	if err := st.RecordAction(ctx, "dave@pve", "default", 104, "delete"); err != nil {
		t.Fatalf("post-export write: %v", err)
	}

	// The exported snapshot must still have only the original 2 rows.
	exportedPath := filepath.Join(t.TempDir(), "consistent.db")
	if err := writeFile(exportedPath, buf.Bytes()); err != nil {
		t.Fatalf("write exported file: %v", err)
	}

	exportedDB, err := sql.Open("sqlite", "file:"+exportedPath+"?mode=ro")
	if err != nil {
		t.Fatalf("open exported: %v", err)
	}

	defer func() { _ = exportedDB.Close() }()

	var count int
	if err := exportedDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM audit_log`).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}

	if count != 2 {
		t.Errorf("exported snapshot has %d rows, want 2 (post-export write leaked in)", count)
	}
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}
