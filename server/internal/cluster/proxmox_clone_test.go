package cluster

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

const (
	cloneTestName      = "test-vm"
	cloneTestUPID      = `{"data":"UPID:pve-node-02:00000001:00000002:00000003:qmclone:9001:pvmss@pve:"}`
	cloneTestNewVMID   = "9001"
	cloneTestVMID9000  = 9000
	cloneTestVMID9001  = 9001
	cloneTestNewID     = 9001
	cloneTestPoolCarol = "pool-carol"
	cloneTestPoolBob   = "pool-bob"
)

// TestProxmox_CloneVM_LinkedClone verifies the clone form sends full=0 and
// no storage when a linked clone is requested (cloud-init not required,
// same storage).
func TestProxmox_CloneVM_LinkedClone(t *testing.T) {
	t.Parallel()

	var capturedForm url.Values

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api2/json/nodes/pve-node-02/qemu/9000/clone", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}

			capturedForm = r.PostForm

			writeJSONFixture(t, w, cloneTestUPID)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	upid, err := p.CloneVM(context.Background(), CloneSpec{
		SourceVMID: cloneTestVMID9000,
		SourceNode: FakeNode02,
		NewVMID:    cloneTestNewID,
		Name:       cloneTestName,
		Full:       false,
		Pool:       FakePoolAlice,
		DiskBus:    string(DiskBusSCSI),
	})
	if err != nil {
		t.Fatalf("CloneVM: %v", err)
	}

	if upid == "" {
		t.Fatal("expected non-empty UPID")
	}

	if capturedForm.Get("full") != "0" {
		t.Errorf("full = %q, want 0", capturedForm.Get("full"))
	}

	if capturedForm.Has("storage") {
		t.Errorf("linked clone should not send storage, got %q", capturedForm.Get("storage"))
	}

	if capturedForm.Get("pool") != FakePoolAlice {
		t.Errorf("pool = %q, want %q", capturedForm.Get("pool"), FakePoolAlice)
	}

	if capturedForm.Get("newid") != cloneTestNewVMID {
		t.Errorf("newid = %q, want %q", capturedForm.Get("newid"), cloneTestNewVMID)
	}

	if capturedForm.Get("name") != cloneTestName {
		t.Errorf("name = %q, want %q", capturedForm.Get("name"), cloneTestName)
	}
}

// TestProxmox_CloneVM_FullClone verifies the clone form sends full=1 and
// the target storage when a full clone is requested (cloud-init capable
// template, different storage).
func TestProxmox_CloneVM_FullClone(t *testing.T) {
	t.Parallel()

	var capturedForm url.Values

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api2/json/nodes/pve-node-02/qemu/9000/clone", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}

			capturedForm = r.PostForm

			writeJSONFixture(t, w, cloneTestUPID)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	_, err := p.CloneVM(context.Background(), CloneSpec{
		SourceVMID: cloneTestVMID9000,
		SourceNode: FakeNode02,
		NewVMID:    cloneTestNewID,
		Name:       cloneTestName,
		Full:       true,
		Storage:    FakeStorageLocal,
		Pool:       FakePoolAlice,
		DiskBus:    string(DiskBusSCSI),
	})
	if err != nil {
		t.Fatalf("CloneVM: %v", err)
	}

	if capturedForm.Get("full") != "1" {
		t.Errorf("full = %q, want 1", capturedForm.Get("full"))
	}

	if capturedForm.Get("storage") != FakeStorageLocal {
		t.Errorf("storage = %q, want %q", capturedForm.Get("storage"), FakeStorageLocal)
	}

	if capturedForm.Get("pool") != FakePoolAlice {
		t.Errorf("pool = %q, want %q", capturedForm.Get("pool"), FakePoolAlice)
	}
}

// TestProxmox_CloneVM_FullCloneSameStorage verifies that a full clone
// without a different storage does not send the storage field.
func TestProxmox_CloneVM_FullCloneSameStorage(t *testing.T) {
	t.Parallel()

	var capturedForm url.Values

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api2/json/nodes/pve-node-02/qemu/9000/clone", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("ParseForm: %v", err)
			}

			capturedForm = r.PostForm

			writeJSONFixture(t, w, cloneTestUPID)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	_, err := p.CloneVM(context.Background(), CloneSpec{
		SourceVMID: cloneTestVMID9000,
		SourceNode: FakeNode02,
		NewVMID:    cloneTestNewID,
		Name:       cloneTestName,
		Full:       true,
		Pool:       FakePoolAlice,
		DiskBus:    string(DiskBusVirtio),
	})
	if err != nil {
		t.Fatalf("CloneVM: %v", err)
	}

	if capturedForm.Get("full") != "1" {
		t.Errorf("full = %q, want 1", capturedForm.Get("full"))
	}

	if capturedForm.Has("storage") {
		t.Errorf("full clone with no storage should not send storage, got %q", capturedForm.Get("storage"))
	}
}

