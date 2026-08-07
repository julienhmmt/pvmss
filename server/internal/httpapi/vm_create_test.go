package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"strings"
	"testing"
	"time"
)

// newVmCreateHandler builds the creation handler over the fake cluster with a
// real seeded store (the catalog fixture comes from migration version 7).
// Every test that creates a VM mutates the fake dataset, so cleanup resets it.
func newVmCreateHandler(t *testing.T) (*httpapi.VmCreate, *httpapi.Auth, *store.Store) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)
	authHandler := newAuthHandler(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	st, err := store.Open(config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "vm-create.db"),
		LogLevel:  "info",
		LogFormat: "json",
		LogOutput: "stdout",
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	return httpapi.NewVmCreate(authHandler, st, cluster.Fake{}, logger), authHandler, st
}

func postVMCreate(t *testing.T, handler *httpapi.VmCreate, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/vms", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if cookie != nil {
		req.AddCookie(cookie)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	return recorder
}

// TestVmCreate_SimpleModeSuccess — T009/US1: a simple-mode request returns
// 202 and the fake receives a spec whose pool is the actor's own.
func TestVmCreate_SimpleModeSuccess(t *testing.T) {
	handler, authHandler, _ := newVmCreateHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	response := postVMCreate(t, handler,
		`{"cluster":"default","name":"web-04","profileId":"medium","startAfterCreate":true}`, cookie)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusAccepted, response.Body.String())
	}

	var result struct {
		Cluster string `json:"cluster"`
		VMID    int    `json:"vmid"`
		Name    string `json:"name"`
		Node    string `json:"node"`
		UPID    string `json:"upid"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode 202 body: %v", err)
	}

	if result.VMID < 1 || result.UPID == "" || result.Name != "web-04" || result.Cluster != "default" {
		t.Fatalf("unexpected result: %+v", result)
	}

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for _, vm := range snap.VMs {
		if vm.VMID == result.VMID {
			if vm.Pool != "pool-alice" {
				t.Fatalf("created VM pool = %q, want pool-alice", vm.Pool)
			}

			return
		}
	}

	t.Fatalf("created VM %d not in snapshot", result.VMID)
}

// TestVmCreate_PoolFieldHasNoEffect — T010/SC-003: a forged pool field in the
// raw body either fails strict decoding or is dropped; the created VM's pool
// is the actor's regardless.
func TestVmCreate_PoolFieldHasNoEffect(t *testing.T) {
	handler, authHandler, _ := newVmCreateHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	response := postVMCreate(t, handler,
		`{"cluster":"default","name":"web-05","profileId":"small","pool":"pool-bob"}`, cookie)
	if response.Code == http.StatusBadRequest {
		return // strict decoder rejected the unknown field — equally valid (quickstart SC-003)
	}

	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202 or 400: %s", response.Code, response.Body.String())
	}

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for _, vm := range snap.VMs {
		if vm.Name == "web-05" && vm.Pool != "pool-alice" {
			t.Fatalf("forged pool took effect: pool = %q", vm.Pool)
		}
	}
}

// TestVmCreate_NoPoolIdentityForbidden — T011/FR-005: the local admin has no
// personal pool; creation is refused with 403 and NextVMID is never called
// (no VMID burned — the fake's create call log stays empty).
func TestVmCreate_NoPoolIdentityForbidden(t *testing.T) {
	handler, authHandler, _ := newVmCreateHandler(t)

	response := serveJSON(authHandler.AdminLogin, "/api/v1/auth/admin-login", `{"password":"pvmss-local-admin"}`)
	if response.Code != http.StatusOK {
		t.Fatalf("admin login status = %d", response.Code)
	}

	cookie := response.Result().Cookies()[0]

	res := postVMCreate(t, handler,
		`{"cluster":"default","name":"web-06","profileId":"small"}`, cookie)
	if res.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d: %s", res.Code, http.StatusForbidden, res.Body.String())
	}

	var body apiErrorEnvelope
	if err := json.Unmarshal(res.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}

	if body.Code != "no_pool" {
		t.Fatalf("error code = %q, want no_pool", body.Code)
	}

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("forbidden request reached the cluster: %+v", calls)
	}
}

// TestVmCreate_CatalogViolation — SC-004: a storage outside the seeded
// catalog is rejected with 400 and no task is created.
func TestVmCreate_CatalogViolation(t *testing.T) {
	handler, authHandler, _ := newVmCreateHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	response := postVMCreate(t, handler,
		`{"cluster":"default","name":"web-07","node":"pve-node-01","disk":{"storage":"not-a-real-storage","sizeGB":20},"network":{"bridge":"vmbr0","model":"virtio"},"cpuCores":1,"memoryMB":1024}`,
		cookie)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
	}

	var body apiErrorEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}

	if body.Code != "not_approved" {
		t.Fatalf("error code = %q, want not_approved", body.Code)
	}

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("rejected request reached the cluster: %+v", calls)
	}
}

// TestVmCreateCatalog_SeededShape — T013/FR-002: the catalog endpoint serves
// the seeded fixture (contract shape), sourced from the store.
func TestVmCreateCatalog_SeededShape(t *testing.T) {
	handler, authHandler, _ := newVmCreateHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vm-create/catalog", nil)
	req.AddCookie(cookie)

	recorder := httptest.NewRecorder()
	handler.ServeCatalog(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var body struct {
		Cluster  string   `json:"cluster"`
		Nodes    []string `json:"nodes"`
		Storages []struct {
			Name string `json:"name"`
			Node string `json:"node"`
		} `json:"storages"`
		Bridges []string `json:"bridges"`
		ISOs    []struct {
			Storage string `json:"storage"`
			File    string `json:"file"`
		} `json:"isos"`
		Profiles []struct {
			ID       string `json:"id"`
			Label    string `json:"label"`
			CPUCores int    `json:"cpuCores"`
			MemoryMB int    `json:"memoryMB"`
			DiskGB   int    `json:"diskGB"`
			Bus      string `json:"bus"`
		} `json:"profiles"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}

	if body.Cluster != "default" {
		t.Errorf("cluster = %q, want default", body.Cluster)
	}

	if len(body.Nodes) != 2 || len(body.Storages) != 3 || len(body.Bridges) != 2 || len(body.ISOs) != 2 || len(body.Profiles) != 3 {
		t.Errorf("catalog size mismatch: %+v", body)
	}

	if body.Profiles[0].ID == "" || body.Profiles[0].CPUCores < 1 {
		t.Errorf("profile row malformed: %+v", body.Profiles[0])
	}
}

