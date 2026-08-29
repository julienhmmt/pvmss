package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
	"strings"
	"testing"
)

// failingAuditErr returns a configured error from RecordAction so the
// audit-error branches of vm.Action / vm.Delete / vm.Patch are reachable and
// the wrapped error is assertable via errors.Is. (create_test.go already
// defines a fixed-message failingAudit for the create path.)
type failingAuditErr struct{ err error }

func (f failingAuditErr) RecordAction(context.Context, string, string, int, string) error {
	return f.err
}

// failingWriter embeds cluster.Fake (which already satisfies cluster.Writer)
// and overrides only the three verbs vm.Action / vm.Delete / vm.Patch call, so
// the cluster-write error branches are reachable without a live Proxmox. When
// the configured error is nil it delegates to the embedded Fake so success
// paths still mutate the dataset.
type failingWriter struct {
	cluster.Fake
	actionErr error
	deleteErr error
	patchErr  error
}

func (w failingWriter) Action(ctx context.Context, node string, vmid int, action string) error {
	if w.actionErr != nil {
		return w.actionErr
	}

	return w.Fake.Action(ctx, node, vmid, action)
}

func (w failingWriter) Delete(ctx context.Context, node string, vmid int) error {
	if w.deleteErr != nil {
		return w.deleteErr
	}

	return w.Fake.Delete(ctx, node, vmid)
}

func (w failingWriter) Patch(ctx context.Context, node string, vmid int, name, description string) error {
	if w.patchErr != nil {
		return w.patchErr
	}

	return w.Fake.Patch(ctx, node, vmid, name, description)
}

// actionsIndex resets the fake dataset and builds a fresh Index from it, so
// every test starts from the full 25-VM fixture regardless of what a previous
// test mutated.
func actionsIndex(t *testing.T) *cluster.Fake {
	t.Helper()
	cluster.ResetFake()
	t.Cleanup(cluster.ResetFake)

	return &cluster.Fake{}
}

// =============================================================================
// IsValidAction
// =============================================================================

func TestIsValidAction(t *testing.T) {
	t.Parallel()

	for _, action := range []string{"start", "stop", "shutdown", "reboot", "reset", "pause", "resume"} {
		if !vm.IsValidAction(action) {
			t.Errorf("IsValidAction(%q) = false, want true", action)
		}
	}

	for _, action := range []string{"", "START", "delete", "migrate", "foo"} {
		if vm.IsValidAction(action) {
			t.Errorf("IsValidAction(%q) = true, want false", action)
		}
	}
}

// =============================================================================
// Action
// =============================================================================

