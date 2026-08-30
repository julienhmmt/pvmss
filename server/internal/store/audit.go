package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// AuditEntry is one recorded row in the audit_log table. It now supports both
// VM-scoped writes and admin mutations (target_type/target_id/detail/ip/severity).
type AuditEntry struct {
	ID         int64
	Actor      string
	Cluster    string
	VMID       *int
	Action     string
	Timestamp  time.Time
	TargetType sql.NullString
	TargetID   sql.NullString
	Detail     sql.NullString
	IPAddress  sql.NullString
	Severity   string
}

// schemaV19 rebuilds audit_log to support admin-action auditing (issue #01).
// The original schema (V6) had only VM-scoped columns and a NOT NULL vmid,
// which made it impossible to record admin mutations (cluster credentials,
// policy, catalog toggles, db import/export) that have no vmid. The rebuild
// makes vmid nullable and adds target_type, target_id, detail (JSON),
// ip_address, and severity (derived from the action verb, default 'info').
// Existing rows keep their vmid and receive severity='info'. SQLite cannot
// drop a NOT NULL constraint in place, so the table is rebuilt like schemaV4.
const schemaV19 = `
ALTER TABLE audit_log RENAME TO audit_log_v18;
CREATE TABLE audit_log (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	actor      TEXT NOT NULL,
	cluster    TEXT NOT NULL,
	vmid       INTEGER,
	action     TEXT NOT NULL,
	timestamp  TEXT NOT NULL,
	target_type TEXT,
	target_id   TEXT,
	detail      TEXT,
	ip_address  TEXT,
	severity    TEXT NOT NULL DEFAULT 'info'
);
INSERT INTO audit_log (id, actor, cluster, vmid, action, timestamp, severity)
	SELECT id, actor, cluster, vmid, action, timestamp, 'info' FROM audit_log_v18;
DROP TABLE audit_log_v18;
`

// RecordAction inserts one audit_log row carrying the real acting username —
// never a service-account name (FR-009, closes S01's traceability gap). The
// timestamp is server-side; a caller cannot supply it. The 15 existing VM
// callers are unchanged; new columns receive empty defaults and the severity
// is derived from the action verb. An audit write failure is logged and
// swallowed so it can never prevent the action it records (spec decision:
// "l'audit ne peut pas casser l'action qu'il enregistre").
func (s *Store) RecordAction(ctx context.Context, actor, cluster string, vmid int, action string) error {
	return s.insertAuditRow(ctx, auditRow{
		Actor:    actor,
		Cluster:  cluster,
		VMID:     &vmid,
		Action:   action,
		Severity: deriveSeverity(action),
	})
}

// RecordAdminAction inserts one audit_log row for an admin mutation that has
// no VM scope (cluster="", vmid=nil). The detail string must be structured
// JSON: {"summary": "...", "changes": [...]}. Like RecordAction, an insert
// failure is logged and never returned, so auditing cannot break the mutation.
func (s *Store) RecordAdminAction(ctx context.Context, actor, action, targetType, targetID, detail, ip string) error {
	return s.insertAuditRow(ctx, auditRow{
		Actor:      actor,
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     detail,
		IPAddress:  ip,
		Severity:   deriveSeverity(action),
	})
}

// auditRow groups the columns written to audit_log so insertAuditRow stays
// under the go:S107 parameter-count limit.
type auditRow struct {
	Actor      string
	Cluster    string
	VMID       *int
	Action     string
	TargetType string
	TargetID   string
	Detail     string
	IPAddress  string
	Severity   string
}

// insertAuditRow writes one audit_log row with all columns. It never returns
// an error that breaks the caller: on insert failure it logs the error via
// the package-level slog default and returns nil. The rule lives here, not in
// each caller, so no audit site can accidentally let a DB error propagate into
// the request path.
func (s *Store) insertAuditRow(ctx context.Context, row auditRow) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO audit_log (actor, cluster, vmid, action, timestamp, target_type, target_id, detail, ip_address, severity) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.Actor, row.Cluster, row.VMID, row.Action, time.Now().UTC().Format(time.RFC3339Nano),
		row.TargetType, row.TargetID, row.Detail, row.IPAddress, row.Severity)
	if err != nil {
		slog.Error("audit log insert failed", "component", "store", "actor", row.Actor, "action", row.Action, "error", err)
		return nil
	}

	return nil
}

