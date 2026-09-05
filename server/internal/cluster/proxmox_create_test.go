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

// TestProxmox_CreateVM_ImageImportFrom asserts the cloud-image disk form:
// import-from requires Proxmox's <storage>:0 target syntax (a non-zero size
// is rejected by check_drive_param), and the source must be a PVE-managed
// volume of vtype 'import' — passed as a volid, not an absolute path
// (absolute paths are root@pam-only). Cloud images live in the storage's
// import/ directory with .qcow2/.raw/.vmdk/.ova extensions.
func TestProxmox_CreateVM_ImageImportFrom(t *testing.T) {
	t.Parallel()

	var gotForm url.Values

	srv := newProxmoxTestServer(t, func(mux *http.ServeMux) {
		mux.HandleFunc("POST /api2/json/nodes/node01/qemu", func(w http.ResponseWriter, r *http.Request) {
			if err := r.ParseForm(); err != nil {
				t.Fatalf("parse form: %v", err)
			}

			gotForm = r.Form

			writeJSONFixture(t, w, `{"data":"UPID:node01:...:qmcreate:111:pvmss@pve:"}`)
		})
	})

	p := Proxmox{BaseURL: srv.URL, APITokenName: testTokenName, APITokenValue: testTokenVal}

	spec := VMSpec{
		VMID: 111, Node: testNodeName, Name: "img-test", Pool: FakePoolAliceShort,
		Sockets: 1, CPUCores: 1, MemoryMB: 2048,
		Disk:    DiskSpec{Storage: FakeStorageLocalLVM, SizeGB: 12, Bus: string(DiskBusSCSI)},
		Network: NetworkSpec{{Bridge: FakeBridgeVMbr0, Model: string(DiskBusVirtio)}},
		Image:   &ImageSpec{Storage: FakeStorageLocal, File: "debian-13-genericcloud-amd64.qcow2"},
	}

	if _, err := p.CreateVM(context.Background(), spec); err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	wantDisk := "local-lvm:0,discard=on,import-from=local:import/debian-13-genericcloud-amd64.qcow2,iothread=1"

	if gotForm.Get(diskKeySCSI0) != wantDisk {
		t.Errorf("scsi0 = %q, want %q", gotForm.Get(diskKeySCSI0), wantDisk)
	}

	// The cloud-init drive is attached in this same create call — ProxMate
	// and pegaprox both do this, never as a later follow-up — so the seed
	// device exists before the create task even finishes.
	if got, want := gotForm.Get(cloudInitDiskKey), FakeStorageLocalLVM+":cloudinit"; got != want {
		t.Errorf("%s = %q, want %q", cloudInitDiskKey, got, want)
	}

	// Network config is applied afterwards through SetCloudInitConfig, not
	// at create time (the REST API cannot write a snippet file, so native
	// keys are the only per-VM cloud-init delivery mechanism) — no ipconfig0
	// form key at create time.
	if gotForm.Get("ipconfig0") != "" {
		t.Errorf("ipconfig0 = %q, want unset at create time", gotForm.Get("ipconfig0"))
	}
}

func TestProxmox_TaskStatus(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name         string
		status       string
		exitStatus   string
		want         TaskState
		wantWarnings string
	}{
		{"running", string(VMRunning), "", TaskRunning, ""},
		{"ok", string(VMStopped), "OK", TaskOK, ""},
		{"warnings one", string(VMStopped), "WARNINGS: 1", TaskOK, "WARNINGS: 1"},
		{"warnings bare", string(VMStopped), "WARNINGS", TaskOK, "WARNINGS"},
		{"error", string(VMStopped), "job errored: disk full", TaskError, ""},
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

			assertTaskStatus(t, got, tc.want, tc.wantWarnings, tc.exitStatus)

			if len(got.Log) != 1 {
				t.Errorf("log = %v, want one line", got.Log)
			}
		})
	}
}

// assertTaskStatus checks the TaskStatus state, warnings, and exit-message
// invariants (Warnings present only on TaskOK, ExitMessage only on TaskError).
// Extracted from TestProxmox_TaskStatus to keep its cognitive complexity
// under SonarQube's go:S3776 limit.
func assertTaskStatus(t *testing.T, got TaskStatus, want TaskState, wantWarnings, exitStatus string) {
	t.Helper()

	if got.State != want {
		t.Errorf("state = %q, want %q", got.State, want)
	}

	if got.Warnings != wantWarnings {
		t.Errorf("warnings = %q, want %q", got.Warnings, wantWarnings)
	}

	if want == TaskError && got.ExitMessage != exitStatus {
		t.Errorf("exitMessage = %q, want %q", got.ExitMessage, exitStatus)
	}

	if want == TaskOK && got.ExitMessage != "" {
		t.Errorf("exitMessage = %q, want empty on TaskOK", got.ExitMessage)
	}

	if want != TaskOK && got.Warnings != "" {
		t.Errorf("warnings = %q, want empty when not TaskOK", got.Warnings)
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

			gotForm := captureDiskDefaultsForm(t, tc.bus)
			assertDiskDefaultsForm(t, gotForm, tc.bus, tc.wantDisk, tc.wantSCSIHW)
		})
	}
}

