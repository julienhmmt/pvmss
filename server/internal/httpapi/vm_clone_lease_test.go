package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strconv"
	"testing"
	"time"
)

func newCloneLeaseHandler(t *testing.T) (*httpapi.VMCreate, *httpapi.Auth, *store.Store) {
	t.Helper()

	handler, authHandler, st := newVMCreateHandler(t)

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	index := inventory.BuildIndex(snap)
	handler.SetInventorySource(inventory.NewRegistryFromIndexes(map[string]*inventory.Index{auditTestCluster: &index}))

	return handler, authHandler, st
}

// A clone of a leased cluster's VM gets a lease, and its creation identity is
// bound once the clone task finishes.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMClone_LeaseAndIdentityBound(t *testing.T) {
	handler, authHandler, st := newCloneLeaseHandler(t)
	seedLeasePolicy(t, st, 30)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	originalPoll := vm.CreateTaskPoll
	vm.CreateTaskPoll = 10 * time.Millisecond

	t.Cleanup(func() { vm.CreateTaskPoll = originalPoll })

	response := postVMClone(t, handler, strconv.Itoa(cloneSourceVMID(t)), `{"name":"lease-clone"}`, cookie)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202: %s", response.Code, response.Body.String())
	}

	var body struct {
		VMID int `json:"vmid"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode clone response: %v", err)
	}

	lease, err := st.GetVMLease(context.Background(), "default", body.VMID)
	if err != nil {
		t.Fatalf("clone has no lease: %v", err)
	}

	if lease.Pool == "" {
		t.Error("clone lease has no owner pool")
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if got, err := st.GetVMLease(context.Background(), "default", body.VMID); err == nil && got.Identity != "" {
			return
		}

		time.Sleep(20 * time.Millisecond)
	}

	t.Fatal("clone lease identity was not bound after the task")
}
