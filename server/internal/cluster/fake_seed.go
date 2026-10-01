package cluster

import "time"

// ResetFake restores the default fake's original 25-VM dataset and clears its
// call log. Tests that mutate a zero-value Fake{} MUST defer this so later
// tests in the same binary see the full fixture. Instances from NewFake are
// isolated and do not need it.
func ResetFake() {
	defaultState().reset("")
}

type fakeIdentity struct {
	password string
	pool     string
	isAdmin  bool
}

// Fixture identifiers shared by the fake dataset and tests across packages.
// Extracted as constants to satisfy goconst and give the magic strings a name.
const (
	poolRoleName = "PVMSSUser"

	// actionDelete is the FakeCall.Action value recorded by the fake cluster's
	// DeleteVM. Also reused as the Proxmox API form key for key deletion
	// (proxmox_writer.go, proxmox_cloudinit.go, proxmox_pools.go) - same word,
	// same intent: remove this key/VM.
	actionDelete = "delete"

	FakeNode01     = "pve-node-01"
	FakeNode02     = "pve-node-02"
	FakeNode03     = "pve-node-03"
	FakePoolAlice  = "pool-alice"
	FakePoolBob    = "pool-bob"
	FakePoolCarol  = "pool-carol"
	FakePoolShared = "pool-shared"
	FakeUserAlice  = "alice@pve"
	FakeUserBob    = "bob@pve"
	FakeUserAdmin  = "admin@pve"
	FakeTagPvmss   = "pvmss"
	// fakeOSType is the kernel family the fake reports for a guest, mirroring
	// the hardcoded ostype the real create path sends (proxmox_create.go).
	fakeOSType = "l26"
	// FakeStorageLocalLVM is the approved local LVM fixture.
	FakeStorageLocalLVM = "local-lvm"
	// FakeStorageLocal is the deterministic default fake storage ("local").
	FakeStorageLocal = "local"
	// FakeStorageBackupNFS is the backup-only NFS fixture.
	FakeStorageBackupNFS = "backup-nfs"
	// FakeStoragePBS is the Proxmox Backup Server fixture.
	FakeStoragePBS = "pbs-backup"
	// FakeSnippetStorage is the deterministic snippets-capable fake storage.
	FakeSnippetStorage = "local"
	// FakeClusterOffline is the cluster name that makes a Fake report
	// ErrUnreachable - the offline demo and tests share it.
	FakeClusterOffline = "offline-demo"
	// FakeCloudInitUser is the demo cloud-init account.
	FakeCloudInitUser = "debian"
	// FakeCloudInitDNS is the demo DNS server.
	FakeCloudInitDNS = "10.0.0.1"
	// FakeBridgeVMbr0 is the primary bridge fixture.
	FakeBridgeVMbr0 = "vmbr0"
	// FakeBridgeVMbr1 is the secondary bridge fixture.
	FakeBridgeVMbr1 = "vmbr1"
	// FakePoolAliceShort is the short pool name used in Proxmox API fixtures.
	FakePoolAliceShort = "alice"
)

func originalFakeIdentities() map[string]fakeIdentity {
	return map[string]fakeIdentity{
		FakeUserAlice: {password: "pvmss-alice", pool: FakePoolAlice}, //nolint:gosec // demo fixture credential
		FakeUserBob:   {password: "pvmss-bob", pool: FakePoolBob},     //nolint:gosec // demo fixture credential
		FakeUserAdmin: {password: "pvmss-admin", isAdmin: true},       //nolint:gosec // fixture credentials for demo mode
	}
}

// The dataset below is production code, reviewed and
// versioned like the rest. Later work extends it as features are added -
// only Node is surfaced by an endpoint; VM, Storage, and Pool ride
// along so later work has something real to work with.

