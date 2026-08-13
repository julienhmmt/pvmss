package store_test

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"pvmss/server/internal/config"
	"pvmss/server/internal/store"
	"testing"
)

const (
	citCluster   = "default"
	citID        = "web-server"
	citLabel     = "Web server"
	citContent   = "#cloud-config\npackages:\n  - nginx\n"
	citStamp     = "2026-01-01T00:00:00Z"
	citStampTwo  = "2026-01-02T00:00:00Z"
	citClusterB  = "other"
	citIDB       = "db-server"
	citLabelB    = "DB server"
	citContentB  = "#cloud-config\npackages:\n  - postgresql\n"
	citMissingID = "nonexistent"
)

// openCloudInitTemplatesStore opens a fully-migrated store ready for
// catalog_cloudinit_templates CRUD.
func openCloudInitTemplatesStore(t *testing.T) *store.Store {
	t.Helper()

	st, err := store.Open(config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "cit.db"),
		LogLevel:  testStoreLogLevel,
		LogFormat: testStoreLogFormat,
		LogOutput: testStoreLogOutput,
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	return st
}

// insertCloudInitTemplateRow inserts a row and fails the test on error — the
// shared setup step for tests that need a pre-existing row.
func insertCloudInitTemplateRow(ctx context.Context, t *testing.T, st *store.Store, cluster, id, label, content string) {
	t.Helper()

	if err := st.InsertCloudInitTemplate(ctx, cluster, id, label, content, citStamp, citStamp); err != nil {
		t.Fatalf("InsertCloudInitTemplate(%q, %q): %v", cluster, id, err)
	}
}

// findTemplateByID returns the row with the given id from the all-rows list, or
// fails the test when absent.
func findTemplateByID(t *testing.T, rows []store.CatalogCloudInitTemplate, id string) store.CatalogCloudInitTemplate {
	t.Helper()

	for _, r := range rows {
		if r.ID == id {
			return r
		}
	}

	t.Fatalf("row %q not found in %d rows", id, len(rows))

	return store.CatalogCloudInitTemplate{}
}

// TestCloudInitTemplates_RoundTripAndStates walks the full CRUD lifecycle via
// focused helpers: empty start, insert, duplicate, isolation, toggle, update,
// and delete — the storage layer contract for T18.
//
//nolint:paralleltest // round trip owns a shared SQLite fixture across ordered steps
func TestCloudInitTemplates_RoundTripAndStates(t *testing.T) {
	ctx := context.Background()
	st := openCloudInitTemplatesStore(t)

	testCITFreshStoreEmpty(ctx, t, st)
	testCITInsertAndVerify(ctx, t, st)
	testCITDuplicateAndIsolation(ctx, t, st)
	testCITDisableAndReenable(ctx, t, st)
	testCITUpdateAndDelete(ctx, t, st)
}

// testCITFreshStoreEmpty verifies both readers return empty slices and the
// exists check is false before any insert.
func testCITFreshStoreEmpty(ctx context.Context, t *testing.T, st *store.Store) {
	t.Helper()

	all, err := st.CatalogCloudInitTemplatesAll(ctx, citCluster)
	if err != nil {
		t.Fatalf("all on fresh store: %v", err)
	}

	if len(all) != 0 {
		t.Fatalf("fresh store all = %d rows, want 0", len(all))
	}

	enabled, err := st.CatalogCloudInitTemplatesEnabled(ctx, citCluster)
	if err != nil {
		t.Fatalf("enabled on fresh store: %v", err)
	}

	if len(enabled) != 0 {
		t.Fatalf("fresh store enabled = %d rows, want 0", len(enabled))
	}

	exists, err := st.CloudInitTemplateExists(ctx, citCluster, citID)
	if err != nil {
		t.Fatalf("exists on fresh store: %v", err)
	}

	if exists {
		t.Fatal("exists should be false before insert")
	}
}

