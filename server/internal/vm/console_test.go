package vm

import (
	"errors"
	"testing"
	"time"
)

// TestConsoleTicketStore_IssueThenConsume_Succeeds — T004: a freshly issued
// ticket is consumable exactly once for the (cluster, vmid) it was bound to.
func TestConsoleTicketStore_IssueThenConsume_Succeeds(t *testing.T) {
	t.Parallel()

	store := NewConsoleTicketStore()

	ticket := store.Issue("default", 101, "pve-node-01", "proxmox-ticket", 5901)

	if ticket.Token == "" {
		t.Fatalf("Issue returned an empty token")
	}

	if ticket.Cluster != "default" || ticket.VMID != 101 {
		t.Fatalf("ticket bound to %+v, want default/101", ticket)
	}

	if ticket.Node != "pve-node-01" || ticket.ProxmoxTicket != "proxmox-ticket" || ticket.Port != 5901 {
		t.Fatalf("ticket carries %+v, want node/ticket/port preserved", ticket)
	}

	if ticket.ExpiresAt.IsZero() {
		t.Fatalf("ExpiresAt is zero, want a real timestamp")
	}

	consumed, err := store.Consume(ticket.Token, "default", 101)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}

	if consumed.Token != ticket.Token {
		t.Fatalf("consumed token = %q, want %q", consumed.Token, ticket.Token)
	}
}

// TestConsoleTicketStore_ConsumeTwice_FailsOnSecondCall — FR-004: a ticket is
// single-use; the second Consume for the same token is rejected.
func TestConsoleTicketStore_ConsumeTwice_FailsOnSecondCall(t *testing.T) {
	t.Parallel()

	store := NewConsoleTicketStore()

	ticket := store.Issue("default", 101, "pve-node-01", "proxmox-ticket", 5901)

	if _, err := store.Consume(ticket.Token, "default", 101); err != nil {
		t.Fatalf("first Consume: %v", err)
	}

	if _, err := store.Consume(ticket.Token, "default", 101); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("second Consume err = %v, want ErrInvalidTicket", err)
	}
}

// TestConsoleTicketStore_ConsumeAfterExpiry_Fails — a ticket whose TTL has
// elapsed is rejected the same way an already-consumed one is.
func TestConsoleTicketStore_ConsumeAfterExpiry_Fails(t *testing.T) {
	t.Parallel()

	store := NewConsoleTicketStore()

	ticket := store.Issue("default", 101, "pve-node-01", "proxmox-ticket", 5901)
	// Force the ticket into the past by rewriting ExpiresAt through the store
	// map. This is a white-box hook only this test uses.
	store.expireForTest(ticket.Token)

	if _, err := store.Consume(ticket.Token, "default", 101); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("expired Consume err = %v, want ErrInvalidTicket", err)
	}
}

// TestConsoleTicketStore_ConsumeWithWrongClusterOrVMID_Fails — FR-002 defense
// in depth: a ticket bound to (default, 101) is rejected against (default, 202)
// or (other, 101).
func TestConsoleTicketStore_ConsumeWithWrongClusterOrVMID_Fails(t *testing.T) {
	t.Parallel()

	store := NewConsoleTicketStore()

	ticket := store.Issue("default", 101, "pve-node-01", "proxmox-ticket", 5901)

	if _, err := store.Consume(ticket.Token, "default", 202); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("wrong vmid Consume err = %v, want ErrInvalidTicket", err)
	}
	// The ticket is NOT consumed by a mismatched attempt — it remains usable
	// for its real (cluster, vmid).
	if _, err := store.Consume(ticket.Token, "default", 101); err != nil {
		t.Fatalf("after wrong-vmid attempt, correct Consume: %v", err)
	}

	ticket2 := store.Issue("default", 101, "pve-node-01", "proxmox-ticket", 5901)
	if _, err := store.Consume(ticket2.Token, "other", 101); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("wrong cluster Consume err = %v, want ErrInvalidTicket", err)
	}
}

// TestConsoleTicketStore_EvictsOldestWhenFull — capacity is 256; the 257th
// Issue evicts the oldest entry (B11's "TTL + éviction du plus ancien").
func TestConsoleTicketStore_EvictsOldestWhenFull(t *testing.T) {
	t.Parallel()

	store := NewConsoleTicketStore()

	var first VNCTicket

	for i := range ticketStoreCapacity {
		ticket := store.Issue("default", 100+i, "pve-node-01", "ticket", 5901)
		if i == 0 {
			first = ticket
		}
	}

	// The first ticket is still consumable — capacity is an upper bound, not
	// a pre-eviction trigger.
	if _, err := store.Consume(first.Token, "default", 100); err != nil {
		t.Fatalf("first ticket before overflow: %v", err)
	}

	// Re-issue the consumed slot so the store is exactly full again, then
	// issue one more — that overflow evicts the oldest remaining ticket.
	store.Issue("default", 100, "pve-node-01", "ticket", 5901)

	oldest := store.oldestTokenForTest()
	if oldest == "" {
		t.Fatalf("expected at least one ticket in the store before overflow")
	}

	store.Issue("default", 999, "pve-node-01", "ticket", 5901)

	if _, err := store.Consume(oldest, "default", 101); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("oldest ticket after overflow err = %v, want ErrInvalidTicket (evicted)", err)
	}
}

// TestConsoleTicketStore_TTLIsThirtySeconds — the TTL is a hardcoded 30s
// constant (plan.md Constraints), not a configuration field.
func TestConsoleTicketStore_TTLIsThirtySeconds(t *testing.T) {
	t.Parallel()

	store := NewConsoleTicketStore()

	before := time.Now()
	ticket := store.Issue("default", 101, "pve-node-01", "proxmox-ticket", 5901)
	after := time.Now()

	want := before.Add(TicketTTL)
	if ticket.ExpiresAt.Before(want) || ticket.ExpiresAt.After(after.Add(TicketTTL)) {
		t.Fatalf("ExpiresAt = %v, want ~%v (TTL=%v)", ticket.ExpiresAt, want, TicketTTL)
	}
}