var fakeNodes = []Node{
	{
		Name:         FakeNode01,
		Status:       NodeOnline,
		CPUCores:     32,
		CPUUsage:     0.42,
		MemoryTotal:  137438953472,
		MemoryUsed:   68719476736,
		StorageTotal: 2199023255552,
		StorageUsed:  879609302220,
	},
	{
		Name:         FakeNode02,
		Status:       NodeOnline,
		CPUCores:     16,
		CPUUsage:     0.15,
		MemoryTotal:  68719476736,
		MemoryUsed:   17179869184,
		StorageTotal: 1099511627776,
		StorageUsed:  219902325555,
	},
	{
		Name:         FakeNode03,
		Status:       NodeOffline,
		CPUCores:     16,
		CPUUsage:     0,
		MemoryTotal:  68719476736,
		MemoryUsed:   0,
		StorageTotal: 1099511627776,
		StorageUsed:  0,
	},
}

var rolePrivileges = []string{
	"VM.Allocate", "VM.Audit", "VM.Console", "VM.Config.Disk",
	"VM.Config.Network", "VM.Config.CPU", "VM.Config.Memory", "VM.Config.Options",
	"VM.Config.Cloudinit", "VM.Config.CDROM", "VM.PowerMgmt", "VM.Snapshot",
	"VM.Snapshot.Rollback", "Datastore.AllocateSpace", "Datastore.Audit", "SDN.Use",
}

func originalFakePools() []Pool {
	return []Pool{
		{Name: FakePoolAlice, Comment: "Alice's personal pool"},
		{Name: FakePoolBob, Comment: "Bob's personal pool"},
		{Name: FakePoolCarol, Comment: "Carol's personal pool"},
		{Name: FakePoolShared, Comment: "Shared infrastructure pool"},
	}
}

var fakeStorages = []Storage{
	{Name: FakeStorageLocal, Node: FakeNode01, Type: storagePluginDir, PluginType: storagePluginDir, Content: "images,iso,vztmpl,backup,snippets", Total: 2199023255552, Used: 879609302220, SupportsVMState: false},
	{Name: FakeStorageLocalLVM, Node: FakeNode01, Type: storagePluginLVMThin, PluginType: storagePluginLVMThin, Content: "images,rootdir", Total: 549755813888, Used: 219902325555, SupportsVMState: true},
	{Name: "ceph-data", Node: FakeNode02, Type: storagePluginCephFS, PluginType: storagePluginCephFS, Content: storageContentImages, Total: 1099511627776, Used: 329853488332, SupportsVMState: true},
	{Name: FakeStorageLocal, Node: FakeNode02, Type: storagePluginDir, PluginType: storagePluginDir, Content: "images,iso", Total: 274877906944, Used: 68719476736, SupportsVMState: false},
	{Name: FakeStorageBackupNFS, Node: FakeNode03, Type: "nfs", PluginType: "nfs", Content: "backup", Total: 5497558138880, Used: 1099511627776, SupportsVMState: false},
	{Name: FakeStoragePBS, Node: FakeNode03, Type: storagePluginPBS, PluginType: storagePluginPBS, Content: "images,backup", Total: 8796093022208, Used: 2199023255552, SupportsVMState: false},
}

// fakeBridges is the bridge discovery dataset. It approves vmbr0 and
// vmbr1; vmbr2 is the demo's unapproved target (fixture table).
var fakeBridges = []Bridge{
	{Name: FakeBridgeVMbr0, Node: FakeNode01, Active: true, Comment: ""},
	{Name: FakeBridgeVMbr1, Node: FakeNode01, Active: true, Comment: ""},
	{Name: "vmbr2", Node: FakeNode02, Active: true, Comment: "guest VLAN"},
}

// fakeISOs is the ISO discovery dataset. It approves debian-12 and
// ubuntu-24 (both on local); rocky-9 is the demo's unapproved target
// (fixture table).
var fakeISOs = []ISOImage{
	{Storage: FakeStorageLocal, Node: FakeNode01, File: "debian-12-generic-amd64.iso", SizeBytes: 691945472},
	{Storage: FakeStorageLocal, Node: FakeNode01, File: "ubuntu-24.04-server-amd64.iso", SizeBytes: 1258291200},
	{Storage: FakeStorageLocal, Node: FakeNode02, File: "rocky-9-generic-x86_64.iso", SizeBytes: 1476395008},
}

