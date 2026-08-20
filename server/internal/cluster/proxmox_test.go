package cluster

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newProxmoxTestServer builds an httptest.Server driven by a ServeMux (Go
// 1.22+ method+path patterns), the same shape a real Proxmox VE API exposes
// under /api2/json.
func newProxmoxTestServer(t *testing.T, routes func(mux *http.ServeMux)) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	routes(mux)
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	return srv
}

func writeJSONFixture(t *testing.T, w http.ResponseWriter, body string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")

	if _, err := w.Write([]byte(body)); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
}

//nolint:gocyclo // one fixture, one snapshot, every field across nodes/vms/storages asserted together
func TestProxmox_Snapshot(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/cluster/resources", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[
				{"type":"node","node":"pve1","status":"online","maxcpu":32,"cpu":0.1,"maxmem":1000,"mem":500,"maxdisk":2000,"disk":1000},
				{"type":"qemu","node":"pve1","vmid":101,"name":"web-1","status":"running","pool":"alice","tags":"pvmss;prod","maxcpu":2,"maxmem":2147483648},
				{"type":"storage","node":"pve1","storage":"local-lvm","plugintype":"lvmthin","content":"images,rootdir","maxdisk":500,"disk":100}
			]}`)
		})
		mux.HandleFunc("GET /api2/json/version", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"version":"8.2.4"}}`)
		})
		mux.HandleFunc("GET /api2/json/nodes/pve1/qemu/101/config", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{
				"sockets":1,"cores":2,"scsi0":"local-lvm:vm-101-disk-0,size=32G",
				"net0":"virtio=AA:BB:CC:DD:EE:01,bridge=vmbr0","description":"demo box"
			}}`)
		})
		mux.HandleFunc("GET /api2/json/nodes/pve1/qemu/101/status/current", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"uptime":3600}}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	snap, err := p.Snapshot(context.Background())
	if err != nil {
		t.Fatalf("Snapshot: %v", err)
	}

	if snap.ProxmoxVersion != "8.2.4" {
		t.Errorf("version = %q, want 8.2.4", snap.ProxmoxVersion)
	}

	if len(snap.Nodes) != 1 || snap.Nodes[0].Name != "pve1" || snap.Nodes[0].Status != NodeOnline {
		t.Fatalf("nodes = %+v", snap.Nodes)
	}

	if len(snap.Storages) != 1 || !snap.Storages[0].SupportsVMState {
		t.Fatalf("storages = %+v, want lvmthin marked SupportsVMState", snap.Storages)
	}

	storage := snap.Storages[0]
	if storage.PluginType != "lvmthin" || storage.Content != "images,rootdir" {
		t.Errorf("storage capabilities = %+v, want lvmthin with images,rootdir", storage)
	}

	if len(snap.VMs) != 1 {
		t.Fatalf("vms = %+v", snap.VMs)
	}

	vm := snap.VMs[0]
	if vm.VMID != 101 || vm.Status != VMRunning || vm.Pool != "alice" {
		t.Fatalf("vm = %+v", vm)
	}

	if len(vm.Tags) != 2 || vm.Tags[0] != "pvmss" {
		t.Fatalf("vm.Tags = %v", vm.Tags)
	}

	if vm.Sockets != 1 || vm.Cores != 2 {
		t.Fatalf("vm sockets/cores = %d/%d", vm.Sockets, vm.Cores)
	}

	if len(vm.Disks) != 1 || vm.Disks[0].Storage != "local-lvm" || vm.Disks[0].SizeGB != 32 {
		t.Fatalf("vm.Disks = %+v", vm.Disks)
	}

	if len(vm.NetworkInterfaces) != 1 || vm.NetworkInterfaces[0].Bridge != "vmbr0" {
		t.Fatalf("vm.NetworkInterfaces = %+v", vm.NetworkInterfaces)
	}

	if vm.Description != "demo box" {
		t.Errorf("vm.Description = %q", vm.Description)
	}

	if vm.Uptime.Seconds() != 3600 {
		t.Errorf("vm.Uptime = %v, want 1h", vm.Uptime)
	}
}

// proxmoxAuthFixture wires the three calls a login exercises: the ticket
// exchange, the caller's own permission check at "/", and (for non-admins)
// the pool listing used to derive their personal pool.
func proxmoxAuthFixture(t *testing.T, isAdmin bool, username, password string) *httptest.Server {
	t.Helper()

	return newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api2/json/access/ticket", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			if r.FormValue("username") != username || r.FormValue("password") != password {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			writeJSONFixture(t, w, `{"data":{"ticket":"tix-123","CSRFPreventionToken":"csrf-123"}}`)
		})
		mux.HandleFunc("GET /api2/json/access/permissions", func(w http.ResponseWriter, r *http.Request) {
			if cookie, err := r.Cookie("PVEAuthCookie"); err != nil || cookie.Value != "tix-123" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			if isAdmin {
				writeJSONFixture(t, w, `{"data":{"Permissions.Modify":1,"Sys.Audit":1}}`)
			} else {
				writeJSONFixture(t, w, `{"data":{"Sys.Audit":1}}`)
			}
		})
		mux.HandleFunc("GET /api2/json/pools", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[{"poolid":"alice","comment":"Alice's pool"}]}`)
		})
	})
}

