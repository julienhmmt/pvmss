//nolint:wsl_v5 // startup persistence setup keeps key and seed handling adjacent
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"pvmss/server/internal/config"

	_ "modernc.org/sqlite" // pure-Go SQLite driver, registers "sqlite"
)

// Open opens the SQLite database, enables WAL mode, and applies pending migrations.
func Open(cfg config.Configuration) (*Store, error) {
	if cfg.DBPath == "" {
		return nil, errors.New("DBPath is required")
	}

	dir := filepath.Dir(cfg.DBPath)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create database directory %q: %w", dir, err)
	}

	dsn := fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", cfg.DBPath)

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database %q: %w", cfg.DBPath, err)
	}

	// SQLite is a single-file database. A single connection serializes writers
	// while WAL mode allows concurrent readers, which is sufficient for this
	// application and avoids "database is locked" errors under load.
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	ctx := context.Background()

	if _, err := db.ExecContext(ctx, `PRAGMA journal_mode=WAL`); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}

	if err := RunMigrations(ctx, db, Migrations); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}

	st := &Store{db: db, staging: NewImportStaging()}
	if cfg.SessionSecret == "" {
		return st, nil
	}

	key, err := deriveEncryptionKey(cfg.SessionSecret)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	st.encryptionKey = key

	// Demo cluster seeds are written only when the operator has selected the
	// fake cluster source — i.e. a non-production deployment. A real Proxmox
	// instance must never start with hardcoded demo credentials in its database.
	if cfg.ClusterSource == "fake" {
		if err := st.ensureSeedClusters(ctx); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("seed clusters: %w", err)
		}
	}

	return st, nil
}
