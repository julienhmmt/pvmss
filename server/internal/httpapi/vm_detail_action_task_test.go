package httpapi_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"pvmss/server/internal/testfixture"
	"testing"
	"time"
)

// taskWriter is the Fake writer plus a Proxmox-style UPID for every power action.
type taskWriter struct {
	cluster.Fake
	upid string
}

func (w taskWriter) ActionTask(ctx context.Context, node string, vmid int, action string) (string, error) {
	if err := w.Fake.Action(ctx, node, vmid, action); err != nil {
		return "", err
	}

	return w.upid, nil
}

// taskCreator reports a fixed task status for every UPID.
type taskCreator struct {
	cluster.Fake
	status cluster.TaskStatus
}

func (c taskCreator) TaskStatus(_ context.Context, _ string) (cluster.TaskStatus, error) {
	return c.status, nil
}

const testActionUPID = "UPID:pve-node-01:000A1B2C:0F00:6500ABCD:qmstart:101:root@pam:"

// newActionTaskHandler wires the VM detail handler with a task-aware writer and
// task reader. The VM is the seeded stopped web-02 (VMID 101) owned by alice.
func newActionTaskHandler(t *testing.T, creator taskCreator) (*httpapi.VMDetail, *httpapi.Auth) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	projection := buildProjectionWithIndex(t, snap, time.Now())
	authHandler := newAuthHandler(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	st, err := store.Open(config.Configuration{
		Port:      50001,
		DBPath:    filepath.Join(t.TempDir(), "action-task.db"),
		LogLevel:  snapshotTestLogLevel,
		LogFormat: snapshotTestLogFormat,
		LogOutput: snapshotTestLogOutput,
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	testfixture.SeedDemoFixtures(t, st)
	seedBridgeApprovals(t, st)

	worker := inventory.NewWorker(cluster.Fake{}, projection, time.Hour, logger)
	handler := httpapi.NewVMDetailWithRegistry(httpapi.VMDetailDeps{
		Projection:   projection,
		Auth:         authHandler,
		Writer:       taskWriter{upid: testActionUPID},
		Store:        st,
		Refresher:    worker,
		StatusReader: cluster.Fake{},
		Creator:      creator,
		Log:          logger,
	})

	return handler, authHandler
}

func postAction(t *testing.T, handler *httpapi.VMDetail, cookie *http.Cookie, action string) (int, map[string]any) {
	t.Helper()

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, detailRequest(http.MethodPost, "/api/v1/vms/default/101/actions", `{"action":"`+action+`"}`, cookie))

	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body %q: %v", rec.Body.String(), err)
	}

	return rec.Code, body
}

func TestVmAction_FailedTaskReturns409TaskFailed(t *testing.T) {
	const exitMsg = "VM is locked (shutdown)"

	handler, authHandler := newActionTaskHandler(t, taskCreator{
		status: cluster.TaskStatus{State: cluster.TaskError, ExitMessage: exitMsg},
	})

	code, body := postAction(t, handler, aliceCookie(t, authHandler), "start")

	if code != http.StatusConflict {
		t.Fatalf("status = %d, want 409; body=%v", code, body)
	}

	if body["code"] != "task_failed" {
		t.Errorf("code = %v, want task_failed", body["code"])
	}

	if body["message"] != exitMsg {
		t.Errorf("message = %v, want %q", body["message"], exitMsg)
	}
}

func TestVmAction_OKTaskReturns200WithUPID(t *testing.T) {
	handler, authHandler := newActionTaskHandler(t, taskCreator{
		status: cluster.TaskStatus{State: cluster.TaskOK},
	})

	code, body := postAction(t, handler, aliceCookie(t, authHandler), "start")

	if code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%v", code, body)
	}

	if body["status"] != "ok" || body["upid"] != testActionUPID {
		t.Errorf("body = %v, want status ok and upid %q", body, testActionUPID)
	}
}

func TestVmAction_RunningTaskPastDeadlineReturns202(t *testing.T) {
	prev := httpapi.ActionTaskWait
	httpapi.ActionTaskWait = 50 * time.Millisecond

	t.Cleanup(func() { httpapi.ActionTaskWait = prev })

	handler, authHandler := newActionTaskHandler(t, taskCreator{
		status: cluster.TaskStatus{State: cluster.TaskRunning},
	})

	code, body := postAction(t, handler, aliceCookie(t, authHandler), "start")

	if code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202; body=%v", code, body)
	}

	if body["status"] != "accepted" || body["upid"] != testActionUPID {
		t.Errorf("body = %v, want status accepted and upid %q", body, testActionUPID)
	}
}
