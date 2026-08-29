package store

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// AuditEntry is one recorded VM write (T05 data-model.md). Write-only from
// this tranche's perspective — no read endpoint is in scope here (that's an
// admin-facing concern, T14); QueryAudit exists only so tests can assert what
// was recorded.
type AuditEntry struct {
	ID        int64
	Actor     string
	Cluster   string
	VMID      int
	Action    string
	Timestamp time.Time
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
	return s.insertAuditRow(ctx, actor, cluster, &vmid, action, "", "", "", "", deriveSeverity(action))
}

// RecordAdminAction inserts one audit_log row for an admin mutation that has
// no VM scope (cluster="", vmid=nil). The detail string must be structured
// JSON: {"summary": "...", "changes": [...]}. Like RecordAction, an insert
// failure is logged and never returned, so auditing cannot break the mutation.
func (s *Store) RecordAdminAction(ctx context.Context, actor, action, targetType, targetID, detail, ip string) error {
	return s.insertAuditRow(ctx, actor, "", nil, action, targetType, targetID, detail, ip, deriveSeverity(action))
}

// insertAuditRow writes one audit_log row with all columns. It never returns
// an error that breaks the caller: on insert failure it logs the error via
// the package-level slog default and returns nil. The rule lives here, not in
// each caller, so no audit site can accidentally let a DB error propagate into
// the request path.
//
//nolint:gosec // table/column names are hardcoded literals, not user input
func (s *Store) insertAuditRow(ctx context.Context, actor, cluster string, vmid *int, action, targetType, targetID, detail, ipAddress, severity string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO audit_log (actor, cluster, vmid, action, timestamp, target_type, target_id, detail, ip_address, severity) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		actor, cluster, vmid, action, time.Now().UTC().Format(time.RFC3339Nano), targetType, targetID, detail, ipAddress, severity)
	if err != nil {
		slog.Error("audit log insert failed", "component", "store", "actor", actor, "action", action, "error", err)
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
		`SELECT id, actor, cluster, vmid, action, timestamp FROM audit_log ORDER BY id`)
	if err != nil {
		return nil, fmt.Errorf("query audit log: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var entries []AuditEntry

	for rows.Next() {
		var (
			entry AuditEntry
			ts    string
		)
		if err := rows.Scan(&entry.ID, &entry.Actor, &entry.Cluster, &entry.VMID, &entry.Action, &ts); err != nil {
			return nil, fmt.Errorf("scan audit log: %w", err)
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
	listSQL := "SELECT id, actor, cluster, vmid, action, timestamp FROM audit_log " +
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
		)
		if err := rows.Scan(&entry.ID, &entry.Actor, &entry.Cluster, &entry.VMID, &entry.Action, &ts); err != nil {
			return AuditPage{}, fmt.Errorf("scan audit log: %w", err)
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

	if len(clauses) == 0 {
		return "", args
	}

	return "WHERE " + strings.Join(clauses, " AND "), args
}
