package vm

import (
	"errors"
	"testing"
	"time"
)

// consoleKindCase is one row of consoleKindCases.
type consoleKindCase struct {
	name   string
	kind   ConsoleKind
	ticket string
	port   int
}

// consoleKindCases is the table shared by every store test below - each case
// runs the same assertions for both KindVNC and KindTerminal, proving the
// single-use, TTL, and (kind, cluster, vmid)-binding invariants hold
// identically for both console paths.
var consoleKindCases = []consoleKindCase{
	{"vnc", KindVNC, "proxmox-ticket", 5901},
	{"terminal", KindTerminal, "proxmox-term-ticket", 5902},
}

// issueTestTicket issues a ticket bound to the shared fixture values
// (testConsoleUser / "default" / "pve-node-01").
func issueTestTicket(s *ConsoleTicketStore, kind ConsoleKind, vmid int, proxmoxTicket string, port int, origin string) ConsoleTicket {
	return s.Issue(ConsoleTicket{
		Kind: kind, Username: testConsoleUser, Cluster: "default", VMID: vmid,
		Node: "pve-node-01", ProxmoxTicket: proxmoxTicket, Port: port, Origin: origin,
	})
}

// TestConsoleTicketStore_IssueThenConsume_Succeeds - a freshly issued
// ticket is consumable exactly once for the (cluster, vmid) it was bound to.
func TestConsoleTicketStore_IssueThenConsume_Succeeds(t *testing.T) {
	t.Parallel()

	for _, tc := range consoleKindCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			assertTicketIssuedAndConsumed(t, tc)
		})
	}
}

func assertTicketIssuedAndConsumed(t *testing.T, tc consoleKindCase) {
	t.Helper()

	store := NewConsoleTicketStore()

	ticket := issueTestTicket(store, tc.kind, 101, tc.ticket, tc.port, testConsoleOrigin)

	if ticket.Token == "" {
		t.Fatalf("Issue returned an empty token")
	}

	if ticket.Cluster != "default" || ticket.VMID != 101 {
		t.Fatalf("ticket bound to %+v, want default/101", ticket)
	}

	if ticket.Node != "pve-node-01" || ticket.ProxmoxTicket != tc.ticket || ticket.Port != tc.port {
		t.Fatalf("ticket carries %+v, want node/ticket/port preserved", ticket)
	}

	if ticket.ExpiresAt.IsZero() {
		t.Fatalf("ExpiresAt is zero, want a real timestamp")
	}

	consumed, err := store.Consume(tc.kind, ticket.Token, testConsoleUser, "default", 101, testConsoleOrigin)
	if err != nil {
		t.Fatalf("Consume: %v", err)
	}

	if consumed.Token != ticket.Token {
		t.Fatalf("consumed token = %q, want %q", consumed.Token, ticket.Token)
	}
}

// TestConsoleTicketStore_ConsumeTwice_FailsOnSecondCall - a ticket is
// single-use; the second Consume for the same token is rejected.
func TestConsoleTicketStore_ConsumeTwice_FailsOnSecondCall(t *testing.T) {
	t.Parallel()

	for _, tc := range consoleKindCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := NewConsoleTicketStore()

			ticket := issueTestTicket(store, tc.kind, 101, tc.ticket, tc.port, testConsoleOrigin)

			if _, err := store.Consume(tc.kind, ticket.Token, testConsoleUser, "default", 101, testConsoleOrigin); err != nil {
				t.Fatalf("first Consume: %v", err)
			}

			if _, err := store.Consume(tc.kind, ticket.Token, testConsoleUser, "default", 101, testConsoleOrigin); !errors.Is(err, ErrInvalidTicket) {
				t.Fatalf("second Consume err = %v, want ErrInvalidTicket", err)
			}
		})
	}
}

