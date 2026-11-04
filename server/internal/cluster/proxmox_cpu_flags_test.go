package cluster

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestProxmox_CreateVM_CPUType(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		cpuType string
		want    string
	}{
		{"unset falls back to v2-AES", "", CPUTypeV2AES},
		{"explicit v3 is honoured", CPUTypeV3, CPUTypeV3},
		{"explicit host is honoured", CPUTypeHost, CPUTypeHost},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			form := captureCPUTypeForm(t, tt.cpuType)
			if got := form.Get("cpu"); got != tt.want {
				t.Errorf("cpu = %q, want %q", got, tt.want)
			}

			if form.Get("cpu") == "kvm64" {
				t.Error("cpu = kvm64, which PVMSS must never create on")
			}
		})
	}
}

func captureCPUTypeForm(t *testing.T, cpuType string) url.Values {
	t.Helper()

	var gotForm url.Values

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api2/json/nodes/node01/qemu", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotForm = r.Form

			writeJSONFixture(t, w, `{"data":"UPID:node01:...:qmcreate:120:pvmss@pve:"}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	spec := VMSpec{
		VMID: 120, Node: testNodeName, Name: "cpu-test", Pool: FakePoolAliceShort,
		Sockets: 1, CPUCores: 2, MemoryMB: 2048,
		Disk:    DiskSpec{Storage: FakeStorageLocalLVM, SizeGB: 16, Bus: string(DiskBusSCSI)},
		Network: NetworkSpec{{Bridge: FakeBridgeVMbr0, Model: string(DiskBusVirtio)}},
		CPUType: cpuType,
	}

	if _, err := p.CreateVM(context.Background(), spec); err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	return gotForm
}

func TestProxmox_HostCPUFlags(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[{"node":"pve1","status":"online"},{"node":"pve2","status":"offline"},{"node":"pve3","status":"online"}]}`)
		})
		mux.HandleFunc("GET /api2/json/nodes/pve1/status", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"cpuinfo":{"flags":"cx16 aes avx2"}}}`)
		})
		mux.HandleFunc("GET /api2/json/nodes/pve3/status", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"cpuinfo":{"flags":"cx16 sse4_2"}}}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	got, err := p.HostCPUFlags(context.Background())
	if err != nil {
		t.Fatalf("HostCPUFlags: %v", err)
	}

	if len(got) != 2 {
		t.Fatalf("got %d node lists, want 2 (offline pve2 skipped)", len(got))
	}

	if strings.Join(got[0], " ") != "cx16 aes avx2" || strings.Join(got[1], " ") != "cx16 sse4_2" {
		t.Errorf("flags = %v, want the flags of pve1 then pve3", got)
	}
}

func TestProxmox_HostCPUFlags_NodeErrorFails(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[{"node":"pve1","status":"online"}]}`)
		})
		mux.HandleFunc("GET /api2/json/nodes/pve1/status", func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "forbidden", http.StatusForbidden)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if _, err := p.HostCPUFlags(context.Background()); err == nil {
		t.Fatal("HostCPUFlags: want error when a node status read fails")
	}
}

func TestProxmox_HostCPUFlags_MissingFlagsFails(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[{"node":"pve1","status":"online"}]}`)
		})
		mux.HandleFunc("GET /api2/json/nodes/pve1/status", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"cpuinfo":{"model":"x"}}}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if _, err := p.HostCPUFlags(context.Background()); !errors.Is(err, errNoCPUFlags) {
		t.Fatalf("HostCPUFlags err = %v, want errNoCPUFlags", err)
	}
}
