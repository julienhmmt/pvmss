//nolint:goconst // test fixture strings
package store_test

import (
	"context"
	"path/filepath"
	"pvmss/server/internal/config"
	"pvmss/server/internal/store"
	"testing"
	"time"
)

// newAuditStore opens a fully-migrated Store for audit tests. The audit_log
// table arrives in schemaV6 (T05), so a fresh Open already has it.
func newAuditStore(t *testing.T) *store.Store {
	t.Helper()
	cfg := config.Configuration{
		Port:      50001,
		DBPath:    filepath.Join(t.TempDir(), "audit.db"),
		LogLevel:  "info",
		LogFormat: "json",
		LogOutput: "stdout",
	}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	return st
}

// TestRecordAction_InsertsOneRowWithRealActor — T005: RecordAction inserts
// exactly one audit_log row carrying the real acting username, never a
// service-account name (FR-009, closes S01's traceability gap).
func TestRecordAction_InsertsOneRowWithRealActor(t *testing.T) {
	st := newAuditStore(t)
	ctx := context.Background()

	before := time.Now()

	if err := st.RecordAction(ctx, "alice@pve", "default", 101, "stop"); err != nil {
		t.Fatalf("RecordAction: %v", err)
	}

	after := time.Now()

	rows, err := st.QueryAudit(ctx)
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("audit rows = %d, want 1", len(rows))
	}

	row := rows[0]
	if row.Actor != "alice@pve" {
		t.Errorf("actor = %q, want alice@pve (never a service-account name)", row.Actor)
	}

	if row.Cluster != "default" {
		t.Errorf("cluster = %q, want default", row.Cluster)
	}

	if row.VMID != 101 {
		t.Errorf("vmid = %d, want 101", row.VMID)
	}

	if row.Action != "stop" {
		t.Errorf("action = %q, want stop", row.Action)
	}

	if row.Timestamp.Before(before) || row.Timestamp.After(after) {
		t.Errorf("timestamp %v outside [%v, %v]", row.Timestamp, before, after)
	}
}

// TestRecordAction_AppendsDistinctRows — each write is its own row, in order.
func TestRecordAction_AppendsDistinctRows(t *testing.T) {
	st := newAuditStore(t)
	ctx := context.Background()

	actions := []struct {
		actor, cluster, action string
		vmid                   int
	}{
		{"alice@pve", "default", "start", 100},
		{"bob@pve", "default", "stop", 103},
		{"admin", "default", "delete", 109},
	}
	for _, a := range actions {
		if err := st.RecordAction(ctx, a.actor, a.cluster, a.vmid, a.action); err != nil {
			t.Fatalf("RecordAction %v: %v", a, err)
		}
	}

	rows, err := st.QueryAudit(ctx)
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	if len(rows) != len(actions) {
		t.Fatalf("rows = %d, want %d", len(rows), len(actions))
	}

	for i, want := range actions {
		if rows[i].Actor != want.actor || rows[i].VMID != want.vmid || rows[i].Action != want.action {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], want)
		}
	}
}
