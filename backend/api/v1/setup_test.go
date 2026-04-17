package apiv1_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiv1 "pvmss/api/v1"
	"pvmss/database"
	envpkg "pvmss/env"
	"pvmss/state"
)

// stubDB is a minimal database.DB stub that controls IsBootstrapComplete.
type stubDB struct {
	database.DB
	bootstrapComplete bool
	completeCalled    bool
}

func (s *stubDB) IsBootstrapComplete() (bool, error) {
	return s.bootstrapComplete, nil
}

func (s *stubDB) CompleteBootstrap(_ string) error {
	s.completeCalled = true
	return nil
}

// stubStateManager is a minimal state.StateManager stub.
type stubStateManager struct {
	state.StateManager
	offline     bool
	proxmoxOK   bool
	envCfg      *envpkg.EnvConfig
	nodesSet    []string
	storagesSet []string
	isosSet     []string
	vmbrsSet    []string
}

func (s *stubStateManager) IsOfflineMode() bool { return s.offline }
func (s *stubStateManager) GetProxmoxStatus() (bool, string) {
	return s.proxmoxOK, ""
}
func (s *stubStateManager) GetEnvConfig() *envpkg.EnvConfig { return s.envCfg }
func (s *stubStateManager) HasDB() bool                     { return true }
func (s *stubStateManager) SetEnabledNodes(nodes []string, _ string) error {
	s.nodesSet = nodes
	return nil
}
func (s *stubStateManager) SetEnabledStorages(storages []string, _ string) error {
	s.storagesSet = storages
	return nil
}
func (s *stubStateManager) SetEnabledISOs(isos []string, _ string) error {
	s.isosSet = isos
	return nil
}
func (s *stubStateManager) SetEnabledVMBRs(vmbrs []string, _ string) error {
	s.vmbrsSet = vmbrs
	return nil
}
func (s *stubStateManager) SetVMLimits(_ *database.VMLimits, _ string) error { return nil }

func newStubState(offline bool, proxmoxOK bool) *stubStateManager {
	return &stubStateManager{
		offline:   offline,
		proxmoxOK: proxmoxOK,
		envCfg: &envpkg.EnvConfig{
			ProxmoxURL: "https://pve.example.com:8006",
			Offline:    offline,
		},
	}
}

// --- Status tests ---

func TestSetupHandler_Status_NotComplete(t *testing.T) {
	db := &stubDB{bootstrapComplete: false}
	sm := newStubState(false, true)
	h := apiv1.MakeSetupHandler(sm, db)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/setup/status", nil)
	w := httptest.NewRecorder()
	h.Status(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp apiv1.SetupStatusResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.Complete)
	assert.True(t, resp.ProxmoxOK)
}

func TestSetupHandler_Status_Complete(t *testing.T) {
	db := &stubDB{bootstrapComplete: true}
	sm := newStubState(false, false)
	h := apiv1.MakeSetupHandler(sm, db)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/setup/status", nil)
	w := httptest.NewRecorder()
	h.Status(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp apiv1.SetupStatusResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Complete)
}

func TestSetupHandler_Status_OfflineMode(t *testing.T) {
	db := &stubDB{bootstrapComplete: false}
	sm := newStubState(true, false)
	h := apiv1.MakeSetupHandler(sm, db)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/setup/status", nil)
	w := httptest.NewRecorder()
	h.Status(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp apiv1.SetupStatusResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.Offline)
}

// --- TestConnection tests ---

func TestSetupHandler_TestConnection_OfflineMode(t *testing.T) {
	db := &stubDB{bootstrapComplete: false}
	sm := newStubState(true, false)
	h := apiv1.MakeSetupHandler(sm, db)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/test-connection", nil)
	w := httptest.NewRecorder()
	h.TestConnection(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp apiv1.SetupConnectionTestResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.OK)
	assert.Contains(t, resp.Error, "offline")
}

func TestSetupHandler_TestConnection_MissingCredentials(t *testing.T) {
	db := &stubDB{bootstrapComplete: false}
	sm := &stubStateManager{
		offline:   false,
		proxmoxOK: false,
		envCfg:    &envpkg.EnvConfig{ProxmoxURL: ""},
	}
	h := apiv1.MakeSetupHandler(sm, db)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/test-connection", nil)
	w := httptest.NewRecorder()
	h.TestConnection(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp apiv1.SetupConnectionTestResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.False(t, resp.OK)
	assert.NotEmpty(t, resp.Error)
}

// --- Complete tests ---

func TestSetupHandler_Complete_PersistsConfigAndMarksBootstrap(t *testing.T) {
	db := &stubDB{bootstrapComplete: false}
	sm := newStubState(false, false)
	h := apiv1.MakeSetupHandler(sm, db)

	body := `{
		"enabled_nodes":    ["node1", "node2"],
		"enabled_storages": ["local-lvm"],
		"enabled_isos":     ["debian-12.iso"],
		"enabled_vmbrs":    ["vmbr0"],
		"limits": {
			"max_vms": 10,
			"max_vm_per_user": 3,
			"max_network_cards": 2,
			"max_disk_per_vm": 4,
			"max_snapshots": 5,
			"allow_custom_yaml": false
		}
	}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/complete", strings.NewReader(body))
	w := httptest.NewRecorder()
	h.Complete(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.True(t, db.completeCalled, "CompleteBootstrap should have been called")
	assert.Equal(t, []string{"node1", "node2"}, sm.nodesSet)
	assert.Equal(t, []string{"local-lvm"}, sm.storagesSet)
	assert.Equal(t, []string{"debian-12.iso"}, sm.isosSet)
	assert.Equal(t, []string{"vmbr0"}, sm.vmbrsSet)
}

func TestSetupHandler_Complete_InvalidBody(t *testing.T) {
	db := &stubDB{bootstrapComplete: false}
	sm := newStubState(false, false)
	h := apiv1.MakeSetupHandler(sm, db)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/complete", strings.NewReader("not json"))
	w := httptest.NewRecorder()
	h.Complete(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.False(t, db.completeCalled)
}

// --- Middleware tests ---

func TestRequireSetupIncomplete_BlocksWhenComplete(t *testing.T) {
	db := &stubDB{bootstrapComplete: true}
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/complete", nil)
	w := httptest.NewRecorder()
	apiv1.RequireSetupIncompleteForTest(db, next).ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.False(t, called, "handler should not have been called")
}

func TestRequireSetupIncomplete_PassesWhenNotComplete(t *testing.T) {
	db := &stubDB{bootstrapComplete: false}
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/setup/complete", nil)
	w := httptest.NewRecorder()
	apiv1.RequireSetupIncompleteForTest(db, next).ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, called, "handler should have been called")
}
