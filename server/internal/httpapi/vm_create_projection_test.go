package httpapi_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/vm"
	"testing"
	"time"
)

// workerResolver resolves the single test cluster to a real inventory worker.
type workerResolver struct {
	worker *inventory.Worker
}

func (r workerResolver) RefresherFor(string) (vm.IndexRefresher, error) {
	return r.worker, nil
}

// TestVMCreate_ProjectionContainsVMAfterTaskCompletes - a create that returns
// 202 must surface the new VM in the inventory projection once its task
// completes, without any manual refresh or task-tray poll.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures, poll var
func TestVMCreate_ProjectionContainsVMAfterTaskCompletes(t *testing.T) {
	origPoll := vm.CreateTaskPoll
	vm.CreateTaskPoll = time.Millisecond

	t.Cleanup(func() { vm.CreateTaskPoll = origPoll })

	handler, authHandler, _ := newVMCreateHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	projection := inventory.NewProjection()
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	worker := inventory.NewWorker(cluster.Fake{}, projection, time.Hour, logger)
	handler.SetInventoryRefreshers(workerResolver{worker: worker})

	response := postVMCreate(t, handler,
		`{"cluster":"default","name":"web-proj","profileId":"medium"}`, cookie)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusAccepted, response.Body.String())
	}

	var result struct {
		VMID int `json:"vmid"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode create response: %v", err)
	}

	deadline := time.Now().Add(15 * time.Second)

	for {
		if idx := projection.Load(); idx != nil {
			if _, ok := idx.ByVMID[result.VMID]; ok {
				return
			}
		}

		if time.Now().After(deadline) {
			t.Fatalf("VM %d never appeared in the inventory projection", result.VMID)
		}

		time.Sleep(10 * time.Millisecond)
	}
}