// deriveSeverity maps an action string to one of three severity levels based
// on its verb (spec decision: 3 levels, hardcoded, not configurable). The
// matching is substring-based to cover both dotted (admin.db_import.rejected)
// and underscored (auth.login_failed) action vocabularies, matching pegaprox's
// approach.
//   - critical: contains fail, denied, rejected (e.g. auth.login_failed,
//     admin.db_import.rejected, auth.csrf_rejected)
//   - warning:  contains delete, remove, destroy, revoke (e.g. admin.tags.delete)
//   - info:     everything else (e.g. admin.clusters.create, vm.power_on)
func deriveSeverity(action string) string {
	switch {
	case strings.Contains(action, "fail"),
		strings.Contains(action, "denied"),
		strings.Contains(action, "rejected"):
		return "critical"
	case strings.Contains(action, "delete"),
		strings.Contains(action, "remove"),
		strings.Contains(action, "destroy"),
		strings.Contains(action, "revoke"):
		return "warning"
	default:
		return "info"
	}
}

// QueryAudit returns every audit_log row in insertion order. Test-only at this
// tranche — production reads belong to T14's admin audit view.
func (s *Store) QueryAudit(ctx context.Context) ([]AuditEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, actor, cluster, vmid, action, timestamp, target_type, target_id, detail, ip_address, severity FROM audit_log ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query audit log: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []AuditEntry

	for rows.Next() {
		var (
			entry AuditEntry
			ts    string
			vmid  sql.NullInt64
		)
		if err := rows.Scan(&entry.ID, &entry.Actor, &entry.Cluster, &vmid, &entry.Action, &ts,
			&entry.TargetType, &entry.TargetID, &entry.Detail, &entry.IPAddress, &entry.Severity); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
		}

		if vmid.Valid {
			v := int(vmid.Int64)
			entry.VMID = &v
		}

		entry.Timestamp, err = time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return nil, fmt.Errorf("parse audit timestamp %q: %w", ts, err)
		}

		entries = append(entries, entry)
	}

	return entries, rows.Err()
}

// AuditFilter holds the optional, AND-combined filters for ListAuditLog
// (T14 data-model.md). Every field is optional; a zero value (empty string,
// nil pointer) means "no filter on this field." Page is 1-based; PageSize
// is capped by the caller (the HTTP handler enforces the configured maximum).
type AuditFilter struct {
	Cluster  string
	VMID     *int
	Actor    string
	Action   string
	From, To *time.Time
	Severity *string
	Page     int
	PageSize int
}

// AuditPage is the paginated envelope returned by ListAuditLog, matching
// T04's established list convention (contracts/vms-list.md) rather than
// inventing a second pagination shape.
type AuditPage struct {
	Items    []AuditEntry
	Total    int
	Page     int
	PageSize int
}

// ListAuditLog returns a filtered, paginated view of T05's audit_log table,
// most recent first (T14 FR-001/FR-002). No schema change, no new action
// string — a single SELECT with optional WHERE clauses and LIMIT/OFFSET.
// A nil Items slice is never returned; an empty result yields a non-nil
// zero-length slice.
func (s *Store) ListAuditLog(ctx context.Context, f AuditFilter) (AuditPage, error) {
	whereClause, args := buildAuditWhere(f)

	// Total count — one scalar query, independent of pagination.
	var total int

	countSQL := "SELECT COUNT(*) FROM audit_log " + whereClause
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return AuditPage{}, fmt.Errorf("count audit log: %w", err)
	}

	// Page defaults — guard against zero values from a malformed request.
	page := max(f.Page, 1)

	pageSize := f.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	offset := (page - 1) * pageSize

	//nolint:gosec // whereClause is built from hardcoded literals, not user input
	listSQL := "SELECT id, actor, cluster, vmid, action, timestamp, target_type, target_id, detail, ip_address, severity FROM audit_log " +
		whereClause + " ORDER BY timestamp DESC, id DESC LIMIT ? OFFSET ?"

	listArgs := make([]any, 0, len(args)+2)
	listArgs = append(listArgs, args...)
	listArgs = append(listArgs, pageSize, offset)

	rows, err := s.db.QueryContext(ctx, listSQL, listArgs...)
	if err != nil {
		return AuditPage{}, fmt.Errorf("query audit log: %w", err)
	}
	defer func() { _ = rows.Close() }()

	items := make([]AuditEntry, 0)

	for rows.Next() {
		var (
			entry AuditEntry
			ts    string
			vmid  sql.NullInt64
		)
		if err := rows.Scan(&entry.ID, &entry.Actor, &entry.Cluster, &vmid, &entry.Action, &ts,
			&entry.TargetType, &entry.TargetID, &entry.Detail, &entry.IPAddress, &entry.Severity); err != nil {
			return AuditPage{}, fmt.Errorf("scan audit log: %w", err)
		}

		if vmid.Valid {
			v := int(vmid.Int64)
			entry.VMID = &v
		}

		entry.Timestamp, err = time.Parse(time.RFC3339Nano, ts)
		if err != nil {
			return AuditPage{}, fmt.Errorf("parse audit timestamp %q: %w", ts, err)
		}

		items = append(items, entry)
	}

	if err := rows.Err(); err != nil {
		return AuditPage{}, fmt.Errorf("iterate audit log: %w", err)
	}

	return AuditPage{Items: items, Total: total, Page: page, PageSize: pageSize}, nil
}

