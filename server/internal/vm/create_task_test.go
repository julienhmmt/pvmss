package vm_test

import (
	"context"
	"errors"
	"log/slog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/policy"
	"pvmss/server/internal/vm"
	"strings"
	"testing"
	"time"
)

// mockTaskCreator is a controllable cluster.Creator for waitCreateTask tests.
// It returns a scripted sequence of task statuses, letting each test exercise
// one scenario (success after polls, task error, transient
// failure, timeout, cancellation) without the fake's 2-second poll delay.
type mockTaskCreator struct {
	statuses []cluster.TaskStatus
	errs     []error
	calls    int
}

func (m *mockTaskCreator) NextVMID(_ context.Context) (int, error) {
	return 200, nil
}

func (m *mockTaskCreator) CreateVM(_ context.Context, _ cluster.VMSpec) (string, error) {
	return "UPID:mock", nil
}

func (m *mockTaskCreator) HostCPUFlags(_ context.Context) ([][]string, error) {
	return nil, nil
}

func (m *mockTaskCreator) CloneVM(_ context.Context, _ cluster.CloneSpec) (string, error) {
	return "UPID:mock", nil
}

func (m *mockTaskCreator) TaskStatus(_ context.Context, _ string) (cluster.TaskStatus, error) {
	idx := m.calls
	m.calls++

	if idx < len(m.errs) && m.errs[idx] != nil {
		return cluster.TaskStatus{}, m.errs[idx]
	}

	if idx < len(m.statuses) {
		return m.statuses[idx], nil
	}

	return cluster.TaskStatus{State: cluster.TaskRunning}, nil
}

// shortenTaskPolls reduces the poll interval for tests that need to exercise
// the retry loop without waiting 2 seconds per poll.
func shortenTaskPolls(t *testing.T) {
	t.Helper()

	orig := vm.CreateTaskPoll
	vm.CreateTaskPoll = 5 * time.Millisecond

	t.Cleanup(func() { vm.CreateTaskPoll = orig })
}

// TestWaitCreateTask_SuccessAfterPolls - the wait succeeds
// after the task reports running for several polls then OK.
//
//nolint:paralleltest // mutates shared vm.CreateTaskPoll
func TestWaitCreateTask_SuccessAfterPolls(t *testing.T) {
	shortenTaskPolls(t)

	creator := &mockTaskCreator{
		statuses: []cluster.TaskStatus{
			{State: cluster.TaskRunning},
			{State: cluster.TaskRunning},
			{State: cluster.TaskOK},
		},
	}

	err := vm.WaitCreateTask(context.Background(), creator, "UPID:mock")
	if err != nil {
		t.Fatalf("WaitCreateTask: %v, want nil", err)
	}

	if creator.calls != 3 {
		t.Errorf("calls = %d, want 3", creator.calls)
	}
}

// TestWaitCreateTask_TaskError - a task error is returned with
// the exit message and trailing log lines.
//
//nolint:paralleltest // mutates shared vm.CreateTaskPoll
func TestWaitCreateTask_TaskError(t *testing.T) {
	shortenTaskPolls(t)

	creator := &mockTaskCreator{
		statuses: []cluster.TaskStatus{
			{State: cluster.TaskError, ExitMessage: "storage full", Log: []string{"line1", "line2"}},
		},
	}

	err := vm.WaitCreateTask(context.Background(), creator, "UPID:mock")
	if err == nil {
		t.Fatalf("WaitCreateTask: expected error, got nil")
	}

	if !strings.Contains(err.Error(), "storage full") {
		t.Errorf("error = %q, want it to contain 'storage full'", err.Error())
	}

	if !strings.Contains(err.Error(), "line2") {
		t.Errorf("error = %q, want it to contain the log tail", err.Error())
	}
}

// TestWaitCreateTask_TransientErrorThenSuccess - a transient
// read error (network blip, 5xx) does not abort the wait - the next poll
// succeeds.
//
//nolint:paralleltest // mutates shared vm.CreateTaskPoll
func TestWaitCreateTask_TransientErrorThenSuccess(t *testing.T) {
	shortenTaskPolls(t)

	creator := &mockTaskCreator{
		errs: []error{errors.New("network blip")},
		statuses: []cluster.TaskStatus{
			{},
			{State: cluster.TaskOK},
		},
	}

	err := vm.WaitCreateTask(context.Background(), creator, "UPID:mock")
	if err != nil {
		t.Fatalf("WaitCreateTask: %v, want nil (transient error should not abort)", err)
	}
}

