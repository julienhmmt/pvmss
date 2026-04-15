package database

import (
	"database/sql"
	"fmt"
)

// AuditEntry represents a single row from the audit_log table.
type AuditEntry struct {
	ID        int64  `json:"id"`
	TableName string `json:"table_name"`
	RecordID  string `json:"record_id"`
	Action    string `json:"action"`
	OldValue  string `json:"old_value,omitempty"`
	NewValue  string `json:"new_value,omitempty"`
	ChangedBy string `json:"changed_by"`
	ChangedAt string `json:"changed_at"`
}

// appendAudit inserts one audit row into audit_log within an existing transaction.
// Callers are responsible for committing or rolling back tx.
func appendAudit(tx *sql.Tx, table, recordID, action, oldVal, newVal, changedBy string) error {
	_, err := tx.Exec(`
		INSERT INTO audit_log (table_name, record_id, action, old_value, new_value, changed_by)
		VALUES (?, ?, ?, ?, ?, ?)
	`, table, recordID, action, nullableString(oldVal), nullableString(newVal), changedBy)
	if err != nil {
		return fmt.Errorf("append audit %s/%s: %w", table, recordID, err)
	}
	return nil
}

// nullableString converts an empty string to nil (stored as NULL) and non-empty to the string.
func nullableString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

// ListAuditLog returns audit entries ordered by most recent first.
// tableFilter limits results to a specific table when non-empty.
// limit=0 returns all matching entries; offset is applied regardless.
func (s *sqliteDB) ListAuditLog(tableFilter string, limit int, offset int) ([]AuditEntry, error) {
	query, args := buildAuditQuery(tableFilter, limit, offset)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query audit_log: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanAuditEntries(rows)
}

// buildAuditQuery constructs the SELECT statement and argument list for ListAuditLog.
func buildAuditQuery(tableFilter string, limit int, offset int) (string, []interface{}) {
	base := `SELECT id, table_name, record_id, action,
	                COALESCE(old_value,''), COALESCE(new_value,''),
	                changed_by, changed_at
	         FROM audit_log`
	args := []interface{}{}
	if tableFilter != "" {
		base += ` WHERE table_name = ?`
		args = append(args, tableFilter)
	}
	base += ` ORDER BY changed_at DESC, id DESC`
	if limit > 0 {
		base += ` LIMIT ? OFFSET ?`
		args = append(args, limit, offset)
	}
	return base, args
}

// scanAuditEntries scans all rows into a slice of AuditEntry.
func scanAuditEntries(rows *sql.Rows) ([]AuditEntry, error) {
	entries := []AuditEntry{}
	for rows.Next() {
		var e AuditEntry
		if err := rows.Scan(&e.ID, &e.TableName, &e.RecordID, &e.Action,
			&e.OldValue, &e.NewValue, &e.ChangedBy, &e.ChangedAt); err != nil {
			return nil, fmt.Errorf("scan audit entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}
