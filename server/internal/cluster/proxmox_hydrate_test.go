package cluster

import (
	"context"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

// Uptime comes from the /cluster/resources row: a running VM costs one config
// read per snapshot and no status/current read.
func TestProxmox_Snapshot_UptimeFromResources(t *testing.T) {
	t.Parallel()

	var statusReads atomic.Int32

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/cluster/resources", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[{"type":"qemu","node":"pve1","vmid":101,"name":"web","status":"running","uptime":3600,"maxcpu":1,"maxmem":1073741824}]}`)
		})
		mux.HandleFunc("GET /api2/json/version", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"version":"8.2.4"}}`)
		})
		mux.HandleFunc("GET /api2/json/nodes/pve1/qemu/101/config", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"name":"web","sockets":1,"cores":1}}`)
		})
		mux.HandleFunc("GET /api2/json/nodes/pve1/qemu/101/status/current", func(w http.ResponseWriter, _ *http.Request) {
			statusReads.Add(1)
			writeJSONFixture(t, w, `{"data":{"uptime":1}}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	snap, err := p.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if got := snap.VMs[0].Uptime; got != time.Hour {
		t.Errorf("uptime = %v, want 1h (from resources row)", got)
	}

	if n := statusReads.Load(); n != 0 {
		t.Errorf("status/current reads = %d, want 0", n)
	}
}