// buildAuditWhere constructs the WHERE clause and args for an AuditFilter.
// All filters combine with AND; an empty filter yields an empty WHERE clause.
func buildAuditWhere(f AuditFilter) (string, []any) {
	var (
		clauses []string
		args    []any
	)

	if f.Cluster != "" {
		clauses = append(clauses, "cluster = ?")
		args = append(args, f.Cluster)
	}

	if f.VMID != nil {
		clauses = append(clauses, "vmid = ?")
		args = append(args, *f.VMID)
	}

	if f.Actor != "" {
		clauses = append(clauses, "actor = ?")
		args = append(args, f.Actor)
	}

	if f.Action != "" {
		clauses = append(clauses, "action = ?")
		args = append(args, f.Action)
	}

	if f.From != nil {
		clauses = append(clauses, "timestamp >= ?")
		args = append(args, f.From.UTC().Format(time.RFC3339Nano))
	}

	if f.To != nil {
		clauses = append(clauses, "timestamp <= ?")
		args = append(args, f.To.UTC().Format(time.RFC3339Nano))
	}

	if f.Severity != nil && *f.Severity != "" {
		clauses = append(clauses, "severity = ?")
		args = append(args, *f.Severity)
	}

	if len(clauses) == 0 {
		return "", args
	}

	return "WHERE " + strings.Join(clauses, " AND "), args
}

// minAuditRetentionDays is the floor below which retention cannot be set.
// Enforced by SetAuditConfig so a UI bug or bad API call cannot silently
// erase the audit trail by setting retention to 0 or 1 day.
const minAuditRetentionDays = 30

// AuditConfig is the single-row audit retention configuration (issue #02).
type AuditConfig struct {
	RetentionDays int
}

// GetAuditConfig returns the current audit retention in days. The audit_config
// table is seeded by schemaV20 with 365 days, so this always returns a value
// after migrations have run.
func (s *Store) GetAuditConfig(ctx context.Context) (AuditConfig, error) {
	var days int

	err := s.db.QueryRowContext(ctx,
		`SELECT retention_days FROM audit_config WHERE id = 1`,
	).Scan(&days)
	if err != nil {
		return AuditConfig{}, fmt.Errorf("get audit config: %w", err)
	}

	return AuditConfig{RetentionDays: days}, nil
}

// SetAuditConfig updates the audit retention in days. It rejects values below
// minAuditRetentionDays (30) so a caller cannot shrink the window enough to
// erase the trail before an incident review can happen.
func (s *Store) SetAuditConfig(ctx context.Context, retentionDays int) error {
	if retentionDays < minAuditRetentionDays {
		return fmt.Errorf("retention_days must be at least %d, got %d", minAuditRetentionDays, retentionDays)
	}

	_, err := s.db.ExecContext(ctx,
		`UPDATE audit_config SET retention_days = ? WHERE id = 1`, retentionDays)
	if err != nil {
		return fmt.Errorf("set audit config: %w", err)
	}

	return nil
}

// PruneAuditLog deletes audit_log rows older than retentionDays and returns the
// number of rows deleted. A retention of 30 deletes rows whose timestamp is
// older than 30 days from now (UTC). Safe to run concurrently with reads and
// inserts — SQLite serializes writers, and the DELETE is a single statement.
func (s *Store) PruneAuditLog(ctx context.Context, retentionDays int) (int64, error) {
	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour).Format(time.RFC3339Nano)

	res, err := s.db.ExecContext(ctx,
		`DELETE FROM audit_log WHERE timestamp < ?`, cutoff)
	if err != nil {
		return 0, fmt.Errorf("prune audit log: %w", err)
	}

	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("prune audit log rows affected: %w", err)
	}

	return n, nil
}

// CountAuditPrunePreview returns the number of audit_log rows older than
// retentionDays, without deleting them. Used by the UI confirmation flow so
// the admin sees the impact before confirming a retention change.
func (s *Store) CountAuditPrunePreview(ctx context.Context, retentionDays int) (int64, error) {
	cutoff := time.Now().UTC().Add(-time.Duration(retentionDays) * 24 * time.Hour).Format(time.RFC3339Nano)

	var n int64

	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE timestamp < ?`, cutoff,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count audit prune preview: %w", err)
	}

	return n, nil
}
