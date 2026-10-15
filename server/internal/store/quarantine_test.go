package store_test

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"pvmss/server/internal/config"
	"pvmss/server/internal/store"
	"strings"
	"testing"
	"time"
)

func TestRunMigrations_UnknownVersions_ReturnsSortedSentinelError(t *testing.T) {
	t.Parallel()

	db := openTestDB(t)
	ctx := context.Background()

	if _, err := db.ExecContext(ctx, `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
		INSERT INTO schema_migrations VALUES (37, 'x'), (2, 'x'), (1, 'x')`); err != nil {
		t.Fatalf("seed: %v", err)
	}

	err := store.RunMigrations(ctx, db, []store.Migration{{Version: 1, DDL: testMigrationDDL}})
	if !errors.Is(err, store.ErrIncompatibleSchema) {
		t.Fatalf("want ErrIncompatibleSchema, got %v", err)
	}

	if !strings.Contains(err.Error(), "[2 37]") {
		t.Fatalf("want sorted unknown versions [2 37] in %q", err)
	}
}

func TestQuarantineDatabase_MovesFilesAndKeepsContent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "pvmss.db")
	for _, f := range []string{path, path + "-wal", path + "-shm"} {
		if err := os.WriteFile(f, []byte(f), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	moved, err := store.QuarantineDatabase(path, time.Date(2026, 10, 6, 21, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("QuarantineDatabase: %v", err)
	}

	if want := path + ".incompatible-20261006T210000Z"; moved != want {
		t.Fatalf("moved = %q, want %q", moved, want)
	}

	for _, ext := range []string{"", "-wal", "-shm"} {
		if _, err := os.Stat(path + ext); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("%s should be gone, stat err = %v", path+ext, err)
		}

		got, err := os.ReadFile(moved + ext) //nolint:gosec // test-owned temp path
		if err != nil || string(got) != path+ext {
			t.Fatalf("%s content = %q, err = %v", moved+ext, got, err)
		}
	}
}

func TestQuarantineDatabase_Errors(t *testing.T) {
	t.Parallel()

	if _, err := store.QuarantineDatabase("", time.Now()); err == nil {
		t.Fatal("empty path must fail")
	}

	if _, err := store.QuarantineDatabase(filepath.Join(t.TempDir(), "missing.db"), time.Now()); err == nil {
		t.Fatal("missing file must fail")
	}
}

// A stale pre-squash database must be rejected by Open, then recoverable.
//
//nolint:paralleltest // serial: other tests swap the global store.Migrations
func TestOpen_StaleDatabase_RejectedThenRecoveredAfterQuarantine(t *testing.T) {
	cfg := config.Configuration{
		Port:      50001,
		DBPath:    filepath.Join(t.TempDir(), "pvmss.db"),
		LogLevel:  testStoreLogLevel,
		LogFormat: testStoreLogFormat,
		LogOutput: testStoreLogOutput,
	}

	stale, err := sql.Open("sqlite", cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := stale.ExecContext(context.Background(), `CREATE TABLE schema_migrations (version INTEGER PRIMARY KEY, applied_at TEXT NOT NULL);
		INSERT INTO schema_migrations VALUES (1, 'x'), (14, 'x')`); err != nil {
		t.Fatal(err)
	}

	_ = stale.Close()

	if _, err := store.Open(cfg); !errors.Is(err, store.ErrIncompatibleSchema) {
		t.Fatalf("want ErrIncompatibleSchema, got %v", err)
	}

	if _, err := store.QuarantineDatabase(cfg.DBPath, time.Now()); err != nil {
		t.Fatal(err)
	}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("fresh Open after quarantine: %v", err)
	}

	_ = st.Close()
}