// fakeCloudImages is the cloud-image discovery dataset. The ubuntu row
// mirrors the catalog_images seed (approved); debian-12 is the demo's
// discover-and-approve target and rocky-9 the unapproved rejection target.
var fakeCloudImages = []CloudImage{
	{Storage: FakeStorageLocal, Node: FakeNode01, File: "ubuntu-24.04-server-cloudimg-amd64.qcow2", SizeBytes: 644245094},
	{Storage: FakeStorageLocal, Node: FakeNode01, File: "debian-12-generic-cloudimg-amd64.qcow2", SizeBytes: 601295421},
	{Storage: FakeStorageLocal, Node: FakeNode02, File: "rocky-9-generic-cloudimg-x86_64.raw", SizeBytes: 734003200},
}

// fakeTemplates is the template discovery dataset. Two template
// VMs on pve-node-02: a cloud-init capable debian-12 cloud image (full
// clone) and a basic alpine appliance (linked clone when storage matches).
var fakeTemplates = []TemplateVM{
	{VMID: 9000, Node: FakeNode02, Name: "debian-12-cloud", CloudInitCapable: true, DiskStorage: FakeStorageLocalLVM, DiskSizeGB: 8, DiskBus: string(DiskBusSCSI)},
	{VMID: 9001, Node: FakeNode02, Name: "alpine-appliance", CloudInitCapable: false, DiskStorage: FakeStorageLocal, DiskSizeGB: 2, DiskBus: string(DiskBusSCSI)},
}

// fakeUptimeOnStart is the uptime the fake assigns when a stopped VM is started
// or a running one is rebooted/reset - a stable, non-zero value so the detail
// view's uptime card shows something meaningful after a power transition.
const fakeUptimeOnStart = 60 * time.Second

