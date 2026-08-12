package store

import (
	"context"
	"fmt"
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

// RecordAction inserts one audit_log row carrying the real acting username —
// never a service-account name (FR-009, closes S01's traceability gap). The
// timestamp is server-side; a caller cannot supply it.
func (s *Store) RecordAction(ctx context.Context, actor, cluster string, vmid int, action string) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO audit_log (actor, cluster, vmid, action, timestamp) VALUES (?, ?, ?, ?, ?)`,
		actor, cluster, vmid, action, time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}

	return nil
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
