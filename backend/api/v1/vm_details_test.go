package apiv1_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	apiv1 "pvmss/api/v1"
	"pvmss/proxmox"
)

// ── GetVMConfig gate behavior ───────────────────────────────────────────────────
//
// GetVMConfig checks the `:id` param first, then the offline gate. In offline
// mode it returns 503 (proxmox_offline) BEFORE any snapshot lookup — there is no
// offline snapshot path for VM config today (see finding in plan 005). These
// tests lock that ordering so the perf refactor (plan 006) regresses loudly.

func TestGetVMConfig_InvalidVMID_ReturnsBadRequest(t *testing.T) {
	sm := newOfflineVMState(t)
	h := apiv1.MakeVMDetailsHandler(sm)

	w := httptest.NewRecorder()
	h.GetVMConfig(w, vmRequest(http.MethodGet, "/api/v1/vms/abc/config", "abc"))

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "invalid vm id")
}

func TestGetVMConfig_ZeroVMID_ReturnsBadRequest(t *testing.T) {
	sm := newOfflineVMState(t)
	h := apiv1.MakeVMDetailsHandler(sm)

	w := httptest.NewRecorder()
	h.GetVMConfig(w, vmRequest(http.MethodGet, "/api/v1/vms/0/config", "0"))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetVMConfig_MissingVMIDParam_ReturnsBadRequest(t *testing.T) {
	sm := newOfflineVMState(t)
	h := apiv1.MakeVMDetailsHandler(sm)

	// No httprouter param injected — requireVMID can't parse and must 400.
	w := httptest.NewRecorder()
	h.GetVMConfig(w, httptest.NewRequest(http.MethodGet, "/api/v1/vms//config", nil))

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestGetVMConfig_Offline_ReturnsServiceUnavailable(t *testing.T) {
	sm := newOfflineVMState(t)
	h := apiv1.MakeVMDetailsHandler(sm)

	w := httptest.NewRecorder()
	h.GetVMConfig(w, vmRequest(http.MethodGet, "/api/v1/vms/100/config", "100"))

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "proxmox_offline")
}

func TestGetVMMetrics_Offline_ReturnsServiceUnavailable(t *testing.T) {
	sm := newOfflineVMState(t)
	h := apiv1.MakeVMDetailsHandler(sm)

	w := httptest.NewRecorder()
	h.GetVMMetrics(w, vmRequest(http.MethodGet, "/api/v1/vms/100/metrics", "100"))

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "proxmox_offline")
}

func TestGetVMSettings_Offline_ReturnsServiceUnavailable(t *testing.T) {
	sm := newOfflineVMState(t)
	h := apiv1.MakeVMDetailsHandler(sm)

	w := httptest.NewRecorder()
	h.GetVMSettings(w, vmRequest(http.MethodGet, "/api/v1/vms/100/settings", "100"))

	require.Equal(t, http.StatusServiceUnavailable, w.Code)
	assert.Contains(t, w.Body.String(), "proxmox_offline")
}

// ── Network parse contract ──────────────────────────────────────────────────────
//
// GetVMConfig builds its Networks field from proxmox.ExtractNetworkInterfaces.
// The offline gate blocks a handler-level integration test, so we lock the parse
// contract directly on the exported func — this is the network shape the perf
// refactor (plan 006) must preserve. parseDisks/parseCloudInit are unexported and
// thus not reachable from this external test package (see plan 005 finding).

func TestExtractNetworkInterfaces_ParsesConfigLine(t *testing.T) {
	cfg := map[string]any{
		"net0": "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,tag=42,rate=10,mtu=1500,firewall=1",
		"net1": "e1000=11:22:33:44:55:66,bridge=vmbr1,link_down=1",
	}

	ifaces := proxmox.ExtractNetworkInterfaces(cfg)
	require.Len(t, ifaces, 2)

	n0 := ifaces[0]
	assert.Equal(t, "net0", n0.Index)
	assert.Equal(t, "virtio", n0.Model)
	assert.Equal(t, "AA:BB:CC:DD:EE:FF", n0.MACAddress)
	assert.Equal(t, "vmbr0", n0.Bridge)
	assert.Equal(t, 42, n0.VLAN)
	assert.Equal(t, "10", n0.Rate)
	assert.Equal(t, "1500", n0.MTU)
	assert.True(t, n0.Firewall)
	assert.False(t, n0.LinkDown)

	n1 := ifaces[1]
	assert.Equal(t, "net1", n1.Index)
	assert.Equal(t, "e1000", n1.Model)
	assert.Equal(t, "vmbr1", n1.Bridge)
	assert.True(t, n1.LinkDown)
	assert.False(t, n1.Firewall)
}

func TestExtractNetworkInterfaces_NilConfig(t *testing.T) {
	assert.Nil(t, proxmox.ExtractNetworkInterfaces(nil))
}

func TestGetVMConfig_ResponseTypeHasParsedNetworks(t *testing.T) {
	// Documents that the config response carries parsed NetworkInterface structs,
	// keeping the JSON contract stable for the frontend/perf refactor.
	var resp apiv1.VMConfigResponse
	resp.Networks = proxmox.ExtractNetworkInterfaces(map[string]any{
		"net0": "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0",
	})
	require.Len(t, resp.Networks, 1)
	assert.Equal(t, "vmbr0", resp.Networks[0].Bridge)
}
