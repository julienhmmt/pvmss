package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
	"strings"
	"testing"
	"time"
)

// mockTaskCreator is a controllable cluster.Creator for waitCreateTask tests.
// It returns a scripted sequence of task statuses, letting each test exercise
// one lifecycle-04 scenario (success after polls, task error, transient
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

// TestWaitCreateTask_SuccessAfterPolls — lifecycle-04: the wait succeeds
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

// TestWaitCreateTask_TaskError — lifecycle-04: a task error is returned with
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

// TestWaitCreateTask_TransientErrorThenSuccess — lifecycle-04: a transient
// read error (network blip, 5xx) does not abort the wait — the next poll
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

// TestWaitCreateTask_ContextCancelled — lifecycle-04: context cancellation
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

// TestWaitCreateTask_Timeout — lifecycle-04: exceeding the max wait returns
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