// testCITInsertAndVerify inserts one row and checks it appears enabled in both
// the all-rows and enabled-only readers, with the exists check returning true.
func testCITInsertAndVerify(ctx context.Context, t *testing.T, st *store.Store) {
	t.Helper()

	insertCloudInitTemplateRow(ctx, t, st, citCluster, citID, citLabel, citContent)

	all, err := st.CatalogCloudInitTemplatesAll(ctx, citCluster)
	if err != nil {
		t.Fatalf("all after insert: %v", err)
	}

	if len(all) != 1 || all[0].ID != citID || all[0].Label != citLabel || all[0].Content != citContent {
		t.Fatalf("all after insert = %+v", all)
	}

	if !all[0].Enabled {
		t.Error("inserted row should be enabled by default")
	}

	enabled, err := st.CatalogCloudInitTemplatesEnabled(ctx, citCluster)
	if err != nil {
		t.Fatalf("enabled after insert: %v", err)
	}

	if len(enabled) != 1 || enabled[0].ID != citID {
		t.Fatalf("enabled after insert = %+v", enabled)
	}

	exists, err := st.CloudInitTemplateExists(ctx, citCluster, citID)
	if err != nil {
		t.Fatalf("exists after insert: %v", err)
	}

	if !exists {
		t.Fatal("exists should be true after insert")
	}
}

// testCITDuplicateAndIsolation verifies the ON CONFLICT DO NOTHING duplicate
// rejection and same-id/different-cluster coexistence (composite key).
func testCITDuplicateAndIsolation(ctx context.Context, t *testing.T, st *store.Store) {
	t.Helper()

	if err := st.InsertCloudInitTemplate(ctx, citCluster, citID, citLabel, citContent, citStamp, citStamp); !errors.Is(err, store.ErrDuplicate) {
		t.Fatalf("duplicate insert err = %v, want ErrDuplicate", err)
	}

	insertCloudInitTemplateRow(ctx, t, st, citClusterB, citID, citLabelB, citContentB)

	otherAll, err := st.CatalogCloudInitTemplatesAll(ctx, citClusterB)
	if err != nil {
		t.Fatalf("all clusterB: %v", err)
	}

	if len(otherAll) != 1 || otherAll[0].ID != citID || otherAll[0].Label != citLabelB {
		t.Fatalf("clusterB row = %+v, want isolated by cluster", otherAll)
	}

	all, err := st.CatalogCloudInitTemplatesAll(ctx, citCluster)
	if err != nil {
		t.Fatalf("all cluster after cross-cluster insert: %v", err)
	}

	if len(all) != 1 {
		t.Fatalf("cluster rows = %d, want 1 (cross-cluster isolation)", len(all))
	}
}

// testCITDisableAndReenable verifies the enabled toggle: disabling filters the
// row from the enabled-only reader (but not the all reader), and re-enabling
// restores it.
func testCITDisableAndReenable(ctx context.Context, t *testing.T, st *store.Store) {
	t.Helper()

	if err := st.SetCloudInitTemplateEnabled(ctx, citCluster, citID, false, citStampTwo); err != nil {
		t.Fatalf("disable: %v", err)
	}

	enabled, err := st.CatalogCloudInitTemplatesEnabled(ctx, citCluster)
	if err != nil {
		t.Fatalf("enabled after disable: %v", err)
	}

	if len(enabled) != 0 {
		t.Fatalf("enabled after disable = %d rows, want 0", len(enabled))
	}

	all, err := st.CatalogCloudInitTemplatesAll(ctx, citCluster)
	if err != nil {
		t.Fatalf("all after disable: %v", err)
	}

	row := findTemplateByID(t, all, citID)

	if row.Enabled {
		t.Error("row should be disabled after toggle")
	}

	if row.UpdatedAt != citStampTwo {
		t.Errorf("row.UpdatedAt = %q, want %q", row.UpdatedAt, citStampTwo)
	}

	if err := st.SetCloudInitTemplateEnabled(ctx, citCluster, citID, true, citStampTwo); err != nil {
		t.Fatalf("re-enable: %v", err)
	}

	enabled, err = st.CatalogCloudInitTemplatesEnabled(ctx, citCluster)
	if err != nil {
		t.Fatalf("enabled after re-enable: %v", err)
	}

	if len(enabled) != 1 || !enabled[0].Enabled {
		t.Fatalf("enabled after re-enable = %+v", enabled)
	}
}

// testCITUpdateAndDelete verifies the update mutation of label/content/stamp
// and that deletion removes the row from every reader with exists returning
// false.
func testCITUpdateAndDelete(ctx context.Context, t *testing.T, st *store.Store) {
	t.Helper()

	if err := st.UpdateCloudInitTemplate(ctx, citCluster, citID, citLabelB, citContentB, citStampTwo); err != nil {
		t.Fatalf("update: %v", err)
	}

	all, err := st.CatalogCloudInitTemplatesAll(ctx, citCluster)
	if err != nil {
		t.Fatalf("all after update: %v", err)
	}

	row := findTemplateByID(t, all, citID)

	if row.Label != citLabelB || row.Content != citContentB || row.UpdatedAt != citStampTwo {
		t.Errorf("updated row = %+v", row)
	}

	if err := st.DeleteCloudInitTemplate(ctx, citCluster, citID); err != nil {
		t.Fatalf("delete: %v", err)
	}

	all, err = st.CatalogCloudInitTemplatesAll(ctx, citCluster)
	if err != nil {
		t.Fatalf("all after delete: %v", err)
	}

	if len(all) != 0 {
		t.Fatalf("all after delete = %d rows, want 0", len(all))
	}

	exists, err := st.CloudInitTemplateExists(ctx, citCluster, citID)
	if err != nil {
		t.Fatalf("exists after delete: %v", err)
	}

	if exists {
		t.Fatal("exists should be false after delete")
	}
}

