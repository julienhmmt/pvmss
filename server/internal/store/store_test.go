package store_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"pvmss/server/internal/config"
	"pvmss/server/internal/store"
	"testing"

	_ "modernc.org/sqlite"
)

//nolint:paralleltest // serial: shared database fixture
func TestOpen_MigrationsValidationFailed(t *testing.T) {
	original := store.Migrations
	defer func() { store.Migrations = original }()

	store.Migrations = []store.Migration{
		{Version: 1, DDL: testMigrationDDL},
		{Version: 3, DDL: `CREATE TABLE t3 (id INTEGER PRIMARY KEY)`},
	}

	dir := t.TempDir()
	cfg := config.Configuration{
		Port:      50001,
		DBPath:    filepath.Join(dir, "pvmss.db"),
		LogLevel:  testStoreLogLevel,
		LogFormat: testStoreLogFormat,
		LogOutput: testStoreLogOutput,
	}

	if _, err := store.Open(cfg); err == nil {
		t.Fatalf("expected error for invalid migrations, got nil")
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestOpen_MkdirAllFailed(t *testing.T) {
	dir := t.TempDir()

	blocked := filepath.Join(dir, "blocked")
	if err := os.WriteFile(blocked, []byte("x"), 0o600); err != nil {
		t.Fatalf("write blocking file: %v", err)
	}

	cfg := config.Configuration{
		Port:      50001,
		DBPath:    filepath.Join(blocked, "pvmss.db"),
		LogLevel:  testStoreLogLevel,
		LogFormat: testStoreLogFormat,
		LogOutput: testStoreLogOutput,
	}

	if _, err := store.Open(cfg); err == nil {
		t.Fatalf("expected error when MkdirAll fails, got nil")
	}
}

//nolint:paralleltest // serial: shared database fixture
func TestOpen(t *testing.T) {
	cases := []struct {
		name       string
		dbPath     string
		wantErr    bool
		wantOpenDB bool
	}{
		{
			name:       "fresh data dir creates and migrates",
			dbPath:     "pvmss.db",
			wantOpenDB: true,
		},
		{
			name:    "empty db path returns error",
			dbPath:  "",
			wantErr: true,
		},
		{
			name:    "invalid db path returns error",
			dbPath:  "/dev/null/invalid/pvmss.db",
			wantErr: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()

			dbPath := c.dbPath
			if !filepath.IsAbs(dbPath) {
				dbPath = filepath.Join(dir, dbPath)
			}

			cfg := config.Configuration{
				Port:      50001,
				DBPath:    dbPath,
				LogLevel:  testStoreLogLevel,
				LogFormat: testStoreLogFormat,
				LogOutput: testStoreLogOutput,
			}

			s, err := store.Open(cfg)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("Open: %v", err)
			}

			defer func() { _ = s.Close() }()

			if _, err := os.Stat(cfg.DBPath); err != nil {
				t.Fatalf("database file not created: %v", err)
			}

			if err := s.Ping(context.Background()); err != nil {
				t.Fatalf("Ping: %v", err)
			}

			db, err := sql.Open("sqlite", cfg.DBPath)
			if err != nil {
				t.Fatalf("open database for verification: %v", err)
			}
			defer func() { _ = db.Close() }()

			var version int
			if err := db.QueryRowContext(
				context.Background(),
				`SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1`,
			).Scan(&version); err != nil {
				t.Fatalf("query schema_migrations: %v", err)
			}

			if version != len(store.Migrations) {
				t.Fatalf("expected migration version %d, got %d", len(store.Migrations), version)
			}
		})
	}
}