// TestProxmox_ListTemplates verifies template discovery via
// /cluster/resources?type=vm, filtering to template=1 rows and reading
// disk config from /nodes/{node}/qemu/{vmid}/config. Disk config uses the
// real Proxmox form "storage:volid,size=NG" (not "storage:size,format=…"),
// and CloudInitCapable is detected by a cloud-init drive in the fixed ide3
// slot (cloudInitDiskKey).
func TestProxmox_ListTemplates(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/cluster/resources", func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Query().Get("type") != "vm" {
				t.Errorf("type query = %q, want vm", r.URL.Query().Get("type"))
			}

			writeJSONFixture(t, w, `{"data":[
				{"type":"qemu","node":"pve-node-02","vmid":9000,"name":"debian-12-cloud","template":1},
				{"type":"qemu","node":"pve-node-02","vmid":9001,"name":"alpine-appliance","template":1},
				{"type":"qemu","node":"pve-node-01","vmid":100,"name":"regular-vm","template":0}
			]}`)
		})

		// debian-12-cloud: cloud-init capable (ide3 holds a cloudinit drive),
		// scsi0 is the primary disk.
		mux.HandleFunc("GET /api2/json/nodes/pve-node-02/qemu/9000/config", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"scsi0":"local-lvm:vm-9000-disk-0,size=8G","ide3":"local-lvm:cloudinit"}}`)
		})

		// alpine-appliance: no cloud-init drive, virtio0 is the primary disk.
		mux.HandleFunc("GET /api2/json/nodes/pve-node-02/qemu/9001/config", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"virtio0":"local:vm-9001-disk-0,size=2G"}}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	templates, err := p.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}

	if len(templates) != 2 {
		t.Fatalf("expected 2 templates, got %d: %+v", len(templates), templates)
	}

	assertTemplateEquals(t, templates[0], cloneTestVMID9000, "pve-node-02", "debian-12-cloud", "local-lvm", 8, "scsi", true)
	assertTemplateEquals(t, templates[1], cloneTestVMID9001, "pve-node-02", "alpine-appliance", FakeStorageLocal, 2, "virtio", false)
}

// assertTemplateEquals checks the key fields of a discovered template.
func assertTemplateEquals(t *testing.T, tmpl TemplateVM, vmid int, node, name, storage string, sizeGB int, bus string, cloudInitCapable bool) {
	t.Helper()

	if tmpl.VMID != vmid || tmpl.Node != node || tmpl.Name != name {
		t.Errorf("template = %+v, want vmid=%d node=%s name=%s", tmpl, vmid, node, name)
	}

	if tmpl.DiskStorage != storage || tmpl.DiskSizeGB != sizeGB || tmpl.DiskBus != bus {
		t.Errorf("template disk = %+v, want storage=%s size=%d bus=%s", tmpl, storage, sizeGB, bus)
	}

	if tmpl.CloudInitCapable != cloudInitCapable {
		t.Errorf("template %d cloudInitCapable = %v, want %v", tmpl.VMID, tmpl.CloudInitCapable, cloudInitCapable)
	}
}

// TestProxmox_ListTemplates_NoTemplates verifies that a cluster with no
// templates returns an empty slice, not an error.
func TestProxmox_ListTemplates_NoTemplates(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/cluster/resources", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[{"type":"qemu","node":"pve1","vmid":100,"name":"vm","template":0}]}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	templates, err := p.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}

	if len(templates) != 0 {
		t.Fatalf("expected 0 templates, got %d", len(templates))
	}
}

// TestProxmox_ListTemplates_DiskOptionsBeforeSize verifies the disk parser
// handles the real Proxmox form where options may precede size= (e.g.
// "local-lvm:vm-100-disk-0,cache=writeback,size=64G"). The original
// parseProxmoxDiskValue misparsed this format; the fixed code reuses
// parseDiskValue which handles it correctly.
func TestProxmox_ListTemplates_DiskOptionsBeforeSize(t *testing.T) {
	t.Parallel()

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("GET /api2/json/cluster/resources", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":[
				{"type":"qemu","node":"pve-node-01","vmid":9100,"name":"big-template","template":1}
			]}`)
		})

		mux.HandleFunc("GET /api2/json/nodes/pve-node-01/qemu/9100/config", func(w http.ResponseWriter, _ *http.Request) {
			writeJSONFixture(t, w, `{"data":{"scsi0":"local-lvm:vm-9100-disk-0,cache=writeback,size=64G"}}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	templates, err := p.ListTemplates(context.Background())
	if err != nil {
		t.Fatalf("ListTemplates: %v", err)
	}

	if len(templates) != 1 {
		t.Fatalf("expected 1 template, got %d", len(templates))
	}

	assertTemplateEquals(t, templates[0], 9100, "pve-node-01", "big-template", "local-lvm", 64, "scsi", false)
}

// TestProxmox_CloneVM_URLEncoding verifies that the source node is properly
// URL-escaped in the clone path.
func TestProxmox_CloneVM_URLEncoding(t *testing.T) {
	t.Parallel()

	var requestPath string

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api2/json/nodes/pve-node-02/qemu/9000/clone", func(w http.ResponseWriter, r *http.Request) {
			requestPath = r.URL.Path

			writeJSONFixture(t, w, `{"data":"UPID:test"}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	_, err := p.CloneVM(context.Background(), CloneSpec{
		SourceVMID: cloneTestVMID9000,
		SourceNode: FakeNode02,
		NewVMID:    cloneTestNewID,
		Name:       cloneTestName,
		Full:       false,
		Pool:       FakePoolAlice,
	})
	if err != nil {
		t.Fatalf("CloneVM: %v", err)
	}

	if !strings.Contains(requestPath, "/nodes/pve-node-02/qemu/9000/clone") {
		t.Errorf("request path = %q, expected clone path", requestPath)
	}
}
