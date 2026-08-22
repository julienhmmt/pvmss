package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
	"testing"
)

// fakeTerminalClient is a minimal TerminalClient for GetTerminalTicket tests —
// it returns a fixed TermProxyTicket and records the node it was called with,
// mirroring fakeConsoleClient so the test can assert FR-007 (node is
// server-resolved, never client-supplied).
type fakeTerminalClient struct {
	ticket  cluster.TermProxyTicket
	err     error
	gotNode string
}

func (f *fakeTerminalClient) GetTermProxy(_ context.Context, _ string, _ int, node string) (cluster.TermProxyTicket, error) {
	f.gotNode = node
	return f.ticket, f.err
}

// TestGetTerminalTicket_ResolveThenIssueThenAudit — the happy path calls
// Resolve (ownership gate), then GetTermProxy (with the server-resolved node),
// then IssueTerminal (opaque token), then RecordAction("console_open"). The
// returned ticket carries the opaque token and the server-resolved node.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestGetTerminalTicket_ResolveThenIssueThenAudit(t *testing.T) {
	idx := buildResolveIndex(t)
	alice := auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}
	store := vm.NewConsoleTicketStore()
	client := &fakeTerminalClient{ticket: cluster.TermProxyTicket{Ticket: "proxmox-term-ticket", Port: 5902}}
	audit := &fakeAuditRecorder{}

	ticket, err := vm.GetTerminalTicket(context.Background(), vm.TerminalTicketDeps{Index: idx, Actor: alice, ClusterName: testClusterName, VMID: 100, Client: client, Store: store, Audit: audit})
	if err != nil {
		t.Fatalf("GetTerminalTicket: %v", err)
	}

	if ticket.Token == "" {
		t.Fatalf("returned ticket has empty token")
	}

	if ticket.Cluster != testClusterName || ticket.VMID != 100 {
		t.Fatalf("ticket bound to %+v, want default/100", ticket)
	}

	// FR-007: the node passed to GetTermProxy is Resolve()'s server-resolved
	// value, not a client-supplied one.
	if client.gotNode != cluster.FakeNode01 {
		t.Fatalf("GetTermProxy called with node %q, want %q (server-resolved)", client.gotNode, cluster.FakeNode01)
	}

	if ticket.Node != cluster.FakeNode01 {
		t.Fatalf("ticket node = %q, want %q", ticket.Node, cluster.FakeNode01)
	}

	if ticket.ProxmoxTicket != "proxmox-term-ticket" || ticket.Port != 5902 {
		t.Fatalf("ticket carries proxmox %+v, want proxmox-term-ticket/5902", ticket)
	}

	if audit.gotAction != "console_open" || audit.gotVMID != 100 {
		t.Fatalf("audit recorded %+v, want console_open/100", audit)
	}
}

// TestGetTerminalTicket_NonOwnerForbidden — Resolve() is the first gate; a
// non-owner gets ErrForbidden before the cluster client or store is touched.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestGetTerminalTicket_NonOwnerForbidden(t *testing.T) {
	idx := buildResolveIndex(t)
	bob := auth.Identity{Username: "bob@pve", Pool: cluster.FakePoolBob}
	store := vm.NewConsoleTicketStore()
	client := &fakeTerminalClient{ticket: cluster.TermProxyTicket{Ticket: "x", Port: 5902}}
	audit := &fakeAuditRecorder{}

	_, err := vm.GetTerminalTicket(context.Background(), vm.TerminalTicketDeps{Index: idx, Actor: bob, ClusterName: testClusterName, VMID: 100, Client: client, Store: store, Audit: audit})
	if !errors.Is(err, vm.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}

	if client.gotNode != "" {
		t.Fatalf("GetTermProxy was called despite Resolve failure")
	}

	if audit.gotAction != "" {
		t.Fatalf("audit was recorded despite Resolve failure")
	}
}

// TestGetTerminalTicket_ClusterClientErrorPropagates — if GetTermProxy fails,
// the error propagates as ErrClusterConsoleUnavailable and no ticket is issued.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestGetTerminalTicket_ClusterClientErrorPropagates(t *testing.T) {
	idx := buildResolveIndex(t)
	alice := auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}
	store := vm.NewConsoleTicketStore()
	client := &fakeTerminalClient{err: errors.New("proxmox unreachable")}
	audit := &fakeAuditRecorder{}

	_, err := vm.GetTerminalTicket(context.Background(), vm.TerminalTicketDeps{Index: idx, Actor: alice, ClusterName: testClusterName, VMID: 100, Client: client, Store: store, Audit: audit})
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	if !errors.Is(err, vm.ErrClusterConsoleUnavailable) {
		t.Fatalf("err = %v, want ErrClusterConsoleUnavailable", err)
	}

	if audit.gotAction != "" {
		t.Fatalf("audit was recorded despite cluster client failure")
	}
}

// TestGetTerminalTicket_AdminBypassesPoolCheck — an admin can open a serial
// terminal for any tagged VM regardless of pool ownership.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestGetTerminalTicket_AdminBypassesPoolCheck(t *testing.T) {
	idx := buildResolveIndex(t)
	admin := auth.Identity{Username: testAdminUser, IsAdmin: true}
	store := vm.NewConsoleTicketStore()
	client := &fakeTerminalClient{ticket: cluster.TermProxyTicket{Ticket: "proxmox-term-ticket", Port: 5902}}
	audit := &fakeAuditRecorder{}

	// VM 103 is in pool-bob, not pool-alice — an admin can still open it.
	ticket, err := vm.GetTerminalTicket(context.Background(), vm.TerminalTicketDeps{Index: idx, Actor: admin, ClusterName: testClusterName, VMID: 103, Client: client, Store: store, Audit: audit})
	if err != nil {
		t.Fatalf("admin GetTerminalTicket: %v", err)
	}

	if ticket.Token == "" {
		t.Fatalf("admin ticket has empty token")
	}

	if audit.gotAction != "console_open" {
		t.Fatalf("audit recorded %+v, want console_open", audit)
	}
}
