package cluster

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

// TestProxmox_GuestNetworkInterfaces verifies the read hits the
// network-get-interfaces endpoint and decodes the agent's nested
// data.result payload into GuestInterface rows (MAC + bound IPs).
func TestProxmox_GuestNetworkInterfaces(t *testing.T) {
	t.Parallel()

	var gotPath string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/agent/network-get-interfaces", func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path

			writeJSONFixture(t, w, `{"data":{"result":[
				{"name":"lo","hardware-address":"00:00:00:00:00:00","ip-addresses":[{"ip-address":"127.0.0.1","ip-address-type":"ipv4","prefix":8}]},
				{"name":"eth0","hardware-address":"bc:24:11:00:00:64","ip-addresses":[
					{"ip-address":"10.10.0.10","ip-address-type":"ipv4","prefix":24},
					{"ip-address":"fe80::1","ip-address-type":"ipv6","prefix":64}
				]},
				{"name":"eth1","hardware-address":"bc:24:11:00:00:65"}
			]}}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	guests, err := p.GuestNetworkInterfaces(context.Background(), testNodeName, testVMID)
	if err != nil {
		t.Fatalf("GuestNetworkInterfaces: %v", err)
	}

	if gotPath != "/api2/json/nodes/node01/qemu/101/agent/network-get-interfaces" {
		t.Errorf("path = %q, want agent/network-get-interfaces", gotPath)
	}

	if len(guests) != 3 {
		t.Fatalf("guests = %+v, want 3 rows", guests)
	}

	if guests[1].MAC != "bc:24:11:00:00:64" {
		t.Errorf("guests[1].MAC = %q", guests[1].MAC)
	}

	if len(guests[1].IPAddresses) != 2 || guests[1].IPAddresses[0] != "10.10.0.10" || guests[1].IPAddresses[1] != "fe80::1" {
		t.Errorf("guests[1].IPAddresses = %v", guests[1].IPAddresses)
	}

	if len(guests[2].IPAddresses) != 0 {
		t.Errorf("guests[2].IPAddresses = %v, want none (interface without addresses)", guests[2].IPAddresses)
	}
}

// TestProxmox_GuestNetworkInterfaces_AgentDown verifies an agent-side error
// (VM stopped, agent absent) propagates — the detail endpoint treats it as
// "no live addresses".
func TestProxmox_GuestNetworkInterfaces_AgentDown(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes/node01/qemu/101/agent/network-get-interfaces", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"data":null,"errors":"QEMU guest agent is not running"}`))
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if _, err := p.GuestNetworkInterfaces(context.Background(), testNodeName, testVMID); err == nil {
		t.Fatal("err = nil, want an agent failure")
	} else if errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want the agent rejection not ErrNotFound", err)
	}
}
