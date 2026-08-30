package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
	"strings"
	"sync"
	"testing"
	"time"
)

// Test action literals used often enough to trip goconst. Kept as unexported
// consts so the test cases stay readable without lint noise.
const (
	actionShutdown = "shutdown"
	actionStop     = "stop"
	actionStart    = "start"
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

// =============================================================================
// Shutdown escalation (ticket 05)
// =============================================================================

// scriptedStatusReader returns a sequence of statuses on successive calls,
// then repeats the last one. Thread-safe via the embedded mutex.
type scriptedStatusReader struct {
	mu     sync.Mutex
	status []cluster.VMStatus
	calls  int
}

func (r *scriptedStatusReader) VMStatus(_ context.Context, _ string, _ int) (cluster.VMLiveStatus, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	idx := r.calls
	if idx >= len(r.status) {
		idx = len(r.status) - 1
	}

	r.calls++

	return cluster.VMLiveStatus{Status: r.status[idx]}, nil
}

// trackingWriter wraps cluster.Fake and records every Action call. For
// shutdown and stop it does NOT delegate to the fake — the scripted status
// reader controls when the VM appears stopped, so the fake's own state
// machine would reject the escalation stop as "already stopped". Other
// actions (start, reboot, etc.) delegate normally.
type trackingWriter struct {
	cluster.Fake
	mu      sync.Mutex
	actions []string
}

func (w *trackingWriter) Action(ctx context.Context, node string, vmid int, action string) error {
	w.mu.Lock()
	w.actions = append(w.actions, action)
	w.mu.Unlock()

	if action == actionShutdown || action == actionStop {
		// Don't change fake state — the scripted reader drives convergence.
		return nil
	}

	return w.Fake.Action(ctx, node, vmid, action)
}

func (w *trackingWriter) recordedActions() []string {
	w.mu.Lock()
	defer w.mu.Unlock()

	cp := make([]string, len(w.actions))
	copy(cp, w.actions)

	return cp
}

//nolint:paralleltest // serial: shared fake VM fixture + global var mutation
func TestAction_ShutdownEscalation_GuestStopsBeforeTimeout(t *testing.T) {
	// Shorten the escalation budget and poll interval so the test runs fast.
	originalEscalation := vm.MaxShutdownEscalationWait
	originalPost := vm.MaxPostEscalationWait
	originalPoll := vm.ShutdownPollInterval
	vm.MaxShutdownEscalationWait = 200 * time.Millisecond
	vm.MaxPostEscalationWait = 200 * time.Millisecond
	vm.ShutdownPollInterval = 20 * time.Millisecond

	t.Cleanup(func() {
		vm.MaxShutdownEscalationWait = originalEscalation
		vm.MaxPostEscalationWait = originalPost
		vm.ShutdownPollInterval = originalPoll
	})

	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	writer := &trackingWriter{Fake: *fake}
	// Guest stops on the 2nd poll.
	reader := &scriptedStatusReader{status: []cluster.VMStatus{cluster.VMRunning, cluster.VMStopped}}

	// VM 100 is running in the fake dataset.
	err := vm.Action(context.Background(), vm.BulkDeps{
		Actor:        aliceIdentity(),
		Writer:       writer,
		Audit:        st,
		Refresher:    noopRefresher{},
		StatusReader: reader,
	}, idx, testClusterName, 100, actionShutdown)
	if err != nil {
		t.Fatalf("Action: %v", err)
	}

	actions := writer.recordedActions()
	// Only shutdown was sent — no escalation to stop.
	if len(actions) != 1 || actions[0] != actionShutdown {
		t.Errorf("actions = %v, want [shutdown]", actions)
	}

	rows, err := st.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	if len(rows) != 1 || rows[0].Action != actionShutdown {
		t.Errorf("audit rows = %v, want 1 shutdown entry", rows)
	}
}

//nolint:paralleltest // serial: shared fake VM fixture + global var mutation
func TestAction_ShutdownEscalation_GuestDoesNotStop_EscalatesToStop(t *testing.T) {
	originalEscalation := vm.MaxShutdownEscalationWait
	originalPost := vm.MaxPostEscalationWait
	originalPoll := vm.ShutdownPollInterval
	vm.MaxShutdownEscalationWait = 100 * time.Millisecond
	vm.MaxPostEscalationWait = 100 * time.Millisecond
	vm.ShutdownPollInterval = 20 * time.Millisecond

	t.Cleanup(func() {
		vm.MaxShutdownEscalationWait = originalEscalation
		vm.MaxPostEscalationWait = originalPost
		vm.ShutdownPollInterval = originalPoll
	})

	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	writer := &trackingWriter{Fake: *fake}
	// Guest never stops — always running.
	reader := &scriptedStatusReader{status: []cluster.VMStatus{cluster.VMRunning}}

	// VM 100 is running in the fake dataset.
	err := vm.Action(context.Background(), vm.BulkDeps{
		Actor:        aliceIdentity(),
		Writer:       writer,
		Audit:        st,
		Refresher:    noopRefresher{},
		StatusReader: reader,
	}, idx, testClusterName, 100, actionShutdown)
	if err != nil {
		t.Fatalf("Action: %v", err)
	}

	actions := writer.recordedActions()
	// shutdown then stop.
	if len(actions) != 2 || actions[0] != actionShutdown || actions[1] != actionStop {
		t.Errorf("actions = %v, want [shutdown stop]", actions)
	}

	rows, err := st.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	if len(rows) != 2 {
		t.Fatalf("audit rows = %d, want 2", len(rows))
	}

	if rows[0].Action != actionShutdown || rows[1].Action != actionStop {
		t.Errorf("audit actions = %s %s, want shutdown stop", rows[0].Action, rows[1].Action)
	}
}

//nolint:paralleltest // serial: shared fake VM fixture + global var mutation
func TestAction_ShutdownForce_SkipsShutdownGoesDirectlyToStop(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	writer := &trackingWriter{Fake: *fake}
	reader := &scriptedStatusReader{status: []cluster.VMStatus{cluster.VMStopped}}

	// VM 100 is running in the fake dataset.
	err := vm.Action(context.Background(), vm.BulkDeps{
		Actor:        aliceIdentity(),
		Writer:       writer,
		Audit:        st,
		Refresher:    noopRefresher{},
		StatusReader: reader,
		Force:        true,
	}, idx, testClusterName, 100, actionShutdown)
	if err != nil {
		t.Fatalf("Action: %v", err)
	}

	actions := writer.recordedActions()
	// Force: only stop, no shutdown.
	if len(actions) != 1 || actions[0] != actionStop {
		t.Errorf("actions = %v, want [stop]", actions)
	}

	rows, err := st.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	if len(rows) != 1 || rows[0].Action != actionStop {
		t.Errorf("audit rows = %v, want 1 stop entry", rows)
	}
}

//nolint:paralleltest // serial: shared fake VM fixture + global var mutation
func TestAction_ShutdownEscalation_ContextCancellation(t *testing.T) {
	originalEscalation := vm.MaxShutdownEscalationWait
	vm.MaxShutdownEscalationWait = 10 * time.Second

	t.Cleanup(func() { vm.MaxShutdownEscalationWait = originalEscalation })

	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	writer := &trackingWriter{Fake: *fake}
	reader := &scriptedStatusReader{status: []cluster.VMStatus{cluster.VMRunning}}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after shutdown is sent but during polling.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	// VM 100 is running in the fake dataset.
	_ = vm.Action(ctx, vm.BulkDeps{
		Actor:        aliceIdentity(),
		Writer:       writer,
		Audit:        st,
		Refresher:    noopRefresher{},
		StatusReader: reader,
	}, idx, testClusterName, 100, actionShutdown)

	actions := writer.recordedActions()
	// shutdown was sent, but stop should NOT have been sent (context cancelled
	// during polling before escalation).
	if len(actions) != 1 || actions[0] != actionShutdown {
		t.Errorf("actions = %v, want [shutdown] (context cancelled before escalation)", actions)
	}
}

// =============================================================================
// Idempotence (ticket 08)
// =============================================================================

//nolint:paralleltest // serial: shared fake VM fixture
func TestAction_Idempotence_TargetStateHolds_NoWriterCall(t *testing.T) {
	cases := []struct {
		name     string
		vmid     int
		action   string
		liveStat cluster.VMStatus
	}{
		{"start on running is a no-op", 100, actionStart, cluster.VMRunning},
		{"stop on stopped is a no-op", 101, actionStop, cluster.VMStopped},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fake := actionsIndex(t)
			idx := buildResolveIndex(t)
			st := bulkTestStore(t)
			writer := &trackingWriter{Fake: *fake}
			reader := &scriptedStatusReader{status: []cluster.VMStatus{tc.liveStat}}

			err := vm.Action(context.Background(), vm.BulkDeps{
				Actor:        aliceIdentity(),
				Writer:       writer,
				Audit:        st,
				Refresher:    noopRefresher{},
				StatusReader: reader,
			}, idx, testClusterName, tc.vmid, tc.action)
			if err != nil {
				t.Fatalf("Action: %v", err)
			}

			// No writer call — the target state already holds.
			actions := writer.recordedActions()
			if len(actions) != 0 {
				t.Errorf("writer calls = %v, want none (target state already holds)", actions)
			}

			// Audit entry is still recorded — the intention is real.
			rows, err := st.QueryAudit(context.Background())
			if err != nil {
				t.Fatalf("QueryAudit: %v", err)
			}

			if len(rows) != 1 || rows[0].Action != tc.action {
				t.Errorf("audit rows = %v, want 1 %s entry", rows, tc.action)
			}
		})
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestAction_Idempotence_RebootOnRunning_WriterCallStillHappens(t *testing.T) {
	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	writer := &trackingWriter{Fake: *fake}
	// VM 100 is running — reboot is a transition, not a target state.
	reader := &scriptedStatusReader{status: []cluster.VMStatus{cluster.VMRunning}}

	err := vm.Action(context.Background(), vm.BulkDeps{
		Actor:        aliceIdentity(),
		Writer:       writer,
		Audit:        st,
		Refresher:    noopRefresher{},
		StatusReader: reader,
	}, idx, testClusterName, 100, "reboot")
	if err != nil {
		t.Fatalf("Action: %v", err)
	}

	// Writer call happened — reboot is not idempotent.
	actions := writer.recordedActions()
	if len(actions) != 1 || actions[0] != "reboot" {
		t.Errorf("writer calls = %v, want [reboot]", actions)
	}
}

// =============================================================================
// Retry-on-lock (ticket 08)
// =============================================================================

// lockErrorWriter returns a "VM is locked (backup)" error for the first N
// Action calls, then delegates to the embedded Fake.
type lockErrorWriter struct {
	cluster.Fake
	mu           sync.Mutex
	lockCount    int
	successAfter int // number of lock errors before success
}

func (w *lockErrorWriter) Action(ctx context.Context, node string, vmid int, action string) error {
	w.mu.Lock()
	w.lockCount++
	count := w.lockCount
	w.mu.Unlock()

	if count <= w.successAfter {
		return errors.New("VM is locked (backup)")
	}

	return w.Fake.Action(ctx, node, vmid, action)
}

//nolint:paralleltest // serial: shared fake VM fixture + global var mutation
func TestAction_RetryOnLock_SucceedsAfterRetries(t *testing.T) {
	originalWait := vm.MaxLockRetryWait
	originalPoll := vm.LockRetryPollInterval
	vm.MaxLockRetryWait = 200 * time.Millisecond
	vm.LockRetryPollInterval = 20 * time.Millisecond

	t.Cleanup(func() {
		vm.MaxLockRetryWait = originalWait
		vm.LockRetryPollInterval = originalPoll
	})

	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	// Lock errors for the first 2 calls, success on the 3rd.
	writer := &lockErrorWriter{Fake: *fake, successAfter: 2}

	// VM 101 is stopped → start should eventually succeed.
	err := vm.Action(context.Background(), vm.BulkDeps{
		Actor:     aliceIdentity(),
		Writer:    writer,
		Audit:     st,
		Refresher: noopRefresher{},
	}, idx, testClusterName, 101, actionStart)
	if err != nil {
		t.Fatalf("Action: %v", err)
	}

	rows, err := st.QueryAudit(context.Background())
	if err != nil {
		t.Fatalf("QueryAudit: %v", err)
	}

	if len(rows) != 1 || rows[0].Action != actionStart {
		t.Errorf("audit rows = %v, want 1 start entry", rows)
	}
}

//nolint:paralleltest // serial: shared fake VM fixture + global var mutation
func TestAction_RetryOnLock_TimeoutNamesTheLock(t *testing.T) {
	originalWait := vm.MaxLockRetryWait
	originalPoll := vm.LockRetryPollInterval
	vm.MaxLockRetryWait = 100 * time.Millisecond
	vm.LockRetryPollInterval = 20 * time.Millisecond

	t.Cleanup(func() {
		vm.MaxLockRetryWait = originalWait
		vm.LockRetryPollInterval = originalPoll
	})

	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	// Always locked.
	writer := &lockErrorWriter{Fake: *fake, successAfter: 999}

	err := vm.Action(context.Background(), vm.BulkDeps{
		Actor:     aliceIdentity(),
		Writer:    writer,
		Audit:     st,
		Refresher: noopRefresher{},
	}, idx, testClusterName, 101, actionStart)
	if err == nil {
		t.Fatal("Action: expected error, got nil")
	}

	if !strings.Contains(err.Error(), "backup") {
		t.Errorf("error = %q, want it to name the lock 'backup'", err.Error())
	}
}

//nolint:paralleltest // serial: shared fake VM fixture
func TestAction_NonLockError_NotRetried(t *testing.T) {
	originalWait := vm.MaxLockRetryWait
	vm.MaxLockRetryWait = 10 * time.Second

	t.Cleanup(func() { vm.MaxLockRetryWait = originalWait })

	fake := actionsIndex(t)
	idx := buildResolveIndex(t)
	st := bulkTestStore(t)
	// A non-lock error (e.g. invalid state transition).
	// VM 100 is running → start fails with "invalid state transition".
	// No StatusReader → no idempotence check → the error goes through
	// retry-on-lock, which sees it's not a lock error and returns immediately.
	err := vm.Action(context.Background(), vm.BulkDeps{
		Actor:     aliceIdentity(),
		Writer:    fake,
		Audit:     st,
		Refresher: noopRefresher{},
	}, idx, testClusterName, 100, actionStart)
	if err == nil {
		t.Fatal("Action: expected error, got nil")
	}
	// Should return quickly (not wait 10s for lock retry).
	if !errors.Is(err, cluster.ErrInvalidStateTransition) {
		t.Fatalf("err = %v, want ErrInvalidStateTransition", err)
	}
}
