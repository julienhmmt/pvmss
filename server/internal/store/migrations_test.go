package store_test

import (
	"context"
	"database/sql"
	"fmt"
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

func BenchmarkRunMigrations(b *testing.B) {
	var i int
	for b.Loop() {
		i++
		path := filepath.Join(b.TempDir(), fmt.Sprintf("bench-%d.db", i))

		db, err := sql.Open("sqlite", path)
		if err != nil {
			b.Fatalf("open db: %v", err)
		}

		migrations := []store.Migration{
			{Version: 1, DDL: testMigrationDDL},
		}
		if err := store.RunMigrations(context.Background(), db, migrations); err != nil {
			b.Fatalf("RunMigrations: %v", err)
		}

		_ = db.Close()
	}
}
