package cluster

import (
	"context"
	"net/http"
	"sync"
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

// readCounter counts requests per path; safe for concurrent handlers.
type readCounter struct {
	mu sync.Mutex
	n  map[string]int
}

func (c *readCounter) add(path string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.n == nil {
		c.n = map[string]int{}
	}

	c.n[path]++
}

func (c *readCounter) of(path string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.n["/api2/json"+path]
}

// incrementalServer serves the rows in *rows and counts config and /version
// reads. Configs answer cores=4 so a reused hydration is visible.
func incrementalServer(t *testing.T, rows *atomic.Pointer[string], reads *readCounter) *Proxmox {
	t.Helper()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/cluster/resources", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[`+*rows.Load()+`]}`)
		})
		mux.HandleFunc("GET /api2/json/version", func(w http.ResponseWriter, r *http.Request) {
			reads.add(r.URL.Path)
			writeJSONFixture(t, w, `{"data":{"version":"8.2.4"}}`)
		})
		mux.HandleFunc("GET /api2/json/nodes/{node}/qemu/{vmid}/config", func(w http.ResponseWriter, r *http.Request) {
			reads.add(r.URL.Path)
			writeJSONFixture(t, w, `{"data":{"sockets":1,"cores":4}}`)
		})
	})

	return &Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}
}

func TestProxmox_SnapshotFrom_RereadsOnlyChangedVMs(t *testing.T) {
	t.Parallel()

	const (
		stable  = `{"type":"qemu","node":"pve1","vmid":100,"name":"stable","status":"running","uptime":10,"maxcpu":4,"maxmem":1073741824}`
		renamed = `{"type":"qemu","node":"pve1","vmid":101,"name":"old","status":"stopped","maxcpu":4,"maxmem":1073741824}`
		moving  = `{"type":"qemu","node":"pve1","vmid":102,"name":"mover","status":"stopped","maxcpu":4,"maxmem":1073741824}`
	)

	var (
		rows  atomic.Pointer[string]
		reads readCounter
	)

	first := stable + "," + renamed + "," + moving
	rows.Store(&first)

	p := incrementalServer(t, &rows, &reads)
	ctx := context.Background()

	prev, err := p.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	second := `{"type":"qemu","node":"pve1","vmid":100,"name":"stable","status":"running","uptime":40,"maxcpu":4,"maxmem":1073741824},` +
		`{"type":"qemu","node":"pve1","vmid":101,"name":"new-name","status":"stopped","maxcpu":4,"maxmem":1073741824},` +
		`{"type":"qemu","node":"pve2","vmid":102,"name":"mover","status":"stopped","maxcpu":4,"maxmem":1073741824},` +
		`{"type":"qemu","node":"pve1","vmid":103,"name":"fresh","status":"stopped","maxcpu":4,"maxmem":1073741824}`
	rows.Store(&second)

	next, err := p.SnapshotFrom(ctx, prev)
	if err != nil {
		t.Fatalf("SnapshotFrom: %v", err)
	}

	for path, want := range map[string]int{
		"/nodes/pve1/qemu/100/config": 1, // unchanged: only the first full read
		"/nodes/pve1/qemu/101/config": 2, // renamed: re-read
		"/nodes/pve2/qemu/102/config": 1, // migrated: read on its new node
		"/nodes/pve1/qemu/103/config": 1, // new VM
	} {
		if got := reads.of(path); got != want {
			t.Errorf("%s reads = %d, want %d", path, got, want)
		}
	}

	if got := reads.of("/version"); got != 1 {
		t.Errorf("/version reads = %d, want 1 (reused from prev)", got)
	}

	if len(next.VMs) != 4 || next.ProxmoxVersion != "8.2.4" {
		t.Fatalf("next = %d VMs, version %q", len(next.VMs), next.ProxmoxVersion)
	}

	reused := next.VMs[0]
	if reused.VMID != 100 || reused.Cores != 4 || reused.Uptime != 40*time.Second {
		t.Errorf("reused VM = id %d cores %d uptime %v, want 100/4/40s (config kept, uptime from row)", reused.VMID, reused.Cores, reused.Uptime)
	}
}