// TestVmCreate_DetailedModeExactSpec — T024: every field explicit; the fake
// receives exactly those values, and no profile is involved.
func TestVmCreate_DetailedModeExactSpec(t *testing.T) {
	handler, authHandler, _ := newVmCreateHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	response := postVMCreate(t, handler, `{
		"cluster":"default","name":"web-03","node":"pve-node-02","tags":["team-web"],
		"cpuCores":3,"memoryMB":3072,
		"disk":{"storage":"ceph-data","sizeGB":64},
		"network":{"bridge":"vmbr1","model":"e1000"},
		"iso":{"storage":"local","file":"debian-12-generic-amd64.iso"},
		"startAfterCreate":true
	}`, cookie)
	if response.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusAccepted, response.Body.String())
	}

	var result struct {
		VMID int `json:"vmid"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode 202 body: %v", err)
	}

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for _, created := range snap.VMs {
		if created.VMID != result.VMID {
			continue
		}

		if created.Node != "pve-node-02" || created.CPUCores != 3 ||
			created.MemoryTotal != 3072*1024*1024 || created.DiskTotal != 64*1024*1024*1024 {
			t.Fatalf("created spec mismatch: %+v", created)
		}

		return
	}

	t.Fatalf("created VM %d not in snapshot", result.VMID)
}

// TestVmCreate_DetailedCatalogViolations — T025/SC-004: each resource kind
// outside the seeded catalog is rejected individually, no task created.
func TestVmCreate_DetailedCatalogViolations(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"node", `{"cluster":"default","name":"web-10","node":"pve-node-03","cpuCores":1,"memoryMB":1024,"disk":{"storage":"local-lvm","sizeGB":20},"network":{"bridge":"vmbr0"}}`},
		{"storage", `{"cluster":"default","name":"web-10","node":"pve-node-01","cpuCores":1,"memoryMB":1024,"disk":{"storage":"nas-scratch","sizeGB":20},"network":{"bridge":"vmbr0"}}`},
		{"bridge", `{"cluster":"default","name":"web-10","node":"pve-node-01","cpuCores":1,"memoryMB":1024,"disk":{"storage":"local-lvm","sizeGB":20},"network":{"bridge":"vmbr9"}}`},
		{"iso", `{"cluster":"default","name":"web-10","node":"pve-node-01","cpuCores":1,"memoryMB":1024,"disk":{"storage":"local-lvm","sizeGB":20},"network":{"bridge":"vmbr0"},"iso":{"storage":"local","file":"not-approved.iso"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, authHandler, _ := newVmCreateHandler(t)
			cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

			response := postVMCreate(t, handler, tc.body, cookie)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
			}

			var body apiErrorEnvelope
			if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode error body: %v", err)
			}

			if body.Code != "not_approved" {
				t.Fatalf("error code = %q, want not_approved", body.Code)
			}

			if calls := cluster.FakeCalls(); len(calls) != 0 {
				t.Fatalf("rejected request reached the cluster: %+v", calls)
			}
		})
	}
}

