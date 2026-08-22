package vm_test

import (
	"context"
	"errors"
	"pvmss/server/internal/auth"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/vm"
	"testing"
)

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

// fakeProxyFetcher returns a vm.ProxyFetcher that records the node it was
// called with (so tests can assert FR-007: node is server-resolved) and
// returns the fixed ticket/port/err triple.
func fakeProxyFetcher(ticket string, port int, err error, gotNode *string) vm.ProxyFetcher {
	return func(_ context.Context, _ string, _ int, node string) (string, int, error) {
		*gotNode = node

		return ticket, port, err
	}
}

// consoleKindCases is the table shared by every GetConsoleTicket domain test —
// each case runs the same assertions for both KindVNC and KindTerminal, proving
// the Resolve → fetch → issue → audit pipeline is identical for both paths.
var consoleKindCases = []struct {
	name   string
	kind   vm.ConsoleKind
	ticket string
	port   int
}{
	{"vnc", vm.KindVNC, "proxmox-ticket", 5901},
	{"terminal", vm.KindTerminal, "proxmox-term-ticket", 5902},
}

// TestGetConsoleTicket_ResolveThenIssueThenAudit — T012: the happy path calls
// Resolve (ownership gate), then the proxy fetcher (with the server-resolved
// node), then Issue (opaque token), then RecordAction("console_open"). The
// returned ticket carries the opaque token and the server-resolved node.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestGetConsoleTicket_ResolveThenIssueThenAudit(t *testing.T) {
	idx := buildResolveIndex(t)
	alice := auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}

	for _, tc := range consoleKindCases {
		t.Run(tc.name, func(t *testing.T) {
			store := vm.NewConsoleTicketStore()

			var gotNode string

			fetcher := fakeProxyFetcher(tc.ticket, tc.port, nil, &gotNode)
			audit := &fakeAuditRecorder{}

			ticket, err := vm.GetConsoleTicket(context.Background(), vm.ConsoleTicketDeps{Index: idx, Actor: alice, ClusterName: testClusterName, VMID: 100, Kind: tc.kind, Fetcher: fetcher, Store: store, Audit: audit})
			if err != nil {
				t.Fatalf("GetConsoleTicket: %v", err)
			}

			if ticket.Token == "" {
				t.Fatalf("returned ticket has empty token")
			}

			if ticket.Cluster != testClusterName || ticket.VMID != 100 {
				t.Fatalf("ticket bound to %+v, want default/100", ticket)
			}
			// FR-007: the node passed to the fetcher is Resolve()'s
			// server-resolved value, not a client-supplied one.
			if gotNode != cluster.FakeNode01 {
				t.Fatalf("fetcher called with node %q, want %q (server-resolved)", gotNode, cluster.FakeNode01)
			}

			if ticket.Node != cluster.FakeNode01 {
				t.Fatalf("ticket node = %q, want %q", ticket.Node, cluster.FakeNode01)
			}

			if ticket.ProxmoxTicket != tc.ticket || ticket.Port != tc.port {
				t.Fatalf("ticket carries proxmox %+v, want %s/%d", ticket, tc.ticket, tc.port)
			}

			if audit.gotAction != vm.AuditActionConsoleOpen || audit.gotVMID != 100 {
				t.Fatalf("audit recorded %+v, want console_open/100", audit)
			}
		})
	}
}

// TestGetConsoleTicket_NonOwnerForbidden — T012: Resolve() is the first gate;
// a non-owner gets ErrForbidden before the fetcher or store is touched.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestGetConsoleTicket_NonOwnerForbidden(t *testing.T) {
	idx := buildResolveIndex(t)
	bob := auth.Identity{Username: cluster.FakeUserBob, Pool: cluster.FakePoolBob}

	for _, tc := range consoleKindCases {
		t.Run(tc.name, func(t *testing.T) {
			store := vm.NewConsoleTicketStore()

			var gotNode string

			fetcher := fakeProxyFetcher("x", tc.port, nil, &gotNode)
			audit := &fakeAuditRecorder{}

			_, err := vm.GetConsoleTicket(context.Background(), vm.ConsoleTicketDeps{Index: idx, Actor: bob, ClusterName: testClusterName, VMID: 100, Kind: tc.kind, Fetcher: fetcher, Store: store, Audit: audit})
			if !errors.Is(err, vm.ErrForbidden) {
				t.Fatalf("err = %v, want ErrForbidden", err)
			}

			if gotNode != "" {
				t.Fatalf("fetcher was called despite Resolve failure")
			}

			if audit.gotAction != "" {
				t.Fatalf("audit was recorded despite Resolve failure")
			}
		})
	}
}

// TestGetConsoleTicket_ClusterClientErrorPropagates — T012: if the fetcher
// fails, the error propagates as ErrClusterConsoleUnavailable and no ticket is
// issued.
//
//nolint:paralleltest // serial: shared fake VM fixture
func TestGetConsoleTicket_ClusterClientErrorPropagates(t *testing.T) {
	idx := buildResolveIndex(t)
	alice := auth.Identity{Username: cluster.FakeUserAlice, Pool: cluster.FakePoolAlice}

	for _, tc := range consoleKindCases {
		t.Run(tc.name, func(t *testing.T) {
			store := vm.NewConsoleTicketStore()

			var gotNode string

			fetcher := fakeProxyFetcher("", 0, errors.New("proxmox unreachable"), &gotNode)
			audit := &fakeAuditRecorder{}

			_, err := vm.GetConsoleTicket(context.Background(), vm.ConsoleTicketDeps{Index: idx, Actor: alice, ClusterName: testClusterName, VMID: 100, Kind: tc.kind, Fetcher: fetcher, Store: store, Audit: audit})
			if err == nil {
				t.Fatalf("expected error, got nil")
			}

			if !errors.Is(err, vm.ErrClusterConsoleUnavailable) {
				t.Fatalf("err = %v, want ErrClusterConsoleUnavailable", err)
			}

			if audit.gotAction != "" {
				t.Fatalf("audit was recorded despite cluster client failure")
			}
		})
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

	for _, tc := range consoleKindCases {
		t.Run(tc.name, func(t *testing.T) {
			store := vm.NewConsoleTicketStore()

			var gotNode string

			fetcher := fakeProxyFetcher(tc.ticket, tc.port, nil, &gotNode)
			audit := &fakeAuditRecorder{}

			// VM 103 is in pool-bob, not pool-alice — an admin can still open it.
			ticket, err := vm.GetConsoleTicket(context.Background(), vm.ConsoleTicketDeps{Index: idx, Actor: admin, ClusterName: testClusterName, VMID: 103, Kind: tc.kind, Fetcher: fetcher, Store: store, Audit: audit})
			if err != nil {
				t.Fatalf("admin GetConsoleTicket: %v", err)
			}

			if ticket.Token == "" {
				t.Fatalf("admin ticket has empty token")
			}

			if audit.gotAction != vm.AuditActionConsoleOpen {
				t.Fatalf("audit recorded %+v, want console_open", audit)
			}
		})
	}
}