// TestConsoleTicketStore_ConsumeAfterExpiry_Fails - a ticket whose TTL has
// elapsed is rejected the same way an already-consumed one is.
func TestConsoleTicketStore_ConsumeAfterExpiry_Fails(t *testing.T) {
	t.Parallel()

	for _, tc := range consoleKindCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := NewConsoleTicketStore()

			ticket := issueTestTicket(store, tc.kind, 101, tc.ticket, tc.port, testConsoleOrigin)
			// Force the ticket into the past by rewriting ExpiresAt through the store
			// map. This is a white-box hook only this test uses.
			store.expireForTest(ticket.Token)

			if _, err := store.Consume(tc.kind, ticket.Token, testConsoleUser, "default", 101, testConsoleOrigin); !errors.Is(err, ErrInvalidTicket) {
				t.Fatalf("expired Consume err = %v, want ErrInvalidTicket", err)
			}
		})
	}
}

// TestConsoleTicketStore_ConsumeWithWrongClusterOrVMID_Fails - defense
// in depth: a ticket bound to (default, 101) is rejected against (default, 202)
// or (other, 101).
func TestConsoleTicketStore_ConsumeWithWrongClusterOrVMID_Fails(t *testing.T) {
	t.Parallel()

	for _, tc := range consoleKindCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			store := NewConsoleTicketStore()

			ticket := issueTestTicket(store, tc.kind, 101, tc.ticket, tc.port, testConsoleOrigin)

			if _, err := store.Consume(tc.kind, ticket.Token, testConsoleUser, "default", 202, testConsoleOrigin); !errors.Is(err, ErrInvalidTicket) {
				t.Fatalf("wrong vmid Consume err = %v, want ErrInvalidTicket", err)
			}
			// The ticket is NOT consumed by a mismatched attempt - it remains usable
			// for its real (cluster, vmid).
			if _, err := store.Consume(tc.kind, ticket.Token, testConsoleUser, "default", 101, testConsoleOrigin); err != nil {
				t.Fatalf("after wrong-vmid attempt, correct Consume: %v", err)
			}

			ticket2 := issueTestTicket(store, tc.kind, 101, tc.ticket, tc.port, testConsoleOrigin)
			if _, err := store.Consume(tc.kind, ticket2.Token, testConsoleUser, "other", 101, testConsoleOrigin); !errors.Is(err, ErrInvalidTicket) {
				t.Fatalf("wrong cluster Consume err = %v, want ErrInvalidTicket", err)
			}
		})
	}
}

// TestConsoleTicketStore_ConsumeWithWrongKind_Fails - a VNC ticket cannot be
// consumed as a terminal ticket, and vice versa. The Kind field prevents
// cross-path consumption even though both kinds share the same map.
func TestConsoleTicketStore_ConsumeWithWrongKind_Fails(t *testing.T) {
	t.Parallel()

	store := NewConsoleTicketStore()

	vnc := issueTestTicket(store, KindVNC, 101, "vnc-ticket", 5901, testConsoleOrigin)

	if _, err := store.Consume(KindTerminal, vnc.Token, testConsoleUser, "default", 101, testConsoleOrigin); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("VNC ticket consumed as terminal err = %v, want ErrInvalidTicket", err)
	}

	// The VNC ticket is NOT consumed by the wrong-kind attempt.
	if _, err := store.Consume(KindVNC, vnc.Token, testConsoleUser, "default", 101, testConsoleOrigin); err != nil {
		t.Fatalf("VNC ticket was consumed by wrong-kind attempt: %v", err)
	}
}

// TestConsoleTicketStore_EvictsOldestWhenFull - capacity is 256; the 257th
// Issue evicts the oldest entry.
func TestConsoleTicketStore_EvictsOldestWhenFull(t *testing.T) {
	t.Parallel()

	store := NewConsoleTicketStore()

	var first ConsoleTicket

	for i := range ticketStoreCapacity {
		ticket := issueTestTicket(store, KindVNC, 100+i, "ticket", 5901, testConsoleOrigin)
		if i == 0 {
			first = ticket
		}
	}

	// The first ticket is still consumable - capacity is an upper bound, not
	// a pre-eviction trigger.
	if _, err := store.Consume(KindVNC, first.Token, testConsoleUser, "default", 100, testConsoleOrigin); err != nil {
		t.Fatalf("first ticket before overflow: %v", err)
	}

	// Re-issue the consumed slot so the store is exactly full again, then
	// issue one more - that overflow evicts the oldest remaining ticket.
	issueTestTicket(store, KindVNC, 100, "ticket", 5901, testConsoleOrigin)

	oldest := store.oldestTokenForTest()
	if oldest == "" {
		t.Fatalf("expected at least one ticket in the store before overflow")
	}

	issueTestTicket(store, KindVNC, 999, "ticket", 5901, testConsoleOrigin)

	if _, err := store.Consume(KindVNC, oldest, testConsoleUser, "default", 101, testConsoleOrigin); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("oldest ticket after overflow err = %v, want ErrInvalidTicket (evicted)", err)
	}
}

