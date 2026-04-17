package database

import (
	"context"
	"database/sql"
	"fmt"
	"pvmss/logger"
)

// allDataTables is the ordered list of tables copied during a RestoreFrom.
// schema_migrations and app_bootstrap are included so the target is fully
// consistent with the source after the restore.
// audit_log is excluded to preserve the current audit trail; the restore
// operation itself is appended after the data is restored.
var allDataTables = []string{
	"schema_migrations",
	"app_bootstrap",
	"vm_limits",
	"node_limits",
	"enabled_nodes",
	"enabled_storages",
	"enabled_isos",
	"enabled_vmbrs",
	"tags",
	"cloudinit_templates",
	"vm_profiles",
	"sftp_config",
}

// RestoreFrom atomically replaces all data in the current database with
// the contents of the SQLite database at srcPath.
//
// Steps:
//  1. Acquire restoreMu to prevent concurrent restore operations.
//  2. Open srcPath with a raw sql.DB (no migrations) and verify it is a
//     completed-bootstrap PVMSS database.
//  3. Acquire a single dedicated connection so ATTACH persists across the
//     transaction boundary.
//  4. ATTACH srcPath as "imported".
//  5. Within one transaction: DELETE + INSERT SELECT for every data table.
//  6. Append audit log entry for the restore operation.
//  7. DETACH and commit.
//
// If any step fails, the current database is left unchanged.
func (s *sqliteDB) RestoreFrom(srcPath string, changedBy string) error {
	s.restoreMu.Lock()
	defer s.restoreMu.Unlock()

	if err := validateSourceDB(srcPath); err != nil {
		return err
	}

	ctx := context.Background()
	conn, err := s.db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection for restore: %w", err)
	}
	defer func() { _ = conn.Close() }()

	if _, err := conn.ExecContext(ctx, `ATTACH DATABASE ? AS imported`, srcPath); err != nil {
		return fmt.Errorf("attach source database: %w", err)
	}
	defer func() {
		if _, err := conn.ExecContext(ctx, `DETACH DATABASE imported`); err != nil {
			logger.Get().Warn().Err(err).Msg("failed to detach imported database during restore")
		}
	}()

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin restore transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	for _, t := range allDataTables {
		if err := copyTable(ctx, tx, t); err != nil {
			return err
		}
	}

	// Append audit entry for the restore operation
	restoreMsg := fmt.Sprintf("restored from %s", srcPath)
	if err := appendAudit(tx, "database", "full_restore", "restore", "", restoreMsg, changedBy); err != nil {
		return fmt.Errorf("append audit for restore: %w", err)
	}

	return tx.Commit()
}

// validateSourceDB opens srcPath as a raw SQLite connection and verifies it is
// a PVMSS database with a completed bootstrap row and compatible schema.
// It does NOT run migrations so the source file is never modified.
func validateSourceDB(srcPath string) error {
	raw, err := sql.Open("sqlite", srcPath)
	if err != nil {
		return fmt.Errorf("open source database: %w", err)
	}
	defer func() { _ = raw.Close() }()

	var completed bool
	err = raw.QueryRow(`SELECT completed FROM app_bootstrap WHERE id = 1`).Scan(&completed)
	if err != nil {
		return fmt.Errorf("source is not a valid PVMSS database: %w", err)
	}
	if !completed {
		return fmt.Errorf("source database bootstrap is not complete")
	}

	// Validate that schema_migrations table exists and has at least one version
	var schemaVersion int
	err = raw.QueryRow(`SELECT version FROM schema_migrations ORDER BY version DESC LIMIT 1`).Scan(&schemaVersion)
	if err != nil {
		return fmt.Errorf("source database schema version unreadable: %w", err)
	}
	if schemaVersion == 0 {
		return fmt.Errorf("source database has no schema migrations")
	}
	// Validate that the source schema version matches the current application version
	if schemaVersion != currentSchemaVersion {
		return fmt.Errorf("source database schema version %d does not match current application version %d", schemaVersion, currentSchemaVersion)
	}

	return nil
}

// copyTable deletes all rows from table t in the current DB then inserts every
// row from the corresponding table in the attached "imported" database.
// Table names come from the hardcoded allDataTables slice, never from user input.
func copyTable(ctx context.Context, tx *sql.Tx, table string) error {
	//nolint:gosec // table is from a hardcoded allowlist, not user input
	if _, err := tx.ExecContext(ctx, "DELETE FROM "+table); err != nil {
		return fmt.Errorf("clear table %s: %w", table, err)
	}
	//nolint:gosec // table is from a hardcoded allowlist, not user input
	if _, err := tx.ExecContext(ctx, "INSERT INTO "+table+" SELECT * FROM imported."+table); err != nil {
		return fmt.Errorf("copy table %s: %w", table, err)
	}
	return nil
}
