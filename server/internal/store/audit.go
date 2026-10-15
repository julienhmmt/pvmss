package store

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"pvmss/server/internal/logctx"
	"pvmss/server/internal/telemetry"
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

// RecordAction inserts one audit_log row carrying the real acting username -
// never a service-account name (closes traceability gap). The
// timestamp is server-side; a caller cannot supply it. The 15 existing VM
// callers are unchanged; new columns receive empty defaults and the severity
// is derived from the action verb. An audit write failure is logged and
// swallowed so it can never prevent the action it records.
func (s *Store) RecordAction(ctx context.Context, actor, cluster string, vmid int, action string) error {
	return s.insertAuditRow(ctx, auditRow{
		Actor:    actor,
		Cluster:  cluster,
		VMID:     &vmid,
		Action:   action,
		Severity: deriveSeverity(action),
	})
}

// RecordVMActionDetail is RecordAction plus a structured JSON detail
// ({"summary": "...", "changes": [...]}) and the VM as the audit target.
func (s *Store) RecordVMActionDetail(ctx context.Context, actor, cluster string, vmid int, action, detail string) error {
	return s.insertAuditRow(ctx, auditRow{
		Actor:      actor,
		Cluster:    cluster,
		VMID:       &vmid,
		Action:     action,
		TargetType: "vm",
		TargetID:   fmt.Sprintf("%s:%d", cluster, vmid),
		Detail:     detail,
		Severity:   deriveSeverity(action),
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

// auditTimeLayout is fixed-width (always nine fractional digits) so the
// stored strings sort in time order. RFC3339Nano trims trailing zeros, which
// put ".12Z" after ".1234Z".
const auditTimeLayout = "2006-01-02T15:04:05.000000000Z07:00"

func auditTimestamp(t time.Time) string {
	return t.UTC().Format(auditTimeLayout)
}

// insertAuditRow writes one audit_log row with all columns. It never returns
// an error that breaks the caller: on insert failure it logs the error via
// the store logger and returns nil. The rule lives here, not in
// each caller, so no audit site can accidentally let a DB error propagate into
// the request path.
func (s *Store) insertAuditRow(ctx context.Context, row auditRow) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO audit_log (actor, cluster, vmid, action, timestamp, target_type, target_id, detail, ip_address, severity) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		row.Actor, row.Cluster, row.VMID, row.Action, auditTimestamp(time.Now()),
		row.TargetType, row.TargetID, row.Detail, row.IPAddress, row.Severity)
	if err != nil {
		s.log().ErrorContext(ctx, "audit log insert failed", "component", "store", "actor", row.Actor, "action", row.Action, "error", err)
	}

	s.mirrorAudit(ctx, row)

	return nil
}

// maxLoggedActor bounds an actor that may be attacker-supplied (a submitted
// login name) before it reaches the log stream.
const maxLoggedActor = 64

// actionRateLimited is logged at Warn although deriveSeverity ranks it info;
// changing the severity would alter /activity.
const actionRateLimited = "auth.rate_limited"

// mirrorAudit emits the audit row on the log stream (single emission point, no
// handler code). The detail JSON is deliberately not logged: it can carry
// values from admin forms. clientIp and requestId come from the request logger.
func (s *Store) mirrorAudit(ctx context.Context, row auditRow) {
	attrs := []slog.Attr{
		slog.String("component", "store"),
		slog.String("event", "audit"),
		slog.String("actor", truncate(row.Actor, maxLoggedActor)),
		slog.String("action", row.Action),
	}
	if row.Cluster != "" {
		attrs = append(attrs, slog.String("cluster", row.Cluster))
	}

	if row.VMID != nil {
		attrs = append(attrs, slog.Int("vmid", *row.VMID))
	}

	if row.TargetType != "" {
		attrs = append(attrs, slog.String("targetType", row.TargetType), slog.String("targetId", truncate(row.TargetID, maxLoggedActor)))
	}

	level := slog.LevelInfo
	if row.Severity != "info" || row.Action == actionRateLimited {
		level = slog.LevelWarn
	}

	logctx.FromOr(ctx, s.log()).LogAttrs(ctx, level, "audit event", attrs...)

	// VM-scoped rows feed the pvmss.vm.actions counter; "critical" severity
	// (fail/denied/rejected verbs) counts as a failed action.
	if row.VMID != nil {
		result := "success"
		if row.Severity == "critical" {
			result = "failure"
		}

		telemetry.RecordVMAction(ctx, row.Action, result)
	}
}

func truncate(v string, limit int) string {
	if len(v) <= limit {
		return v
	}

	return v[:limit]
}

// deriveSeverity maps an action string to one of three severity levels based
// on its verb (3 levels, hardcoded, not configurable). The
// matching is substring-based to cover both dotted (admin.db_import.rejected)
// and underscored (auth.login_failed) action vocabularies, matching pegaprox's
// approach.
// - critical: contains fail, denied, rejected (e.g. auth.login_failed,
// admin.db_import.rejected, auth.csrf_rejected)
//   - warning: contains delete, remove, destroy, revoke (e.g. admin.tags.delete)
//   - info: everything else (e.g. admin.clusters.create, vm.power_on)
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

// QueryAudit returns every audit_log row in insertion order. Test-only for
// now - production reads belong to the admin audit view.
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

// AuditFilter holds the optional, AND-combined filters for ListAuditLog.
// Every field is optional; a zero value (empty string,
// nil pointer) means "no filter on this field." Page is 1-based; PageSize
// is capped by the caller (the HTTP handler enforces the configured maximum).
type AuditFilter struct {
	Cluster string
	VMID    *int
	// CurrentIncarnation limits VMID-scoped results to the rows of the VM
	// that currently owns cluster+vmid (see buildAuditWhere).
	CurrentIncarnation bool
	Actor              string
	Action             string
	TargetType         string
	TargetID           string
	From, To           *time.Time
	Severity           *string
	Page               int
	PageSize           int
}

// AuditPage is the paginated envelope returned by ListAuditLog, matching
// the established list convention rather than
// inventing a second pagination shape.
type AuditPage struct {
	Items    []AuditEntry
	Total    int
	Page     int
	PageSize int
}

// ListAuditLog returns a filtered, paginated view of the audit_log table,
// most recent first. No schema change, no new action
// string - a single SELECT with optional WHERE clauses and LIMIT/OFFSET.
// A nil Items slice is never returned; an empty result yields a non-nil
// zero-length slice.
func (s *Store) ListAuditLog(ctx context.Context, f AuditFilter) (AuditPage, error) {
	whereClause, args := buildAuditWhere(f)

	// Total count - one scalar query, independent of pagination.
	var total int

	countSQL := "SELECT COUNT(*) FROM audit_log " + whereClause
	if err := s.db.QueryRowContext(ctx, countSQL, args...).Scan(&total); err != nil {
		return AuditPage{}, fmt.Errorf("count audit log: %w", err)
	}

	// Page defaults - guard against zero values from a malformed request.
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

// appendIncarnationClauses restricts a VMID-scoped query to the current
// incarnation of the VM when f.CurrentIncarnation is set. Proxmox reuses VMIDs:
// keep only rows at or after the latest vm_create and strictly after the latest
// delete of this cluster+vmid. Compared by id (AUTOINCREMENT insertion order),
// not timestamp: rows written before the fixed-width auditTimeLayout carry
// RFC3339Nano strings that do not sort chronologically. Callers must ensure f.VMID is non-nil.
func appendIncarnationClauses(clauses []string, args []any, f AuditFilter) ([]string, []any) {
	if !f.CurrentIncarnation || f.Cluster == "" {
		return clauses, args
	}

	clauses = append(clauses,
		"id >= COALESCE((SELECT MAX(id) FROM audit_log WHERE cluster = ? AND vmid = ? AND action = 'vm_create'), 0)",
		"id > COALESCE((SELECT MAX(id) FROM audit_log WHERE cluster = ? AND vmid = ? AND action = 'delete'), 0)")
	for range 2 {
		args = append(args, f.Cluster, *f.VMID)
	}

	return clauses, args
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
		clauses, args = appendIncarnationClauses(clauses, args, f)
	}

	if f.Actor != "" {
		clauses = append(clauses, "actor = ?")
		args = append(args, f.Actor)
	}

	if f.Action != "" {
		clauses = append(clauses, "action = ?")
		args = append(args, f.Action)
	}

	if f.TargetType != "" {
		clauses = append(clauses, "target_type = ?")
		args = append(args, f.TargetType)
	}

	if f.TargetID != "" {
		clauses = append(clauses, "target_id = ?")
		args = append(args, f.TargetID)
	}

	if f.From != nil {
		clauses = append(clauses, "timestamp >= ?")
		args = append(args, auditTimestamp(*f.From))
	}

	if f.To != nil {
		clauses = append(clauses, "timestamp <= ?")
		args = append(args, auditTimestamp(*f.To))
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

// AuditConfig is the single-row audit retention configuration.
type AuditConfig struct {
	RetentionDays int
}

// GetAuditConfig returns the current audit retention in days. The audit_config
// table is seeded by the baseline migration with 365 days, so this always returns a value
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
// inserts - SQLite serializes writers, and the DELETE is a single statement.
func (s *Store) PruneAuditLog(ctx context.Context, retentionDays int) (int64, error) {
	cutoff := auditTimestamp(time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour))

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
	cutoff := auditTimestamp(time.Now().Add(-time.Duration(retentionDays) * 24 * time.Hour))

	var n int64

	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM audit_log WHERE timestamp < ?`, cutoff,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count audit prune preview: %w", err)
	}

	return n, nil
}
