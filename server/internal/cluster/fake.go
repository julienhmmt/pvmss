//nolint:wsl_v5 // fake state methods keep mutation and call recording adjacent
package cluster

import (
	"context"
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
	Online        bool
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
