//nolint:noctx // test scaffolding does not need real context
package httpapi_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"pvmss/server/internal/catalog"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/config"
	"pvmss/server/internal/httpapi"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"strings"
	"testing"
	"time"
)

type vmCreateSnapshotClient struct {
	cluster.Fake
	snapshot cluster.Snapshot
}

func (client vmCreateSnapshotClient) Snapshot(context.Context) (cluster.Snapshot, error) {
	return client.snapshot, nil
}

type vmCreateClientProvider struct {
	clients map[string]cluster.Client
}

func (provider vmCreateClientProvider) Client(name string) (cluster.Client, error) {
	client, ok := provider.clients[name]
	if !ok {
		return nil, cluster.ErrClusterNotFound
	}

	return client, nil
}

func (provider vmCreateClientProvider) List() []string {
	names := make([]string, 0, len(provider.clients))
	for name := range provider.clients {
		names = append(names, name)
	}

	return names
}

// newVMCreateHandler builds the creation handler over the fake cluster with a
// real seeded store (the catalog fixture comes from migration version 7).
// Every test that creates a VM mutates the fake dataset, so cleanup resets it.
func newVMCreateHandler(t *testing.T) (*httpapi.VMCreate, *httpapi.Auth, *store.Store) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)
	authHandler := newAuthHandler(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	st, err := store.Open(config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "vm-create.db"),
		LogLevel:  snapshotTestLogLevel,
		LogFormat: snapshotTestLogFormat,
		LogOutput: snapshotTestLogOutput,
	})
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })
	seedBridgeApprovals(t, st)
	seedISOApprovals(t, st)
	seedTagApprovals(t, st)

	provider := vmCreateClientProvider{clients: map[string]cluster.Client{auditTestCluster: cluster.Fake{}}}

	return httpapi.NewVMCreateWithRegistry(
		authHandler,
		st,
		provider,
		cluster.Fake{},
		cluster.Fake{},
		logger,
	), authHandler, st
}

func seedBridgeApprovals(t *testing.T, st *store.Store) {
	t.Helper()

	for _, node := range []string{cluster.FakeNode01, cluster.FakeNode02} {
		for _, name := range []string{testBridgeVmbr0, "vmbr1"} {
			if err := st.SetBridgeEnabled(context.Background(), "default", node, name, true); err != nil {
				t.Fatalf("seed bridge approval: %v", err)
			}
		}
	}
}

func seedISOApprovals(t *testing.T, st *store.Store) {
	t.Helper()

	isos := []struct {
		node, storage, file string
	}{
		{cluster.FakeNode01, cluster.FakeStorageLocal, "debian-12-generic-amd64.iso"},
		{cluster.FakeNode01, cluster.FakeStorageLocal, "ubuntu-24.04-server-amd64.iso"},
		{cluster.FakeNode02, cluster.FakeStorageLocal, "rocky-9-generic-x86_64.iso"},
	}
	for _, iso := range isos {
		if err := st.SetISOEnabled(context.Background(), "default", iso.node, iso.storage, iso.file, true); err != nil {
			t.Fatalf("seed iso approval: %v", err)
		}
	}
}

func seedStaleStorageApprovals(t *testing.T, st *store.Store) {
	t.Helper()

	for _, name := range []string{cluster.FakeStorageBackupNFS, cluster.FakeStoragePBS} {
		if err := st.SetStorageEnabled(context.Background(), "default", name, cluster.FakeNode03, true); err != nil {
			t.Fatalf("seed stale storage approval %q: %v", name, err)
		}
	}
}

// seedTagApprovals seeds the admin-curated tags the create tests reference
// (FR-013: tags outside the catalog are rejected).
func seedTagApprovals(t *testing.T, st *store.Store) {
	t.Helper()

	if err := st.InsertTag(context.Background(), "default", "team-web", "#3b82f6", time.Now().UTC().Format(time.RFC3339)); err != nil {
		t.Fatalf("seed tag approval: %v", err)
	}
}

func assertNoIneligibleStorageNames(t *testing.T, names []string) {
	t.Helper()

	for _, name := range names {
		if name == cluster.FakeStorageBackupNFS || name == cluster.FakeStoragePBS {
			t.Errorf("ineligible storage %q returned in options", name)
		}
	}
}