// originalFakeVMs returns the pristine 25-VM dataset. Kept as a function so
// ResetFake / NewFake can restore a fresh copy after a mutation.
func originalFakeVMs() []VM {
	vms := []VM{
		{VMID: 100, Name: "web-01", Node: FakeNode01, Status: VMRunning, Pool: FakePoolAlice, Tags: []string{FakeTagPvmss, "web"}, CPUCores: 2, MemoryTotal: 4294967296, DiskTotal: 34359738368, Uptime: 86400 * time.Second, Description: "Alice's primary web server", HasSerial: true},
		{VMID: 101, Name: "web-02", Node: FakeNode01, Status: VMStopped, Pool: FakePoolAlice, Tags: []string{FakeTagPvmss, "web"}, CPUCores: 2, MemoryTotal: 4294967296, DiskTotal: 45097156608, HasSerial: false},
		{VMID: 102, Name: "db-01", Node: FakeNode01, Status: VMRunning, Pool: FakePoolAlice, Tags: []string{FakeTagPvmss, "db"}, CPUCores: 4, MemoryTotal: 8589934592, DiskTotal: 137438953472, Uptime: 172800 * time.Second, Description: "Primary database"},
		{VMID: 103, Name: "cache-01", Node: FakeNode01, Status: VMRunning, Pool: FakePoolBob, Tags: []string{FakeTagPvmss, "cache"}, CPUCores: 2, MemoryTotal: 2147483648, DiskTotal: 10737418240, Uptime: 43200 * time.Second},
		{VMID: 104, Name: "build-01", Node: FakeNode01, Status: VMStopped, Pool: FakePoolBob, Tags: []string{FakeTagPvmss, "ci"}, CPUCores: 4, MemoryTotal: 8589934592, DiskTotal: 68719476736},
		{VMID: 105, Name: "test-01", Node: FakeNode02, Status: VMRunning, Pool: FakePoolBob, Tags: []string{FakeTagPvmss, "ci"}, CPUCores: 2, MemoryTotal: 4294967296, DiskTotal: 21474836480, Uptime: 3600 * time.Second},
		{VMID: 106, Name: "test-02", Node: FakeNode02, Status: VMStopped, Pool: FakePoolBob, Tags: []string{FakeTagPvmss, "ci"}, CPUCores: 2, MemoryTotal: 4294967296, DiskTotal: 21474836480},
		{VMID: 107, Name: "mail-01", Node: FakeNode02, Status: VMRunning, Pool: FakePoolCarol, Tags: []string{FakeTagPvmss, "mail"}, CPUCores: 2, MemoryTotal: 4294967296, DiskTotal: 42949672960, Uptime: 259200 * time.Second},
		{VMID: 108, Name: "proxy-01", Node: FakeNode02, Status: VMRunning, Pool: FakePoolCarol, Tags: []string{FakeTagPvmss, "proxy"}, CPUCores: 1, MemoryTotal: 1073741824, DiskTotal: 10737418240, Uptime: 259200 * time.Second},
		{VMID: 109, Name: "legacy-01", Node: FakeNode02, Status: VMStopped, Pool: FakePoolCarol, Tags: nil, CPUCores: 4, MemoryTotal: 8589934592, DiskTotal: 68719476736},
		{VMID: 110, Name: "legacy-02", Node: FakeNode02, Status: VMStopped, Pool: FakePoolCarol, Tags: nil, CPUCores: 4, MemoryTotal: 8589934592, DiskTotal: 68719476736},
		{VMID: 111, Name: "backup-01", Node: FakeNode03, Status: VMStopped, Pool: FakePoolShared, Tags: []string{FakeTagPvmss, "backup"}, CPUCores: 2, MemoryTotal: 4294967296, DiskTotal: 1099511627776},
		{VMID: 112, Name: "monitor-01", Node: FakeNode01, Status: VMRunning, Pool: FakePoolShared, Tags: []string{FakeTagPvmss, "monitoring"}, CPUCores: 2, MemoryTotal: 4294967296, DiskTotal: 21474836480, Uptime: 432000 * time.Second},
		{VMID: 113, Name: "monitor-02", Node: FakeNode01, Status: VMPaused, Pool: FakePoolShared, Tags: []string{FakeTagPvmss, "monitoring"}, CPUCores: 2, MemoryTotal: 4294967296, DiskTotal: 21474836480},
		{VMID: 114, Name: "sandbox-01", Node: FakeNode02, Status: VMStopped, Pool: FakePoolAlice, Tags: []string{FakeTagPvmss, "sandbox"}, CPUCores: 1, MemoryTotal: 1073741824, DiskTotal: 5368709120},
		{VMID: 115, Name: "sandbox-02", Node: FakeNode02, Status: VMStopped, Pool: FakePoolAlice, Tags: []string{FakeTagPvmss, "sandbox"}, CPUCores: 1, MemoryTotal: 1073741824, DiskTotal: 5368709120},
		{VMID: 116, Name: "app-01", Node: FakeNode01, Status: VMRunning, Pool: FakePoolBob, Tags: []string{FakeTagPvmss, "app"}, CPUCores: 4, MemoryTotal: 8589934592, DiskTotal: 42949672960, Uptime: 86400 * time.Second},
		{VMID: 117, Name: "app-02", Node: FakeNode01, Status: VMRunning, Pool: FakePoolBob, Tags: []string{FakeTagPvmss, "app"}, CPUCores: 4, MemoryTotal: 8589934592, DiskTotal: 42949672960, Uptime: 86400 * time.Second},
		{VMID: 118, Name: "app-03", Node: FakeNode02, Status: VMRunning, Pool: FakePoolBob, Tags: []string{FakeTagPvmss, "app"}, CPUCores: 4, MemoryTotal: 8589934592, DiskTotal: 42949672960, Uptime: 86400 * time.Second},
		{VMID: 119, Name: "queue-01", Node: FakeNode02, Status: VMRunning, Pool: FakePoolCarol, Tags: []string{FakeTagPvmss, "queue"}, CPUCores: 2, MemoryTotal: 4294967296, DiskTotal: 21474836480, Uptime: 172800 * time.Second},
		{VMID: 120, Name: "search-01", Node: FakeNode01, Status: VMRunning, Pool: FakePoolCarol, Tags: []string{FakeTagPvmss, "search"}, CPUCores: 4, MemoryTotal: 17179869184, DiskTotal: 137438953472, Uptime: 345600 * time.Second},
		{VMID: 121, Name: "archive-01", Node: FakeNode03, Status: VMStopped, Pool: FakePoolShared, Tags: nil, CPUCores: 2, MemoryTotal: 4294967296, DiskTotal: 549755813888},
		{VMID: 122, Name: "archive-02", Node: FakeNode03, Status: VMStopped, Pool: FakePoolShared, Tags: nil, CPUCores: 2, MemoryTotal: 4294967296, DiskTotal: 549755813888},
		{VMID: 123, Name: "dev-01", Node: FakeNode01, Status: VMRunning, Pool: FakePoolAlice, Tags: []string{FakeTagPvmss, "dev"}, CPUCores: 2, MemoryTotal: 4294967296, DiskTotal: 21474836480, Uptime: 7200 * time.Second, Description: "Alice's dev box"},
		{VMID: 124, Name: "dev-02", Node: FakeNode01, Status: VMStopped, Pool: FakePoolAlice, Tags: []string{FakeTagPvmss, "dev"}, CPUCores: 2, MemoryTotal: 4294967296, DiskTotal: 21474836480},
	}
	seedFakeHardware(vms)

	return vms
}

