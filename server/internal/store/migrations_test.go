package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"pvmss/server/internal/store"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

//nolint:paralleltest // serial: shared database fixture
func TestRunMigrations_FreshDB_AppliesInOrder(t *testing.T) {
	db := openTestDB(t)
	migrations := []store.Migration{
		{Version: 1, DDL: testMigrationDDL},
		{Version: 2, DDL: `ALTER TABLE t1 ADD COLUMN name TEXT`},
	}

	if err := store.RunMigrations(context.Background(), db, migrations); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	versions := appliedVersions(t, db)
	if len(versions) != 2 {
		t.Fatalf("expected two applied versions, got %v", versions)
	}

	for _, v := range []int{1, 2} {
		if _, ok := versions[v]; !ok {
			t.Fatalf("expected version %d applied, got %v", v, versions)
		}
	}

	if _, err := db.ExecContext(context.Background(), `INSERT INTO t1 (id, name) VALUES (1, 'x')`); err != nil {
		t.Fatalf("insert into t1: %v", err)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestRunMigrations_Rerun_IsNoOp(t *testing.T) {
	db := openTestDB(t)
	migrations := []store.Migration{
		{Version: 1, DDL: testMigrationDDL},
	}

	if err := store.RunMigrations(context.Background(), db, migrations); err != nil {
		t.Fatalf("first RunMigrations: %v", err)
	}

	if err := store.RunMigrations(context.Background(), db, migrations); err != nil {
		t.Fatalf("second RunMigrations: %v", err)
	}

	versions := appliedVersions(t, db)
	if len(versions) != 1 {
		t.Fatalf("expected one applied version, got %v", versions)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestRunMigrations_PartiallyApplied_AppliesRemaining(t *testing.T) {
	db := openTestDB(t)

	migrations := []store.Migration{
		{Version: 1, DDL: testMigrationDDL},
	}
	if err := store.RunMigrations(context.Background(), db, migrations); err != nil {
		t.Fatalf("first RunMigrations: %v", err)
	}

	migrations = append(migrations, store.Migration{Version: 2, DDL: `ALTER TABLE t1 ADD COLUMN name TEXT`})
	if err := store.RunMigrations(context.Background(), db, migrations); err != nil {
		t.Fatalf("second RunMigrations: %v", err)
	}

	versions := appliedVersions(t, db)
	for _, v := range []int{1, 2} {
		if _, ok := versions[v]; !ok {
			t.Fatalf("expected version %d applied, got %v", v, versions)
		}
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestRunMigrations_EmptyList_ReturnsError(t *testing.T) {
	db := openTestDB(t)
	if err := store.RunMigrations(context.Background(), db, []store.Migration{}); err == nil {
		t.Fatalf("expected error for empty migration list, got nil")
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestRunMigrations_MissingVersion_Detected(t *testing.T) {
	db := openTestDB(t)
	migrations := []store.Migration{
		{Version: 1, DDL: testMigrationDDL},
		{Version: 3, DDL: `CREATE TABLE t3 (id INTEGER PRIMARY KEY)`},
	}

	err := store.RunMigrations(context.Background(), db, migrations)
	if err == nil {
		t.Fatalf("expected error for missing version 2, got nil")
	}

	if err.Error() != `migration version 2 is missing from the migration list` {
		t.Fatalf("unexpected error: %v", err)
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestRunMigrations_Validate(t *testing.T) {
	db := openTestDB(t)

	cases := []struct {
		name    string
		mig     []store.Migration
		wantErr string
	}{
		{
			name: "negative version",
			mig: []store.Migration{
				{Version: -1, DDL: testMigrationDDL},
			},
			wantErr: "migration version -1 is not positive",
		},
		{
			name: "empty DDL",
			mig: []store.Migration{
				{Version: 1, DDL: "   "},
			},
			wantErr: "migration 1 has no ddl",
		},
		{
			name: "missing version",
			mig: []store.Migration{
				{Version: 1, DDL: testMigrationDDL},
				{Version: 3, DDL: `CREATE TABLE t3 (id INTEGER PRIMARY KEY)`},
			},
			wantErr: "migration version 2 is missing from the migration list",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := store.RunMigrations(context.Background(), db, c.mig)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}

			if err.Error() != c.wantErr {
				t.Fatalf("error = %q, want %q", err.Error(), c.wantErr)
			}
		})
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestRunMigrations_ClosedDB_ReturnsError(t *testing.T) {
	db := openTestDB(t)
	_ = db.Close()

	migrations := []store.Migration{
		{Version: 1, DDL: testMigrationDDL},
	}
	if err := store.RunMigrations(context.Background(), db, migrations); err == nil {
		t.Fatalf("expected error for closed database, got nil")
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestRunMigrations_RecordsAppliedAt(t *testing.T) {
	db := openTestDB(t)
	migrations := []store.Migration{
		{Version: 1, DDL: testMigrationDDL},
	}

	before := time.Now().UTC()

	if err := store.RunMigrations(context.Background(), db, migrations); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	var appliedAt string
	if err := db.QueryRowContext(context.Background(), `SELECT applied_at FROM schema_migrations WHERE version = 1`).Scan(&appliedAt); err != nil {
		t.Fatalf("query applied_at: %v", err)
	}

	parsed, err := time.Parse(time.RFC3339, appliedAt)
	if err != nil {
		t.Fatalf("applied_at %q is not RFC3339: %v", appliedAt, err)
	}

	if parsed.Before(before.Add(-time.Second)) {
		t.Fatalf("applied_at %q is before migration started", appliedAt)
	}
}

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("open db: %v", err)
	}

	t.Cleanup(func() { _ = db.Close() })

	return db
}

func appliedVersions(t *testing.T, db *sql.DB) map[int]struct{} {
	t.Helper()

	rows, err := db.QueryContext(context.Background(), `SELECT version FROM schema_migrations`)
	if err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}

	defer func() { _ = rows.Close() }()

	result := make(map[int]struct{}, 8)

	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan version: %v", err)
		}

		result[v] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		t.Fatalf("iterating schema_migrations: %v", err)
	}

	return result
}

//nolint:paralleltest // serial: shared database fixture
func TestRunMigrations_InvalidDDL_ReturnsError(t *testing.T) {
	db := openTestDB(t)
	migrations := []store.Migration{
		{Version: 1, DDL: `CREATE TABLE`},
	}

	err := store.RunMigrations(context.Background(), db, migrations)
	if err == nil {
		t.Fatalf("expected error for invalid DDL, got nil")
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestRunMigrations_UnknownAppliedVersion_ReturnsError(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	// Simulate a database created by a pre-release development build: its
	// schema_migrations rows reference versions the squashed baseline no
	// longer carries. Applying nothing would leave a stale schema silently,
	// so the runner must refuse.
	if _, err := db.ExecContext(ctx, `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL)`); err != nil {
		t.Fatalf("create migrations table: %v", err)
	}

	for version := 1; version <= 3; version++ {
		if _, err := db.ExecContext(ctx, `INSERT INTO schema_migrations (version, applied_at) VALUES (?, '2026-01-01T00:00:00Z')`, version); err != nil {
			t.Fatalf("mark migration %d applied: %v", version, err)
		}
	}

	migrations := []store.Migration{
		{Version: 1, DDL: testMigrationDDL},
	}

	err := store.RunMigrations(ctx, db, migrations)
	if err == nil {
		t.Fatalf("expected error for unknown applied versions, got nil")
	}
}

// baselineTables is every table the v0.4 baseline must create - the final
// form of the collapsed migration chain. Tables dropped during development
// (api_tokens, user_cloudinit_files, cloudinit_publications) are absent.
var baselineTables = []string{
	"sessions",
	"audit_log",
	"clusters",
	"catalog_nodes",
	"catalog_storages",
	"catalog_bridges",
	"catalog_isos",
	"catalog_images",
	"catalog_templates",
	"catalog_profiles",
	"catalog_tags",
	"catalog_cloudinit_templates",
	"documentation_pages",
	"managed_pools",
	"vm_limits",
	"node_limits",
	"audit_config",
	"vm_baseline_state",
	"vm_cloudinit_documents",
	"vm_cloudinit_snippets",
	"profile_ssh_keys",
}

//nolint:paralleltest // serial: shared database fixture
func TestMigrations_BaselineCreatesFinalSchema(t *testing.T) {
	db := openTestDB(t)
	ctx := context.Background()

	if err := store.RunMigrations(ctx, db, store.Migrations); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	for _, table := range baselineTables {
		var name string

		err := db.QueryRowContext(ctx,
			`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, table).Scan(&name)
		if err != nil {
			t.Fatalf("table %q missing from baseline schema: %v", table, err)
		}
	}

	// The baseline carries no demo data: only the structural audit_config
	// singleton is seeded. vm_limits and catalog_tags rows arrive at runtime
	// (CreateCluster, catalog.ListTags).
	for table, want := range map[string]int{
		"catalog_nodes": 0,
		"vm_limits":     0,
		"catalog_tags":  0,
		"audit_config":  1,
	} {
		var got int
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+table).Scan(&got); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}

		if got != want {
			t.Errorf("%s rows = %d, want %d", table, got, want)
		}
	}
}
