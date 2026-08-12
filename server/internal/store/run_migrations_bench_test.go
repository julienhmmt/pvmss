package store_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"pvmss/server/internal/store"
	"testing"

	_ "modernc.org/sqlite"
)

// BenchmarkRunMigrations measures the realistic cold path: open a fresh
// on-disk SQLite database, run the first migration, close it, and remove
// the file before the next loop so the temp directory does not bloat.
func BenchmarkRunMigrations(b *testing.B) {
	migrations := []store.Migration{
		{Version: 1, DDL: testMigrationDDL},
	}
	dbPath := filepath.Join(b.TempDir(), "bench.db")

	b.ReportAllocs()

	for b.Loop() {
		db, err := sql.Open("sqlite", dbPath)
		if err != nil {
			b.Fatalf("open db: %v", err)
		}

		db.SetMaxOpenConns(1)
		db.SetMaxIdleConns(1)

		if err := store.RunMigrations(context.Background(), db, migrations); err != nil {
			b.Fatalf("RunMigrations: %v", err)
		}

		if err := db.Close(); err != nil {
			b.Fatalf("close db: %v", err)
		}

		b.StopTimer()

		_ = os.Remove(dbPath)

		b.StartTimer()
	}
}

// BenchmarkRunMigrationsInMemory isolates the migration runner logic by using
// a single in-memory SQLite connection and resetting the schema between loops.
func BenchmarkRunMigrationsInMemory(b *testing.B) {
	migrations := []store.Migration{
		{Version: 1, DDL: testMigrationDDL},
	}

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		b.Fatalf("open db: %v", err)
	}

	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	b.ReportAllocs()
	b.Cleanup(func() { _ = db.Close() })

	for b.Loop() {
		b.StopTimer()

		if _, err := db.ExecContext(context.Background(), `DROP TABLE IF EXISTS t1`); err != nil {
			b.Fatalf("reset t1: %v", err)
		}

		if _, err := db.ExecContext(context.Background(), `DROP TABLE IF EXISTS schema_migrations`); err != nil {
			b.Fatalf("reset schema_migrations: %v", err)
		}

		b.StartTimer()

		if err := store.RunMigrations(context.Background(), db, migrations); err != nil {
			b.Fatalf("RunMigrations: %v", err)
		}
	}
}
