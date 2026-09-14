package httpapi_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"pvmss/server/internal/cluster"
	"testing"
)

//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_Network_ChangesApprovedBridge(t *testing.T) {
	cluster.ResetFake()

	handler, authHandler, _, _ := newVMDetailHandler(t)
	request := detailRequest(http.MethodPut, "/api/v1/vms/default/101/network", `{"interfaces":[{"index":0,"bridge":"vmbr1","model":"virtio"}]}`, aliceCookie(t, authHandler))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	assertSingleFakeCall(t, 101, "update_network")
}

//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_Get_RunningVMReportsGuestIPs(t *testing.T) {
	cluster.ResetFake()

	handler, authHandler, _, _ := newVMDetailHandler(t)
	request := detailRequest(http.MethodGet, "/api/v1/vms/default/100", "", aliceCookie(t, authHandler))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var body struct {
		GuestAgent        string `json:"guestAgent"`
		NetworkInterfaces []struct {
			MAC         string   `json:"mac"`
			IPAddresses []string `json:"ipAddresses"`
		} `json:"networkInterfaces"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.GuestAgent != "ok" {
		t.Errorf("guestAgent = %q, want ok", body.GuestAgent)
	}

	if len(body.NetworkInterfaces) != 1 {
		t.Fatalf("networkInterfaces = %+v, want the seeded NIC", body.NetworkInterfaces)
	}

	if got := body.NetworkInterfaces[0].IPAddresses; len(got) != 1 || got[0] != "10.10.100.10" {
		t.Errorf("ipAddresses = %v, want the fake guest agent's address", got)
	}
}

//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_Get_StoppedVMReportsNoIPs(t *testing.T) {
	cluster.ResetFake()

	handler, authHandler, _, _ := newVMDetailHandler(t)
	request := detailRequest(http.MethodGet, "/api/v1/vms/default/101", "", aliceCookie(t, authHandler))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var body struct {
		GuestAgent        string `json:"guestAgent"`
		NetworkInterfaces []struct {
			IPAddresses []string `json:"ipAddresses"`
		} `json:"networkInterfaces"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.GuestAgent != "" {
		t.Errorf("guestAgent = %q, want absent for a stopped VM", body.GuestAgent)
	}

	if len(body.NetworkInterfaces) != 1 {
		t.Fatalf("networkInterfaces = %+v, want the seeded NIC", body.NetworkInterfaces)
	}

	if got := body.NetworkInterfaces[0].IPAddresses; len(got) != 0 {
		t.Errorf("ipAddresses = %v, want empty for a stopped VM (no agent call)", got)
	}
}

//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_Get_AgentDisabledExplainsAbsentIPs(t *testing.T) {
	cluster.ResetFake()

	handler, authHandler, _, _ := newVMDetailHandler(t)
	request := detailRequest(http.MethodGet, "/api/v1/vms/default/103", "", bobCookie(t, authHandler))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var body struct {
		GuestAgent string `json:"guestAgent"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.GuestAgent != "disabled" {
		t.Errorf("guestAgent = %q, want disabled (VM 103 is seeded agent=0)", body.GuestAgent)
	}

	// A disabled channel is known from the config — no agent probe may run.
	for _, call := range cluster.FakeCallsFor(103) {
		if call.Action == "guest_network_interfaces" {
			t.Fatal("guest agent probed a VM whose config disables the channel")
		}
	}
}

//nolint:paralleltest // serial: shared fake cluster dataset + failure knob
func TestVMDetail_Get_AgentUnreachableExplainsAbsentIPs(t *testing.T) {
	cluster.ResetFake()

	cluster.SetFakeGuestAgentPingFailures(1)
	defer cluster.SetFakeGuestAgentPingFailures(0)

	handler, authHandler, _, _ := newVMDetailHandler(t)
	request := detailRequest(http.MethodGet, "/api/v1/vms/default/100", "", aliceCookie(t, authHandler))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusOK, recorder.Body.String())
	}

	var body struct {
		GuestAgent        string `json:"guestAgent"`
		NetworkInterfaces []struct {
			IPAddresses []string `json:"ipAddresses"`
		} `json:"networkInterfaces"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}

	if body.GuestAgent != "unreachable" {
		t.Errorf("guestAgent = %q, want unreachable (agent call failed)", body.GuestAgent)
	}

	if len(body.NetworkInterfaces) == 1 && len(body.NetworkInterfaces[0].IPAddresses) != 0 {
		t.Errorf("ipAddresses = %v, want empty when the agent does not answer", body.NetworkInterfaces[0].IPAddresses)
	}
}

//nolint:paralleltest // serial: shared fake cluster dataset
func TestVMDetail_Network_RejectsForgedGuestFields(t *testing.T) {
	cluster.ResetFake()

	handler, authHandler, _, _ := newVMDetailHandler(t)
	request := detailRequest(http.MethodPut, "/api/v1/vms/default/101/network", `{"interfaces":[{"index":0,"bridge":"vmbr1","model":"virtio","mac":"00:00:00:00:00:00"}]}`, aliceCookie(t, authHandler))
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", recorder.Code, http.StatusBadRequest, recorder.Body.String())
	}

	if calls := cluster.FakeCallsFor(101); len(calls) != 0 {
		t.Fatalf("fake calls = %+v, want none", calls)
	}
}
