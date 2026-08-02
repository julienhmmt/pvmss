package store

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

// RunMigrations applies every pending migration in order.
// Already-applied versions are skipped. The list must be ordered by version.
func RunMigrations(db *sql.DB, migrations []Migration) error {
	if err := validateMigrations(migrations); err != nil {
		return err
	}
	if err := ensureMigrationsTable(db); err != nil {
		return fmt.Errorf("ensure migrations table: %w", err)
	}
	applied, err := appliedVersions(db, len(migrations))
	if err != nil {
		return fmt.Errorf("query applied migrations: %w", err)
	}
	for _, m := range migrations {
		if _, ok := applied[m.Version]; ok {
			continue
		}
		if err := applyMigration(db, m); err != nil {
			return fmt.Errorf("apply migration %d: %w", m.Version, err)
		}
	}
	return nil
}

func validateMigrations(migrations []Migration) error {
	if len(migrations) == 0 {
		return fmt.Errorf("migration list is empty")
	}

	var previous int
	for i, m := range migrations {
		if m.Version < 1 {
			return fmt.Errorf("migration version %d is not positive", m.Version)
		}
		if i > 0 {
			if m.Version == previous {
				return fmt.Errorf("migration version %d is duplicated", m.Version)
			}
			if m.Version < previous {
				return fmt.Errorf("migration version %d is out of order", m.Version)
			}
		}
		if m.Version != i+1 {
			return fmt.Errorf("migration version %d is missing from the migration list", i+1)
		}
		if strings.TrimSpace(m.DDL) == "" {
			return fmt.Errorf("migration %d has no DDL", m.Version)
		}
		previous = m.Version
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

func appliedVersions(db *sql.DB, hint int) (map[int]struct{}, error) {
	rows, err := db.Query(`SELECT version FROM schema_migrations`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	result := make(map[int]struct{}, hint)
	for rows.Next() {
		var v int
		if err := rows.Scan(&v); err != nil {
			return nil, fmt.Errorf("scan version: %w", err)
		}
		result[v] = struct{}{}
	}
	return result, rows.Err()
}

func applyMigration(db *sql.DB, m Migration) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if _, err := tx.Exec(m.DDL); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("exec DDL: %w", err)
	}
	appliedAt := time.Now().UTC().Format(time.RFC3339)
	if _, err := tx.Exec(`INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)`, m.Version, appliedAt); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("record migration: %w", err)
	}
	return tx.Commit()
}