// captureDiskDefaultsForm runs CreateVM with a form-capturing test server for
// the disk-defaults test cases. Returns the submitted form.
func captureDiskDefaultsForm(t *testing.T, bus string) url.Values {
	t.Helper()

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
		Disk:    DiskSpec{Storage: FakeStorageLocalLVM, SizeGB: 20, Bus: bus},
		Network: NetworkSpec{{Bridge: FakeBridgeVMbr0, Model: string(DiskBusVirtio)}},
	}

	if _, err := p.CreateVM(context.Background(), spec); err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	return gotForm
}

// assertDiskDefaultsForm checks the disk and scsihw form fields. Extracted
// from TestProxmox_CreateVM_DiskDefaults to keep its cognitive complexity
// under go:S3776's ceiling.
func assertDiskDefaultsForm(t *testing.T, form url.Values, bus, wantDisk, wantSCSIHW string) {
	t.Helper()

	diskKey := bus + "0"
	if form.Get(diskKey) != wantDisk {
		t.Errorf("%s = %q, want %q", diskKey, form.Get(diskKey), wantDisk)
	}

	if wantSCSIHW != "" {
		if form.Get("scsihw") != wantSCSIHW {
			t.Errorf("scsihw = %q, want %q", form.Get("scsihw"), wantSCSIHW)
		}
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

			gotForm := captureUEFIForm(t, tc.bios, tc.machine, tc.tpm)
			assertUEFIForm(t, gotForm, tc.wantMachine, tc.wantEFI, tc.wantTPM)
		})
	}
}

// captureUEFIForm runs CreateVM with a form-capturing test server for the
// UEFI/TPM test cases. Returns the submitted form.
func captureUEFIForm(t *testing.T, bios, machine string, tpm bool) url.Values {
	t.Helper()

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
		BIOS:    bios, Machine: machine, TPM: tpm,
	}

	if _, err := p.CreateVM(context.Background(), spec); err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	return gotForm
}

// assertUEFIForm checks the bios/machine/efidisk0/tpmstate0 form fields.
// Extracted from TestProxmox_CreateVM_UEFI to keep its cognitive complexity
// under go:S3776's ceiling.
func assertUEFIForm(t *testing.T, form url.Values, wantMachine, wantEFI, wantTPM string) {
	t.Helper()

	if form.Get("bios") != testBIOSOVMF {
		t.Errorf("bios = %q, want ovmf", form.Get("bios"))
	}

	if form.Get("machine") != wantMachine {
		t.Errorf("machine = %q, want %q", form.Get("machine"), wantMachine)
	}

	if form.Get("efidisk0") != wantEFI {
		t.Errorf("efidisk0 = %q, want %q", form.Get("efidisk0"), wantEFI)
	}

	if form.Get("tpmstate0") != wantTPM {
		t.Errorf("tpmstate0 = %q, want %q", form.Get("tpmstate0"), wantTPM)
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
			name:     "disk and ISO (CD-ROM first so the VM boots from the installer)",
			disk:     DiskSpec{Storage: FakeStorageLocalLVM, SizeGB: 32, Bus: string(DiskBusSCSI)},
			iso:      &ISOSpec{Storage: FakeStorageLocal, File: "debian-12.iso"},
			wantBoot: "order=ide2;scsi0",
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

			gotForm := captureCreateVMForm(t, tc.disk, tc.iso)

			assertAgentOSTypeBootForm(t, gotForm, tc.wantBoot)
		})
	}
}

// assertAgentOSTypeBootForm checks the issue-03 form keys (agent=1,
// ostype=l26, boot=order=...). Extracted from TestProxmox_CreateVM_AgentOSTypeBoot
// to keep its cognitive complexity under go:S3776's ceiling.
func assertAgentOSTypeBootForm(t *testing.T, form url.Values, wantBoot string) {
	t.Helper()

	if form.Get("agent") != "1" {
		t.Errorf("agent = %q, want 1 (QEMU guest agent must be enabled at create time)", form.Get("agent"))
	}

	if form.Get("ostype") != "l26" {
		t.Errorf("ostype = %q, want l26 (every catalog entry is Linux)", form.Get("ostype"))
	}

	if form.Get("boot") != wantBoot {
		t.Errorf("boot = %q, want %q", form.Get("boot"), wantBoot)
	}
}

// captureCreateVMForm runs CreateVM with a form-capturing test server and
// returns the submitted form. Shared by the issue-03 boot-order test cases.
func captureCreateVMForm(t *testing.T, disk DiskSpec, iso *ISOSpec) url.Values {
	t.Helper()

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
		Disk:    disk,
		Network: NetworkSpec{{Bridge: FakeBridgeVMbr0, Model: string(DiskBusVirtio)}},
		ISO:     iso,
	}

	if _, err := p.CreateVM(context.Background(), spec); err != nil {
		t.Fatalf("CreateVM: %v", err)
	}

	return gotForm
}