func postVMCreate(t *testing.T, handler *httpapi.VMCreate, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
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

// TestVMCreate_SimpleModeSuccess — T009/US1: a simple-mode request returns
// 202 and the fake receives a spec whose pool is the actor's own.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreate_SimpleModeSuccess(t *testing.T) {
	handler, authHandler, _ := newVMCreateHandler(t)
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

	if result.VMID < 1 || result.UPID == "" || result.Name != "web-04" || result.Cluster != auditTestCluster {
		t.Fatalf("unexpected result: %+v", result)
	}

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	for _, vm := range snap.VMs {
		if vm.VMID == result.VMID {
			if vm.Pool != cluster.FakePoolAlice {
				t.Fatalf("created VM pool = %q, want pool-alice", vm.Pool)
			}

			return
		}
	}

	t.Fatalf("created VM %d not in snapshot", result.VMID)
}

// TestVMCreate_PoolFieldHasNoEffect — T010/SC-003: a forged pool field in the
// raw body either fails strict decoding or is dropped; the created VM's pool
// is the actor's regardless.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreate_PoolFieldHasNoEffect(t *testing.T) {
	handler, authHandler, _ := newVMCreateHandler(t)
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

// TestVMCreate_AdminCannotCreate — an admin (local or cluster) cannot create
// VMs through the self-service portal; the response is 403 Forbidden with the
// admin_cannot_create error code.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreate_AdminCannotCreate(t *testing.T) {
	handler, authHandler, _ := newVMCreateHandler(t)

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

	if !strings.Contains(res.Body.String(), "admin_cannot_create") {
		t.Fatalf("response body = %s, want admin_cannot_create error code", res.Body.String())
	}
}

// TestVMCreate_CatalogViolation — SC-004: a storage outside the seeded
// catalog is rejected with 400 and no task is created.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreate_CatalogViolation(t *testing.T) {
	handler, authHandler, _ := newVMCreateHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	response := postVMCreate(t, handler,
		`{"cluster":"default","name":"web-07","node":"pve-node-01","disk":{"storage":"not-a-real-storage","sizeGB":20},"network":[{"bridge":"vmbr0","model":"virtio"}],"cpuCores":1,"memoryMB":1024}`,
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

// TestVMCreateCatalog_SeededShape — T013/FR-002: the catalog endpoint serves
// the seeded fixture (contract shape), sourced from the store.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreateCatalog_SeededShape(t *testing.T) {
	handler, authHandler, st := newVMCreateHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)
	seedStaleStorageApprovals(t, st)

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
		Bridges []struct {
			Name string `json:"name"`
			Node string `json:"node"`
		} `json:"bridges"`
		ISOs []struct {
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

	if body.Cluster != auditTestCluster {
		t.Errorf("cluster = %q, want default", body.Cluster)
	}

	if len(body.Nodes) != 2 || len(body.Storages) != 3 || len(body.Bridges) != 2 || len(body.ISOs) != 3 || len(body.Profiles) != 3 {
		t.Errorf("catalog size mismatch: %+v", body)
	}

	storageNames := make([]string, 0, len(body.Storages))
	for _, storage := range body.Storages {
		storageNames = append(storageNames, storage.Name)
	}

	assertNoIneligibleStorageNames(t, storageNames)

	if body.Profiles[0].ID == "" || body.Profiles[0].CPUCores < 1 {
		t.Errorf("profile row malformed: %+v", body.Profiles[0])
	}
}