// TestVmCreate_DetailedInvalidHostname — T026: the detailed path enforces the
// same hostname rule as simple mode (FR-007).
func TestVmCreate_DetailedInvalidHostname(t *testing.T) {
	handler, authHandler, _ := newVmCreateHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	response := postVMCreate(t, handler,
		`{"cluster":"default","name":"-bad-","node":"pve-node-01","cpuCores":1,"memoryMB":1024,"disk":{"storage":"local-lvm","sizeGB":20},"network":{"bridge":"vmbr0"}}`,
		cookie)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
	}

	var body apiErrorEnvelope
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body: %v", err)
	}

	if body.Code != "invalid_name" {
		t.Fatalf("error code = %q, want invalid_name", body.Code)
	}
}

// TestVmCreate_DetailedOutOfRange — T027/FR-008: hardware values past the
// fixed technical ceiling are rejected before any cluster call.
func TestVmCreate_DetailedOutOfRange(t *testing.T) {
	cases := []struct {
		name   string
		cpu    int
		memory int
		diskGB int
	}{
		{"cpu over ceiling", 33, 4096, 40},
		{"memory over ceiling", 2, 65537, 40},
		{"disk over ceiling", 2, 4096, 2049},
		{"cpu zero", 0, 4096, 40},
		{"memory zero", 2, 0, 40},
		{"disk zero", 2, 4096, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, authHandler, _ := newVmCreateHandler(t)
			cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

			body := fmt.Sprintf(
				`{"cluster":"default","name":"web-11","node":"pve-node-01","cpuCores":%d,"memoryMB":%d,"disk":{"storage":"local-lvm","sizeGB":%d},"network":{"bridge":"vmbr0"}}`,
				tc.cpu, tc.memory, tc.diskGB)

			response := postVMCreate(t, handler, body, cookie)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
			}

			var envelope apiErrorEnvelope
			if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
				t.Fatalf("decode error body: %v", err)
			}

			if envelope.Code != "out_of_range" {
				t.Fatalf("error code = %q, want out_of_range", envelope.Code)
			}

			if calls := cluster.FakeCalls(); len(calls) != 0 {
				t.Fatalf("rejected request reached the cluster: %+v", calls)
			}
		})
	}
}

// newTasksHandler builds the task-status handler with a real worker so the// ok-transition genuinely rebuilds the projection (FR-018).
func newTasksHandler(t *testing.T) (*httpapi.Tasks, *httpapi.Auth, *inventory.Projection) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)
	authHandler := newAuthHandler(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	projection := buildProjectionWithIndex(t, snap, time.Now())
	worker := inventory.NewWorker(cluster.Fake{}, projection, time.Hour, logger)

	return httpapi.NewTasks(authHandler, cluster.Fake{}, worker, logger), authHandler, projection
}
