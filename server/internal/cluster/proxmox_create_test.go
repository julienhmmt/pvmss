package cluster

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

func TestProxmox_NextVMID(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		body string
		want int
	}{
		{"string payload", `{"data":"105"}`, 105},
		{"numeric payload", `{"data":105}`, 105},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
				mux.HandleFunc("GET /api2/json/cluster/nextid", func(w http.ResponseWriter, _ *http.Request) {
					writeJSONFixture(t, w, tc.body)
				})
			})

			p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

			got, err := p.NextVMID(context.Background())
			if err != nil {
				t.Fatalf("NextVMID: %v", err)
			}

			if got != tc.want {
				t.Errorf("NextVMID = %d, want %d", got, tc.want)
			}
		})
	}
}

//nolint:gocyclo // one fixture, one VM, every form field asserted together — splitting would hurt readability, not help it
func TestProxmox_CreateVM(t *testing.T) {
	t.Parallel()

	var gotForm url.Values

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api2/json/nodes/node01/qemu", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotForm = r.Form

			writeJSONFixture(t, w, `{"data":"UPID:node01:...:qmcreate:105:pvmss@pve:"}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	spec := VMSpec{
		VMID: 105, Node: "node01", Name: "web-1", Pool: "alice", Tags: []string{"pvmss"},
		CPUCores: 4, MemoryMB: 4096,
		Disk:             DiskSpec{Storage: "local-lvm", SizeGB: 32, Bus: "scsi"},
		Network:          NetworkSpec{Bridge: "vmbr0", Model: "virtio"},
		ISO:              &ISOSpec{Storage: "local", File: "debian-12.iso"},
		StartAfterCreate: true,
	}

	upid, err := p.CreateVM(context.Background(), spec)
	if err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	if upid == "" {
		t.Error("expected a non-empty UPID")
	}

	if gotForm.Get("vmid") != "105" || gotForm.Get("name") != "web-1" {
		t.Errorf("vmid/name = %q/%q", gotForm.Get("vmid"), gotForm.Get("name"))
	}

	if gotForm.Get("sockets") != "1" || gotForm.Get("cores") != "4" || gotForm.Get("memory") != "4096" {
		t.Errorf("sockets/cores/memory = %q/%q/%q", gotForm.Get("sockets"), gotForm.Get("cores"), gotForm.Get("memory"))
	}

	if gotForm.Get("pool") != "alice" || gotForm.Get("tags") != "pvmss" {
		t.Errorf("pool/tags = %q/%q", gotForm.Get("pool"), gotForm.Get("tags"))
	}

	if gotForm.Get("scsi0") != "local-lvm:32" || gotForm.Get("scsihw") != "virtio-scsi-pci" {
		t.Errorf("scsi0/scsihw = %q/%q", gotForm.Get("scsi0"), gotForm.Get("scsihw"))
	}

	if gotForm.Get("net0") != "virtio,bridge=vmbr0" {
		t.Errorf("net0 = %q", gotForm.Get("net0"))
	}

	if gotForm.Get("ide2") != "local:iso/debian-12.iso,media=cdrom" {
		t.Errorf("ide2 = %q", gotForm.Get("ide2"))
	}

	if gotForm.Get("start") != "1" {
		t.Errorf("start = %q, want 1", gotForm.Get("start"))
	}
}

func TestProxmox_TaskStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		status     string
		exitStatus string
		want       TaskState
	}{
		{"running", "running", "", TaskRunning},
		{"ok", "stopped", "OK", TaskOK},
		{"error", "stopped", "job errored: disk full", TaskError},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			upid := "UPID:node01:00000001:00000002:00000003:qmcreate:105:pvmss@pve:"

			srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
				mux.HandleFunc("GET /api2/json/nodes/node01/tasks/"+upid+"/status", func(w http.ResponseWriter, _ *http.Request) {
					writeJSONFixture(t, w, `{"data":{"status":"`+tc.status+`","exitstatus":"`+tc.exitStatus+`"}}`)
				})
				mux.HandleFunc("GET /api2/json/nodes/node01/tasks/"+upid+"/log", func(w http.ResponseWriter, _ *http.Request) {
					writeJSONFixture(t, w, `{"data":[{"t":"starting task..."}]}`)
				})
			})

			p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

			got, err := p.TaskStatus(context.Background(), upid)
			if err != nil {
				t.Fatalf("TaskStatus: %v", err)
			}

			if got.State != tc.want {
				t.Errorf("state = %q, want %q", got.State, tc.want)
			}

			if tc.want == TaskError && got.ExitMessage != tc.exitStatus {
				t.Errorf("exitMessage = %q, want %q", got.ExitMessage, tc.exitStatus)
			}

			if len(got.Log) != 1 {
				t.Errorf("log = %v, want one line", got.Log)
			}
		})
	}
}

func TestProxmoxUPIDNode(t *testing.T) {
	t.Parallel()

	node, err := proxmoxUPIDNode("UPID:pve1:00000001:00000002:00000003:qmcreate:105:pvmss@pve:")
	if err != nil {
		t.Fatalf("proxmoxUPIDNode: %v", err)
	}

	if node != "pve1" {
		t.Errorf("node = %q, want pve1", node)
	}

	if _, err := proxmoxUPIDNode("not-a-upid"); err == nil {
		t.Error("expected an error for a malformed UPID")
	}
}
