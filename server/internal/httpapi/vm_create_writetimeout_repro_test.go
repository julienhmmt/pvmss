package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/vm"
	"strings"
	"testing"
	"time"
)

// TestVMCreate_SlowClone_StillReachesClientDespiteShortServerWriteTimeout is
// the regression test for the reported bug: a user creating a VM from a
// template saw "Échec de la création" even though the VM was created in
// Proxmox, because ServeHTTP blocks on vm.WaitCreateTask (up to
// vm.MaxCreateTaskWait) before writing any response, and the server's global
// WriteTimeout (10s, cmd/pvmss/main.go) is far shorter than a real clone can
// take. ServeHTTP now extends its own response write deadline
// (http.ResponseController.SetWriteDeadline) past MaxCreateTaskWait, so a
// slow creation still reaches the client with a normal 202 body instead of a
// dead connection. This test pins a server WriteTimeout far shorter than the
// simulated task wait to prove the handler's own deadline extension — not
// the configured server timeout — is what lets the response through.
//
//nolint:paralleltest,noctx // serial: shared fake dataset
func TestVMCreate_SlowClone_StillReachesClientDespiteShortServerWriteTimeout(t *testing.T) {
	handler, authHandler, _ := newVMCreateHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	// Shrink the task-wait poll interval so the fake's poll-counted task
	// (Running, Running, OK) still takes noticeably longer than the server's
	// WriteTimeout below, keeping the test fast while preserving the real
	// mechanism (a synchronous wait that would outlive a short deadline).
	originalPoll := vm.CreateTaskPoll
	vm.CreateTaskPoll = 100 * time.Millisecond
	t.Cleanup(func() { vm.CreateTaskPoll = originalPoll })

	ts := httptest.NewUnstartedServer(handler)
	// Far shorter than the ~200ms the fake task wait needs (two 100ms
	// polls) — mirrors production's 10s WriteTimeout being far shorter
	// than a real template clone. Without ServeHTTP's own deadline
	// extension, this would kill the response before it's written.
	ts.Config.WriteTimeout = 1 * time.Millisecond
	ts.Start()
	defer ts.Close()

	req, err := http.NewRequest(http.MethodPost, ts.URL+"/",
		strings.NewReader(`{"cluster":"default","name":"slow-clone-repro","profileId":"medium","startAfterCreate":true}`))
	if err != nil {
		t.Fatalf("build request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	client := &http.Client{Timeout: 5 * time.Second}
	resp, doErr := client.Do(req)
	if doErr != nil {
		t.Fatalf("client.Do: %v", doErr)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusAccepted)
	}

	var result struct {
		VMID int    `json:"vmid"`
		UPID string `json:"upid"`
		Name string `json:"name"`
	}
	if decodeErr := json.NewDecoder(resp.Body).Decode(&result); decodeErr != nil {
		t.Fatalf("decode 202 body: %v", decodeErr)
	}
	if result.VMID < 1 || result.UPID == "" || result.Name != "slow-clone-repro" {
		t.Fatalf("unexpected result: %+v", result)
	}
}
