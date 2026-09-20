//nolint:wsl_v5 // fake state methods keep mutation and call recording adjacent
package cluster

import (
	"context"
	"fmt"
	"io"
	"slices"
	"time"
)

const (
	actionStart    = "start"
	actionStop     = "stop"
	actionShutdown = "shutdown"
	actionReboot   = "reboot"
	actionReset    = "reset"
	actionPause    = "pause"
	actionResume   = "resume"
)

// Fake is the built-in cluster substitute. It requires no
// external service and serves a stable, hand-authored dataset. Neither this
// type nor Proxmox reports which one it is - callers cannot tell them apart.
//
// Writes (Action/Delete/Patch) mutate the instance's in-memory dataset under
// a mutex and append to a call log so tests can assert exactly which calls
// reached the "cluster" (proof of concept, inverted: zero calls for a forbidden request).
//
// Prefer NewFake(name) so each cluster (and each test) owns its state.
// A zero-value Fake{} still works: it shares a process-wide default dataset.
// Tests that mutate that default MUST defer ResetFake so later tests in the
// same binary see the full 25-VM fixture.
type Fake struct {
	ClusterName string
	state       *fakeState
}

// FakeCall is one recorded write against the fake cluster.
type FakeCall struct {
	Node          string
	VMID          int
	Action        string
	Name          string
	Pool          string
	Full          bool
	DiskKey       string
	Bus           string
	Storage       string
	Filename      string
	Content       string
	SizeGB        int
	Sockets       int
	Cores         int
	MemoryMB      int
	BootOrder     []string
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

type fakeCloudInitKey struct {
	node string
	vmid int
}

// Snapshot implements Client. It returns the dataset (3 nodes, 25
// VMs, 4 pools, 5 storages) reshaped into one call - the same content
// ListNodes used to surface, plus the VMs and storages later work needs.
// Writes mutate the live dataset, so a Snapshot taken after a delete reflects
// it (write-then-invalidate).
func (fake Fake) Snapshot(_ context.Context) (Snapshot, error) {
	state := fake.stateOrDefault()
	if fake.unavailable() {
		return Snapshot{}, ErrUnreachable
	}
	state.vmMu.RLock()
	defer state.vmMu.RUnlock()
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

// DisplayName implements Client. The fake reports a deterministic display
// name derived from its logical name so multi-cluster tests can distinguish
// clusters without a real /cluster/status endpoint.
func (fake Fake) DisplayName(_ context.Context) (string, error) {
	if fake.unavailable() {
		return "", ErrUnreachable
	}
	if fake.ClusterName == "" {
		return "fake-cluster", nil
	}
	return fake.ClusterName, nil
}

// Authenticate implements Client using demonstration-only PVE identities.
func (fake Fake) Authenticate(_ context.Context, username, password string) (Identity, error) {
	state := fake.stateOrDefault()
	if fake.unavailable() {
		return Identity{}, ErrUnreachable
	}
	state.identMu.RLock()
	defer state.identMu.RUnlock()

	identity, ok := state.identities[username]
	if !ok || password != identity.password {
		return Identity{}, ErrNotFound
	}

	return Identity{Username: username, Pool: identity.pool, IsAdmin: identity.isAdmin}, nil
}

// ChangePassword implements Client against the same in-memory demo table
// Authenticate reads - the fake's own storage, analogous to a real cluster's
// user database; the fake must demonstrate every feature.
func (fake Fake) ChangePassword(_ context.Context, username, oldPassword, newPassword string) error {
	state := fake.stateOrDefault()
	if fake.unavailable() {
		return ErrUnreachable
	}
	state.identMu.Lock()
	defer state.identMu.Unlock()

	identity, ok := state.identities[username]
	if !ok || oldPassword != identity.password {
		return ErrNotFound
	}

	state.identities[username] = fakeIdentity{password: newPassword, pool: identity.pool, isAdmin: identity.isAdmin}

	return nil
}

// GetVNCTicket implements ConsoleRelay with a fixed fabricated ticket and port
// - no network call, no state beyond what the fixture already tracks for the
// VM. The browser never sees either value; only the opaque ConsoleTicketStore
// token does. The fake must demonstrate the feature,
// so the ticket is real enough for the relay to echo back, just not from
// Proxmox.
func (Fake) GetVNCTicket(_ context.Context, _ string, _ int, _ string) (VNCProxyTicket, error) {
	return VNCProxyTicket{Ticket: "fake-vnc-ticket", Port: 5901}, nil
}

// RelayConsole implements ConsoleRelay by speaking the minimal RFB 3.8
// handshake directly against peer - there is no second, separately-dialed
// connection in the fake path; the "relay" IS the fake server.
// Blocks until peer closes or the context is cancelled.
func (Fake) RelayConsole(ctx context.Context, _ string, _ int, _ VNCProxyTicket, peer io.ReadWriteCloser) error {
	return rfbFakeServe(ctx, peer)
}

// GetTermProxy implements TerminalRelay with a fixed fabricated ticket and port
// - no network call, no state beyond what the fixture already tracks for the
// VM. The browser never sees either value; only the opaque ConsoleTicketStore
// token does. Mirrors GetVNCTicket's fake so the serial feature is genuinely
// functional offline.
func (Fake) GetTermProxy(_ context.Context, _ string, _ int, _ string) (TermProxyTicket, error) {
	return TermProxyTicket{Ticket: "fake-term-ticket", Port: 5902}, nil
}

// RelaySerial implements TerminalRelay as a minimal echo/byte-pipe against
// peer - there is no second, separately-dialed connection in the fake path.
// It reads bytes the browser writes and echoes them back prefixed with a "0:len:" data frame so
// an xterm.js client sees its own keystrokes render,
// which is enough to demonstrate the serial feature offline without pretending
// to be a real OS. Blocks until peer closes or the context
// is cancelled.
func (Fake) RelaySerial(ctx context.Context, _ string, _ int, _ TermProxyTicket, peer io.ReadWriteCloser) error {
	return serialFakeServe(ctx, peer)
}

// GetCloudInitConfig implements CloudInitReader with live per-VM fake state.
func (fake Fake) GetCloudInitConfig(_ context.Context, node string, vmid int) (CloudInitConfig, error) {
	state := fake.stateOrDefault()
	state.vmMu.RLock()
	defer state.vmMu.RUnlock()

	if state.findVM(node, vmid) < 0 {
		return CloudInitConfig{}, ErrNotFound
	}

	config, ok := state.cloudInitConfigs[fakeCloudInitKey{node: node, vmid: vmid}]
	if !ok {
		return CloudInitConfig{IPMode: CloudInitIPModeDHCP}, nil
	}

	return cloneCloudInitConfig(config), nil
}

// FindSnippetStorage implements CloudInitReader with deterministic fake cluster data.
func (fake Fake) FindSnippetStorage(_ context.Context, node string) (string, error) {
	state := fake.stateOrDefault()
	state.vmMu.RLock()
	defer state.vmMu.RUnlock()

	for _, fakeNode := range state.nodes {
		if fakeNode.Name == node {
			return FakeSnippetStorage, nil
		}
	}

	return "", ErrNotFound
}

// ListBridges implements Client. Returns the fake bridge dataset - a superset
// of what the catalog approves (vmbr0, vmbr1) so the admin demo has vmbr2 to discover
// and approve (fixture table).
func (fake Fake) ListBridges(_ context.Context) ([]Bridge, error) {
	if fake.unavailable() {
		return nil, ErrUnreachable
	}
	return slices.Clone(fakeBridges), nil
}

// ListISOs implements Client. Returns the fake ISO dataset - a superset of
// what the catalog approves (debian-12, ubuntu-24, both on local) so the admin demo
// has rocky-9 to discover and approve (fixture table).
func (fake Fake) ListISOs(_ context.Context) ([]ISOImage, error) {
	if fake.unavailable() {
		return nil, ErrUnreachable
	}
	return slices.Clone(fakeISOs), nil
}

// ListCloudImages implements Client. Returns the fake cloud-image dataset -
// a superset of what the catalog seed approved (ubuntu-24.04 cloudimg on
// local/node-01) so the admin demo has debian-12-generic-cloudimg to
// discover and approve, rocky-9 as the unapproved target.
func (fake Fake) ListCloudImages(_ context.Context) ([]CloudImage, error) {
	if fake.unavailable() {
		return nil, ErrUnreachable
	}
	return slices.Clone(fakeCloudImages), nil
}

// ListTemplates implements Client. Returns the fake template dataset - two
// template VMs the admin demo can discover and approve.
func (fake Fake) ListTemplates(_ context.Context) ([]TemplateVM, error) {
	if fake.unavailable() {
		return nil, ErrUnreachable
	}
	return slices.Clone(fakeTemplates), nil
}

// TemplateByVMID implements Client: a single-template lookup over the fake
// dataset. Unknown VMIDs are ErrNotFound; fake templates are always readable.
func (fake Fake) TemplateByVMID(_ context.Context, vmid int) (TemplateVM, error) {
	if fake.unavailable() {
		return TemplateVM{}, ErrUnreachable
	}

	for _, tmpl := range fakeTemplates {
		if tmpl.VMID == vmid {
			return tmpl, nil
		}
	}

	return TemplateVM{}, ErrNotFound
}

// StorageFreeSpace returns the available bytes on a storage backend on a node.
// The fake computes avail = Total - Used from the static
// storage dataset. Returns ErrNotFound for an unknown (node, storage) pair.
func (fake Fake) StorageFreeSpace(_ context.Context, node, storage string) (int64, error) {
	if fake.unavailable() {
		return 0, ErrUnreachable
	}

	for _, s := range fakeStorages {
		if s.Node == node && s.Name == storage {
			return s.Total - s.Used, nil
		}
	}

	return 0, ErrNotFound
}

// ListPools implements Client and returns a defensive copy of the live pool table.
func (fake Fake) ListPools(_ context.Context) ([]Pool, error) {
	state := fake.stateOrDefault()
	if fake.unavailable() {
		return nil, ErrUnreachable
	}
	state.vmMu.RLock()
	defer state.vmMu.RUnlock()

	return slices.Clone(state.pools), nil
}

// EnsurePoolRole creates the shared PVMSSUser role once and never rewrites it.
func (fake Fake) EnsurePoolRole(_ context.Context) error {
	state := fake.stateOrDefault()
	if fake.unavailable() {
		return ErrUnreachable
	}
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	if state.roleState == nil {
		state.roleState = make(map[string][]string)
	}
	if _, exists := state.roleState[poolRoleName]; exists {
		return nil
	}

	privileges := slices.Clone(rolePrivileges)
	state.roleState[poolRoleName] = privileges
	state.roleCallLog = append(state.roleCallLog, RoleCall{Privileges: slices.Clone(privileges), At: time.Now().UTC()})
	state.record(FakeCall{Action: "ensure_role", Name: poolRoleName})

	return nil
}

// EnsurePoolUser creates the pool login once and returns its PVE username.
func (fake Fake) EnsurePoolUser(_ context.Context, pool, password string) (string, error) {
	state := fake.stateOrDefault()
	if fake.unavailable() {
		return "", ErrUnreachable
	}
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	username := pool + "@pve"
	state.identMu.Lock()
	if _, exists := state.identities[username]; !exists {
		state.identities[username] = fakeIdentity{password: password, pool: pool}
	}
	state.identMu.Unlock()
	state.record(FakeCall{Action: "ensure_user", Name: username})

	return username, nil
}

// CreatePool inserts a pool only when its name is absent.
func (fake Fake) CreatePool(_ context.Context, poolID, comment string) error {
	state := fake.stateOrDefault()
	if fake.unavailable() {
		return ErrUnreachable
	}
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	for _, pool := range state.pools {
		if pool.Name == poolID {
			return nil
		}
	}

	state.pools = append(state.pools, Pool{Name: poolID, Comment: comment})
	state.record(FakeCall{Action: "create_pool", Name: poolID})

	return nil
}

// SetPoolACL records a pool-to-role binding.
func (fake Fake) SetPoolACL(_ context.Context, username, poolID, role string) error {
	state := fake.stateOrDefault()
	if fake.unavailable() {
		return ErrUnreachable
	}
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	state.acls = append(state.acls, ACLEntry{Username: username, PoolID: poolID, Role: role})
	state.record(FakeCall{Action: "set_acl", Name: username})

	return nil
}

// DeletePool removes a pool and its ACL entries. It is idempotent for cleanup.
func (fake Fake) DeletePool(_ context.Context, poolID string) error {
	state := fake.stateOrDefault()
	if fake.unavailable() {
		return ErrUnreachable
	}
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	index := slices.IndexFunc(state.pools, func(pool Pool) bool { return pool.Name == poolID })
	if index < 0 {
		return ErrNotFound
	}
	state.pools = slices.Delete(state.pools, index, index+1)
	state.acls = slices.DeleteFunc(state.acls, func(acl ACLEntry) bool { return acl.PoolID == poolID })
	state.record(FakeCall{Action: "delete_pool", Name: poolID})

	return nil
}

// DeleteUser removes a PVE identity. Tests can force a best-effort failure.
func (fake Fake) DeleteUser(_ context.Context, username string) error {
	state := fake.stateOrDefault()
	if fake.unavailable() {
		return ErrUnreachable
	}
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	if state.errDeleteUser != nil {
		return state.errDeleteUser
	}
	state.identMu.Lock()
	delete(state.identities, username)
	state.identMu.Unlock()
	state.record(FakeCall{Action: "delete_user", Name: username})

	return nil
}

// FakeRoleCalls returns a defensive copy of the default fake's role creation log.
func FakeRoleCalls() []RoleCall {
	return defaultState().roleCalls()
}

// SetFakeDeleteUserError configures a deterministic user deletion failure on
// the default fake used by zero-value Fake{}.
func SetFakeDeleteUserError(err error) {
	state := defaultState()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()
	state.errDeleteUser = err
}

// EnsureCloudInitDrive implements Writer and records drive assurance.
func (fake Fake) EnsureCloudInitDrive(_ context.Context, node string, vmid int) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	if state.findVM(node, vmid) < 0 {
		return ErrNotFound
	}

	state.cloudInitDrives[fakeCloudInitKey{node: node, vmid: vmid}] = true
	state.record(FakeCall{Node: node, VMID: vmid, Action: "ensure_cloudinit_drive"})

	return nil
}

// SetCloudInitConfig implements Writer and ensures a cloud-init drive first.
func (fake Fake) SetCloudInitConfig(ctx context.Context, node string, vmid int, config CloudInitConfig) error {
	state := fake.stateOrDefault()
	if err := fake.EnsureCloudInitDrive(ctx, node, vmid); err != nil {
		return err
	}

	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	state.cloudInitConfigs[fakeCloudInitKey{node: node, vmid: vmid}] = cloneCloudInitConfig(config)
	state.record(FakeCall{Node: node, VMID: vmid, Action: "set_cloudinit_config", CloudInitData: cloneCloudInitConfig(config)})

	return nil
}

// SnippetWriteAvailable implements Writer. The fake always has a write
// target so the dev stack and tests see cloud-init documents enabled.
func (fake Fake) SnippetWriteAvailable() bool {
	return true
}

// PushCloudInitSnippet implements Writer and records the server-owned target
// and content. On success it also marks the file present so HasSnippet
// answers true for the same (node, storage, filename) triple - matching the
// real client's write-then-verify contract.
func (fake Fake) PushCloudInitSnippet(_ context.Context, node, storage, filename string, vmid int, content string) error {
	state := fake.stateOrDefault()
	state.pushMu.RLock()
	err := state.pushErr
	state.pushMu.RUnlock()

	state.record(FakeCall{Node: node, VMID: vmid, Action: "push_cloudinit_snippet", Storage: storage, Filename: filename, Content: content})

	if err != nil {
		return err
	}

	state.snippetMu.Lock()
	if state.snippetPushMarksPresent {
		state.snippetPresence[fakeSnippetKey{node: node, storage: storage, filename: filename}] = true
	}
	state.snippetMu.Unlock()

	return nil
}

// AttachCloudInitSnippet implements Writer and records the cicustom attach.
// Like the real client, attaching ensures the cloud-init drive first -
// Proxmox silently ignores cicustom without one - so the fake's call log
// shows the same ensure-then-attach order the contract test asserts.
func (fake Fake) AttachCloudInitSnippet(ctx context.Context, node, storage, filename string, vmid int) error {
	if filename != "" {
		if err := fake.EnsureCloudInitDrive(ctx, node, vmid); err != nil {
			return err
		}
	}

	state := fake.stateOrDefault()
	if state.findVM(node, vmid) < 0 {
		return ErrNotFound
	}
	state.record(FakeCall{Node: node, VMID: vmid, Action: "attach_cloudinit_snippet", Storage: storage, Filename: filename})
	return nil
}

// HasSnippet implements Writer. The default answer is false - the fake
// cannot invent an admin-preplaced file - unless a test opts a
// (node, storage, filename) triple in via SetFakeSnippetPresent.
func (fake Fake) HasSnippet(_ context.Context, node, storage, filename string) (bool, error) {
	state := fake.stateOrDefault()
	state.snippetMu.RLock()
	defer state.snippetMu.RUnlock()

	return state.snippetPresence[fakeSnippetKey{node: node, storage: storage, filename: filename}], nil
}

// ReadSnippet implements Writer. Returns the content a test placed via
// SetFakeSnippetContent, or an empty string when the file is not present.
func (fake Fake) ReadSnippet(_ context.Context, node, storage, filename string) (string, error) {
	state := fake.stateOrDefault()
	state.snippetMu.RLock()
	defer state.snippetMu.RUnlock()

	key := fakeSnippetKey{node: node, storage: storage, filename: filename}

	if !state.snippetPresence[key] {
		return "", ErrNotFound
	}

	return state.snippetContent[key], nil
}

// RemoveCloudInitSnippet implements Writer and records the removal, clearing
// the presence flag so a subsequent HasSnippet returns false.
func (fake Fake) RemoveCloudInitSnippet(_ context.Context, storage, filename string) error {
	state := fake.stateOrDefault()
	state.record(FakeCall{Action: "remove_cloudinit_snippet", Storage: storage, Filename: filename})

	state.snippetMu.Lock()
	for key := range state.snippetPresence {
		if key.storage == storage && key.filename == filename {
			delete(state.snippetPresence, key)
		}
	}
	state.snippetMu.Unlock()

	return nil
}

// SetCloudInitPassword implements Writer and records the agent password apply
// with its target user. The password itself is never retained.
// Tests can inject a failure for the next N calls (SetFakeGuestPasswordError)
// to exercise the caller's retry-on-missing-account loop.
func (fake Fake) SetCloudInitPassword(_ context.Context, node string, vmid int, user, _ string) error {
	state := fake.stateOrDefault()
	if state.findVM(node, vmid) < 0 {
		return ErrNotFound
	}

	state.pingMu.Lock()
	err := state.guestPasswordErr
	if state.guestPasswordErrLeft > 0 {
		state.guestPasswordErrLeft--
		if state.guestPasswordErrLeft == 0 {
			state.guestPasswordErr = nil
		}
	}
	state.pingMu.Unlock()

	state.record(FakeCall{Node: node, VMID: vmid, Action: "set_cloudinit_password", Name: user})

	return err
}

// SetFakeGuestPasswordError makes the next n SetCloudInitPassword calls return
// err (cluster.ErrGuestUserUnknown exercises the retry path; nil clears it).
func SetFakeGuestPasswordError(err error, count int) {
	state := defaultState()
	state.pingMu.Lock()
	defer state.pingMu.Unlock()
	state.guestPasswordErr = err
	state.guestPasswordErrLeft = count
}

// PingGuestAgent implements Writer and records the probe. Tests can make the
// first N pings fail (SetFakeGuestAgentPingFailures) to exercise the caller's
// bounded wait loop.
func (fake Fake) PingGuestAgent(_ context.Context, node string, vmid int) error {
	state := fake.stateOrDefault()
	if state.findVM(node, vmid) < 0 {
		return ErrNotFound
	}

	state.pingMu.Lock()
	remaining := state.agentPingFailures
	if remaining > 0 {
		state.agentPingFailures--
	}
	state.pingMu.Unlock()

	state.record(FakeCall{Node: node, VMID: vmid, Action: "ping_guest_agent"})

	if remaining > 0 {
		return ErrUnreachable
	}

	return nil
}

// GuestNetworkInterfaces implements GuestNetworkReader. The fake has no real
// guest agent, so a running VM reports each configured NIC's stored
// IPAddresses, or - when none were seeded - a deterministic 10.10.x.y
// address so the demo shows what a real cluster's agent would report. A
// stopped or paused guest cannot answer an agent call: ErrUnreachable, the
// same answer the real endpoint gives. agentPingFailures (the
// SetFakeGuestAgentPingFailures knob) doubles as "the agent channel is down
// for the next n calls" so tests can simulate a running VM whose agent does
// not answer.
func (fake Fake) GuestNetworkInterfaces(_ context.Context, node string, vmid int) ([]GuestInterface, error) {
	state := fake.stateOrDefault()

	state.vmMu.Lock()
	idx := state.findVM(node, vmid)
	if idx < 0 {
		state.vmMu.Unlock()
		return nil, ErrNotFound
	}
	running := state.vms[idx].Status == VMRunning
	nics := cloneNetworkInterfaces(state.vms[idx].NetworkInterfaces)
	state.vmMu.Unlock()

	state.record(FakeCall{Node: node, VMID: vmid, Action: "guest_network_interfaces"})

	if !running {
		return nil, ErrUnreachable
	}

	state.pingMu.Lock()
	fail := state.agentPingFailures > 0
	if fail {
		state.agentPingFailures--
	}
	state.pingMu.Unlock()

	if fail {
		return nil, ErrUnreachable
	}

	guests := make([]GuestInterface, 0, len(nics))
	for _, nic := range nics {
		ips := nic.IPAddresses
		if len(ips) == 0 {
			ips = []string{fmt.Sprintf("10.10.%d.%d", vmid%250, 10+nic.Index)}
		}
		guests = append(guests, GuestInterface{MAC: nic.MAC, IPAddresses: ips})
	}

	return guests, nil
}

// SetFakeGuestAgentPingFailures makes the next n PingGuestAgent calls fail
// with ErrUnreachable before succeeding, so tests can exercise the bounded
// ping loop without sleeps.
func SetFakeGuestAgentPingFailures(n int) {
	state := defaultState()
	state.pingMu.Lock()
	defer state.pingMu.Unlock()
	state.agentPingFailures = n
}

// SetFakeCloudInitPushError configures the default fake's push failure used by tests.
func SetFakeCloudInitPushError(err error) {
	state := defaultState()
	state.pushMu.Lock()
	defer state.pushMu.Unlock()
	state.pushErr = err
}

// SetFakeSnippetPresent configures the default fake's HasSnippet answer for
// one (node, storage, filename) triple - tests use it to exercise the
// baseline-snippet-found branch of image-mode create.
func SetFakeSnippetPresent(node, storage, filename string, present bool) {
	state := defaultState()
	state.snippetMu.Lock()
	defer state.snippetMu.Unlock()
	state.snippetPresence[fakeSnippetKey{node: node, storage: storage, filename: filename}] = present
}

// SetFakeSnippetContent sets the content a test wants ReadSnippet to return
// for one (node, storage, filename) triple, and marks it present so HasSnippet
// also returns true. Used to exercise the cluster-wide baseline override path
func SetFakeSnippetContent(node, storage, filename, content string) {
	state := defaultState()
	state.snippetMu.Lock()
	defer state.snippetMu.Unlock()
	key := fakeSnippetKey{node: node, storage: storage, filename: filename}
	state.snippetPresence[key] = true
	state.snippetContent[key] = content
}

// SetFakeSnippetVisibility controls whether a successful PushCloudInitSnippet
// marks the file visible to HasSnippet. True (the default) is the real
// client's write-then-verify contract; false simulates a wrong mount - the
// write succeeds on the PVMSS side but Proxmox never lists the file.
func SetFakeSnippetVisibility(marksPresent bool) {
	state := defaultState()
	state.snippetMu.Lock()
	defer state.snippetMu.Unlock()
	state.snippetPushMarksPresent = marksPresent
}

// SetFakeCreateError configures the default fake's CreateVM error for the
// next count calls (tests inject cluster.ErrVMIDTaken to exercise the retry loop). count=0
// means unlimited until cleared by reset.
func SetFakeCreateError(err error, count int) {
	state := defaultState()
	state.createMu.Lock()
	defer state.createMu.Unlock()
	state.createErr = err
	state.createErrCount = count
}

// SetFakeTaskError configures the default fake so the next registered task
// reports TaskError with the given exit message on its first TaskStatus poll
// (tests inject a task error to exercise the rollback path).
func SetFakeTaskError(exitMessage string) {
	state := defaultState()
	state.createMu.Lock()
	defer state.createMu.Unlock()
	state.taskErr = exitMessage
}

// AddSSHKey implements Writer and records the agent-side key injection. It
// never merges into the fake's cloud-init config (the guest is the source of
// truth for an injected key); tests assert the call reached the fake.
func (fake Fake) AddSSHKey(_ context.Context, node string, vmid int, user, key string) error {
	state := fake.stateOrDefault()
	state.sshMu.Lock()
	err := state.sshErr
	state.sshMu.Unlock()

	if state.findVM(node, vmid) < 0 {
		return ErrNotFound
	}

	state.record(FakeCall{Node: node, VMID: vmid, Action: "add_ssh_key", Name: user, Content: key})

	if err != nil {
		return err
	}

	return nil
}

// SetFakeSSHKeyError configures a deterministic AddSSHKey failure on the
// default fake used by zero-value Fake{}.
func SetFakeSSHKeyError(err error) {
	state := defaultState()
	state.sshMu.Lock()
	defer state.sshMu.Unlock()
	state.sshErr = err
}

// Action implements Writer - a power transition on the Index-resolved node.
// It mutates the VM's Status so a subsequent Snapshot reflects it (the fake
// demonstrates the feature), and records the call.
//
// status-incompatible transitions are rejected - start on an
// already-running VM, stop/shutdown on an already-stopped one, reboot/reset on
// a stopped one. This mirrors what real Proxmox rejects natively; never
// built it because no single-VM caller needed it, but the bulk
// 2 is the first caller that does.
func (fake Fake) Action(_ context.Context, node string, vmid int, action string) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := slices.IndexFunc(state.vms, func(v VM) bool { return v.VMID == vmid && v.Node == node })
	if idx < 0 {
		return ErrNotFound
	}

	status := state.vms[idx].Status
	if err := validateTransition(action, status); err != nil {
		return err
	}

	switch action {
	case actionStart, actionReboot, actionReset, actionResume:
		state.vms[idx].Status = VMRunning
		state.vms[idx].Uptime = fakeUptimeOnStart
	case actionStop, actionShutdown:
		state.vms[idx].Status = VMStopped
		state.vms[idx].Uptime = 0
	case actionPause:
		state.vms[idx].Status = VMPaused
	default:
		return ErrInvalidAction
	}

	state.record(FakeCall{Node: node, VMID: vmid, Action: action})

	return nil
}