// --- Terminal ticket store tests (serial console) — mirror the VNC store
// tests to prove the same single-use + TTL + (cluster,vmid)-binding invariants
// hold for the parallel terminal-ticket map. ---

// TestConsoleTicketStore_IssueTerminalThenConsume_Succeeds — a freshly issued
// terminal ticket is consumable exactly once for the (cluster, vmid) it was
// bound to.
func TestConsoleTicketStore_IssueTerminalThenConsume_Succeeds(t *testing.T) {
	t.Parallel()

	store := NewConsoleTicketStore()

	ticket := store.IssueTerminal("default", 101, "pve-node-01", "proxmox-term-ticket", 5902)

	if ticket.Token == "" {
		t.Fatalf("IssueTerminal returned an empty token")
	}

	if ticket.Cluster != "default" || ticket.VMID != 101 {
		t.Fatalf("ticket bound to %+v, want default/101", ticket)
	}

	if ticket.Node != "pve-node-01" || ticket.ProxmoxTicket != "proxmox-term-ticket" || ticket.Port != 5902 {
		t.Fatalf("ticket carries %+v, want node/ticket/port preserved", ticket)
	}

	consumed, err := store.ConsumeTerminal(ticket.Token, "default", 101)
	if err != nil {
		t.Fatalf("ConsumeTerminal: %v", err)
	}

	if consumed.Token != ticket.Token {
		t.Fatalf("consumed token = %q, want %q", consumed.Token, ticket.Token)
	}
}

// TestConsoleTicketStore_ConsumeTerminalTwice_FailsOnSecondCall — a terminal
// ticket is single-use; the second ConsumeTerminal is rejected.
func TestConsoleTicketStore_ConsumeTerminalTwice_FailsOnSecondCall(t *testing.T) {
	t.Parallel()

	store := NewConsoleTicketStore()

	ticket := store.IssueTerminal("default", 101, "pve-node-01", "proxmox-term-ticket", 5902)

	if _, err := store.ConsumeTerminal(ticket.Token, "default", 101); err != nil {
		t.Fatalf("first ConsumeTerminal: %v", err)
	}

	if _, err := store.ConsumeTerminal(ticket.Token, "default", 101); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("second ConsumeTerminal err = %v, want ErrInvalidTicket", err)
	}
}

// TestConsoleTicketStore_ConsumeTerminalAfterExpiry_Fails — a terminal ticket
// whose TTL has elapsed is rejected.
func TestConsoleTicketStore_ConsumeTerminalAfterExpiry_Fails(t *testing.T) {
	t.Parallel()

	store := NewConsoleTicketStore()

	ticket := store.IssueTerminal("default", 101, "pve-node-01", "proxmox-term-ticket", 5902)
	store.expireTerminalForTest(ticket.Token)

	if _, err := store.ConsumeTerminal(ticket.Token, "default", 101); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("expired ConsumeTerminal err = %v, want ErrInvalidTicket", err)
	}
}

// TestConsoleTicketStore_ConsumeTerminalWithWrongClusterOrVMID_Fails — a
// terminal ticket bound to (default, 101) is rejected against (default, 202)
// or (other, 101), and the mismatched attempt does NOT consume it.
func TestConsoleTicketStore_ConsumeTerminalWithWrongClusterOrVMID_Fails(t *testing.T) {
	t.Parallel()

	store := NewConsoleTicketStore()

	ticket := store.IssueTerminal("default", 101, "pve-node-01", "proxmox-term-ticket", 5902)

	if _, err := store.ConsumeTerminal(ticket.Token, "default", 202); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("wrong vmid ConsumeTerminal err = %v, want ErrInvalidTicket", err)
	}

	if _, err := store.ConsumeTerminal(ticket.Token, "default", 101); err != nil {
		t.Fatalf("after wrong-vmid attempt, correct ConsumeTerminal: %v", err)
	}

	ticket2 := store.IssueTerminal("default", 101, "pve-node-01", "proxmox-term-ticket", 5902)
	if _, err := store.ConsumeTerminal(ticket2.Token, "other", 101); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("wrong cluster ConsumeTerminal err = %v, want ErrInvalidTicket", err)
	}
}

// TestConsoleTicketStore_TerminalAndVNCTicketsAreIndependent — issuing and
// consuming a terminal ticket does not affect VNC tickets in the same store,
// and vice versa. Proves the two maps are genuinely parallel.
func TestConsoleTicketStore_TerminalAndVNCTicketsAreIndependent(t *testing.T) {
	t.Parallel()

	store := NewConsoleTicketStore()

	vnc := store.Issue("default", 101, "pve-node-01", "vnc-ticket", 5901)
	term := store.IssueTerminal("default", 101, "pve-node-01", "term-ticket", 5902)

	// Consuming the terminal ticket must not touch the VNC ticket.
	if _, err := store.ConsumeTerminal(term.Token, "default", 101); err != nil {
		t.Fatalf("ConsumeTerminal: %v", err)
	}

	if _, err := store.Consume(vnc.Token, "default", 101); err != nil {
		t.Fatalf("VNC ticket was affected by terminal consume: %v", err)
	}

	// The terminal token must now be unusable, while a fresh VNC ticket still
	// works.
	if _, err := store.ConsumeTerminal(term.Token, "default", 101); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("re-consume terminal err = %v, want ErrInvalidTicket", err)
	}
}