// TestWaitCreateTask_ContextCancelled - context cancellation
// during a transient error propagates immediately.
//
//nolint:paralleltest // mutates shared vm.CreateTaskPoll
func TestWaitCreateTask_ContextCancelled(t *testing.T) {
	shortenTaskPolls(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before the first poll

	creator := &mockTaskCreator{
		errs: []error{errors.New("network blip")},
	}

	err := vm.WaitCreateTask(ctx, creator, "UPID:mock")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
}

// TestWaitCreateTask_Timeout - exceeding the max wait returns
// an explicit timeout error.
//
//nolint:paralleltest // mutates shared vm.CreateTaskPoll and vm.MaxCreateTaskWait
func TestWaitCreateTask_Timeout(t *testing.T) {
	shortenTaskPolls(t)

	orig := vm.MaxCreateTaskWait
	vm.MaxCreateTaskWait = 20 * time.Millisecond

	t.Cleanup(func() { vm.MaxCreateTaskWait = orig })

	creator := &mockTaskCreator{
		statuses: []cluster.TaskStatus{
			{State: cluster.TaskRunning},
		},
	}

	err := vm.WaitCreateTask(context.Background(), creator, "UPID:mock")
	if err == nil {
		t.Fatalf("WaitCreateTask: expected timeout error, got nil")
	}

	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error = %q, want it to contain 'timed out'", err.Error())
	}
}

// unreadableTaskCreator fails every task-status read, as a cluster that cannot
// be reached while the clone runs.
type unreadableTaskCreator struct {
	cluster.Fake
}

func (unreadableTaskCreator) TaskStatus(_ context.Context, _ string) (cluster.TaskStatus, error) {
	return cluster.TaskStatus{}, errors.New("cluster unreachable")
}

// shortenTaskWait reduces MaxCreateTaskWait for one test.
func shortenTaskWait(t *testing.T, d time.Duration) {
	t.Helper()

	orig := vm.MaxCreateTaskWait
	vm.MaxCreateTaskWait = d

	t.Cleanup(func() { vm.MaxCreateTaskWait = orig })
}

