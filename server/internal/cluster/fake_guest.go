//nolint:wsl_v5 // fake state methods keep mutation and call recording adjacent
package cluster

import (
	"context"
	"fmt"
)

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
	nics := CloneNetworkInterfaces(state.vms[idx].NetworkInterfaces)
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