func TestProxmox_Authenticate_Admin(t *testing.T) {
	t.Parallel()

	srv := proxmoxAuthFixture(t, true, "admin1@pve", "s3cret")
	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	identity, err := p.Authenticate(context.Background(), "admin1@pve", "s3cret")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	if !identity.IsAdmin || identity.Pool != "" {
		t.Fatalf("identity = %+v, want admin with no pool", identity)
	}
}

func TestProxmox_Authenticate_NonAdminOwnsPool(t *testing.T) {
	t.Parallel()

	srv := proxmoxAuthFixture(t, false, "alice@pve", "pvmss-alice")
	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	identity, err := p.Authenticate(context.Background(), "alice@pve", "pvmss-alice")
	if err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	if identity.IsAdmin || identity.Pool != "alice" {
		t.Fatalf("identity = %+v, want non-admin pool=alice", identity)
	}
}

func TestProxmox_Authenticate_WrongPassword(t *testing.T) {
	t.Parallel()

	srv := proxmoxAuthFixture(t, false, "alice@pve", "pvmss-alice")
	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	_, err := p.Authenticate(context.Background(), "alice@pve", "wrong-password")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestProxmox_ChangePassword(t *testing.T) {
	t.Parallel()

	var gotNewPassword string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api2/json/access/ticket", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			if r.FormValue("username") != "alice@pve" || r.FormValue("password") != "old-pass" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}

			writeJSONFixture(t, w, `{"data":{"ticket":"tix-123","CSRFPreventionToken":"csrf-123"}}`)
		})
		mux.HandleFunc("PUT /api2/json/access/password", func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("CSRFPreventionToken") != "csrf-123" {
				w.WriteHeader(http.StatusForbidden)
				return
			}

			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotNewPassword = r.FormValue("password")

			writeJSONFixture(t, w, `{"data":null}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	if err := p.ChangePassword(context.Background(), "alice@pve", "old-pass", "new-pass"); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	if gotNewPassword != "new-pass" {
		t.Errorf("new password sent = %q, want %q", gotNewPassword, "new-pass")
	}
}

func TestProxmox_ChangePassword_WrongOldPassword(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api2/json/access/ticket", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	err := p.ChangePassword(context.Background(), "alice@pve", "wrong-old", "new-pass")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("err = %v, want ErrNotFound", err)
	}
}

func TestProxmox_ListBridges(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/nodes", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[{"node":"pve1"}]}`)
		})
		mux.HandleFunc("GET /api2/json/nodes/pve1/network", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[
				{"iface":"vmbr0","type":"bridge","active":1,"comments":""},
				{"iface":"eth0","type":"eth","active":1},
				{"iface":"vmbr1","type":"bridge","active":0,"comments":"guest VLAN"}
			]}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	bridges, err := p.ListBridges(context.Background())
	if err != nil {
		t.Fatalf("ListBridges: %v", err)
	}

	if len(bridges) != 2 {
		t.Fatalf("bridges = %+v, want 2 (eth0 excluded)", bridges)
	}

	if bridges[0].Name != "vmbr0" || !bridges[0].Active {
		t.Errorf("bridges[0] = %+v", bridges[0])
	}

	if bridges[1].Name != "vmbr1" || bridges[1].Active || bridges[1].Comment != "guest VLAN" {
		t.Errorf("bridges[1] = %+v", bridges[1])
	}
}

func TestProxmox_ListISOs(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/cluster/resources", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[{"type":"storage","node":"pve1","storage":"local"}]}`)
		})
		mux.HandleFunc("GET /api2/json/nodes/pve1/storage/local/content", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("content") != "iso" {
				t.Errorf("content query = %q, want iso", r.URL.Query().Get("content"))
			}

			writeJSONFixture(t, w, `{"data":[{"volid":"local:iso/debian-12.iso","size":691945472}]}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	isos, err := p.ListISOs(context.Background())
	if err != nil {
		t.Fatalf("ListISOs: %v", err)
	}

	if len(isos) != 1 || isos[0].File != "debian-12.iso" || isos[0].Storage != "local" || isos[0].SizeBytes != 691945472 {
		t.Fatalf("isos = %+v", isos)
	}
}