// VMStatus implements VMStatusReader. The fake's in-memory state already tracks
// each VM's status and uptime; the lock comes from the injectable vmLocks map
// (empty by default = unlocked, matching Proxmox's /status/current).
func (fake Fake) VMStatus(_ context.Context, node string, vmid int) (VMLiveStatus, error) {
	state := fake.stateOrDefault()
	state.vmMu.RLock()
	defer state.vmMu.RUnlock()

	idx := slices.IndexFunc(state.vms, func(v VM) bool { return v.VMID == vmid && v.Node == node })
	if idx < 0 {
		return VMLiveStatus{}, ErrNotFound
	}

	return VMLiveStatus{
		Status: state.vms[idx].Status,
		Lock:   state.vmLocks[vmid],
		Uptime: state.vms[idx].Uptime,
	}, nil
}

// SetVMLock injects a Proxmox lock name on a VM for testing retry-on-lock
// and the lock field in VMLiveStatus. An empty lockName clears it.
func (fake Fake) SetVMLock(vmid int, lockName string) {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()
	if lockName == "" {
		delete(state.vmLocks, vmid)
	} else {
		state.vmLocks[vmid] = lockName
	}
}

// validateTransition rejects a power action that makes no sense for the VM's
// current status. Real Proxmox rejects these natively; the fake mirrors that
// so the bulk scenarios produce the same per-target error entries a real
// cluster would.
func validateTransition(action string, status VMStatus) error {
	switch action {
	case actionStart:
		if status == VMRunning {
			return fmt.Errorf("%w: vm already running", ErrInvalidStateTransition)
		}
	case actionStop, actionShutdown:
		if status == VMStopped {
			return fmt.Errorf("%w: vm already stopped", ErrInvalidStateTransition)
		}
	case actionReboot, actionReset:
		if status == VMStopped {
			return fmt.Errorf("%w: vm is not running", ErrInvalidStateTransition)
		}
	case actionPause:
		if status != VMRunning {
			return fmt.Errorf("%w: vm is not running", ErrInvalidStateTransition)
		}
	case actionResume:
		if status != VMPaused {
			return fmt.Errorf("%w: vm is not paused", ErrInvalidStateTransition)
		}
	}

	return nil
}