// TestConsoleTicketStore_TTLIsThirtySeconds - the TTL is a hardcoded 30s
// constant, not a configuration field.
func TestConsoleTicketStore_TTLIsThirtySeconds(t *testing.T) {
	t.Parallel()

	store := NewConsoleTicketStore()

	before := time.Now()
	ticket := issueTestTicket(store, KindVNC, 101, "proxmox-ticket", 5901, testConsoleOrigin)
	after := time.Now()

	want := before.Add(TicketTTL)
	if ticket.ExpiresAt.Before(want) || ticket.ExpiresAt.After(after.Add(TicketTTL)) {
		t.Fatalf("ExpiresAt = %v, want ~%v (TTL=%v)", ticket.ExpiresAt, want, TicketTTL)
	}
}

const testConsoleUser = "alice@pve"

// A console token is a bearer secret in the URL: a leaked link must not
// open the console for another signed-in user.
func TestConsoleTicketStore_OtherUserCannotConsume(t *testing.T) {
	t.Parallel()

	store := NewConsoleTicketStore()
	ticket := issueTestTicket(store, KindVNC, 101, "vnc-ticket", 5901, testConsoleOrigin)

	if _, err := store.Consume(KindVNC, ticket.Token, "bob@pve", "default", 101, testConsoleOrigin); !errors.Is(err, ErrInvalidTicket) {
		t.Fatalf("other user consume: err = %v, want ErrInvalidTicket", err)
	}

	if _, err := store.Consume(KindVNC, ticket.Token, testConsoleUser, "default", 101, testConsoleOrigin); err != nil {
		t.Fatalf("issuer consume after a rejected attempt: %v", err)
	}
}

const testConsoleOrigin = "https://pvmss.example.com"

// The WebSocket must come from the page that requested the ticket. Comparing
// Origin to the ticket's Origin (not to Host) keeps working behind proxies
// that rewrite Host, and a mismatch must not burn the ticket.
func TestConsoleTicketStore_OriginBinding(t *testing.T) {
	t.Parallel()

	store := NewConsoleTicketStore()
	ticket := issueTestTicket(store, KindVNC, 101, "vnc-ticket", 5901, testConsoleOrigin)

	for _, origin := range []string{"", "https://evil.example.net", "https://pvmss.example.com:8443", "http://pvmss.example.com"} {
		if _, err := store.Consume(KindVNC, ticket.Token, testConsoleUser, "default", 101, origin); !errors.Is(err, ErrConsoleOriginMismatch) {
			t.Fatalf("consume from origin %q: err = %v, want ErrConsoleOriginMismatch", origin, err)
		}
	}

	if _, err := store.Consume(KindVNC, ticket.Token, testConsoleUser, "default", 101, "HTTPS://PVMSS.example.com"); err != nil {
		t.Fatalf("consume from the issuing origin (case-insensitive): %v", err)
	}
}

// A ticket requested without an Origin can never be consumed.
func TestConsoleTicketStore_EmptyIssuingOriginNeverMatches(t *testing.T) {
	t.Parallel()

	store := NewConsoleTicketStore()
	ticket := issueTestTicket(store, KindVNC, 101, "vnc-ticket", 5901, "")

	if _, err := store.Consume(KindVNC, ticket.Token, testConsoleUser, "default", 101, ""); !errors.Is(err, ErrConsoleOriginMismatch) {
		t.Fatalf("empty origins: err = %v, want ErrConsoleOriginMismatch", err)
	}
}
