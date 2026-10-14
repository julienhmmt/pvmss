package cluster

import (
	"context"
	"net/http"
	"testing"
)

// TestProxmox_Snapshot_ConfigWinsOverLaggingResources reproduces the live
// lag: /cluster/resources still reports the pre-write name and hardware
// while /config already holds the values just written. The snapshot must
// carry the config values, otherwise every post-write refresh rebuilds the
// projection with stale data.
//
//nolint:dupl // same fixture shape as TestProxmox_Snapshot by design
func TestProxmox_Snapshot_ConfigWinsOverLaggingResources(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/cluster/resources", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[
				{"type":"qemu","node":"pve1","vmid":101,"name":"t2","status":"running","pool":"alice","tags":"old","maxcpu":1,"maxmem":1073741824}
			]}`)
		})
		mux.HandleFunc("GET /api2/json/version", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"version":"8.2.4"}}`)
		})
		mux.HandleFunc("GET /api2/json/nodes/pve1/qemu/101/config", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"name":"t3","sockets":1,"cores":2,"memory":1536,"tags":"pvmss;lagcheck"}}`)
		})
		mux.HandleFunc("GET /api2/json/nodes/pve1/qemu/101/status/current", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"uptime":60}}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	snap, err := p.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if len(snap.VMs) != 1 {
		t.Fatalf("VMs = %d, want 1", len(snap.VMs))
	}

	vm := snap.VMs[0]

	if vm.Name != "t3" {
		t.Errorf("name = %q, want t3 (config value)", vm.Name)
	}

	if vm.CPUCores != 2 {
		t.Errorf("cpuCores = %d, want 2 (sockets*cores from config)", vm.CPUCores)
	}

	if want := int64(1536) * 1024 * 1024; vm.MemoryTotal != want {
		t.Errorf("memoryTotal = %d, want %d", vm.MemoryTotal, want)
	}

	if len(vm.Tags) != 2 || vm.Tags[0] != "pvmss" || vm.Tags[1] != "lagcheck" {
		t.Errorf("tags = %v, want [pvmss lagcheck] (config value)", vm.Tags)
	}
}