// TestVMCreateCatalog_SelectsStorageClientByCluster verifies catalog discovery
// uses the requested cluster's client rather than the default client.
//
//nolint:paralleltest // serial: shared fake identity fixture
func TestVMCreateCatalog_SelectsStorageClientByCluster(t *testing.T) {
	const secondaryStorage = "secondary-images"

	_, authHandler, st := newVMCreateHandler(t)

	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	if err := st.SetStorageEnabled(
		context.Background(),
		crossSecondaryCluster,
		secondaryStorage,
		cluster.FakeNode02,
		true,
	); err != nil {
		t.Fatalf("approve secondary storage: %v", err)
	}

	provider := vmCreateClientProvider{clients: map[string]cluster.Client{
		auditTestCluster: vmCreateSnapshotClient{snapshot: cluster.Snapshot{Storages: []cluster.Storage{{
			Name: secondaryStorage, Node: cluster.FakeNode02, Type: "nfs", PluginType: "nfs", Content: "backup",
		}}}},
		crossSecondaryCluster: vmCreateSnapshotClient{snapshot: cluster.Snapshot{Storages: []cluster.Storage{{
			Name: secondaryStorage, Node: cluster.FakeNode02, Type: "dir", PluginType: "dir", Content: "images",
		}}}},
	}}

	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))
	handler := httpapi.NewVMCreateWithRegistry(
		authHandler,
		st,
		provider,
		cluster.Fake{},
		cluster.Fake{},
		logger,
	)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vm-create/catalog?cluster="+crossSecondaryCluster, nil)
	req.AddCookie(cookie)

	recorder := httptest.NewRecorder()
	handler.ServeCatalog(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d: %s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var body struct {
		Cluster  string `json:"cluster"`
		Storages []struct {
			Name string `json:"name"`
		} `json:"storages"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}

	if body.Cluster != crossSecondaryCluster {
		t.Errorf("cluster = %q, want secondary", body.Cluster)
	}

	if len(body.Storages) != 1 || body.Storages[0].Name != secondaryStorage {
		t.Errorf("storages = %+v, want selected cluster storage", body.Storages)
	}
}

// TestVMCreate_DetailedModeExactSpec — T024: every field explicit; the fake
// receives exactly those values, and no profile is involved.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreate_DetailedModeExactSpec(t *testing.T) {
	handler, authHandler, _ := newVMCreateHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	response := postVMCreate(t, handler, `{
		"cluster":"default","name":"web-03","node":"pve-node-02","tags":["team-web"],
		"cpuCores":3,"memoryMB":3072,
		"disk":{"storage":"ceph-data","sizeGB":64},
		"network":[{"bridge":"vmbr1","model":"e1000"}],
		"iso":{"storage":"local","file":"rocky-9-generic-x86_64.iso"},
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

// TestVMCreate_DetailedCatalogViolations — T025/SC-004: each resource kind
// outside the seeded catalog is rejected individually, no task created.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreate_DetailedCatalogViolations(t *testing.T) {
	cases := []struct {
		name string
		body string
	}{
		{"node", `{"cluster":"default","name":"web-10","node":"pve-node-03","cpuCores":1,"memoryMB":1024,"disk":{"storage":"local-lvm","sizeGB":20},"network":[{"bridge":"vmbr0"}]}`},
		{"storage", `{"cluster":"default","name":"web-10","node":"pve-node-01","cpuCores":1,"memoryMB":1024,"disk":{"storage":"nas-scratch","sizeGB":20},"network":[{"bridge":"vmbr0"}]}`},
		{"bridge", `{"cluster":"default","name":"web-10","node":"pve-node-01","cpuCores":1,"memoryMB":1024,"disk":{"storage":"local-lvm","sizeGB":20},"network":[{"bridge":"vmbr9"}]}`},
		{"iso", `{"cluster":"default","name":"web-10","node":"pve-node-01","cpuCores":1,"memoryMB":1024,"disk":{"storage":"local-lvm","sizeGB":20},"network":[{"bridge":"vmbr0"}],"iso":{"storage":"local","file":"not-approved.iso"}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			handler, authHandler, _ := newVMCreateHandler(t)
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

// TestVMCreate_DetailedInvalidHostname — T026: the detailed path enforces the
// same hostname rule as simple mode (FR-007).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreate_DetailedInvalidHostname(t *testing.T) {
	handler, authHandler, _ := newVMCreateHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	response := postVMCreate(t, handler,
		`{"cluster":"default","name":"-bad-","node":"pve-node-01","cpuCores":1,"memoryMB":1024,"disk":{"storage":"local-lvm","sizeGB":20},"network":[{"bridge":"vmbr0"}]}`,
		cookie)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", response.Code, http.StatusBadRequest, response.Body.String())
	}

	assertAPIError(t, response.Body.Bytes(), "invalid_name")
}

// TestVMCreate_DetailedOutOfRange — T027/FR-008: hardware values past the
// fixed technical ceiling are rejected before any cluster call.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreate_DetailedOutOfRange(t *testing.T) {
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
			handler, authHandler, _ := newVMCreateHandler(t)
			cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

			body := fmt.Sprintf(
				`{"cluster":"default","name":"web-11","node":"pve-node-01","cpuCores":%d,"memoryMB":%d,"disk":{"storage":"local-lvm","sizeGB":%d},"network":[{"bridge":"vmbr0"}]}`,
				tc.cpu, tc.memory, tc.diskGB,
			)

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

// --- T18 cloud-init template additive cases (T012) ---

// createCatalogTemplate inserts an enabled cloud-init template directly via the
// catalog layer and returns its id, for vm-create handler tests.
func createCatalogTemplate(t *testing.T, st *store.Store) string {
	t.Helper()

	tmpl, err := catalog.CreateCloudInitTemplate(context.Background(), st, auditTestCluster, "Web server", "#cloud-config\npackages:\n  - nginx\n")
	if err != nil {
		t.Fatalf("CreateCloudInitTemplate: %v", err)
	}

	return tmpl.ID
}

// TestVMCreateCatalog_CloudInitTemplatesEmptyWhileFeatureDisabled — GET
// .../catalog's cloudInitTemplates field is always empty while
// cloudImageFeatureEnabled is false (vm_create.go, 2026-09-04: per-VM
// cloud-init forking needs PVMSS to hold SSH credentials to every Proxmox
// node, judged too much scope for now — paused, not removed). An approved,
// enabled template still exists in the catalog store underneath; this
// confirms the HTTP layer's single choke point hides it regardless.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreateCatalog_CloudInitTemplatesEmptyWhileFeatureDisabled(t *testing.T) {
	handler, authHandler, st := newVMCreateHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)
	createCatalogTemplate(t, st)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/vm-create/catalog", nil)
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	handler.ServeCatalog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}

	var body struct {
		CloudInitTemplates []struct {
			ID    string `json:"id"`
			Label string `json:"label"`
		} `json:"cloudInitTemplates"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}

	if len(body.CloudInitTemplates) != 0 {
		t.Fatalf("cloudInitTemplates = %+v, want empty while the feature is disabled", body.CloudInitTemplates)
	}
}

// TestVMCreate_WithCloudInitTemplate_Success — POST /api/v1/vms with a valid
// template id returns 202 and the response includes cloudInitTemplateId.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreate_WithCloudInitTemplate_Success(t *testing.T) {
	handler, authHandler, st := newVMCreateHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)
	tmplID := createCatalogTemplate(t, st)

	// Proxmox's REST API cannot write a snippets-content file, so the
	// template's catalog Content never reaches the cluster directly — the
	// fake models the admin-placed baseline file via SetFakeSnippetPresent
	// (cluster.Writer.HasSnippet).
	cluster.SetFakeSnippetPresent(cluster.FakeNode01, cluster.FakeSnippetStorage, "pvmss-template-"+tmplID+".yml", true)
	t.Cleanup(func() {
		cluster.SetFakeSnippetPresent(cluster.FakeNode01, cluster.FakeSnippetStorage, "pvmss-template-"+tmplID+".yml", false)
	})

	rec := postVMCreate(t, handler,
		`{"cluster":"default","name":"web-20","profileId":"medium","cloudInitTemplateId":"`+tmplID+`"}`, cookie)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	var result struct {
		VMID                int    `json:"vmid"`
		CloudInitTemplateID string `json:"cloudInitTemplateId"`
		CloudInitPushError  string `json:"cloudInitPushError"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode 202: %v", err)
	}

	if result.CloudInitTemplateID != tmplID {
		t.Errorf("cloudInitTemplateId = %q, want %q", result.CloudInitTemplateID, tmplID)
	}

	if result.CloudInitPushError != "" {
		t.Errorf("cloudInitPushError = %q, want empty", result.CloudInitPushError)
	}
}

// TestVMCreate_WithCloudInitTemplate_PushFailure — a simulated push failure
// still returns 202 but the response carries cloudInitPushError (FR-008).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreate_WithCloudInitTemplate_PushFailure(t *testing.T) {
	handler, authHandler, st := newVMCreateHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)
	tmplID := createCatalogTemplate(t, st)

	cluster.SetFakeCloudInitPushError(errors.New("cluster client: push failed"))
	t.Cleanup(func() { cluster.SetFakeCloudInitPushError(nil) })

	rec := postVMCreate(t, handler,
		`{"cluster":"default","name":"web-21","profileId":"medium","cloudInitTemplateId":"`+tmplID+`"}`, cookie)
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusAccepted, rec.Body.String())
	}

	var result struct {
		CloudInitPushError string `json:"cloudInitPushError"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatalf("decode 202: %v", err)
	}

	if result.CloudInitPushError == "" {
		t.Error("cloudInitPushError should be non-empty on push failure")
	}
}

// TestVMCreate_UnknownCloudInitTemplate_Returns400 — an unknown template id is
// rejected with 400 not_approved before any VMID is allocated (FR-006, SC-004).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMCreate_UnknownCloudInitTemplate_Returns400(t *testing.T) {
	handler, authHandler, _ := newVMCreateHandler(t)
	cookie := loginCookie(t, authHandler, `{"username":"alice","password":"pvmss-alice"}`)

	rec := postVMCreate(t, handler,
		`{"cluster":"default","name":"web-22","profileId":"medium","cloudInitTemplateId":"does-not-exist"}`, cookie)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}

	assertAPIError(t, rec.Body.Bytes(), "not_approved")

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Fatalf("rejected request reached the cluster: %+v", calls)
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

	provider := vmCreateClientProvider{clients: map[string]cluster.Client{auditTestCluster: cluster.Fake{}}}

	return httpapi.NewTasksWithRegistry(authHandler, provider, cluster.Fake{}, worker, nil, logger), authHandler, projection
}