// Delete implements Writer - the VM and its disks are removed from the
// dataset. Irreversible: no soft-delete, no undo. A running VM is
// rejected with ErrVMRunning, mirroring real Proxmox (which returns HTTP 500
// "VM X is running - destroy failed"); callers must stop it first.
func (fake Fake) Delete(_ context.Context, node string, vmid int) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := slices.IndexFunc(state.vms, func(v VM) bool { return v.VMID == vmid && v.Node == node })
	if idx < 0 {
		return ErrNotFound
	}

	if state.vms[idx].Status == VMRunning {
		return ErrVMRunning
	}

	state.vms = slices.Delete(state.vms, idx, idx+1)

	state.record(FakeCall{Node: node, VMID: vmid, Action: actionDelete})

	return nil
}

// Patch implements Writer - name and/or description update. Empty arguments
// are ignored; the caller (vm.Patch) decides which fields to send.
func (fake Fake) Patch(_ context.Context, node string, vmid int, name, description string) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := slices.IndexFunc(state.vms, func(v VM) bool { return v.VMID == vmid && v.Node == node })
	if idx < 0 {
		return ErrNotFound
	}

	if name != "" {
		state.vms[idx].Name = name
	}

	if description != "" {
		state.vms[idx].Description = description
	}

	state.record(FakeCall{Node: node, VMID: vmid, Action: "patch", Name: name})

	return nil
}

