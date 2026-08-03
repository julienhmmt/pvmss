package database

import (
	"database/sql"
	"fmt"
	"time"
)

// currentSchemaVersion is the highest known migration version.
const currentSchemaVersion = 3

// migrations maps each schema version to its forward-only DDL.
// Versions must be consecutive integers starting at 1.
var migrations = map[int]string{
	1: schemaV1,
	2: schemaV2,
	3: schemaV3,
}

// RunMigrations applies every pending migration to db in order.
// Already-applied versions are skipped. Idempotent on repeated calls.
func RunMigrations(db *sql.DB) error {
	if err := ensureMigrationsTable(db); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}
	applied, err := appliedVersions(db)
	if err != nil {
		return err
	}
	for v := 1; v <= currentSchemaVersion; v++ {
		if applied[v] {
			continue
		}
		if err := applyMigration(db, v); err != nil {
			return fmt.Errorf("apply migration v%d: %w", v, err)
		}
	}
	return nil
}

// ensureMigrationsTable creates schema_migrations if it does not yet exist.
// Called before any version queries so the table is always available.
func ensureMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`)
	return err
}

// appliedVersions returns a set of migration versions already recorded in schema_migrations.
func appliedVersions(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, fmt.Errorf("query schema_migrations: %w", err)
	}
	defer func() { _ = rows.Close() }()
	result := make(map[int]bool)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		result[v] = true
	}
	return result, rows.Err()
}

// applyMigration executes the DDL for version v inside a transaction.
// Records the version in schema_migrations on success; rolls back on error.
func applyMigration(db *sql.DB, version int) error {
	ddl, ok := migrations[version]
	if !ok {
		return fmt.Errorf("no DDL registered for version %d", version)
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(ddl); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("exec DDL: %w", err)
	}
	appliedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, version, appliedAt); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record migration: %w", err)
	}
	return tx.Commit()
}
