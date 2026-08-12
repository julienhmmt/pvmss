//nolint:wsl_v5 // fake state methods keep mutation and call recording adjacent
package cluster

import (
	"context"
	"fmt"
	"io"
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
type Fake struct {
	ClusterName string
}

// FakeCall is one recorded write against the fake cluster.
type FakeCall struct {
	Node          string
	VMID          int
	Action        string
	Name          string
	DiskKey       string
	Bus           string
	Storage       string
	Filename      string
	Content       string
	SizeGB        int
	Sockets       int
	Cores         int
	MemoryMB      int
	CloudInitData CloudInitConfig
}

// RoleCall records an actual shared-role creation in the fake cluster.
type RoleCall struct {
	Privileges []string
	At         time.Time
}

// ACLEntry records a pool ACL binding in the fake cluster.
type ACLEntry struct {
	Username string
	PoolID   string
	Role     string
}

var (
	fakeVMMutex       sync.RWMutex
	fakeCallMu        sync.Mutex
	fakeCallLog       []FakeCall
	fakeRoleState     map[string][]string
	fakeRoleCallLog   []RoleCall
	fakeACLs          []ACLEntry
	errDeleteUser     error
	fakeCloudInitPush struct {
		sync.RWMutex
		err error
	}
	fakeCloudInitConfigs = originalFakeCloudInitConfigs()
	fakeCloudInitDrives  = make(map[fakeCloudInitKey]bool)
	fakeSnapshots        = make(map[fakeSnapshotKey][]VMSnapshot)
)

type fakeCloudInitKey struct {
	node string
	vmid int
}

// Snapshot implements Client. It returns the T01/T02 dataset (3 nodes, 25
// VMs, 4 pools, 5 storages) reshaped into one call — the same content
// ListNodes used to surface, plus the VMs and storages later tranches need.
// Writes mutate the live dataset, so a Snapshot taken after a delete reflects
// it (AC03 §3.2 write-then-invalidate).
func (fake Fake) Snapshot(_ context.Context) (Snapshot, error) {
	if fake.unavailable() {
		return Snapshot{}, ErrUnreachable
	}
	fakeVMMutex.RLock()
	defer fakeVMMutex.RUnlock()
	nodes, sourceVMs, storages, version := fake.snapshotSources()
	nodesCopy := slices.Clone(nodes)
	vms := slices.Clone(sourceVMs)
	for i, vm := range sourceVMs {
		vms[i].Cluster = fake.ClusterName
		vms[i].Tags = append([]string(nil), vm.Tags...)
		vms[i].BootOrder = append([]string(nil), vm.BootOrder...)
		vms[i].Disks = append([]Disk(nil), vm.Disks...)
		vms[i].NetworkInterfaces = cloneNetworkInterfaces(vm.NetworkInterfaces)
		for diskIndex := range vms[i].Disks {
			vms[i].Disks[diskIndex].IsBoot = false
		}
	}
	return Snapshot{Nodes: nodesCopy, VMs: vms, Storages: slices.Clone(storages), ProxmoxVersion: version}, nil
}

// Authenticate implements Client using demonstration-only PVE identities.
func (fake Fake) Authenticate(_ context.Context, username, password string) (Identity, error) {
	if fake.unavailable() {
		return Identity{}, ErrUnreachable
	}
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
func (fake Fake) ChangePassword(_ context.Context, username, oldPassword, newPassword string) error {
	if fake.unavailable() {
		return ErrUnreachable
	}
	fakeIdentitiesMutex.Lock()
	defer fakeIdentitiesMutex.Unlock()

	identity, ok := fakeIdentities[username]
	if !ok || oldPassword != identity.password {
		return ErrNotFound
	}

	fakeIdentities[username] = fakeIdentity{password: newPassword, pool: identity.pool, isAdmin: identity.isAdmin}

	return nil
}

// GetVNCTicket implements ConsoleRelay with a fixed fabricated ticket and port
// — no network call, no state beyond what the fixture already tracks for the
// VM. The browser never sees either value; only the opaque ConsoleTicketStore
// token does (FR-002). The fake must demonstrate the feature (constitution XI),
// so the ticket is real enough for the relay to echo back, just not from
// Proxmox.
func (Fake) GetVNCTicket(_ context.Context, _ string, _ int, _ string) (VNCProxyTicket, error) {
	return VNCProxyTicket{Ticket: "fake-vnc-ticket", Port: 5901}, nil
}

// RelayConsole implements ConsoleRelay by speaking the minimal RFB 3.8
// handshake directly against peer — there is no second, separately-dialed
// connection in the fake path; the "relay" IS the fake server (data-model.md).
// Blocks until peer closes or the context is cancelled.
func (Fake) RelayConsole(ctx context.Context, _ string, _ int, _ VNCProxyTicket, peer io.ReadWriteCloser) error {
	return rfbFakeServe(ctx, peer)
}

// GetCloudInitConfig implements CloudInitReader with live per-VM fake state.
func (Fake) GetCloudInitConfig(_ context.Context, node string, vmid int) (CloudInitConfig, error) {
	fakeVMMutex.RLock()
	defer fakeVMMutex.RUnlock()

	if findFakeVM(node, vmid) < 0 {
		return CloudInitConfig{}, ErrNotFound
	}

	config, ok := fakeCloudInitConfigs[fakeCloudInitKey{node: node, vmid: vmid}]
	if !ok {
		return CloudInitConfig{IPMode: CloudInitIPModeDHCP}, nil
	}

	return cloneCloudInitConfig(config), nil
}

// FindSnippetStorage implements CloudInitReader with deterministic fake cluster data.
func (Fake) FindSnippetStorage(_ context.Context, node string) (string, error) {
	fakeVMMutex.RLock()
	defer fakeVMMutex.RUnlock()

	for _, fakeNode := range fakeNodes {
		if fakeNode.Name == node {
			return FakeSnippetStorage, nil
		}
	}

	return "", ErrNotFound
}

// ListBridges implements Client. Returns the fake bridge dataset — a superset
// of what T06 approved (vmbr0, vmbr1) so the admin demo has vmbr2 to discover
// and approve (data-model.md fixture table).
func (fake Fake) ListBridges(_ context.Context) ([]Bridge, error) {
	if fake.unavailable() {
		return nil, ErrUnreachable
	}
	return slices.Clone(fakeBridges), nil
}

// ListISOs implements Client. Returns the fake ISO dataset — a superset of
// what T06 approved (debian-12, ubuntu-24, both on local) so the admin demo
// has rocky-9 to discover and approve (data-model.md fixture table).
func (fake Fake) ListISOs(_ context.Context) ([]ISOImage, error) {
	if fake.unavailable() {
		return nil, ErrUnreachable
	}
	return slices.Clone(fakeISOs), nil
}

// ListPools implements Client and returns a defensive copy of the live pool table.
func (fake Fake) ListPools(_ context.Context) ([]Pool, error) {
	if fake.unavailable() {
		return nil, ErrUnreachable
	}
	fakeVMMutex.RLock()
	defer fakeVMMutex.RUnlock()

	return slices.Clone(fakePools), nil
}

// EnsurePoolRole creates the shared PVMSSUser role once and never rewrites it.
func (fake Fake) EnsurePoolRole(_ context.Context) error {
	if fake.unavailable() {
		return ErrUnreachable
	}
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	if fakeRoleState == nil {
		fakeRoleState = make(map[string][]string)
	}
	if _, exists := fakeRoleState[poolRoleName]; exists {
		return nil
	}

	privileges := slices.Clone(rolePrivileges)
	fakeRoleState[poolRoleName] = privileges
	fakeRoleCallLog = append(fakeRoleCallLog, RoleCall{Privileges: slices.Clone(privileges), At: time.Now().UTC()})
	recordCall(FakeCall{Action: "ensure_role", Name: poolRoleName})

	return nil
}

// EnsurePoolUser creates the pool login once and returns its PVE username.
func (fake Fake) EnsurePoolUser(_ context.Context, pool, password string) (string, error) {
	if fake.unavailable() {
		return "", ErrUnreachable
	}
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	username := pool + "@pve"
	fakeIdentitiesMutex.Lock()
	if _, exists := fakeIdentities[username]; !exists {
		fakeIdentities[username] = fakeIdentity{password: password, pool: pool}
	}
	fakeIdentitiesMutex.Unlock()
	recordCall(FakeCall{Action: "ensure_user", Name: username})

	return username, nil
}

// CreatePool inserts a pool only when its name is absent.
func (fake Fake) CreatePool(_ context.Context, poolID, comment string) error {
	if fake.unavailable() {
		return ErrUnreachable
	}
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	for _, pool := range fakePools {
		if pool.Name == poolID {
			return nil
		}
	}

	fakePools = append(fakePools, Pool{Name: poolID, Comment: comment})
	recordCall(FakeCall{Action: "create_pool", Name: poolID})

	return nil
}

// SetPoolACL records a pool-to-role binding.
func (fake Fake) SetPoolACL(_ context.Context, username, poolID, role string) error {
	if fake.unavailable() {
		return ErrUnreachable
	}
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	fakeACLs = append(fakeACLs, ACLEntry{Username: username, PoolID: poolID, Role: role})
	recordCall(FakeCall{Action: "set_acl", Name: username})

	return nil
}

// DeletePool removes a pool and its ACL entries. It is idempotent for cleanup.
func (fake Fake) DeletePool(_ context.Context, poolID string) error {
	if fake.unavailable() {
		return ErrUnreachable
	}
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	index := slices.IndexFunc(fakePools, func(pool Pool) bool { return pool.Name == poolID })
	if index < 0 {
		return ErrNotFound
	}
	fakePools = slices.Delete(fakePools, index, index+1)
	fakeACLs = slices.DeleteFunc(fakeACLs, func(acl ACLEntry) bool { return acl.PoolID == poolID })
	recordCall(FakeCall{Action: "delete_pool", Name: poolID})

	return nil
}

// DeleteUser removes a PVE identity. Tests can force a best-effort failure.
func (fake Fake) DeleteUser(_ context.Context, username string) error {
	if fake.unavailable() {
		return ErrUnreachable
	}
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	if errDeleteUser != nil {
		return errDeleteUser
	}
	fakeIdentitiesMutex.Lock()
	delete(fakeIdentities, username)
	fakeIdentitiesMutex.Unlock()
	recordCall(FakeCall{Action: "delete_user", Name: username})

	return nil
}

// FakeRoleCalls returns a defensive copy of the role creation log.
func FakeRoleCalls() []RoleCall {
	fakeVMMutex.RLock()
	defer fakeVMMutex.RUnlock()

	calls := make([]RoleCall, len(fakeRoleCallLog))
	for index, call := range fakeRoleCallLog {
		calls[index] = RoleCall{Privileges: slices.Clone(call.Privileges), At: call.At}
	}

	return calls
}

// SetFakeDeleteUserError configures a deterministic user deletion failure.
func SetFakeDeleteUserError(err error) {
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	errDeleteUser = err
}

// EnsureCloudInitDrive implements Writer and records drive assurance.
func (Fake) EnsureCloudInitDrive(_ context.Context, node string, vmid int) error {
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	if findFakeVM(node, vmid) < 0 {
		return ErrNotFound
	}

	fakeCloudInitDrives[fakeCloudInitKey{node: node, vmid: vmid}] = true
	recordCall(FakeCall{Node: node, VMID: vmid, Action: "ensure_cloudinit_drive"})

	return nil
}

// SetCloudInitConfig implements Writer and ensures a cloud-init drive first.
func (Fake) SetCloudInitConfig(ctx context.Context, node string, vmid int, config CloudInitConfig) error {
	if err := (Fake{}).EnsureCloudInitDrive(ctx, node, vmid); err != nil {
		return err
	}

	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	fakeCloudInitConfigs[fakeCloudInitKey{node: node, vmid: vmid}] = cloneCloudInitConfig(config)
	recordCall(FakeCall{Node: node, VMID: vmid, Action: "set_cloudinit_config", CloudInitData: cloneCloudInitConfig(config)})

	return nil
}

// PushCloudInitSnippet implements Writer and records the server-owned target and content.
func (Fake) PushCloudInitSnippet(_ context.Context, node, storage, filename string, vmid int, content string) error {
	fakeCloudInitPush.RLock()
	err := fakeCloudInitPush.err
	fakeCloudInitPush.RUnlock()

	recordCall(FakeCall{Node: node, VMID: vmid, Action: "push_cloudinit_snippet", Storage: storage, Filename: filename, Content: content})

	if err != nil {
		return err
	}

	return nil
}

// SetFakeCloudInitPushError configures the fake push failure used by tests.
func SetFakeCloudInitPushError(err error) {
	fakeCloudInitPush.Lock()
	defer fakeCloudInitPush.Unlock()

	fakeCloudInitPush.err = err
}

// Action implements Writer — a power transition on the Index-resolved node.
// It mutates the VM's Status so a subsequent Snapshot reflects it (the fake
// demonstrates the feature, constitution XI), and records the call.
//
// T17 (T001b): status-incompatible transitions are rejected — start on an
// already-running VM, stop/shutdown on an already-stopped one, reboot/reset on
// a stopped one. This mirrors what real Proxmox rejects natively; T05 never
// built it because no single-VM caller needed it, but T17's bulk User Story 1
// Acceptance Scenario 2 is the first caller that does.
func (Fake) Action(_ context.Context, node string, vmid int, action string) error {
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	idx := slices.IndexFunc(fakeVMs, func(v VM) bool { return v.VMID == vmid && v.Node == node })
	if idx < 0 {
		return ErrNotFound
	}

	status := fakeVMs[idx].Status
	if err := validateTransition(action, status); err != nil {
		return err
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

// validateTransition rejects a power action that makes no sense for the VM's
// current status. Real Proxmox rejects these natively; the fake mirrors that
// so T17's bulk scenarios produce the same per-target error entries a real
// cluster would.
func validateTransition(action string, status VMStatus) error {
	switch action {
	case "start":
		if status == VMRunning {
			return fmt.Errorf("%w: vm already running", ErrInvalidStateTransition)
		}
	case "stop", "shutdown":
		if status == VMStopped {
			return fmt.Errorf("%w: vm already stopped", ErrInvalidStateTransition)
		}
	case "reboot", "reset":
		if status == VMStopped {
			return fmt.Errorf("%w: vm is not running", ErrInvalidStateTransition)
		}
	}

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

// AddDisk implements Writer and appends a disk to the requested VM.
func (Fake) AddDisk(_ context.Context, node string, vmid int, bus, storage string, sizeGB int) (string, error) {
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	idx := findFakeVM(node, vmid)
	if idx < 0 {
		return "", ErrNotFound
	}

	busIndex := nextBusIndex(fakeVMs[idx].Disks, DiskBus(bus))
	key := fmt.Sprintf("%s%d", bus, busIndex)
	fakeVMs[idx].Disks = append(fakeVMs[idx].Disks, Disk{Key: key, Bus: DiskBus(bus), BusIndex: busIndex, Storage: storage, SizeGB: sizeGB})
	fakeVMs[idx].DiskTotal += int64(sizeGB) * 1024 * 1024 * 1024
	recordCall(FakeCall{Node: node, VMID: vmid, Action: "add_disk", DiskKey: key, Bus: bus, Storage: storage, SizeGB: sizeGB})

	return key, nil
}

// ResizeDisk implements Writer and grows an existing disk in the fake dataset.
func (Fake) ResizeDisk(_ context.Context, node string, vmid int, diskKey string, sizeGB int) error {
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	idx := findFakeVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	for diskIndex := range fakeVMs[idx].Disks {
		if fakeVMs[idx].Disks[diskIndex].Key != diskKey {
			continue
		}

		previous := fakeVMs[idx].Disks[diskIndex].SizeGB
		fakeVMs[idx].Disks[diskIndex].SizeGB = sizeGB
		fakeVMs[idx].DiskTotal += int64(sizeGB-previous) * 1024 * 1024 * 1024
		recordCall(FakeCall{Node: node, VMID: vmid, Action: "resize_disk", DiskKey: diskKey, SizeGB: sizeGB})

		return nil
	}

	return ErrNotFound
}

// DeleteDisk implements Writer and removes a disk from the fake dataset.
func (Fake) DeleteDisk(_ context.Context, node string, vmid int, diskKey string) error {
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	idx := findFakeVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	for diskIndex, disk := range fakeVMs[idx].Disks {
		if disk.Key != diskKey {
			continue
		}

		fakeVMs[idx].Disks = slices.Delete(fakeVMs[idx].Disks, diskIndex, diskIndex+1)
		fakeVMs[idx].DiskTotal -= int64(disk.SizeGB) * 1024 * 1024 * 1024

		recordCall(FakeCall{Node: node, VMID: vmid, Action: "delete_disk", DiskKey: diskKey})

		return nil
	}

	return ErrNotFound
}

// SetCDROM implements Writer and changes the fake VM's CD-ROM state.
func (Fake) SetCDROM(_ context.Context, node string, vmid int, state CDROMState) error {
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	idx := findFakeVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	fakeVMs[idx].CDROM = state

	recordCall(FakeCall{Node: node, VMID: vmid, Action: "set_cdrom"})

	return nil
}

// UpdateNetwork implements Writer and replaces the fake VM's network interfaces.
func (Fake) UpdateNetwork(_ context.Context, node string, vmid int, interfaces []NetworkInterface) error {
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	idx := findFakeVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	fakeVMs[idx].NetworkInterfaces = cloneNetworkInterfaces(interfaces)

	recordCall(FakeCall{Node: node, VMID: vmid, Action: "update_network"})

	return nil
}

// UpdateHardware implements Writer and updates the fake VM's CPU, memory, and tags.
func (Fake) UpdateHardware(_ context.Context, node string, vmid, sockets, cores, memoryMB int, tags []string) error {
	fakeVMMutex.Lock()
	defer fakeVMMutex.Unlock()

	idx := findFakeVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	fakeVMs[idx].Sockets = sockets
	fakeVMs[idx].Cores = cores
	fakeVMs[idx].CPUCores = sockets * cores
	fakeVMs[idx].MemoryTotal = int64(memoryMB) * 1024 * 1024

	fakeVMs[idx].Tags = append([]string(nil), tags...)

	recordCall(FakeCall{Node: node, VMID: vmid, Action: "update_hardware", Sockets: sockets, Cores: cores, MemoryMB: memoryMB})

	return nil
}

func findFakeVM(node string, vmid int) int {
	return slices.IndexFunc(fakeVMs, func(v VM) bool { return v.VMID == vmid && v.Node == node })
}

func nextBusIndex(disks []Disk, bus DiskBus) int {
	index := 0
	for _, disk := range disks {
		if disk.Bus == bus && disk.BusIndex >= index {
			index = disk.BusIndex + 1
		}
	}

	return index
}

func cloneNetworkInterfaces(interfaces []NetworkInterface) []NetworkInterface {
	cloned := make([]NetworkInterface, len(interfaces))
	for i, iface := range interfaces {
		cloned[i] = iface
		cloned[i].IPAddresses = append([]string(nil), iface.IPAddresses...)
	}

	return cloned
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
	fakePools = originalFakePools()
	fakeCloudInitConfigs = originalFakeCloudInitConfigs()
	fakeCloudInitDrives = make(map[fakeCloudInitKey]bool)
	fakeSnapshots = make(map[fakeSnapshotKey][]VMSnapshot)
	fakeRoleState = make(map[string][]string)
	fakeRoleCallLog = nil
	fakeACLs = nil
	errDeleteUser = nil
	fakeCallLog = nil

	fakeIdentitiesMutex.Lock()
	fakeIdentities = originalFakeIdentities()
	fakeIdentitiesMutex.Unlock()

	fakeCloudInitPush.Lock()
	fakeCloudInitPush.err = nil
	fakeCloudInitPush.Unlock()

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
	poolRoleName = "PVMSSUser"

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
	// FakeStorageLocalLVM is the approved local LVM fixture.
	FakeStorageLocalLVM = "local-lvm"
	// FakeStorageLocal is the deterministic default fake storage ("local").
	FakeStorageLocal = "local"
	// FakeSnippetStorage is the deterministic snippets-capable fake storage.
	FakeSnippetStorage = "local"
	// FakeCloudInitUser is the demo cloud-init account.
	FakeCloudInitUser = "debian"
	// FakeCloudInitDNS is the demo DNS server.
	FakeCloudInitDNS = "10.0.0.1"
	// FakeBridgeVMbr0 is the primary bridge fixture.
	FakeBridgeVMbr0 = "vmbr0"
)

var fakeIdentitiesMutex sync.RWMutex

var fakeIdentities = originalFakeIdentities()

func originalFakeIdentities() map[string]fakeIdentity {
	return map[string]fakeIdentity{
		FakeUserAlice: {password: "pvmss-alice", pool: FakePoolAlice}, //nolint:gosec // demo fixture credential
		FakeUserBob:   {password: "pvmss-bob", pool: FakePoolBob},     //nolint:gosec // demo fixture credential
		FakeUserAdmin: {password: "pvmss-admin", isAdmin: true},       //nolint:gosec // fixture credentials for demo mode
	}
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

var rolePrivileges = []string{
	"VM.Allocate", "VM.Audit", "VM.Console", "VM.Config.Disk",
	"VM.Config.Network", "VM.Config.CPU", "VM.Config.Memory", "VM.Config.Options",
	"VM.Config.Cloudinit", "VM.Config.CDROM", "VM.PowerMgmt", "VM.Snapshot",
	"VM.Snapshot.Rollback", "Datastore.AllocateSpace", "Datastore.Audit", "SDN.Use",
}

var fakePools = originalFakePools()

func originalFakePools() []Pool {
	return []Pool{
		{Name: FakePoolAlice, Comment: "Alice's personal pool"},
		{Name: FakePoolBob, Comment: "Bob's personal pool"},
		{Name: FakePoolCarol, Comment: "Carol's personal pool"},
		{Name: FakePoolShared, Comment: "Shared infrastructure pool"},
	}
}

var fakeStorages = []Storage{
	{Name: FakeStorageLocal, Node: FakeNode01, Type: "dir", Total: 2199023255552, Used: 879609302220, SupportsVMState: false},
	{Name: FakeStorageLocalLVM, Node: FakeNode01, Type: "lvm", Total: 549755813888, Used: 219902325555, SupportsVMState: true},
	{Name: "ceph-data", Node: FakeNode02, Type: "cephfs", Total: 1099511627776, Used: 329853488332, SupportsVMState: true},
	{Name: FakeStorageLocal, Node: FakeNode02, Type: "dir", Total: 274877906944, Used: 68719476736, SupportsVMState: false},
	{Name: "backup-nfs", Node: FakeNode03, Type: "nfs", Total: 5497558138880, Used: 1099511627776, SupportsVMState: false},
}

// fakeBridges is the T11 bridge discovery dataset. T06 approved vmbr0 and
// vmbr1; vmbr2 is the demo's unapproved target (data-model.md fixture table).
var fakeBridges = []Bridge{
	{Name: FakeBridgeVMbr0, Node: FakeNode01, Active: true, Comment: ""},
	{Name: "vmbr1", Node: FakeNode01, Active: true, Comment: ""},
	{Name: "vmbr2", Node: FakeNode02, Active: true, Comment: "guest VLAN"},
}

// fakeISOs is the T11 ISO discovery dataset. T06 approved debian-12 and
// ubuntu-24 (both on local); rocky-9 is the demo's unapproved target
// (data-model.md fixture table).
var fakeISOs = []ISOImage{
	{Storage: "local", Node: FakeNode01, File: "debian-12-generic-amd64.iso", SizeBytes: 691945472},
	{Storage: "local", Node: FakeNode01, File: "ubuntu-24.04-server-amd64.iso", SizeBytes: 1258291200},
	{Storage: "local", Node: FakeNode02, File: "rocky-9-generic-x86_64.iso", SizeBytes: 1476395008},
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
	vms := []VM{
		{VMID: 100, Name: "web-01", Node: FakeNode01, Status: VMRunning, Pool: FakePoolAlice, Tags: []string{FakeTagPvmss, "web"}, CPUCores: 2, MemoryTotal: 4294967296, DiskTotal: 34359738368, Uptime: 86400 * time.Second, Description: "Alice's primary web server"},
		{VMID: 101, Name: "web-02", Node: FakeNode01, Status: VMStopped, Pool: FakePoolAlice, Tags: []string{FakeTagPvmss, "web"}, CPUCores: 2, MemoryTotal: 4294967296, DiskTotal: 45097156608},
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
	}

	for index := range vms {
		if vms[index].VMID != 101 {
			continue
		}

		vms[index].Disks = []Disk{
			{Key: "scsi0", Bus: DiskBusSCSI, BusIndex: 0, Storage: FakeStorageLocalLVM, SizeGB: 32},
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
}