//nolint:paralleltest // serial: shared fake VM fixture
func TestAction_SuccessRecordsAuditAndRefreshes(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)

	err := vm.Action(context.Background(), vm.BulkDeps{
		Actor: aliceIdentity(), Writer: fake, Audit: st, Refresher: noopRefresher{},
	}, idx, testClusterName, 101, "start")
	if err != nil {
		t.Fatalf("Action: %v", err)
	}

	rows, err := st.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	if len(rows) != 1 {
		t.Fatalf("audit rows = %d, want 1", len(rows))
	}

	if rows[0].Action != "start" || *rows[0].VMID != 101 || rows[0].Actor != cluster.FakeUserAlice {
		t.Errorf("audit row = %+v, want action=start vmid=101 actor=alice", rows[0])
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestAction_InvalidActionRejectedBeforeResolve(t *testing.T) {
	idx := buildResolveIndex(t)

	// A nonexistent VMID with an invalid action still returns ErrActionRejected
	// — the action enum check runs before Resolve (constitution XIII).
	err := vm.Action(context.Background(), vm.BulkDeps{
		Actor: aliceIdentity(), Writer: cluster.Fake{}, Audit: noopAudit{}, Refresher: noopRefresher{},
	}, idx, testClusterName, 999, "foo")
	if !errors.Is(err, vm.ErrActionRejected) {
		t.Fatalf("err = %v, want ErrActionRejected", err)
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestAction_NonOwnerForbidden(t *testing.T) {
	idx := buildResolveIndex(t)

	err := vm.Action(context.Background(), vm.BulkDeps{
		Actor: bobIdentity(), Writer: cluster.Fake{}, Audit: noopAudit{}, Refresher: noopRefresher{},
	}, idx, testClusterName, 100, "stop")
	if !errors.Is(err, vm.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestAction_NotFound(t *testing.T) {
	idx := buildResolveIndex(t)

	err := vm.Action(context.Background(), vm.BulkDeps{
		Actor: aliceIdentity(), Writer: cluster.Fake{}, Audit: noopAudit{}, Refresher: noopRefresher{},
	}, idx, testClusterName, 999, "start")
	if !errors.Is(err, vm.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestAction_ClusterWriteErrorWrapped(t *testing.T) {
	idx := buildResolveIndex(t)

	// start on a running VM → the fake rejects with ErrInvalidStateTransition;
	// vm.Action wraps it as "cluster action: %w".
	err := vm.Action(context.Background(), vm.BulkDeps{
		Actor: aliceIdentity(), Writer: cluster.Fake{}, Audit: noopAudit{}, Refresher: noopRefresher{},
	}, idx, testClusterName, 100, "start")
	if !errors.Is(err, cluster.ErrInvalidStateTransition) {
		t.Fatalf("err = %v, want ErrInvalidStateTransition", err)
	}

	if !strings.Contains(err.Error(), "cluster action") {
		t.Errorf("err = %q, want it wrapped with \"cluster action\"", err.Error())
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestAction_AuditErrorWrapped(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	auditErr := errors.New("audit store down")

	err := vm.Action(context.Background(), vm.BulkDeps{
		Actor: aliceIdentity(), Writer: fake, Audit: failingAuditErr{err: auditErr}, Refresher: noopRefresher{},
	}, idx, testClusterName, 101, "start")
	if !errors.Is(err, auditErr) {
		t.Fatalf("err = %v, want the audit error", err)
	}

	if !strings.Contains(err.Error(), "record audit") {
		t.Errorf("err = %q, want it wrapped with \"record audit\"", err.Error())
	}
}

// =============================================================================
// Delete
// =============================================================================

//nolint:paralleltest // serial: shared fake VM fixture
func TestDelete_SuccessRemovesVMAndRecordsAudit(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)

	err := vm.Delete(context.Background(), vm.WriteDeps{Index: idx, Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 101, Writer: fake, Audit: st, Refresher: noopRefresher{}})
	if err != nil {
		t.Fatalf("Delete: %v", err)
	}

	snap, err := fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for _, v := range snap.VMs {
		if v.VMID == 101 {
			t.Errorf("VM 101 still present after delete: %+v", v)
		}
	}

	rows, err := st.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	if len(rows) != 1 || rows[0].Action != "delete" || *rows[0].VMID != 101 {
		t.Errorf("audit rows = %+v, want one delete row for 101", rows)
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestDelete_NonOwnerForbidden(t *testing.T) {
	idx := buildResolveIndex(t)

	err := vm.Delete(context.Background(), vm.WriteDeps{Index: idx, Actor: bobIdentity(), ClusterName: testClusterName, VMID: 100, Writer: cluster.Fake{}, Audit: noopAudit{}, Refresher: noopRefresher{}})
	if !errors.Is(err, vm.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestDelete_NotFound(t *testing.T) {
	idx := buildResolveIndex(t)

	err := vm.Delete(context.Background(), vm.WriteDeps{Index: idx, Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 999, Writer: cluster.Fake{}, Audit: noopAudit{}, Refresher: noopRefresher{}})
	if !errors.Is(err, vm.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestDelete_ClusterWriteErrorWrapped(t *testing.T) {
	idx := buildResolveIndex(t)
	writeErr := errors.New("cluster delete refused")

	err := vm.Delete(context.Background(), vm.WriteDeps{Index: idx, Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 101, Writer: failingWriter{deleteErr: writeErr}, Audit: noopAudit{}, Refresher: noopRefresher{}})
	if !errors.Is(err, writeErr) {
		t.Fatalf("err = %v, want the writer error", err)
	}

	if !strings.Contains(err.Error(), "cluster delete") {
		t.Errorf("err = %q, want it wrapped with \"cluster delete\"", err.Error())
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestDelete_AuditErrorWrapped(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	auditErr := errors.New("audit store down")

	err := vm.Delete(context.Background(), vm.WriteDeps{Index: idx, Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 101, Writer: fake, Audit: failingAuditErr{err: auditErr}, Refresher: noopRefresher{}})
	if !errors.Is(err, auditErr) {
		t.Fatalf("err = %v, want the audit error", err)
	}

	if !strings.Contains(err.Error(), "record audit") {
		t.Errorf("err = %q, want it wrapped with \"record audit\"", err.Error())
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestDelete_RunningVMRejectedWithoutForce(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)

	// VM 100 (web-01) is running and owned by alice.
	err := vm.Delete(context.Background(), vm.WriteDeps{Index: idx, Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 100, Writer: fake, Audit: st, Refresher: noopRefresher{}})
	if !errors.Is(err, cluster.ErrVMRunning) {
		t.Fatalf("err = %v, want ErrVMRunning", err)
	}

	// The VM must still be present — no stop, no delete.
	snap, err := fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for _, v := range snap.VMs {
		if v.VMID == 100 {
			return
		}
	}

	t.Errorf("VM 100 was removed without force")
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestDelete_ForceStopsRunningVMThenDeletes(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)

	// VM 100 (web-01) is running and owned by alice. Force=true stops it first.
	err := vm.Delete(context.Background(), vm.WriteDeps{Index: idx, Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 100, Writer: fake, Audit: st, Refresher: noopRefresher{}, Force: true})
	if err != nil {
		t.Fatalf("Delete with force: %v", err)
	}

	snap, err := fake.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for _, v := range snap.VMs {
		if v.VMID == 100 {
			t.Errorf("VM 100 still present after force-delete")
		}
	}

	// The force-stop is recorded as its own "stop" audit entry, plus the delete.
	rows, err := st.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	var hasStop, hasDelete bool

	for _, row := range rows {
		if row.Action == "stop" && *row.VMID == 100 {
			hasStop = true
		}

		if row.Action == "delete" && *row.VMID == 100 {
			hasDelete = true
		}
	}

	if !hasStop {
		t.Errorf("audit rows missing force-stop entry for VM 100: %+v", rows)
	}

	if !hasDelete {
		t.Errorf("audit rows missing delete entry for VM 100: %+v", rows)
	}
}

// =============================================================================
// Patch
// =============================================================================

//nolint:paralleltest // serial: shared fake VM fixture
func TestPatch_EmptyPatchRejected(t *testing.T) {
	idx := buildResolveIndex(t)

	for _, tc := range []struct {
		name        string
		description string
	}{
		{"both empty", ""},
		{"whitespace description", "   "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := vm.Patch(context.Background(), vm.WriteDeps{Index: idx, Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 101, Writer: cluster.Fake{}, Audit: noopAudit{}, Refresher: noopRefresher{}}, "", tc.description)
			if !errors.Is(err, vm.ErrEmptyPatch) {
				t.Fatalf("err = %v, want ErrEmptyPatch", err)
			}
		})
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestPatch_InvalidNameRejectedBeforeResolve(t *testing.T) {
	idx := buildResolveIndex(t)

	err := vm.Patch(context.Background(), vm.WriteDeps{Index: idx, Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 101, Writer: cluster.Fake{}, Audit: noopAudit{}, Refresher: noopRefresher{}}, "Bad_Name!", "")
	if !errors.Is(err, vm.ErrInvalidName) {
		t.Fatalf("err = %v, want ErrInvalidName", err)
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestPatch_DescriptionTooLongRejectedBeforeResolve(t *testing.T) {
	idx := buildResolveIndex(t)

	err := vm.Patch(context.Background(), vm.WriteDeps{Index: idx, Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 101, Writer: cluster.Fake{}, Audit: noopAudit{}, Refresher: noopRefresher{}}, "", strings.Repeat("a", vm.MaxDescriptionLength+1))
	if !errors.Is(err, vm.ErrDescriptionTooLong) {
		t.Fatalf("err = %v, want ErrDescriptionTooLong", err)
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestPatch_AuditActionRecorded(t *testing.T) {
	cases := []struct {
		name        string
		patchName   string
		description string
		wantAction  string
	}{
		{"rename records rename audit", "web-02-renamed", "", "rename"},
		{"description-only records edit_description audit", "", "new description", "edit_description"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := actionsIndex(t)
			idx := buildResolveIndex(t)
			st := bulkTestStore(t)

			err := vm.Patch(context.Background(), vm.WriteDeps{Index: idx, Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 101, Writer: fake, Audit: st, Refresher: noopRefresher{}}, tc.patchName, tc.description)
			if err != nil {
				t.Fatalf("Patch: %v", err)
			}

			rows, err := st.QueryAudit(context.Background())
			if err != nil {
				t.Fatalf("QueryAudit: %v", err)
			}

			if len(rows) != 1 || rows[0].Action != tc.wantAction || *rows[0].VMID != 101 {
				t.Errorf("audit rows = %+v, want one %q row for 101", rows, tc.wantAction)
			}
		})
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestPatch_NonOwnerForbidden(t *testing.T) {
	idx := buildResolveIndex(t)

	err := vm.Patch(context.Background(), vm.WriteDeps{Index: idx, Actor: bobIdentity(), ClusterName: testClusterName, VMID: 100, Writer: cluster.Fake{}, Audit: noopAudit{}, Refresher: noopRefresher{}}, "renamed", "")
	if !errors.Is(err, vm.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestPatch_NotFound(t *testing.T) {
	idx := buildResolveIndex(t)

	err := vm.Patch(context.Background(), vm.WriteDeps{Index: idx, Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 999, Writer: cluster.Fake{}, Audit: noopAudit{}, Refresher: noopRefresher{}}, "renamed", "")
	if !errors.Is(err, vm.ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestPatch_ClusterWriteErrorWrapped(t *testing.T) {
	idx := buildResolveIndex(t)
	writeErr := errors.New("cluster patch refused")

	err := vm.Patch(context.Background(), vm.WriteDeps{Index: idx, Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 101, Writer: failingWriter{patchErr: writeErr}, Audit: noopAudit{}, Refresher: noopRefresher{}}, "renamed", "")
	if !errors.Is(err, writeErr) {
		t.Fatalf("err = %v, want the writer error", err)
	}

	if !strings.Contains(err.Error(), "cluster patch") {
		t.Errorf("err = %q, want it wrapped with \"cluster patch\"", err.Error())
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestPatch_AuditErrorWrapped(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	auditErr := errors.New("audit store down")

	err := vm.Patch(context.Background(), vm.WriteDeps{Index: idx, Actor: aliceIdentity(), ClusterName: testClusterName, VMID: 101, Writer: fake, Audit: failingAuditErr{err: auditErr}, Refresher: noopRefresher{}}, "renamed", "")
	if !errors.Is(err, auditErr) {
		t.Fatalf("err = %v, want the audit error", err)
	}

	if !strings.Contains(err.Error(), "record audit") {
		t.Errorf("err = %q, want it wrapped with \"record audit\"", err.Error())
	}
}
