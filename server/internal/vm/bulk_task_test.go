package vm_test

import (
	"context"
	"fmt"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
	"testing"
	"time"
)

// taskBulkWriter starts a Proxmox task per power action (ActionTasker) and
// reports one target's task as failed on TaskStatus; every other task is OK.
type taskBulkWriter struct {
	cluster.Fake

	failVMID int
}

func taskBulkUPID(vmid int) string {
	return fmt.Sprintf("UPID:pve1:%d", vmid)
}

func (w taskBulkWriter) ActionTask(_ context.Context, _ string, vmid int, _ string) (string, error) {
	return taskBulkUPID(vmid), nil
}

func (w taskBulkWriter) TaskStatus(_ context.Context, upid string) (cluster.TaskStatus, error) {
	if upid == taskBulkUPID(w.failVMID) {
		return cluster.TaskStatus{UPID: upid, State: cluster.TaskError, ExitMessage: "VM is locked (backup)"}, nil
	}

	return cluster.TaskStatus{UPID: upid, State: cluster.TaskOK}, nil
}

// TestBulkAction_FailedTaskReportsError - a power action whose Proxmox task
// fails is reported as "error" with the task's exit message, while the other
// target's OK task stays "ok". Both results carry their UPID.
//
//nolint:paralleltest // serial: shared fake dataset
func TestBulkAction_FailedTaskReportsError(t *testing.T) {
	results := vm.BulkAction(context.Background(), vm.BulkDeps{
		Resolver:  bulkTestResolver(t),
		Actor:     aliceIdentity(),
		Writer:    taskBulkWriter{failVMID: 101},
		Audit:     noopAudit{},
		Refresher: noopRefresher{},
		TaskWait:  time.Second,
	}, []vm.BulkTarget{
		{Cluster: testClusterName, VMID: 101},
		{Cluster: testClusterName, VMID: 124},
	}, "start")

	if len(results) != 2 {
		t.Fatalf("results = %d, want 2", len(results))
	}

	failed := results[0]
	if failed.Status != bulkStatusError || failed.Message != "VM is locked (backup)" {
		t.Errorf("failed task result = %+v, want error with exit message", failed)
	}

	okResult := results[1]
	if okResult.Status != "ok" || okResult.Upid != taskBulkUPID(124) {
		t.Errorf("ok task result = %+v, want ok with upid %s", okResult, taskBulkUPID(124))
	}
}
