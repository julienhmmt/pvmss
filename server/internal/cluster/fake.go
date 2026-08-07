package cluster

import (
	"context"
	"slices"
	"sync"
	"time"
)

// Fake is the built-in cluster substitute (constitution XI). It requires no
// external service and serves a stable, hand-authored dataset. Neither this
// type nor Proxmox reports which one it is — callers cannot tell them apart.
//
// Writes (Action/Delete/Patch) mutate the in-memory dataset under a mutex and
// append to a call log so tests can assert exactly which calls reached the
// "cluster" (S01's proof of concept, inverted: zero calls for a forbidden
// request). ResetFake restores the original dataset and clears the log; any
// test that mutates MUST defer it so later tests in the same binary see the
// full 25-VM dataset.
type Fake struct{}

// FakeCall is one recorded write against the fake cluster.
type FakeCall struct {
	Node   string
	VMID   int
	Action string // "start"|"stop"|"shutdown"|"reboot"|"reset"|"delete"|"patch"|"create"
	Name   string // set only for patch
}

var (
	fakeVMMutex sync.RWMutex
	fakeCallMu  sync.Mutex
	fakeCallLog []FakeCall
)

// Snapshot implements Client. It returns the T01/T02 dataset (3 nodes, 25
// VMs, 4 pools, 5 storages) reshaped into one call — the same content
// ListNodes used to surface, plus the VMs and storages later tranches need.
// Writes mutate the live dataset, so a Snapshot taken after a delete reflects
// it (AC03 §3.2 write-then-invalidate).
func (Fake) Snapshot(_ context.Context) (Snapshot, error) {
	fakeVMMutex.RLock()
	defer fakeVMMutex.RUnlock()

	nodes := make([]Node, len(fakeNodes))
	copy(nodes, fakeNodes)

	vms := make([]VM, len(fakeVMs))
	copy(vms, fakeVMs)

	for i, vm := range fakeVMs {
		if vm.Tags != nil {
			vms[i].Tags = append([]string(nil), vm.Tags...)
		}
	}

	storages := make([]Storage, len(fakeStorages))
	copy(storages, fakeStorages)

	return Snapshot{Nodes: nodes, VMs: vms, Storages: storages}, nil
}

// Authenticate implements Client using demonstration-only PVE identities.
func (Fake) Authenticate(_ context.Context, username, password string) (Identity, error) {
	fakeIdentitiesMutex.RLock()
	defer fakeIdentitiesMutex.RUnlock()

	identity, ok := fakeIdentities[username]
	if !ok || password != identity.password {
		return Identity{}, ErrNotFound
	}

	return Identity{Username: username, Pool: identity.pool, IsAdmin: identity.isAdmin}, nil
}

// ChangePassword implements Client against the same in-memory demo table
// Authenticate reads — the fake's own storage, analogous to a real cluster's
// user database (constitution XI: the fake must demonstrate every feature).
func (Fake) ChangePassword(_ context.Context, username, oldPassword, newPassword string) error {
	fakeIdentitiesMutex.Lock()
	defer fakeIdentitiesMutex.Unlock()

	identity, ok := fakeIdentities[username]
	if !ok || oldPassword != identity.password {
		return ErrNotFound
	}

	fakeIdentities[username] = fakeIdentity{password: newPassword, pool: identity.pool, isAdmin: identity.isAdmin}

	return nil
}

// Action implements Writer — a power transition on the Index-resolved node.
// It mutates the VM's Status so a subsequent Snapshot reflects it (the fake
// demonstrates the feature, constitution XI), and records the call.
func (Fake) Action(_ context.Context, node string, vmid int, action string) error {
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	idx := slices.IndexFunc(fakeVMs, func(v VM) bool { return v.VMID == vmid && v.Node == node })
	if idx < 0 {
		return ErrNotFound
	}

	switch action {
	case "start":
		fakeVMs[idx].Status = VMRunning
		fakeVMs[idx].Uptime = fakeUptimeOnStart
	case "stop", "shutdown":
		fakeVMs[idx].Status = VMStopped
		fakeVMs[idx].Uptime = 0
	case "reboot":
		fakeVMs[idx].Status = VMRunning
		fakeVMs[idx].Uptime = fakeUptimeOnStart
	case "reset":
		fakeVMs[idx].Status = VMRunning
		fakeVMs[idx].Uptime = fakeUptimeOnStart
	default:
		return ErrInvalidAction
	}

	recordCall(FakeCall{Node: node, VMID: vmid, Action: action})

	return nil
}

// Delete implements Writer — the VM and its disks are removed from the
// dataset. Irreversible (V14): no soft-delete, no undo.
func (Fake) Delete(_ context.Context, node string, vmid int) error {
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	idx := slices.IndexFunc(fakeVMs, func(v VM) bool { return v.VMID == vmid && v.Node == node })
	if idx < 0 {
		return ErrNotFound
	}

	fakeVMs = slices.Delete(fakeVMs, idx, idx+1)

	recordCall(FakeCall{Node: node, VMID: vmid, Action: "delete"})

	return nil
}