// TestCloudInitTemplates_UpdateNotFound — updating a missing row returns
// sql.ErrNoRows (the row vanished between the catalog layer's existence check
// and the UPDATE).
//
//nolint:paralleltest // serial: owns a SQLite fixture
func TestCloudInitTemplates_UpdateNotFound(t *testing.T) {
	ctx := context.Background()
	st := openCloudInitTemplatesStore(t)

	if err := st.UpdateCloudInitTemplate(ctx, citCluster, citMissingID, citLabel, citContent, citStamp); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("update missing err = %v, want sql.ErrNoRows", err)
	}
}

// TestCloudInitTemplates_DeleteNotFound — deleting a missing row returns
// sql.ErrNoRows.
//
//nolint:paralleltest // serial: owns a SQLite fixture
func TestCloudInitTemplates_DeleteNotFound(t *testing.T) {
	ctx := context.Background()
	st := openCloudInitTemplatesStore(t)

	if err := st.DeleteCloudInitTemplate(ctx, citCluster, citMissingID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("delete missing err = %v, want sql.ErrNoRows", err)
	}
}

// TestCloudInitTemplates_SetEnabledNotFound — toggling a missing row returns
// sql.ErrNoRows.
//
//nolint:paralleltest // serial: owns a SQLite fixture
func TestCloudInitTemplates_SetEnabledNotFound(t *testing.T) {
	ctx := context.Background()
	st := openCloudInitTemplatesStore(t)

	if err := st.SetCloudInitTemplateEnabled(ctx, citCluster, citMissingID, true, citStamp); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("set enabled missing err = %v, want sql.ErrNoRows", err)
	}
}

// TestCloudInitTemplates_StoreErrors — once the underlying store is closed,
// every catalog_cloudinit_templates operation surfaces a non-sentinel error
// (not sql.ErrNoRows, not store.ErrDuplicate). This covers the transport-error
// branches the happy-path and not-found tests cannot reach.
//
//nolint:paralleltest // serial: owns a SQLite fixture that is intentionally closed
func TestCloudInitTemplates_StoreErrors(t *testing.T) {
	ctx := context.Background()
	st := openCloudInitTemplatesStore(t)

	if err := st.Close(); err != nil {
		t.Fatalf("close store: %v", err)
	}

	if _, err := st.CatalogCloudInitTemplatesAll(ctx, citCluster); err == nil {
		t.Fatal("CatalogCloudInitTemplatesAll on closed store: expected error, got nil")
	}

	if _, err := st.CatalogCloudInitTemplatesEnabled(ctx, citCluster); err == nil {
		t.Fatal("CatalogCloudInitTemplatesEnabled on closed store: expected error, got nil")
	}

	err := st.InsertCloudInitTemplate(ctx, citCluster, citID, citLabel, citContent, citStamp, citStamp)
	assertCloudInitStoreError(t, err)

	err = st.UpdateCloudInitTemplate(ctx, citCluster, citID, citLabel, citContent, citStamp)
	assertCloudInitStoreError(t, err)

	err = st.DeleteCloudInitTemplate(ctx, citCluster, citID)
	assertCloudInitStoreError(t, err)

	err = st.SetCloudInitTemplateEnabled(ctx, citCluster, citID, true, citStamp)
	assertCloudInitStoreError(t, err)

	_, err = st.CloudInitTemplateExists(ctx, citCluster, citID)
	assertCloudInitStoreError(t, err)
}

// assertCloudInitStoreError fails when err is nil or one of the store sentinels
// — a closed store must surface a non-sentinel transport error.
func assertCloudInitStoreError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("expected error on closed store, got nil")
	}

	if errors.Is(err, sql.ErrNoRows) || errors.Is(err, store.ErrDuplicate) {
		t.Fatalf("expected non-sentinel store error, got %v", err)
	}
}
