package vm

import (
	"context"
	"errors"
	"fmt"
	"pvmss/server/internal/cluster"
	"testing"
	"time"
)

// withAgentPingTiming temporarily shortens the password wait constants so the
// bounded loop runs in milliseconds. Restores the originals on cleanup. These
// tests are serial because they mutate package-level vars.
func withAgentPingTiming(t *testing.T, poll, wait time.Duration) {
	t.Helper()

	prevPoll, prevWait := agentPingPoll, maxAgentPingWait
	agentPingPoll = poll
	maxAgentPingWait = wait

	t.Cleanup(func() {
		agentPingPoll = prevPoll
		maxAgentPingWait = prevWait
	})
}

// agentWriterStub embeds the fake and overrides only the two agent primitives
// applyCloudInitPassword uses, so the bounded loop is exercised without sleeps.
type agentWriterStub struct {
	cluster.Fake
	pingFailures  int
	passwordCalls int
	unknownFirst  int
}

func (w *agentWriterStub) PingGuestAgent(context.Context, string, int) error {
	if w.pingFailures > 0 {
		w.pingFailures--

		return cluster.ErrUnreachable
	}

	return nil
}

func (w *agentWriterStub) SetCloudInitPassword(context.Context, string, int, string, string) error {
	w.passwordCalls++
	if w.passwordCalls <= w.unknownFirst {
		return fmt.Errorf("agent error: user 'debian' does not exist: %w", cluster.ErrGuestUserUnknown)
	}

	return nil
}

// TestApplyCloudInitPassword_PingsUntilReachable verifies the bounded wait:
// the agent is unreachable for the first two probes, then the password is
// applied — with a request count bounded by the window, not a busy loop.
//
//nolint:paralleltest // serial: mutates package-level ping timing vars
func TestApplyCloudInitPassword_PingsUntilReachable(t *testing.T) {
	withAgentPingTiming(t, 5*time.Millisecond, 500*time.Millisecond)

	writer := &agentWriterStub{pingFailures: 2}
	entity := Entity{Node: cluster.FakeNode01, VMID: 101}

	if err := applyCloudInitPassword(context.Background(), writer, entity, "debian", "pw"); err != nil {
		t.Fatalf("applyCloudInitPassword: %v", err)
	}

	if writer.passwordCalls != 1 {
		t.Errorf("password calls = %d, want 1 (applied once the agent answers)", writer.passwordCalls)
	}
}

// TestApplyCloudInitPassword_RetriesWhileAccountMissing verifies option (b)
// of ticket 05: a "user does not exist" rejection is retried within the same
// bounded window until cloud-init has created the account.
//
//nolint:paralleltest // serial: mutates package-level ping timing vars
func TestApplyCloudInitPassword_RetriesWhileAccountMissing(t *testing.T) {
	withAgentPingTiming(t, 5*time.Millisecond, 500*time.Millisecond)

	writer := &agentWriterStub{unknownFirst: 3}
	entity := Entity{Node: cluster.FakeNode01, VMID: 101}

	if err := applyCloudInitPassword(context.Background(), writer, entity, "debian", "pw"); err != nil {
		t.Fatalf("applyCloudInitPassword: %v", err)
	}

	if writer.passwordCalls != 4 {
		t.Errorf("password calls = %d, want 4 (three user-unknown retries, then success)", writer.passwordCalls)
	}
}

// TestApplyCloudInitPassword_AgentNeverReachable verifies the deadline error
// names the agent and the ping count stays bounded by the window.
//
//nolint:paralleltest // serial: mutates package-level ping timing vars
func TestApplyCloudInitPassword_AgentNeverReachable(t *testing.T) {
	withAgentPingTiming(t, 5*time.Millisecond, 60*time.Millisecond)

	writer := &agentWriterStub{pingFailures: 1 << 30}
	entity := Entity{Node: cluster.FakeNode01, VMID: 101}

	err := applyCloudInitPassword(context.Background(), writer, entity, "debian", "pw")
	if !errors.Is(err, ErrGuestAgentUnreachable) {
		t.Fatalf("err = %v, want ErrGuestAgentUnreachable", err)
	}

	if writer.passwordCalls != 0 {
		t.Errorf("password calls = %d, want 0 (never reached the agent)", writer.passwordCalls)
	}
}

// TestApplyCloudInitPassword_AccountNeverAppears verifies the deadline error
// names the missing account when the agent answers but cloud-init never
// creates the user inside the window.
//
//nolint:paralleltest // serial: mutates package-level ping timing vars
func TestApplyCloudInitPassword_AccountNeverAppears(t *testing.T) {
	withAgentPingTiming(t, 5*time.Millisecond, 60*time.Millisecond)

	writer := &agentWriterStub{unknownFirst: 1 << 30}
	entity := Entity{Node: cluster.FakeNode01, VMID: 101}

	err := applyCloudInitPassword(context.Background(), writer, entity, "debian", "pw")
	if err == nil || !errors.Is(err, cluster.ErrGuestUserUnknown) {
		t.Fatalf("err = %v, want the wrapped ErrGuestUserUnknown", err)
	}

	if writer.passwordCalls < 2 {
		t.Errorf("password calls = %d, want several retries before the deadline", writer.passwordCalls)
	}
}