// Patch implements Writer — name and/or description update. Empty arguments
// are ignored; the caller (vm.Patch) decides which fields to send.
func (Fake) Patch(_ context.Context, node string, vmid int, name, description string) error {
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	idx := slices.IndexFunc(fakeVMs, func(v VM) bool { return v.VMID == vmid && v.Node == node })
	if idx < 0 {
		return ErrNotFound
	}

	if name != "" {
		fakeVMs[idx].Name = name
	}

	if description != "" {
		fakeVMs[idx].Description = description
	}

	recordCall(FakeCall{Node: node, VMID: vmid, Action: "patch", Name: name})

	return nil
}

// FakeCalls returns a copy of the recorded write calls since the last reset.
// Tests assert on this to prove a forbidden request reached the cluster zero
// times (S01 SC-001).
func FakeCalls() []FakeCall {
	fakeCallMu.Lock()
	defer fakeCallMu.Unlock()

	return append([]FakeCall(nil), fakeCallLog...)
}

// FakeCallsFor returns the calls recorded for one VMID.
func FakeCallsFor(vmid int) []FakeCall {
	all := FakeCalls()

	out := make([]FakeCall, 0, len(all))
	for _, c := range all {
		if c.VMID == vmid {
			out = append(out, c)
		}
	}

	return out
}

// ResetFake restores the original 25-VM dataset and clears the call log. Any
// test that mutates the fake MUST defer this so later tests in the same binary
// see the full dataset (test isolation — Go runs tests in a package sequentially
// unless t.Parallel, and no test in this repo uses t.Parallel).
func ResetFake() {
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	fakeCallMu.Lock()
	defer fakeCallMu.Unlock()

	fakeVMs = originalFakeVMs()
	fakeCallLog = nil

	resetFakeCreateState()
}

func recordCall(call FakeCall) {
	fakeCallMu.Lock()

	fakeCallLog = append(fakeCallLog, call)
	fakeCallMu.Unlock()
}

type fakeIdentity struct {
	password string
	pool     string
	isAdmin  bool
}

// Fixture identifiers shared by the fake dataset and tests across packages.
// Extracted as constants to satisfy goconst and give the magic strings a name.
const (
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
)

var fakeIdentitiesMutex sync.RWMutex

var fakeIdentities = map[string]fakeIdentity{
	FakeUserAlice: {password: "pvmss-alice", pool: FakePoolAlice}, //nolint:gosec // demo fixture credential
	FakeUserBob:   {password: "pvmss-bob", pool: FakePoolBob},     //nolint:gosec // demo fixture credential
	FakeUserAdmin: {password: "pvmss-admin", isAdmin: true},       //nolint:gosec // fixture credentials for demo mode
}

// The dataset below is production code (constitution XI), reviewed and
// versioned like the rest. Later tranches extend it as they add features —
// only Node is surfaced by an endpoint at T01; VM, Storage, and Pool ride
// along so those tranches have something real to work with.

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

var fakePools = []Pool{
	{Name: FakePoolAlice, Comment: "Alice's personal pool"},
	{Name: FakePoolBob, Comment: "Bob's personal pool"},
	{Name: FakePoolCarol, Comment: "Carol's personal pool"},
	{Name: FakePoolShared, Comment: "Shared infrastructure pool"},
}

var fakeStorages = []Storage{
	{Name: "local", Node: FakeNode01, Type: "dir", Total: 2199023255552, Used: 879609302220},
	{Name: "local-lvm", Node: FakeNode01, Type: "lvm", Total: 549755813888, Used: 219902325555},
	{Name: "ceph-data", Node: FakeNode02, Type: "cephfs", Total: 1099511627776, Used: 329853488332},
	{Name: "local", Node: FakeNode02, Type: "dir", Total: 274877906944, Used: 68719476736},
	{Name: "backup-nfs", Node: FakeNode03, Type: "nfs", Total: 5497558138880, Used: 1099511627776},
}

// fakeUptimeOnStart is the uptime the fake assigns when a stopped VM is started
// or a running one is rebooted/reset — a stable, non-zero value so the detail
// view's uptime card shows something meaningful after a power transition.
const fakeUptimeOnStart = 60 * time.Second

// fakeVMs is the live, mutable dataset. Writes (Action/Delete/Patch) mutate it
// under fakeVMMutex; Snapshot copies it. ResetFake restores it from
// originalFakeVMs.
var fakeVMs = originalFakeVMs()

// originalFakeVMs returns the pristine 25-VM dataset. Kept as a function so
// ResetFake can restore a fresh copy after a test mutates the live slice.
func originalFakeVMs() []VM {
	return []VM{
		{VMID: 100, Name: "web-01", Node: FakeNode01, Status: VMRunning, Pool: FakePoolAlice, Tags: []string{FakeTagPvmss, "web"}, CPUCores: 2, MemoryTotal: 4294967296, DiskTotal: 34359738368, Uptime: 86400 * time.Second, Description: "Alice's primary web server"},
		{VMID: 101, Name: "web-02", Node: FakeNode01, Status: VMStopped, Pool: FakePoolAlice, Tags: []string{FakeTagPvmss, "web"}, CPUCores: 2, MemoryTotal: 4294967296, DiskTotal: 34359738368},
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
}
