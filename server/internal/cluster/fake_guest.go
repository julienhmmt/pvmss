//nolint:wsl_v5 // fake state methods keep mutation and call recording adjacent
package cluster

import (
	"context"
	"fmt"
)

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

// SetFakeGuestAgentPingFailures makes the next n guest-agent calls fail
// with ErrUnreachable, so tests can simulate an agent that does not answer.
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
