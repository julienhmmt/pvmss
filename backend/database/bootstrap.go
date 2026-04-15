package database

import (
	"database/sql"
	"errors"
	"fmt"
)

// IsBootstrapComplete returns true when the first-run setup wizard has been
// completed, i.e. app_bootstrap.completed = 1. Returns false (no error) when
// no row exists yet (fresh database).
func (s *sqliteDB) IsBootstrapComplete() (bool, error) {
	var completed bool
	err := s.db.QueryRow(`SELECT completed FROM app_bootstrap WHERE id = 1`).Scan(&completed)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("query app_bootstrap: %w", err)
	}
	return completed, nil
}

// CompleteBootstrap marks the first-run setup as complete, recording the
// application version that completed it. Safe to call multiple times (UPSERT).
func (s *sqliteDB) CompleteBootstrap(version string) error {
	_, err := s.db.Exec(`
		INSERT INTO app_bootstrap (id, completed, completed_at, version)
		VALUES (1, 1, CURRENT_TIMESTAMP, ?)
		ON CONFLICT(id) DO UPDATE SET
			completed    = 1,
			completed_at = CURRENT_TIMESTAMP,
			version      = excluded.version
	`, version)
	if err != nil {
		return fmt.Errorf("complete bootstrap: %w", err)
	}
	return nil
}