// AddDisk implements Writer and appends a disk to the requested VM.
func (fake Fake) AddDisk(_ context.Context, node string, vmid int, bus, storage string, sizeGB int) (string, error) {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return "", ErrNotFound
	}

	busIndex := nextBusIndex(state.vms[idx].Disks, DiskBus(bus))
	key := fmt.Sprintf("%s%d", bus, busIndex)
	state.vms[idx].Disks = append(state.vms[idx].Disks, Disk{Key: key, Bus: DiskBus(bus), BusIndex: busIndex, Storage: storage, SizeGB: sizeGB})
	state.vms[idx].DiskTotal += int64(sizeGB) * 1024 * 1024 * 1024
	state.record(FakeCall{Node: node, VMID: vmid, Action: "add_disk", DiskKey: key, Bus: bus, Storage: storage, SizeGB: sizeGB})

	return key, nil
}

// ResizeDisk implements Writer and grows an existing disk in the fake dataset.
func (fake Fake) ResizeDisk(_ context.Context, node string, vmid int, diskKey string, sizeGB int) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	for diskIndex := range state.vms[idx].Disks {
		if state.vms[idx].Disks[diskIndex].Key != diskKey {
			continue
		}

		previous := state.vms[idx].Disks[diskIndex].SizeGB
		state.vms[idx].Disks[diskIndex].SizeGB = sizeGB
		state.vms[idx].DiskTotal += int64(sizeGB-previous) * 1024 * 1024 * 1024
		state.record(FakeCall{Node: node, VMID: vmid, Action: "resize_disk", DiskKey: diskKey, SizeGB: sizeGB})

		return nil
	}

	return ErrNotFound
}

