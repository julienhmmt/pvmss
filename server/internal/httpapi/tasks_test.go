package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
)

func getTask(t *testing.T, handler *httpapi.Tasks, upid string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/"+upid, nil)
	if cookie != nil {
		req.AddCookie(cookie)
	}
	// Path values are populated by the ServeMux; tests call the handler
	// directly, so set it manually (same convention as vm_detail_test.go).
	req.SetPathValue("upid", upid)
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder
}

// TestTasks_PollTransitions — T012/SC-006: the fake's poll-count state machine
// surfaces as running, running, then ok across three GET calls.
func TestTasks_PollTransitions(t *testing.T) {
	handler, authHandler, projection := newTasksHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	ctx := context.Background()
	vmid, err := (cluster.Fake{}).NextVMID(ctx)
	if err != nil {
		t.Fatalf("NextVMID: %v", err)
	}
	upid, err := (cluster.Fake{}).CreateVM(ctx, cluster.VMSpec{
		VMID:     vmid,
		Node:     "pve-node-01",
		Name:     "polled-vm",
		Pool:     "pool-alice",
		Tags:     []string{"pvmss"},
		CPUCores: 1,
		MemoryMB: 2048,
		Disk:     cluster.DiskSpec{Storage: "local-lvm", SizeGB: 20},
		Network:  cluster.NetworkSpec{Bridge: "vmbr0", Model: "virtio"},
	})
	if err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	for i, want := range []string{"running", "running", "ok"} {
		response := getTask(t, handler, upid, cookie)
		if response.Code != http.StatusOK {
			t.Fatalf("call %d: status = %d, want 200: %s", i+1, response.Code, response.Body.String())
		}
		var body struct {
			UPID  string   `json:"upid"`
			State string   `json:"state"`
			Log   []string `json:"log"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
			t.Fatalf("call %d: decode: %v", i+1, err)
		}
		if body.State != want {
			t.Fatalf("call %d: state = %q, want %q", i+1, body.State, want)
		}
		if body.UPID != upid {
			t.Errorf("call %d: upid = %q, want %q", i+1, body.UPID, upid)
		}
	}

	// FR-018: the ok observation invalidated the index — the refreshed
	// projection now contains the created VM.
	index := projection.Load()
	if index == nil {
		t.Fatalf("projection not populated after task completion")
	}
	created, ok := index.ByVMID[vmid]
	if !ok {
		t.Fatalf("created VM %d absent from projection after task ok", vmid)
	}
	if created.Name != "polled-vm" {
		t.Errorf("projection VM name = %q, want polled-vm", created.Name)
	}
}

// TestTasks_UnknownUPID — T012: an unknown upid is a 404.
func TestTasks_UnknownUPID(t *testing.T) {
	handler, authHandler, _ := newTasksHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	response := getTask(t, handler, "UPID:pve-node-01:00000000:00000000:00000000:qmcreate:999:nobody@pve:", cookie)
	if response.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusNotFound, response.Body.String())
	}
	var body apiErrorEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}
	if body.Code != "not_found" {
		t.Fatalf("error code = %q, want not_found", body.Code)
	}
}

// TestTasks_RequiresAuth — the endpoint is authenticated (T02), like every
// /api/v1 route.
func TestTasks_RequiresAuth(t *testing.T) {
	handler, _, _ := newTasksHandler(t)
	response := getTask(t, handler, "UPID:x", nil)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusUnauthorized)
	}
}
