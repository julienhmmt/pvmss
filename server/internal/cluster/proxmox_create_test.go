package cluster

import (
	"context"
	"net/http"
	"net/url"
	"testing"
)

// US6/issue-06: UEFI/TPM test fixtures — repeated Proxmox form values
// centralized for goconst and readability.
const (
	testBIOSOVMF     = "ovmf"
	testMachineQ35   = "q35"
	testEFIDiskValue = "local-lvm:1,efitype=4m,pre-enrolled-keys=1"
	testTPMDiskValue = "local-lvm:1,version=v2.0"
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
		t.Run(tc.name, func(t *testing.T) { //nolint:dupl // structural similarity to runDisplayNameCase is incidental (shared test-server pattern)
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
		VMID: 105, Node: testNodeName, Name: "web-1", Pool: FakePoolAliceShort, Tags: []string{FakeTagPvmss},
		Sockets: 1, CPUCores: 4, MemoryMB: 4096,
		Disk:             DiskSpec{Storage: FakeStorageLocalLVM, SizeGB: 32, Bus: string(DiskBusSCSI)},
		Network:          NetworkSpec{{Bridge: FakeBridgeVMbr0, Model: string(DiskBusVirtio)}},
		ISO:              &ISOSpec{Storage: FakeStorageLocal, File: "debian-12.iso"},
		StartAfterCreate: true,
	}

	upid, err := p.CreateVM(context.Background(), spec)
	if err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	if upid == "" {
		t.Error("expected a non-empty UPID")
	}

	assertCreateVMForm(t, gotForm)
}

// assertCreateVMForm asserts every form field captured by the
// TestProxmox_CreateVM fixture. Extracted from TestProxmox_CreateVM to satisfy
// the cognitive-complexity ceiling (go:S3776); assertion logic is unchanged.
func assertCreateVMForm(t *testing.T, form url.Values) {
	t.Helper()

	if form.Get("vmid") != "105" || form.Get("name") != "web-1" {
		t.Errorf("vmid/name = %q/%q", form.Get("vmid"), form.Get("name"))
	}

	assertCPUForm(t, form)

	if form.Get("pool") != FakePoolAliceShort || form.Get("tags") != FakeTagPvmss {
		t.Errorf("pool/tags = %q/%q", form.Get("pool"), form.Get("tags"))
	}

	if form.Get(diskKeySCSI0) != "local-lvm:32,discard=on,iothread=1" || form.Get("scsihw") != "virtio-scsi-pci" {
		t.Errorf("scsi0/scsihw = %q/%q", form.Get(diskKeySCSI0), form.Get("scsihw"))
	}

	if form.Get("net0") != "virtio,bridge=vmbr0,firewall=1" {
		t.Errorf("net0 = %q", form.Get("net0"))
	}

	if form.Get(cdromDiskKey) != cdromMountedValue {
		t.Errorf("ide2 = %q", form.Get(cdromDiskKey))
	}

	if form.Get("start") != "1" {
		t.Errorf("start = %q, want 1", form.Get("start"))
	}

	if form.Get("serial0") != "socket" {
		t.Errorf("serial0 = %q, want socket (serial console must be provisioned at create time)", form.Get("serial0"))
	}
}

// assertCPUForm checks the sockets/cores/memory form fields. Extracted from
// assertCreateVMForm to keep its cyclomatic complexity under the gocyclo limit.
func assertCPUForm(t *testing.T, form url.Values) {
	t.Helper()

	if form.Get("sockets") != "1" || form.Get("cores") != "4" || form.Get("memory") != "4096" {
		t.Errorf("sockets/cores/memory = %q/%q/%q", form.Get("sockets"), form.Get("cores"), form.Get("memory"))
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
		{"running", string(VMRunning), "", TaskRunning},
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

// TestProxmox_CreateVM_SocketsAndMultiNIC asserts that sockets=2 produces
// sockets=2 in the Proxmox form, and that two NICs produce net0 and net1
// (US2/D3a, D3b — T037/T038 form-level assertions).
func TestProxmox_CreateVM_SocketsAndMultiNIC(t *testing.T) {
	t.Parallel()

	var gotForm url.Values

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api2/json/nodes/node01/qemu", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotForm = r.Form

			writeJSONFixture(t, w, `{"data":"UPID:node01:...:qmcreate:106:pvmss@pve:"}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	spec := VMSpec{
		VMID: 106, Node: testNodeName, Name: "web-2", Pool: FakePoolAliceShort, Tags: []string{FakeTagPvmss},
		Sockets: 2, CPUCores: 4, MemoryMB: 8192,
		Disk: DiskSpec{Storage: FakeStorageLocalLVM, SizeGB: 32, Bus: string(DiskBusSCSI)},
		Network: NetworkSpec{
			{Bridge: FakeBridgeVMbr0, Model: string(DiskBusVirtio)},
			{Bridge: FakeBridgeVMbr1, Model: "e1000"},
		},
	}

	if _, err := p.CreateVM(context.Background(), spec); err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	if gotForm.Get("sockets") != "2" {
		t.Errorf("sockets = %q, want 2", gotForm.Get("sockets"))
	}

	if gotForm.Get("cores") != "4" {
		t.Errorf("cores = %q, want 4", gotForm.Get("cores"))
	}

	if gotForm.Get("net0") != "virtio,bridge=vmbr0,firewall=1" {
		t.Errorf("net0 = %q, want virtio,bridge=vmbr0,firewall=1", gotForm.Get("net0"))
	}

	if gotForm.Get("net1") != "e1000,bridge=vmbr1,firewall=1" {
		t.Errorf("net1 = %q, want e1000,bridge=vmbr1,firewall=1", gotForm.Get("net1"))
	}
}

// TestProxmox_CreateVM_DiskDefaults asserts the US6/issue-06 D6a disk
// defaults: discard=on always, iothread=1 only on SCSI bus.
func TestProxmox_CreateVM_DiskDefaults(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		bus        string
		wantDisk   string
		wantSCSIHW string
	}{
		{"scsi bus gets discard and iothread", string(DiskBusSCSI), "local-lvm:20,discard=on,iothread=1", "virtio-scsi-pci"},
		{"virtio bus gets discard only", string(DiskBusVirtio), "local-lvm:20,discard=on", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotForm url.Values

			srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
				mux.HandleFunc("POST /api2/json/nodes/node01/qemu", func(w http.ResponseWriter, r *http.Request) {
					if err := r.ParseForm(); err != nil {
						t.Fatalf("parse form: %v", err)
					}

					gotForm = r.Form

					writeJSONFixture(t, w, `{"data":"UPID:node01:...:qmcreate:107:pvmss@pve:"}`)
				})
			})

			p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

			spec := VMSpec{
				VMID: 107, Node: testNodeName, Name: "disk-test", Pool: FakePoolAliceShort,
				Sockets: 1, CPUCores: 2, MemoryMB: 2048,
				Disk:    DiskSpec{Storage: FakeStorageLocalLVM, SizeGB: 20, Bus: tc.bus},
				Network: NetworkSpec{{Bridge: FakeBridgeVMbr0, Model: string(DiskBusVirtio)}},
			}

			if _, err := p.CreateVM(context.Background(), spec); err != nil {
				t.Fatalf("CreateVM: %v", err)
			}

			diskKey := tc.bus + "0"
			if gotForm.Get(diskKey) != tc.wantDisk {
				t.Errorf("%s = %q, want %q", diskKey, gotForm.Get(diskKey), tc.wantDisk)
			}

			if tc.wantSCSIHW != "" {
				if gotForm.Get("scsihw") != tc.wantSCSIHW {
					t.Errorf("scsihw = %q, want %q", gotForm.Get("scsihw"), tc.wantSCSIHW)
				}
			}
		})
	}
}

// TestProxmox_CreateVM_UEFI asserts the US6/issue-06 UEFI/TPM emission:
// bios=ovmf forces machine=q35, provisions efidisk0, and tpmstate0 when TPM
// is set. EFI/TPM storage falls back to the disk's storage.
func TestProxmox_CreateVM_UEFI(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name        string
		bios        string
		machine     string
		tpm         bool
		wantMachine string
		wantEFI     string
		wantTPM     string
	}{
		{
			name:        "ovmf without machine forces q35",
			bios:        testBIOSOVMF,
			machine:     "",
			wantMachine: testMachineQ35,
			wantEFI:     testEFIDiskValue,
			wantTPM:     "",
		},
		{
			name:        "ovmf with i440fx forces q35",
			bios:        testBIOSOVMF,
			machine:     "i440fx",
			wantMachine: testMachineQ35,
			wantEFI:     testEFIDiskValue,
			wantTPM:     "",
		},
		{
			name:        "ovmf with q35 stays q35",
			bios:        testBIOSOVMF,
			machine:     testMachineQ35,
			wantMachine: testMachineQ35,
			wantEFI:     testEFIDiskValue,
			wantTPM:     "",
		},
		{
			name:        "ovmf with tpm provisions tpmstate0",
			bios:        testBIOSOVMF,
			machine:     "",
			tpm:         true,
			wantMachine: testMachineQ35,
			wantEFI:     testEFIDiskValue,
			wantTPM:     testTPMDiskValue,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotForm url.Values

			srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
				mux.HandleFunc("POST /api2/json/nodes/node01/qemu", func(w http.ResponseWriter, r *http.Request) {
					if err := r.ParseForm(); err != nil {
						t.Fatalf("parse form: %v", err)
					}

					gotForm = r.Form

					writeJSONFixture(t, w, `{"data":"UPID:node01:...:qmcreate:108:pvmss@pve:"}`)
				})
			})

			p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

			spec := VMSpec{
				VMID: 108, Node: testNodeName, Name: "uefi-test", Pool: FakePoolAliceShort,
				Sockets: 1, CPUCores: 2, MemoryMB: 4096,
				Disk:    DiskSpec{Storage: FakeStorageLocalLVM, SizeGB: 32, Bus: string(DiskBusSCSI)},
				Network: NetworkSpec{{Bridge: FakeBridgeVMbr0, Model: string(DiskBusVirtio)}},
				BIOS:    tc.bios, Machine: tc.machine, TPM: tc.tpm,
			}

			if _, err := p.CreateVM(context.Background(), spec); err != nil {
				t.Fatalf("CreateVM: %v", err)
			}

			if gotForm.Get("bios") != testBIOSOVMF {
				t.Errorf("bios = %q, want ovmf", gotForm.Get("bios"))
			}

			if gotForm.Get("machine") != tc.wantMachine {
				t.Errorf("machine = %q, want %q", gotForm.Get("machine"), tc.wantMachine)
			}

			if gotForm.Get("efidisk0") != tc.wantEFI {
				t.Errorf("efidisk0 = %q, want %q", gotForm.Get("efidisk0"), tc.wantEFI)
			}

			if gotForm.Get("tpmstate0") != tc.wantTPM {
				t.Errorf("tpmstate0 = %q, want %q", gotForm.Get("tpmstate0"), tc.wantTPM)
			}
		})
	}
}

// TestProxmox_CreateVM_NoUEFI asserts seabios/default emits no bios/machine/
// efidisk0/tpmstate0 keys.
func TestProxmox_CreateVM_NoUEFI(t *testing.T) {
	t.Parallel()

	var gotForm url.Values

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api2/json/nodes/node01/qemu", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotForm = r.Form

			writeJSONFixture(t, w, `{"data":"UPID:node01:...:qmcreate:109:pvmss@pve:"}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	spec := VMSpec{
		VMID: 109, Node: testNodeName, Name: "seabios-test", Pool: FakePoolAliceShort,
		Sockets: 1, CPUCores: 2, MemoryMB: 4096,
		Disk:    DiskSpec{Storage: FakeStorageLocalLVM, SizeGB: 32, Bus: string(DiskBusSCSI)},
		Network: NetworkSpec{{Bridge: FakeBridgeVMbr0, Model: string(DiskBusVirtio)}},
	}

	if _, err := p.CreateVM(context.Background(), spec); err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	for _, key := range []string{"bios", "machine", "efidisk0", "tpmstate0"} {
		if gotForm.Get(key) != "" {
			t.Errorf("%s = %q, want empty (seabios default)", key, gotForm.Get(key))
		}
	}
}

// TestProxmox_CreateVM_AgentOSTypeBoot asserts the issue-03 form keys:
// agent=1 and ostype=l26 are always present, and boot=order= is built only
// from the devices the spec actually created (disk bus + ISO cdrom key).
func TestProxmox_CreateVM_AgentOSTypeBoot(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		disk     DiskSpec
		iso      *ISOSpec
		wantBoot string
	}{
		{
			name:     "disk and ISO",
			disk:     DiskSpec{Storage: FakeStorageLocalLVM, SizeGB: 32, Bus: string(DiskBusSCSI)},
			iso:      &ISOSpec{Storage: FakeStorageLocal, File: "debian-12.iso"},
			wantBoot: "order=scsi0;ide2",
		},
		{
			name:     "disk alone",
			disk:     DiskSpec{Storage: FakeStorageLocalLVM, SizeGB: 32, Bus: string(DiskBusSCSI)},
			iso:      nil,
			wantBoot: "order=scsi0",
		},
		{
			name:     "virtio bus disk",
			disk:     DiskSpec{Storage: FakeStorageLocalLVM, SizeGB: 32, Bus: string(DiskBusVirtio)},
			iso:      nil,
			wantBoot: "order=virtio0",
		},
		{
			name:     "neither disk nor ISO omits boot key",
			disk:     DiskSpec{Bus: string(DiskBusSCSI)},
			iso:      nil,
			wantBoot: "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var gotForm url.Values

			srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
				mux.HandleFunc("POST /api2/json/nodes/node01/qemu", func(w http.ResponseWriter, r *http.Request) {
					if err := r.ParseForm(); err != nil {
						t.Fatalf("parse form: %v", err)
					}

					gotForm = r.Form

					writeJSONFixture(t, w, `{"data":"UPID:node01:...:qmcreate:110:pvmss@pve:"}`)
				})
			})

			p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

			spec := VMSpec{
				VMID: 110, Node: testNodeName, Name: "boot-test", Pool: FakePoolAliceShort,
				Sockets: 1, CPUCores: 2, MemoryMB: 2048,
				Disk:    tc.disk,
				Network: NetworkSpec{{Bridge: FakeBridgeVMbr0, Model: string(DiskBusVirtio)}},
				ISO:     tc.iso,
			}

			if _, err := p.CreateVM(context.Background(), spec); err != nil {
				t.Fatalf("CreateVM: %v", err)
			}

			if gotForm.Get("agent") != "1" {
				t.Errorf("agent = %q, want 1 (QEMU guest agent must be enabled at create time)", gotForm.Get("agent"))
			}

			if gotForm.Get("ostype") != "l26" {
				t.Errorf("ostype = %q, want l26 (every catalog entry is Linux)", gotForm.Get("ostype"))
			}

			if gotForm.Get("boot") != tc.wantBoot {
				t.Errorf("boot = %q, want %q", gotForm.Get("boot"), tc.wantBoot)
			}
		})
	}
}