// DeleteDisk implements Writer and removes a disk from the fake dataset.
func (fake Fake) DeleteDisk(_ context.Context, node string, vmid int, diskKey string) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	for diskIndex, disk := range state.vms[idx].Disks {
		if disk.Key != diskKey {
			continue
		}

		state.vms[idx].Disks = slices.Delete(state.vms[idx].Disks, diskIndex, diskIndex+1)
		state.vms[idx].DiskTotal -= int64(disk.SizeGB) * 1024 * 1024 * 1024

		state.record(FakeCall{Node: node, VMID: vmid, Action: "delete_disk", DiskKey: diskKey})

		return nil
	}

	return ErrNotFound
}

// SetCDROM implements Writer and changes the fake VM's CD-ROM state.
func (fake Fake) SetCDROM(_ context.Context, node string, vmid int, cdrom CDROMState) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	state.vms[idx].CDROM = cdrom

	state.record(FakeCall{Node: node, VMID: vmid, Action: "set_cdrom"})

	return nil
}

// SetBootOrder implements Writer and replaces the fake VM's boot order.
// An empty order clears the explicit boot config (Proxmox default behavior).
func (fake Fake) SetBootOrder(_ context.Context, node string, vmid int, order []string) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	state.vms[idx].BootOrder = append([]string(nil), order...)

	state.record(FakeCall{Node: node, VMID: vmid, Action: "set_boot_order", BootOrder: append([]string(nil), order...)})

	return nil
}