func seedFakeHardware(vms []VM) {
	for index := range vms {
		vms[index].Sockets = 1
		vms[index].Cores = vms[index].CPUCores
		// The real create path always sends agent=1 (proxmox_create.go), so
		// seeded VMs mirror an enabled guest-agent channel - except running
		// VM 103, which keeps agent=0 like a VM not created through PVMSS,
		// so the detail endpoint's "agent disabled" explanation stays live.
		vms[index].Agent = vms[index].VMID != 103
		// The real create path always sends ostype=l26 (proxmox_create.go),
		// so seeded VMs mirror it.
		vms[index].OSType = "l26"
	}

	// A few guests are Windows so the list's OS mark has more than one family
	// to show. 114 is in Alice's pool, so the signed-in demo user actually
	// sees both families; 109/110 are the "legacy" pair.
	for index := range vms {
		switch vms[index].VMID {
		case 109, 110, 114:
			vms[index].OSType = "win11"
		}
	}

	for index := range vms {
		if vms[index].VMID != 101 {
			continue
		}

		vms[index].Disks = []Disk{
			{Key: diskKeySCSI0, Bus: DiskBusSCSI, BusIndex: 0, Storage: FakeStorageLocalLVM, SizeGB: 32},
			{Key: "scsi1", Bus: DiskBusSCSI, BusIndex: 1, Storage: FakeStorageLocalLVM, SizeGB: 10},
		}
		vms[index].CDROM = CDROMState{State: CDROMMounted, ISOVolID: "local:iso/debian-12-generic-amd64.iso"}
		vms[index].NetworkInterfaces = []NetworkInterface{{
			Index:  0,
			Bridge: FakeBridgeVMbr0,
			Model:  string(DiskBusVirtio),
			MAC:    "BC:24:11:00:00:65",
		}}
	}

	// A running VM needs a NIC for the guest-agent IP read to have something
	// to report - the stopped 101 above exercises the "agent unreachable" side.
	for index := range vms {
		if vms[index].VMID != 100 {
			continue
		}

		vms[index].NetworkInterfaces = []NetworkInterface{{
			Index:  0,
			Bridge: FakeBridgeVMbr0,
			Model:  string(DiskBusVirtio),
			MAC:    "BC:24:11:00:00:64",
		}}
	}
}
