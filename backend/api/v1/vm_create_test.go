package apiv1_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiv1 "pvmss/api/v1"
	"pvmss/database"
	"pvmss/state"
)

// newOfflineVMState builds a StateManager backed by a real in-memory DB with
// bootstrap completed and offline mode enabled. Handlers are called directly
// (bypassing JWT middleware); usernameFromCtx returns "" when the auth context
// key is absent. Offline mode makes the VM handlers characterize the
// no-Proxmox code paths (settings-derived data / offline gates) deterministically.
func newOfflineVMState(t *testing.T) state.StateManager {
	t.Helper()
	db, err := database.OpenMemory()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, db.CompleteBootstrap("test"))

	sm := state.MakeAppStateWithDB(db)
	require.NoError(t, sm.LoadSettingsFromDB())
	sm.SetOfflineMode()
	return sm
}

// vmRequest builds a GET request carrying the httprouter `:id` param in context,
// mirroring how the router injects it at runtime so requireVMID resolves.
func vmRequest(method, target, vmid string) *http.Request {
	r := httptest.NewRequest(method, target, nil)
	if vmid != "" {
		ctx := context.WithValue(r.Context(), httprouter.ParamsKey, httprouter.Params{
			{Key: "id", Value: vmid},
		})
		r = r.WithContext(ctx)
	}
	return r
}

// ── GET /api/v1/vm-create/settings (offline) ───────────────────────────────────

// TestGetVMCreateSettings_Offline_ReturnsSettingsDerivedData characterizes the
// offline path: ProxmoxConnected is false and nodes/storages/bridges/ISOs are
// derived from settings (not a live snapshot).
func TestGetVMCreateSettings_Offline_ReturnsSettingsDerivedData(t *testing.T) {
	sm := newOfflineVMState(t)

	s := sm.GetSettings()
	s.EnabledStorages = []string{"pve1:local-lvm"}
	s.VMBRs = []string{"pve1:vmbr0"}
	s.ISOs = []string{"local:iso/debian-12.iso"}
	s.Tags = []string{"web", "db"}
	require.NoError(t, sm.SetSettings(s))

	h := apiv1.MakeVMCreateHandler(sm)
	w := httptest.NewRecorder()
	h.GetSettings(w, httptest.NewRequest(http.MethodGet, "/api/v1/vm-create/settings", nil))

	require.Equal(t, http.StatusOK, w.Code)
	var resp apiv1.VMCreateSettingsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.False(t, resp.ProxmoxConnected, "offline mode must report not connected")

	require.Len(t, resp.Storages, 1)
	assert.Equal(t, "local-lvm", resp.Storages[0].Name)
	assert.Equal(t, "pve1", resp.Storages[0].Node)

	require.Len(t, resp.Bridges, 1)
	assert.Equal(t, "vmbr0", resp.Bridges[0].Name)
	assert.Equal(t, "pve1", resp.Bridges[0].Node)

	require.Len(t, resp.ISOs, 1)
	assert.Equal(t, "local:iso/debian-12.iso", resp.ISOs[0].VolID)
	assert.Equal(t, "debian-12.iso", resp.ISOs[0].Name, "ISO name is the basename of the volid")

	assert.ElementsMatch(t, []string{"web", "db"}, resp.Tags)
}

// TestGetVMCreateSettings_Offline_OnlyEnabledStoragesAppear characterizes the
// allowlist behavior offline: storages/bridges come solely from the settings
// allowlists, so anything not enabled never appears in the response.
func TestGetVMCreateSettings_Offline_OnlyEnabledStoragesAppear(t *testing.T) {
	sm := newOfflineVMState(t)

	s := sm.GetSettings()
	s.EnabledStorages = []string{"pve1:allowed-a", "pve1:allowed-b"}
	require.NoError(t, sm.SetSettings(s))

	h := apiv1.MakeVMCreateHandler(sm)
	w := httptest.NewRecorder()
	h.GetSettings(w, httptest.NewRequest(http.MethodGet, "/api/v1/vm-create/settings", nil))

	require.Equal(t, http.StatusOK, w.Code)
	var resp apiv1.VMCreateSettingsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	names := make([]string, 0, len(resp.Storages))
	for _, st := range resp.Storages {
		names = append(names, st.Name)
	}
	assert.ElementsMatch(t, []string{"allowed-a", "allowed-b"}, names)
	assert.NotContains(t, names, "denied", "a storage not in EnabledStorages must not appear")
}

// TestGetVMCreateSettings_Offline_EmptyListsWhenNoSettings characterizes the
// zero-value case: with empty allowlists the response carries empty (non-nil)
// slices so the JSON is `[]` rather than `null`.
func TestGetVMCreateSettings_Offline_EmptyListsWhenNoSettings(t *testing.T) {
	sm := newOfflineVMState(t)

	h := apiv1.MakeVMCreateHandler(sm)
	w := httptest.NewRecorder()
	h.GetSettings(w, httptest.NewRequest(http.MethodGet, "/api/v1/vm-create/settings", nil))

	require.Equal(t, http.StatusOK, w.Code)
	var resp apiv1.VMCreateSettingsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	assert.NotNil(t, resp.Nodes)
	assert.NotNil(t, resp.Storages)
	assert.NotNil(t, resp.Bridges)
	assert.Empty(t, resp.Storages)
	assert.Empty(t, resp.Bridges)
}
