package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// RunMigrations applies every pending migration in order.
// Already-applied versions are skipped. The map must contain every version from 1 to its maximum.
func RunMigrations(db *sql.DB, migrations map[int]string) error {
	if err := validateMigrations(migrations); err != nil {
		return err
	}
	if err := ensureMigrationsTable(db); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}
	applied, err := appliedVersions(db)
	if err != nil {
		return fmt.Errorf("query applied migrations: %w", err)
	}
	for v := 1; v <= currentVersion(migrations); v++ {
		if applied[v] {
			continue
		}
		if err := applyMigration(db, v, migrations); err != nil {
			return fmt.Errorf("apply migration %d: %w", v, err)
		}
	}
	return nil
}

// currentVersion returns the highest version in a migration map.
func currentVersion(migrations map[int]string) int {
	max := 0
	for v := range migrations {
		if v > max {
			max = v
		}
	}
	return max
}

func validateMigrations(migrations map[int]string) error {
	if len(migrations) == 0 {
		return nil
	}
	for v := range migrations {
		if v < 1 {
			return fmt.Errorf("migration version %d is not positive", v)
		}
		if strings.TrimSpace(migrations[v]) == "" {
			return fmt.Errorf("migration %d has no DDL", v)
		}
	}
	for v := 1; v <= currentVersion(migrations); v++ {
		if _, ok := migrations[v]; !ok {
			return fmt.Errorf("migration version %d is missing from the migration map", v)
		}
	}
	return nil
}

func ensureMigrationsTable(db *sql.DB) error {
	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS schema_migrations (
		version    INTEGER PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`)
	return err
}

func appliedVersions(db *sql.DB) (map[int]bool, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
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

func applyMigration(db *sql.DB, version int, migrations map[int]string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	ddl := migrations[version]
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
