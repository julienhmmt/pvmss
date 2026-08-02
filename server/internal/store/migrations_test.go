package store_test

import (
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"pvmss/server/internal/store"

	_ "modernc.org/sqlite"
)

func TestRunMigrations_FreshDB_AppliesInOrder(t *testing.T) {
	db := openTestDB(t)
	migrations := map[int]string{
		1: `CREATE TABLE t1 (id INTEGER PRIMARY KEY)`,
		2: `ALTER TABLE t1 ADD COLUMN name TEXT`,
	}

	if err := store.RunMigrations(db, migrations); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	versions := appliedVersions(t, db)
	if len(versions) != 2 || !versions[1] || !versions[2] {
		t.Fatalf("expected versions 1 and 2 applied, got %v", versions)
	}

	if _, err := db.Exec(`INSERT INTO t1 (id, name) VALUES (1, 'x')`); err != nil {
		t.Fatalf("insert into t1: %v", err)
	}
}

func TestRunMigrations_Rerun_IsNoOp(t *testing.T) {
	db := openTestDB(t)
	migrations := map[int]string{
		1: `CREATE TABLE t1 (id INTEGER PRIMARY KEY)`,
	}

	if err := store.RunMigrations(db, migrations); err != nil {
		t.Fatalf("first RunMigrations: %v", err)
	}
	if err := store.RunMigrations(db, migrations); err != nil {
		t.Fatalf("second RunMigrations: %v", err)
	}

	versions := appliedVersions(t, db)
	if len(versions) != 1 {
		t.Fatalf("expected one applied version, got %v", versions)
	}
}

func TestRunMigrations_PartiallyApplied_AppliesRemaining(t *testing.T) {
	db := openTestDB(t)
	migrations := map[int]string{
		1: `CREATE TABLE t1 (id INTEGER PRIMARY KEY)`,
	}
	if err := store.RunMigrations(db, migrations); err != nil {
		t.Fatalf("first RunMigrations: %v", err)
	}

	migrations[2] = `ALTER TABLE t1 ADD COLUMN name TEXT`
	if err := store.RunMigrations(db, migrations); err != nil {
		t.Fatalf("second RunMigrations: %v", err)
	}

	versions := appliedVersions(t, db)
	if !versions[1] || !versions[2] {
		t.Fatalf("expected versions 1 and 2, got %v", versions)
	}
}

func TestRunMigrations_EmptyMap_IsNoOp(t *testing.T) {
	db := openTestDB(t)
	if err := store.RunMigrations(db, map[int]string{}); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
}

func TestRunMigrations_MissingVersion_Detected(t *testing.T) {
	db := openTestDB(t)
	migrations := map[int]string{
		1: `CREATE TABLE t1 (id INTEGER PRIMARY KEY)`,
		3: `CREATE TABLE t3 (id INTEGER PRIMARY KEY)`,
	}

	err := store.RunMigrations(db, migrations)
	if err == nil {
		t.Fatalf("expected error for missing version 2, got nil")
	}
	if err.Error() != `migration version 2 is missing from the migration map` {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunMigrations_Validate(t *testing.T) {
	db := openTestDB(t)

	cases := []struct {
		name    string
		mig     map[int]string
		wantErr string
	}{
		{
			name:    "negative version",
			mig:     map[int]string{-1: `CREATE TABLE t1 (id INTEGER PRIMARY KEY)`},
			wantErr: "migration version -1 is not positive",
		},
		{
			name:    "empty DDL",
			mig:     map[int]string{1: "   "},
			wantErr: "migration 1 has no DDL",
		},
		{
			name: "missing version",
			mig: map[int]string{
				1: `CREATE TABLE t1 (id INTEGER PRIMARY KEY)`,
				3: `CREATE TABLE t3 (id INTEGER PRIMARY KEY)`,
			},
			wantErr: "migration version 2 is missing from the migration map",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := store.RunMigrations(db, c.mig)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			if err.Error() != c.wantErr {
				t.Fatalf("error = %q, want %q", err.Error(), c.wantErr)
			}
		})
	}
}

func TestRunMigrations_ClosedDB_ReturnsError(t *testing.T) {
	db := openTestDB(t)
	_ = db.Close()

	migrations := map[int]string{
		1: `CREATE TABLE t1 (id INTEGER PRIMARY KEY)`,
	}
	if err := store.RunMigrations(db, migrations); err == nil {
		t.Fatalf("expected error for closed database, got nil")
	}
}

func TestRunMigrations_RecordsAppliedAt(t *testing.T) {
	db := openTestDB(t)
	migrations := map[int]string{
		1: `CREATE TABLE t1 (id INTEGER PRIMARY KEY)`,
	}

	before := time.Now().UTC()
	if err := store.RunMigrations(db, migrations); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	var appliedAt string
	if err := db.QueryRow(`SELECT applied_at FROM schema_migrations WHERE version = 1`).Scan(&appliedAt); err != nil {
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

func appliedVersions(t *testing.T, db *sql.DB) map[int]bool {
	t.Helper()
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		t.Fatalf("query schema_migrations: %v", err)
	}
	defer func() { _ = rows.Close() }()
	result := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			t.Fatalf("scan version: %v", err)
		}
		result[v] = true
	}
	return result
}

func TestRunMigrations_InvalidDDL_ReturnsError(t *testing.T) {
	db := openTestDB(t)
	migrations := map[int]string{
		1: `CREATE TABLE`,
	}

	err := store.RunMigrations(db, migrations)
	if err == nil {
		t.Fatalf("expected error for invalid DDL, got nil")
	}
}

func BenchmarkRunMigrations(b *testing.B) {
	for i := 0; i < b.N; i++ {
		path := filepath.Join(b.TempDir(), fmt.Sprintf("bench-%d.db", i))
		db, err := sql.Open("sqlite", path)
		if err != nil {
			b.Fatalf("open db: %v", err)
		}
		migrations := map[int]string{
			1: `CREATE TABLE t1 (id INTEGER PRIMARY KEY)`,
		}
		if err := store.RunMigrations(db, migrations); err != nil {
			b.Fatalf("RunMigrations: %v", err)
		}
		_ = db.Close()
	}
}