// UpdateNetwork implements Writer and replaces the fake VM's network interfaces.
func (fake Fake) UpdateNetwork(_ context.Context, node string, vmid int, interfaces []NetworkInterface) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	state.vms[idx].NetworkInterfaces = cloneNetworkInterfaces(interfaces)

	state.record(FakeCall{Node: node, VMID: vmid, Action: "update_network"})

	return nil
}

// UpdateHardware implements Writer and updates the fake VM's CPU, memory, and tags.
func (fake Fake) UpdateHardware(_ context.Context, node string, vmid, sockets, cores, memoryMB int, tags []string) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	state.vms[idx].Sockets = sockets
	state.vms[idx].Cores = cores
	state.vms[idx].CPUCores = sockets * cores
	state.vms[idx].MemoryTotal = int64(memoryMB) * 1024 * 1024

	state.vms[idx].Tags = append([]string(nil), tags...)

	state.record(FakeCall{Node: node, VMID: vmid, Action: "update_hardware", Sockets: sockets, Cores: cores, MemoryMB: memoryMB})

	return nil
}

// SetTags implements Writer and updates only the fake VM's tags, leaving
// hardware untouched. Used by the clone path when no hardware override was
// requested but the mandatory pvmss tag still needs to be stamped.
func (fake Fake) SetTags(_ context.Context, node string, vmid int, tags []string) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	state.vms[idx].Tags = append([]string(nil), tags...)

	state.record(FakeCall{Node: node, VMID: vmid, Action: "set_tags"})

	return nil
}

