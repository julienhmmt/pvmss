//nolint:noctx // test scaffolding does not need real context
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
	"pvmss/server/internal/vm"
	"strings"
	"testing"
	"time"
)

// bulkTargetDTO mirrors the request body's target shape.
type bulkTargetDTO struct {
	Cluster string `json:"cluster"`
	VMID    int    `json:"vmid"`
}

type bulkActionRequestDTO struct {
	Action  string          `json:"action"`
	Targets []bulkTargetDTO `json:"targets"`
}

type bulkTargetResultDTO struct {
	Cluster string `json:"cluster"`
	VMID    int    `json:"vmid"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type bulkActionResponseDTO struct {
	Results []bulkTargetResultDTO `json:"results"`
}

// newVMBulkHandler builds the bulk handler over the fake dataset with a real
// audit store and a real worker (so post-write refresh rebuilds the
// projection from the fake's mutated state). Every test that triggers a write
// MUST defer cluster.ResetFake() so later tests see the full 25-VM dataset.
func newVMBulkHandler(t *testing.T) (*httpapi.VMBulk, *httpapi.Auth) {
	t.Helper()
	t.Cleanup(cluster.ResetFake)

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	projection := buildProjectionWithIndex(t, snap, time.Now())
	authHandler := newAuthHandler(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	cfg := config.Configuration{
		Port:      50001,
		DBPath:    filepath.Join(t.TempDir(), "vm-bulk.db"),
		LogLevel:  snapshotTestLogLevel,
		LogFormat: snapshotTestLogFormat,
		LogOutput: snapshotTestLogOutput,
	}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	worker := inventory.NewWorker(cluster.Fake{}, projection, time.Hour, logger)
	handler := httpapi.NewVMBulk(projection, authHandler, cluster.Fake{}, st, worker, logger)

	return handler, authHandler
}

func bulkRequest(body string, cookie *http.Cookie) *http.Request {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/vms/bulk-action", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	if cookie != nil {
		req.AddCookie(cookie)
	}

	return req
}

func serveBulk(handler *httpapi.VMBulk, req *http.Request) (*httptest.ResponseRecorder, bulkActionResponseDTO) {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var resp bulkActionResponseDTO
	if rec.Code == http.StatusOK {
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	}

	return rec, resp
}

func serveBulkError(handler *httpapi.VMBulk, req *http.Request) (*httptest.ResponseRecorder, apiErrorEnvelope) {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	var env apiErrorEnvelope

	_ = json.Unmarshal(rec.Body.Bytes(), &env)

	return rec, env
}

func bulkTargets(ids ...int) []bulkTargetDTO {
	out := make([]bulkTargetDTO, len(ids))
	for i, id := range ids {
		out[i] = bulkTargetDTO{Cluster: auditTestCluster, VMID: id}
	}

	return out
}

// bulkNoopRefresher satisfies vm.IndexRefresher without touching the inventory
// worker — used in cross-cluster tests where a real per-cluster worker isn't
// wired.
type bulkNoopRefresher struct{}

func (bulkNoopRefresher) Refresh(context.Context) (time.Time, error) { return time.Now(), nil }

func bulkBody(action string, targets []bulkTargetDTO) string {
	body, _ := json.Marshal(bulkActionRequestDTO{Action: action, Targets: targets})
	return string(body)
}

// =============================================================================
// Phase 3 — User Story 1: POST /vms/bulk-action (T005–T009)
// =============================================================================

// TestVMBulk_AllOwnedStatusCompatible — T005: valid batch, all targets owned
// and status-compatible → 200, results has one ok entry per target.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMBulk_AllOwnedStatusCompatible(t *testing.T) {
	handler, authHandler := newVMBulkHandler(t)
	cookie := aliceCookie(t, authHandler)

	// VM 101 and 124 are stopped, owned by alice — start succeeds on both.
	rec, resp := serveBulk(handler, bulkRequest(bulkBody("start", bulkTargets(101, 124)), cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if len(resp.Results) != 2 {
		t.Fatalf("results = %d, want 2", len(resp.Results))
	}

	for i, r := range resp.Results {
		if r.Status != "ok" {
			t.Errorf("result[%d].Status = %q, want ok; message=%q", i, r.Status, r.Message)
		}
	}
}

// TestVMBulk_SpansTwoClusters — T006: batch spanning two clusters → each
// target resolved against its own cluster's index, both succeed independently.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMBulk_SpansTwoClusters(t *testing.T) {
	t.Cleanup(cluster.ResetFake)

	snap, err := (cluster.Fake{}).Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	defaultIdx := inventory.BuildIndexForCluster(auditTestCluster, snap)
	secondaryIdx := inventory.BuildIndexForCluster(crossSecondaryCluster, snap)
	registry := inventory.NewRegistryFromIndexes(map[string]*inventory.Index{auditTestCluster: &defaultIdx, crossSecondaryCluster: &secondaryIdx})
	projection := inventory.NewProjectionFromIndex(&defaultIdx)
	authHandler := newAuthHandler(t)
	logger := slog.New(slog.NewTextHandler(testWriter{t}, nil))

	cfg := config.Configuration{
		DBPath:    filepath.Join(t.TempDir(), "vm-bulk-cross.db"),
		LogLevel:  snapshotTestLogLevel,
		LogFormat: snapshotTestLogFormat,
		LogOutput: snapshotTestLogOutput,
	}

	st, err := store.Open(cfg)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}

	t.Cleanup(func() { _ = st.Close() })

	handler := httpapi.NewVMBulkWithRegistry(httpapi.VMBulkRegistryDeps{Registry: registry, Projection: projection, Auth: authHandler, Writer: cluster.Fake{}, Store: st, Refresher: bulkNoopRefresher{}, Log: logger, Clients: nil})
	cookie := aliceCookie(t, authHandler)

	// VM 101 in default (stopped) and VM 124 in secondary (stopped) — both
	// owned by alice, both start succeeds.
	body := `{"action":"start","targets":[{"cluster":"default","vmid":101},{"cluster":"secondary","vmid":124}]}`

	rec, resp := serveBulk(handler, bulkRequest(body, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if len(resp.Results) != 2 {
		t.Fatalf("results = %d, want 2", len(resp.Results))
	}

	if resp.Results[0].Cluster != "default" || resp.Results[0].Status != "ok" {
		t.Errorf("result[0] = %+v, want cluster=default status=ok", resp.Results[0])
	}

	if resp.Results[1].Cluster != "secondary" || resp.Results[1].Status != "ok" {
		t.Errorf("result[1] = %+v, want cluster=secondary status=ok", resp.Results[1])
	}
}

// TestVMBulk_InvalidAction — T007: invalid action string → 400 invalid_action,
// no target processed (fake client call log: 0 entries for this request).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMBulk_InvalidAction(t *testing.T) {
	handler, authHandler := newVMBulkHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveBulkError(handler, bulkRequest(bulkBody("foo", bulkTargets(101)), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if env.Code != "invalid_action" {
		t.Errorf("code = %q, want invalid_action", env.Code)
	}

	if !strings.Contains(env.Message, "foo") {
		t.Errorf("message = %q, want it to mention the unknown action", env.Message)
	}

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Errorf("fake calls = %d, want 0 (no target processed)", len(calls))
	}
}

// TestVMBulk_EmptyTargets — T008: empty targets → 400 empty_targets.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMBulk_EmptyTargets(t *testing.T) {
	handler, authHandler := newVMBulkHandler(t)
	cookie := aliceCookie(t, authHandler)

	rec, env := serveBulkError(handler, bulkRequest(bulkBody("start", bulkTargets()), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if env.Code != "empty_targets" {
		t.Errorf("code = %q, want empty_targets", env.Code)
	}
}

// TestVMBulk_TooManyTargets — T009: targets with 101 entries → 400
// too_many_targets, fake client call log: 0 entries (SC-003).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMBulk_TooManyTargets(t *testing.T) {
	handler, authHandler := newVMBulkHandler(t)
	cookie := aliceCookie(t, authHandler)

	targets := make([]bulkTargetDTO, vm.MaxBulkTargets+1)
	for i := range targets {
		targets[i] = bulkTargetDTO{Cluster: "default", VMID: 101}
	}

	rec, env := serveBulkError(handler, bulkRequest(bulkBody("start", targets), cookie))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}

	if env.Code != "too_many_targets" {
		t.Errorf("code = %q, want too_many_targets", env.Code)
	}

	if calls := cluster.FakeCalls(); len(calls) != 0 {
		t.Errorf("fake calls = %d, want 0 (ceiling rejected before any target runs)", len(calls))
	}
}

// TestVMBulk_Unauthenticated — no cookie → 401.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMBulk_Unauthenticated(t *testing.T) {
	handler, _ := newVMBulkHandler(t)

	rec, _ := serveBulkError(handler, bulkRequest(bulkBody("start", bulkTargets(101)), nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
	}
}

// =============================================================================
// Phase 4 — User Story 2: A mixed batch never fails as a whole (T017–T019)
// =============================================================================

// TestVMBulk_MixedOwnedAndNonOwned — T017: batch of 3 where 2 targets belong
// to alice and 1 to bob (forged directly) → 200, 3 result entries, bob's entry
// status "error", alice's 2 entries reflect their real outcome.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMBulk_MixedOwnedAndNonOwned(t *testing.T) {
	handler, authHandler := newVMBulkHandler(t)
	cookie := aliceCookie(t, authHandler)

	// 101: alice's, stopped → start → ok.
	// 124: alice's, stopped → start → ok.
	// 103: bob's (pool-bob), running → alice tries start → error (forbidden).
	body := `{"action":"start","targets":[{"cluster":"default","vmid":101},{"cluster":"default","vmid":124},{"cluster":"default","vmid":103}]}`

	rec, resp := serveBulk(handler, bulkRequest(body, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if len(resp.Results) != 3 {
		t.Fatalf("results = %d, want 3", len(resp.Results))
	}

	if resp.Results[0].Status != "ok" {
		t.Errorf("result[0] (101 alice) = %q, want ok", resp.Results[0].Status)
	}

	if resp.Results[1].Status != "ok" {
		t.Errorf("result[1] (124 alice) = %q, want ok", resp.Results[1].Status)
	}

	if resp.Results[2].Status != "error" {
		t.Errorf("result[2] (103 bob) = %q, want error", resp.Results[2].Status)
	}
}

// TestVMBulk_NonOwnedTargetZeroClientCalls — T018: same batch — fake client
// call log records zero calls for bob's (cluster, vmid) (S01-closure guarantee,
// per-target inside a batch).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMBulk_NonOwnedTargetZeroClientCalls(t *testing.T) {
	handler, authHandler := newVMBulkHandler(t)
	cookie := aliceCookie(t, authHandler)

	body := `{"action":"start","targets":[{"cluster":"default","vmid":101},{"cluster":"default","vmid":103}]}`

	rec, _ := serveBulk(handler, bulkRequest(body, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	// VM 103 is bob's — zero fake client calls for it.
	if calls := cluster.FakeCallsFor(103); len(calls) != 0 {
		t.Errorf("fake calls for 103 (non-owned) = %d, want 0", len(calls))
	}
	// VM 101 is alice's — one fake client call (the start).
	if calls := cluster.FakeCallsFor(101); len(calls) != 1 {
		t.Errorf("fake calls for 101 (owned) = %d, want 1", len(calls))
	}
}

// TestVMBulk_NonexistentVMNotFoundMessage — T019: a target naming a
// nonexistent VM → that entry's message is the same error T05's single-VM
// endpoint uses for the same case (vm.ErrNotFound).
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMBulk_NonexistentVMNotFoundMessage(t *testing.T) {
	handler, authHandler := newVMBulkHandler(t)
	cookie := aliceCookie(t, authHandler)

	body := `{"action":"start","targets":[{"cluster":"default","vmid":101},{"cluster":"default","vmid":999}]}`

	rec, resp := serveBulk(handler, bulkRequest(body, cookie))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	if len(resp.Results) != 2 {
		t.Fatalf("results = %d, want 2", len(resp.Results))
	}

	if resp.Results[1].Status != "error" {
		t.Errorf("result[1] (999 nonexistent) = %q, want error", resp.Results[1].Status)
	}
	// The message is vm.ErrNotFound.Error() — the same error T05's Action()
	// returns for a nonexistent VM, carried verbatim.
	if resp.Results[1].Message != vm.ErrNotFound.Error() {
		t.Errorf("result[1].Message = %q, want %q (vm.ErrNotFound verbatim)", resp.Results[1].Message, vm.ErrNotFound.Error())
	}
}

// =============================================================================
// Phase 5 — User Story 3: bearer-token auth (T022)
// =============================================================================

// TestVMBulk_BearerTokenAuth — T022: bearer-token-authenticated request →
// identical response shape and per-target semantics to the session-cookie
// path.
//
//nolint:paralleltest // serial: shared fake VM and database fixtures
func TestVMBulk_BearerTokenAuth(t *testing.T) {
	handler, authHandler := newVMBulkHandler(t)

	// Create a bearer token for alice.
	token := createBearerToken(t, authHandler, "alice", "pvmss-alice")

	req := httptest.NewRequest(http.MethodPost, "/api/v1/vms/bulk-action", strings.NewReader(bulkBody("start", bulkTargets(101))))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	rec, resp := serveBulk(handler, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	if len(resp.Results) != 1 {
		t.Fatalf("results = %d, want 1", len(resp.Results))
	}

	if resp.Results[0].Status != "ok" {
		t.Errorf("result[0] = %q, want ok; message=%q", resp.Results[0].Status, resp.Results[0].Message)
	}
}

// createBearerToken logs in as the given user and creates a bearer token via
// the auth tokens endpoint.
func createBearerToken(t *testing.T, authHandler *httpapi.Auth, username, password string) string {
	t.Helper()
	cookie := loginCookie(t, authHandler, `{"username":"`+username+`","password":"`+password+`"}`)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/tokens", strings.NewReader(`{"label":"bulk-test","scope":"read_write"}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)

	rec := httptest.NewRecorder()
	authHandler.CreateToken(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("CreateToken status = %d, want %d; body=%s", rec.Code, http.StatusCreated, rec.Body.String())
	}

	var tokenResp struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &tokenResp); err != nil {
		t.Fatalf("decode token response: %v", err)
	}

	return tokenResp.Value
}
