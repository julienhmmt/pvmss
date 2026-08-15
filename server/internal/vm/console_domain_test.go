package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
	"testing"
)

// fakeConsoleClient is a minimal ConsoleClient for GetConsoleTicket tests — it
// returns a fixed VNCProxyTicket and records the node it was called with, so
// the test can assert FR-007 (node is server-resolved, never client-supplied).
type fakeConsoleClient struct {
	ticket  cluster.VNCProxyTicket
	err     error
	gotNode string
}

func (f *fakeConsoleClient) GetVNCTicket(_ context.Context, _ string, _ int, node string) (cluster.VNCProxyTicket, error) {
	f.gotNode = node
	return f.ticket, f.err
}

// fakeAuditRecorder records the last audit entry. Returns nil by default.
type fakeAuditRecorder struct {
	gotAction string
	gotVMID   int
	err       error
}

func (f *fakeAuditRecorder) RecordAction(_ context.Context, _, _ string, vmid int, action string) error {
	f.gotAction = action
	f.gotVMID = vmid

	return f.err
}

// TestGetConsoleTicket_ResolveThenIssueThenAudit — T012: the happy path calls
// Resolve (ownership gate), then GetVNCTicket (with the server-resolved node),
// then Issue (opaque token), then RecordAction("console_open"). The returned
// ticket carries the opaque token and the server-resolved node.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestGetConsoleTicket_ResolveThenIssueThenAudit(t *testing.T) {
	idx := buildResolveIndex(t)
	alice := auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}
	store := vm.NewConsoleTicketStore()
	client := &fakeConsoleClient{ticket: cluster.VNCProxyTicket{Ticket: "proxmox-ticket", Port: 5901}}
	audit := &fakeAuditRecorder{}

	ticket, err := vm.GetConsoleTicket(context.Background(), vm.ConsoleTicketDeps{Index: idx, Actor: alice, ClusterName: testClusterName, VMID: 100, Client: client, Store: store, Audit: audit})
	if err != nil {
		t.Fatalf("GetConsoleTicket: %v", err)
	}

	if ticket.Token == "" {
		t.Fatalf("returned ticket has empty token")
	}

	if ticket.Cluster != testClusterName || ticket.VMID != 100 {
		t.Fatalf("ticket bound to %+v, want default/100", ticket)
	}
	// FR-007: the node passed to GetVNCTicket is Resolve()'s server-resolved
	// value, not a client-supplied one.
	if client.gotNode != cluster.FakeNode01 {
		t.Fatalf("GetVNCTicket called with node %q, want %q (server-resolved)", client.gotNode, cluster.FakeNode01)
	}

	if ticket.Node != cluster.FakeNode01 {
		t.Fatalf("ticket node = %q, want %q", ticket.Node, cluster.FakeNode01)
	}

	if ticket.ProxmoxTicket != "proxmox-ticket" || ticket.Port != 5901 {
		t.Fatalf("ticket carries proxmox %+v, want proxmox-ticket/5901", ticket)
	}

	if audit.gotAction != "console_open" || audit.gotVMID != 100 {
		t.Fatalf("audit recorded %+v, want console_open/100", audit)
	}
}

// TestGetConsoleTicket_NonOwnerForbidden — T012: Resolve() is the first gate;
// a non-owner gets ErrForbidden before the cluster client or store is touched.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestGetConsoleTicket_NonOwnerForbidden(t *testing.T) {
	idx := buildResolveIndex(t)
	bob := auth.Identity{Username: "bob@pve", Pool: cluster.FakePoolBob}
	store := vm.NewConsoleTicketStore()
	client := &fakeConsoleClient{ticket: cluster.VNCProxyTicket{Ticket: "x", Port: 5901}}
	audit := &fakeAuditRecorder{}

	_, err := vm.GetConsoleTicket(context.Background(), vm.ConsoleTicketDeps{Index: idx, Actor: bob, ClusterName: testClusterName, VMID: 100, Client: client, Store: store, Audit: audit})
	if !errors.Is(err, vm.ErrForbidden) {
		t.Fatalf("err = %v, want ErrForbidden", err)
	}

	if client.gotNode != "" {
		t.Fatalf("GetVNCTicket was called despite Resolve failure")
	}

	if audit.gotAction != "" {
		t.Fatalf("audit was recorded despite Resolve failure")
	}
}

// TestGetConsoleTicket_ClusterClientErrorPropagates — T012: if GetVNCTicket
// fails, the error propagates as ErrClusterConsoleUnavailable and no ticket is
// issued.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestGetConsoleTicket_ClusterClientErrorPropagates(t *testing.T) {
	idx := buildResolveIndex(t)
	alice := auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}
	store := vm.NewConsoleTicketStore()
	client := &fakeConsoleClient{err: errors.New("proxmox unreachable")}
	audit := &fakeAuditRecorder{}

	_, err := vm.GetConsoleTicket(context.Background(), vm.ConsoleTicketDeps{Index: idx, Actor: alice, ClusterName: testClusterName, VMID: 100, Client: client, Store: store, Audit: audit})
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

// TestGetConsoleTicket_AdminBypassesPoolCheck — T011: an admin can open a
// console for any tagged VM regardless of pool ownership (same as every other
// Resolve()-gated endpoint).
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestGetConsoleTicket_AdminBypassesPoolCheck(t *testing.T) {
	idx := buildResolveIndex(t)
	admin := auth.Identity{Username: testAdminUser, IsAdmin: true}
	store := vm.NewConsoleTicketStore()
	client := &fakeConsoleClient{ticket: cluster.VNCProxyTicket{Ticket: "proxmox-ticket", Port: 5901}}
	audit := &fakeAuditRecorder{}

	// VM 103 is in pool-bob, not pool-alice — an admin can still open it.
	ticket, err := vm.GetConsoleTicket(context.Background(), vm.ConsoleTicketDeps{Index: idx, Actor: admin, ClusterName: testClusterName, VMID: 103, Client: client, Store: store, Audit: audit})
	if err != nil {
		t.Fatalf("admin GetConsoleTicket: %v", err)
	}

	if ticket.Token == "" {
		t.Fatalf("admin ticket has empty token")
	}

	if audit.gotAction != "console_open" {
		t.Fatalf("audit recorded %+v, want console_open", audit)
	}
}