// EnableSerial implements Writer and flips the fake VM's HasSerial flag on.
func (fake Fake) EnableSerial(_ context.Context, node string, vmid int) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	state.vms[idx].HasSerial = true

	state.record(FakeCall{Node: node, VMID: vmid, Action: "enable_serial"})

	return nil
}

// ReadFirmwareConfig returns the live firmware config of a fake VM. The fake
// stores BIOS/Machine/EFIDisk/TPMState/SecureBoot on the VM struct at create
// time, so this just reads them back.
func (fake Fake) ReadFirmwareConfig(_ context.Context, node string, vmid int) (FirmwareConfig, error) {
	state := fake.stateOrDefault()
	state.vmMu.RLock()
	defer state.vmMu.RUnlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return FirmwareConfig{}, ErrNotFound
	}

	v := state.vms[idx]
	return FirmwareConfig{
		BIOS:       v.BIOS,
		Machine:    v.Machine,
		HasEFIDisk: v.EFIDisk,
		HasTPM:     v.TPMState,
		SecureBoot: v.SecureBoot,
	}, nil
}

// SetFakeSecureBoot marks one fake VM as carrying Secure Boot (efidisk0 with
// pre-enrolled-keys=1). PVMSS never creates such a VM any more, but ones made
// before Secure Boot was dropped - or by hand in Proxmox - still exist, so
// the SeaBIOS retrofit's refusal path keeps its coverage.
func SetFakeSecureBoot(vmid int) {
	state := defaultState()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	for i := range state.vms {
		if state.vms[i].VMID == vmid {
			state.vms[i].SecureBoot = true

			return
		}
	}
}

// RetrofitToSeaBIOS removes the UEFI firmware keys from a fake VM: clears
// BIOS, Machine, EFIDisk, TPMState, and SecureBoot. The caller
// must have already refused VMs with TPM state or Secure Boot and stopped
// the VM.
func (fake Fake) RetrofitToSeaBIOS(_ context.Context, node string, vmid int) error {
	state := fake.stateOrDefault()
	state.vmMu.Lock()
	defer state.vmMu.Unlock()

	idx := state.findVM(node, vmid)
	if idx < 0 {
		return ErrNotFound
	}

	state.vms[idx].BIOS = ""
	state.vms[idx].Machine = ""
	state.vms[idx].EFIDisk = false
	state.vms[idx].TPMState = false
	state.vms[idx].SecureBoot = false

	state.record(FakeCall{Node: node, VMID: vmid, Action: "retrofit_seabios"})

	return nil
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
// times.
func FakeCalls() []FakeCall {
	return defaultState().calls()
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
