//nolint:goconst // test fixtures reuse actor/action/cluster strings across seed and assertion sites
package store_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"pvmss/server/internal/config"
	"pvmss/server/internal/store"
	"testing"
	"time"
)

const (
	testStoreLogLevel  = "info"
	testStoreLogFormat = "json"
	testStoreLogOutput = "stdout"
	testStoreCluster   = "default"
	testMigrationDDL   = `CREATE TABLE t1 (id INTEGER PRIMARY KEY)`
	testAuditActor     = "alice@pve"
	testAuditAction    = "start"
)

// newAuditStore opens a fully-migrated Store for audit tests. The audit_log
// table arrives in schemaV6 (T05), so a fresh Open already has it.
func newAuditStore(t *testing.T) *store.Store {
	t.Helper()
	cfg := config.Configuration{
		Port:      50001,
		DBPath:    filepath.Join(t.TempDir(), "audit.db"),
		LogLevel:  testStoreLogLevel,
		LogFormat: testStoreLogFormat,
		LogOutput: testStoreLogOutput,
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
//
//nolint:paralleltest // serial: shared database fixture
func TestRecordAction_InsertsOneRowWithRealActor(t *testing.T) {
	st := newAuditStore(t)
	ctx := context.Background()

	before := time.Now()

	if err := st.RecordAction(ctx, testAuditActor, testStoreCluster, 101, "stop"); err != nil {
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
	if row.Actor != testAuditActor {
		t.Errorf("actor = %q, want alice@pve (never a service-account name)", row.Actor)
	}

	if row.Cluster != testStoreCluster {
		t.Errorf("cluster = %q, want default", row.Cluster)
	}

	if *row.VMID != 101 {
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
//
//nolint:paralleltest // serial: shared database fixture
func TestRecordAction_AppendsDistinctRows(t *testing.T) {
	st := newAuditStore(t)
	ctx := context.Background()

	actions := []struct {
		actor, cluster, action string
		vmid                   int
	}{
		{testAuditActor, testStoreCluster, testAuditAction, 100},
		{"bob@pve", testStoreCluster, "stop", 103},
		{"admin", testStoreCluster, "delete", 109},
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
		if rows[i].Actor != want.actor || *rows[i].VMID != want.vmid || rows[i].Action != want.action {
			t.Errorf("row %d = %+v, want %+v", i, rows[i], want)
		}
	}
}

// seedAuditRows records a fixed set of audit rows for ListAuditLog filter
// tests. The set is ordered oldest-first by insertion; ListAuditLog returns
// most-recent first, so tests assert against the reverse insertion order.
func seedAuditRows(t *testing.T, st *store.Store) {
	t.Helper()

	ctx := context.Background()

	rows := []struct {
		actor, cluster, action string
		vmid                   int
	}{
		{testAuditActor, "default", testAuditAction, 101},
		{"bob@pve", "default", "stop", 102},
		{testAuditActor, "default", "edit_cloudinit_snippet", 101},
		{"admin", "default", "vm_create", 200},
		{testAuditActor, "secondary", testAuditAction, 301},
	}
	for _, r := range rows {
		if err := st.RecordAction(ctx, r.actor, r.cluster, r.vmid, r.action); err != nil {
			t.Fatalf("RecordAction %v: %v", r, err)
		}
	}
}

// TestListAuditLog_NoFilter_ReturnsAllMostRecentFirst — T002: no filter
// returns every row, most recent (last inserted) first, with the pagination
// envelope populated.
//
//nolint:paralleltest // serial: shared database fixture
func TestListAuditLog_NoFilter_ReturnsAllMostRecentFirst(t *testing.T) {
	st := newAuditStore(t)
	ctx := context.Background()

	seedAuditRows(t, st)

	page, err := st.ListAuditLog(ctx, store.AuditFilter{Page: 1, PageSize: 100})
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}

	if page.Total != 5 {
		t.Fatalf("total = %d, want 5", page.Total)
	}

	if len(page.Items) != 5 {
		t.Fatalf("items = %d, want 5", len(page.Items))
	}

	if page.Page != 1 || page.PageSize != 100 {
		t.Errorf("envelope = page %d pageSize %d, want 1/100", page.Page, page.PageSize)
	}
	// Most recent first: last inserted was alice@pve secondary start 301.
	if *page.Items[0].VMID != 301 || page.Items[0].Action != testAuditAction {
		t.Errorf("first item = %+v, want vmid 301 start", page.Items[0])
	}
}

// TestListAuditLog_Filters — T002: every filter combination is applied
// server-side with AND semantics.
//
//nolint:paralleltest // serial: shared database fixture
func TestListAuditLog_Filters(t *testing.T) {
	st := newAuditStore(t)
	ctx := context.Background()

	seedAuditRows(t, st)

	vmid101 := 101
	vmid200 := 200
	fromTime := time.Now().Add(-1 * time.Hour)
	toTime := time.Now().Add(1 * time.Hour)

	tests := []struct {
		name      string
		filter    store.AuditFilter
		wantVMIDs []int
	}{
		{
			name:      "by actor alice",
			filter:    store.AuditFilter{Actor: testAuditActor, Page: 1, PageSize: 100},
			wantVMIDs: []int{301, 101, 101},
		},
		{
			name:      "by action start",
			filter:    store.AuditFilter{Action: testAuditAction, Page: 1, PageSize: 100},
			wantVMIDs: []int{301, 101},
		},
		{
			name:      "by vmid 101",
			filter:    store.AuditFilter{VMID: &vmid101, Page: 1, PageSize: 100},
			wantVMIDs: []int{101, 101},
		},
		{
			name:      "by cluster secondary",
			filter:    store.AuditFilter{Cluster: "secondary", Page: 1, PageSize: 100},
			wantVMIDs: []int{301},
		},
		{
			name:      "by actor and action",
			filter:    store.AuditFilter{Actor: testAuditActor, Action: "edit_cloudinit_snippet", Page: 1, PageSize: 100},
			wantVMIDs: []int{101},
		},
		{
			name:      "by vmid 200",
			filter:    store.AuditFilter{VMID: &vmid200, Page: 1, PageSize: 100},
			wantVMIDs: []int{200},
		},
		{
			name:      "by from/to range inclusive",
			filter:    store.AuditFilter{From: &fromTime, To: &toTime, Page: 1, PageSize: 100},
			wantVMIDs: []int{301, 200, 101, 102, 101},
		},
		{
			name: "by future from range matches nothing",
			//nolint:modernize // ptrTime returns &t, not new(t) which would be a zero time
			filter:    store.AuditFilter{From: ptrTime(time.Now().Add(1 * time.Hour)), Page: 1, PageSize: 100},
			wantVMIDs: nil,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			page, err := st.ListAuditLog(ctx, tc.filter)
			if err != nil {
				t.Fatalf("ListAuditLog: %v", err)
			}

			gotVMIDs := make([]int, len(page.Items))
			for i, item := range page.Items {
				gotVMIDs[i] = *item.VMID
			}

			if !equalIntSlices(gotVMIDs, tc.wantVMIDs) {
				t.Errorf("vmids = %v, want %v", gotVMIDs, tc.wantVMIDs)
			}
		})
	}
}

// TestListAuditLog_Pagination — page 2 returns the next slice using the
// same page/pageSize/total envelope T04 established.
//
//nolint:paralleltest,gocyclo // serial: shared database fixture; pagination has 3 sequential pages
func TestListAuditLog_Pagination(t *testing.T) {
	st := newAuditStore(t)
	ctx := context.Background()

	seedAuditRows(t, st)

	page1, err := st.ListAuditLog(ctx, store.AuditFilter{Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("ListAuditLog page 1: %v", err)
	}

	if page1.Total != 5 || len(page1.Items) != 2 || page1.Page != 1 || page1.PageSize != 2 {
		t.Fatalf("page1 envelope = total %d items %d page %d pageSize %d", page1.Total, len(page1.Items), page1.Page, page1.PageSize)
	}

	page2, err := st.ListAuditLog(ctx, store.AuditFilter{Page: 2, PageSize: 2})
	if err != nil {
		t.Fatalf("ListAuditLog page 2: %v", err)
	}

	if page2.Total != 5 || len(page2.Items) != 2 || page2.Page != 2 || page2.PageSize != 2 {
		t.Fatalf("page2 envelope = total %d items %d page %d pageSize %d", page2.Total, len(page2.Items), page2.Page, page2.PageSize)
	}

	page3, err := st.ListAuditLog(ctx, store.AuditFilter{Page: 3, PageSize: 2})
	if err != nil {
		t.Fatalf("ListAuditLog page 3: %v", err)
	}

	if len(page3.Items) != 1 {
		t.Fatalf("page3 items = %d, want 1", len(page3.Items))
	}

	// Pages must not overlap.
	if page1.Items[0].ID == page2.Items[0].ID || page1.Items[1].ID == page2.Items[1].ID {
		t.Error("page 1 and page 2 overlap")
	}
}

// TestListAuditLog_EmptyStore — no rows returns an empty (not nil) items
// slice and a zero total.
//
//nolint:paralleltest // serial: shared database fixture
func TestListAuditLog_EmptyStore(t *testing.T) {
	st := newAuditStore(t)
	ctx := context.Background()

	page, err := st.ListAuditLog(ctx, store.AuditFilter{Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListAuditLog: %v", err)
	}

	if page.Total != 0 {
		t.Errorf("total = %d, want 0", page.Total)
	}

	if len(page.Items) != 0 {
		t.Errorf("items = %d, want 0", len(page.Items))
	}
}

// TestListAuditLog_Sc002_AllActionsFromT05ToT10 — SC-002: one action from
// each of T05, T06, T08, T09, T10 is recorded and retrievable by its own
// action string via ListAuditLog (no HTTP).
//
//nolint:paralleltest // serial: shared database fixture
func TestListAuditLog_Sc002_AllActionsFromT05ToT10(t *testing.T) {
	st := newAuditStore(t)
	ctx := context.Background()

	actions := []struct {
		actor, cluster, action string
		vmid                   int
	}{
		{testAuditActor, "default", testAuditAction, 101},          // T05
		{testAuditActor, "default", "vm_create", 200},              // T06
		{testAuditActor, "default", "edit_cloudinit_snippet", 101}, // T08
		{testAuditActor, "default", "vm_snapshot_create", 101},     // T09
		{testAuditActor, "default", "console_open", 101},           // T10
	}
	for _, a := range actions {
		if err := st.RecordAction(ctx, a.actor, a.cluster, a.vmid, a.action); err != nil {
			t.Fatalf("RecordAction %s: %v", a.action, err)
		}
	}

	for _, want := range actions {
		page, err := st.ListAuditLog(ctx, store.AuditFilter{Action: want.action, Page: 1, PageSize: 100})
		if err != nil {
			t.Fatalf("ListAuditLog action=%s: %v", want.action, err)
		}

		if len(page.Items) != 1 {
			t.Errorf("action=%s: got %d items, want 1", want.action, len(page.Items))
			continue
		}

		got := page.Items[0]
		if got.Action != want.action || *got.VMID != want.vmid || got.Actor != want.actor {
			t.Errorf("action=%s: got %+v, want %+v", want.action, got, want)
		}
	}
}

//go:fix inline
//nolint:modernize // new(t) returns a zero time, not a pointer to t
func ptrTime(t time.Time) *time.Time { return &t }

func equalIntSlices(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

// TestRecordAdminAction_InsertsRowWithAdminFields — admin mutations write a
// row with cluster="" and vmid=nil, plus the new target/detail/IP/severity
// columns. Verified via a direct SELECT since QueryAudit only reads the
// original six columns.
//
//nolint:paralleltest // serial: shared database fixture
func TestRecordAdminAction_InsertsRowWithAdminFields(t *testing.T) {
	st := newAuditStore(t)
	ctx := context.Background()

	if err := st.RecordAdminAction(ctx, "admin", "admin.clusters.create", "cluster", "prod", `{"summary":"cluster=prod created"}`, "203.0.113.5"); err != nil {
		t.Fatalf("RecordAdminAction: %v", err)
	}

	var (
		cluster, action, targetType, targetID, detail, ip, severity string
		vmid                                                        sql.NullInt64
	)
	err := st.DB().QueryRowContext(ctx,
		`SELECT cluster, action, vmid, target_type, target_id, detail, ip_address, severity FROM audit_log ORDER BY id DESC LIMIT 1`,
	).Scan(&cluster, &action, &vmid, &targetType, &targetID, &detail, &ip, &severity)
	if err != nil {
		t.Fatalf("query audit row: %v", err)
	}

	if cluster != "" {
		t.Errorf("cluster = %q, want empty", cluster)
	}

	if action != "admin.clusters.create" {
		t.Errorf("action = %q, want admin.clusters.create", action)
	}

	if vmid.Valid {
		t.Errorf("vmid = %d, want NULL", vmid.Int64)
	}

	if targetType != "cluster" {
		t.Errorf("target_type = %q, want cluster", targetType)
	}

	if targetID != "prod" {
		t.Errorf("target_id = %q, want prod", targetID)
	}

	if detail != `{"summary":"cluster=prod created"}` {
		t.Errorf("detail = %q, want JSON summary", detail)
	}

	if ip != "203.0.113.5" {
		t.Errorf("ip_address = %q, want 203.0.113.5", ip)
	}

	if severity != "info" {
		t.Errorf("severity = %q, want info", severity)
	}
}

// TestRecordAction_StillWritesSeverity — the 15 existing VM callers are
// unchanged in signature but the row now carries a derived severity.
//
//nolint:paralleltest // serial: shared database fixture
func TestRecordAction_StillWritesSeverity(t *testing.T) {
	st := newAuditStore(t)
	ctx := context.Background()

	if err := st.RecordAction(ctx, testAuditActor, testStoreCluster, 101, "vm.destroy"); err != nil {
		t.Fatalf("RecordAction: %v", err)
	}

	var severity string
	if err := st.DB().QueryRowContext(ctx,
		`SELECT severity FROM audit_log ORDER BY id DESC LIMIT 1`,
	).Scan(&severity); err != nil {
		t.Fatalf("query severity: %v", err)
	}

	if severity != "warning" {
		t.Errorf("severity = %q, want warning (contains 'destroy')", severity)
	}
}