// TestWaitCreateTask_UnreadableStatusTimesOut - a task whose status can never
// be read still ends at the deadline with the timeout sentinel. The safety
// context stops a regression from hanging the test.
//
//nolint:paralleltest // mutates shared vm.CreateTaskPoll and vm.MaxCreateTaskWait
func TestWaitCreateTask_UnreadableStatusTimesOut(t *testing.T) {
	shortenTaskPolls(t)
	shortenTaskWait(t, 20*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := vm.WaitCreateTask(ctx, unreadableTaskCreator{}, "UPID:mock")
	if !errors.Is(err, vm.ErrCreateTaskTimeout) {
		t.Fatalf("error = %v, want ErrCreateTaskTimeout", err)
	}
}

// cancelingTaskCreator cancels the request on the first status read, then
// reports the clone task as failed. It models a browser dropping the connection
// while the clone runs.
type cancelingTaskCreator struct {
	cluster.Fake
	cancel context.CancelFunc
	polls  int
}

func (c *cancelingTaskCreator) TaskStatus(_ context.Context, upid string) (cluster.TaskStatus, error) {
	c.polls++
	if c.polls == 1 {
		c.cancel()

		return cluster.TaskStatus{UPID: upid, State: cluster.TaskRunning}, nil
	}

	return cluster.TaskStatus{UPID: upid, State: cluster.TaskError, ExitMessage: "clone exploded"}, nil
}

// runningTaskCreator reports the clone task as still running forever.
type runningTaskCreator struct {
	cluster.Fake
}

func (runningTaskCreator) TaskStatus(_ context.Context, upid string) (cluster.TaskStatus, error) {
	return cluster.TaskStatus{UPID: upid, State: cluster.TaskRunning}, nil
}

// deleteRecorder is a Writer that records each Delete call and whether its
// context was already cancelled when the rollback ran.
type deleteRecorder struct {
	cluster.Fake
	deletes      int
	ctxCancelled bool
}

func (w *deleteRecorder) Delete(ctx context.Context, node string, vmid int) error {
	w.deletes++
	w.ctxCancelled = ctx.Err() != nil

	return w.Fake.Delete(ctx, node, vmid)
}

// TestCloneOwn_CancelledRequestStillPurgesFailedClone - the request is
// cancelled while the clone runs and the task then fails. The wait and the
// purge must still run, on a context the cancellation does not reach.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCloneOwn_CancelledRequestStillPurgesFailedClone(t *testing.T) {
	shortenTaskPolls(t)

	fixture := newCloneFixture(t)
	source := fixture.aliceSource(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writer := &deleteRecorder{Fake: fixture.fake}

	deps := fixture.deps()
	deps.Creator = &cancelingTaskCreator{Fake: fixture.fake, cancel: cancel}
	deps.Writer = writer

	_, err := vm.CloneOwn(ctx, aliceIdentity(), fixture.index, testClusterName, source.VMID, "cancelled-clone", deps)
	if !errors.Is(err, vm.ErrClusterCreate) {
		t.Fatalf("err = %v, want ErrClusterCreate", err)
	}

	if writer.deletes != 1 {
		t.Fatalf("Delete calls = %d, want 1 (the failed clone must be purged)", writer.deletes)
	}

	if writer.ctxCancelled {
		t.Fatal("rollback Delete ran on a cancelled context")
	}
}

// TestCloneOwn_TimedOutWaitKeepsVM - a clone still running at the deadline is
// not deleted: the VM may be mid-clone and Proxmox would refuse the delete.
//
//nolint:paralleltest // mutates shared vm.CreateTaskPoll and vm.MaxCreateTaskWait
func TestCloneOwn_TimedOutWaitKeepsVM(t *testing.T) {
	shortenTaskPolls(t)
	shortenTaskWait(t, 20*time.Millisecond)

	fixture := newCloneFixture(t)
	source := fixture.aliceSource(t)

	writer := &deleteRecorder{Fake: fixture.fake}

	deps := fixture.deps()
	deps.Creator = runningTaskCreator{Fake: fixture.fake}
	deps.Writer = writer

	_, err := vm.CloneOwn(context.Background(), aliceIdentity(), fixture.index, testClusterName, source.VMID, "slow-clone", deps)
	// The wait context and the poll deadline both expire at MaxCreateTaskWait,
	// so either error may surface; both mean the clone is still running.
	if !errors.Is(err, vm.ErrCreateTaskTimeout) && !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("err = %v, want a timeout in the chain", err)
	}

	if writer.deletes != 0 {
		t.Fatalf("Delete calls = %d, want 0 (a clone still running must not be deleted)", writer.deletes)
	}
}

// TestCreate_TemplateClone_CancelledRequestStillPurgesFailedClone - the same
// cancellation through the template create path: the wait and the purge
// survive the request being cancelled.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestCreate_TemplateClone_CancelledRequestStillPurgesFailedClone(t *testing.T) {
	shortenTaskPolls(t)

	fixture := newCreateFixture(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	writer := &deleteRecorder{Fake: fixture.fake}
	req := templateRequest(9000)

	_, err := vm.Create(ctx, aliceIdentity(), req.Cluster, req, vm.CreateDeps{
		Store:     fixture.store,
		Creator:   &cancelingTaskCreator{Fake: fixture.fake, cancel: cancel},
		Writer:    writer,
		FreeSpace: fixture.fake,
		Audit:     fixture.store,
		Log:       slog.New(slog.DiscardHandler),
		Policy:    policy.New(fixture.store, nil),
	})
	if err == nil {
		t.Fatal("Create: nil error, want the clone task failure")
	}

	if writer.deletes != 1 {
		t.Fatalf("Delete calls = %d, want 1 (the failed clone must be purged)", writer.deletes)
	}

	if writer.ctxCancelled {
		t.Fatal("rollback Delete ran on a cancelled context")
	}
}
