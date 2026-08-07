package store

import (
	"context"
	"fmt"
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
