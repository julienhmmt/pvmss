package cluster

import (
	"context"
	"fmt"
	"strings"
)

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

// SnippetStorageID implements SnippetChecker.
func (fake Fake) SnippetStorageID() string { return FakeSnippetStorage }

// CheckSnippet implements SnippetChecker: HasSnippet on every fake node.
func (fake Fake) CheckSnippet(ctx context.Context, filename string) ([]NodeSnippetStatus, error) {
	state := fake.stateOrDefault()
	state.vmMu.RLock()
	nodes := make([]string, 0, len(state.nodes))
	for _, n := range state.nodes {
		nodes = append(nodes, n.Name)
	}
	state.vmMu.RUnlock()

	results := make([]NodeSnippetStatus, 0, len(nodes))
	for _, node := range nodes {
		present, _ := fake.HasSnippet(ctx, node, FakeSnippetStorage, filename)
		results = append(results, NodeSnippetStatus{Node: node, Present: present})
	}

	return results, nil
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

// HasSnippet implements Writer. A file is present once PublishSnippet put it
// on the node, or when a test opts a (node, storage, filename) triple in via
// SetFakeSnippetPresent.
func (fake Fake) HasSnippet(_ context.Context, node, storage, filename string) (bool, error) {
	state := fake.stateOrDefault()
	state.snippetMu.RLock()
	defer state.snippetMu.RUnlock()

	if present, ok := state.snippetPresence[fakeSnippetKey{node: node, storage: storage, filename: filename}]; ok {
		return present, nil
	}

	// The fake cannot receive a pasted file: every PVMSS document on the
	// fake snippet storage is present unless a test turns that off.
	return state.snippetsPresentByDefault && storage == FakeSnippetStorage && strings.HasPrefix(filename, "pvmss-"), nil
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

// SetFakeSnippetPresent configures the default fake's HasSnippet answer for
// one (node, storage, filename) triple - tests use it to exercise the
// baseline-snippet-found branch of image-mode create.
func SetFakeSnippetPresent(node, storage, filename string, present bool) {
	state := defaultState()
	state.snippetMu.Lock()
	defer state.snippetMu.Unlock()
	state.snippetPresence[fakeSnippetKey{node: node, storage: storage, filename: filename}] = present
}

// SetFakeSnippetVisibility controls whether PVMSS documents (pvmss-*) are
// listed on every fake node by default. True (the default) is the demo
// stack; false simulates an admin who has not pasted the file yet.
func SetFakeSnippetVisibility(presentByDefault bool) {
	state := defaultState()
	state.snippetMu.Lock()
	defer state.snippetMu.Unlock()
	state.snippetsPresentByDefault = presentByDefault
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
