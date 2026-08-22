package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strings"

	"github.com/coder/websocket"
)

// VMSerialConsole serves the two serial-terminal endpoints, both gated by the
// same vm.Resolve() every other write uses (FR-001), mirroring VMConsole but
// for the text/serial console path:
//   - POST /api/v1/vms/:cluster/:vmid/serial-ticket — issues an opaque,
//     single-use serial terminal ticket.
//   - GET  /api/v1/vms/:cluster/:vmid/serial/websocket?token=<opaque> —
//     upgrades to WebSocket, consumes the ticket, and relays raw bytes between
//     the browser (xterm.js) and the cluster's serial terminal.
//
// The VNC flow in vm_console.go is untouched; this is a parallel handler that
// reuses the same ConsoleTicketStore (parallel terminal-ticket map), the same
// CSWSH origin check, and the same deadline-clearing logic.
type VMSerialConsole struct {
	projection *inventory.Projection
	resolver   vm.ClusterIndexResolver
	auth       *Auth
	relay      cluster.TerminalRelay
	clients    cluster.ClientProvider
	tickets    *vm.ConsoleTicketStore
	store      *store.Store
	log        *slog.Logger
}

// NewVMSerialConsole creates the handler bound to a single cluster. Mirrors
// NewVMConsole. Use NewVMSerialConsoleWithRegistry for multi-cluster.
func NewVMSerialConsole(projection *inventory.Projection, authHandler *Auth, relay cluster.TerminalRelay, tickets *vm.ConsoleTicketStore, st *store.Store, log *slog.Logger) *VMSerialConsole {
	return &VMSerialConsole{projection: projection, resolver: singleClusterResolver{projection: projection}, auth: authHandler, relay: relay, tickets: tickets, store: st, log: log}
}

// VMSerialConsoleRegistryDeps groups the collaborators
// NewVMSerialConsoleWithRegistry needs for multi-cluster wiring. Mirrors
// VMConsoleRegistryDeps.
type VMSerialConsoleRegistryDeps struct {
	Source     inventory.LookupSource
	Projection *inventory.Projection
	Auth       *Auth
	Relay      cluster.TerminalRelay
	Clients    cluster.ClientProvider
	Tickets    *vm.ConsoleTicketStore
	Store      *store.Store
	Log        *slog.Logger
}

// NewVMSerialConsoleWithRegistry creates the handler with per-request index and
// cluster.TerminalRelay resolution, keyed on the request's own :cluster path
// value. Mirrors NewVMConsoleWithRegistry.
func NewVMSerialConsoleWithRegistry(deps VMSerialConsoleRegistryDeps) *VMSerialConsole {
	handler := NewVMSerialConsole(deps.Projection, deps.Auth, deps.Relay, deps.Tickets, deps.Store, deps.Log)
	if registry, ok := deps.Source.(*inventory.Registry); ok {
		handler.resolver = registryResolver{registry: registry}
	}

	handler.clients = deps.Clients

	return handler
}

// ServeHTTP dispatches between the serial ticket endpoint and the serial
// WebSocket endpoint based on the path suffix.
func (h *VMSerialConsole) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/serial-ticket") {
		h.handleSerialTicket(w, r)
		return
	}

	if strings.HasSuffix(r.URL.Path, "/serial/websocket") {
		h.handleSerialWebSocket(w, r)
		return
	}

	h.writeSerialError(w, http.StatusNotFound, "not_found", "unknown serial console endpoint")
}

// handleSerialTicket serves POST /api/v1/vms/:cluster/:vmid/serial-ticket.
// Calls vm.GetConsoleTicket with KindTerminal (Resolve → GetTermProxy → Issue
// → audit), returns only the opaque token. Mirrors handleVNCTicket.
func (h *VMSerialConsole) handleSerialTicket(w http.ResponseWriter, r *http.Request) {
	serveConsoleTicket(w, r, h.auth, h.resolver, h.clients, h.relay, h.tickets, h.store, h.log,
		consoleTicketParams{
			kind:           vm.KindTerminal,
			capabilityName: "TerminalRelay",
			fetcher:        terminalProxyFetcher(h.relay),
			invalidMsg:     "serial terminal is not available for this VM",
			noTokenMsg:     "serial ticket has no token",
			issuedMsg:      "serial ticket issued",
		},
		h.parseSerialPath, h.writeSerialError,
	)
}

// handleSerialWebSocket serves GET
// /api/v1/vms/:cluster/:vmid/serial/websocket?token=<opaque>. Validates the
// ticket (single-use, TTL, bound to the path's cluster+vmid), upgrades to
// WebSocket, and relays raw bytes until either side closes. Mirrors
// handleWebSocket.
//
//nolint:dupl // VNC and serial websocket handlers are intentionally parallel: different relay interfaces (ConsoleRelay vs TerminalRelay), proxy ticket types (VNCProxyTicket vs TermProxyTicket), and websocket message types (Binary vs Text). The shared setup is extracted into acceptConsoleWebSocket; the relay-specific tail cannot be unified without erasing the type safety that distinguishes the two paths.
func (h *VMSerialConsole) handleSerialWebSocket(w http.ResponseWriter, r *http.Request) {
	ticket, conn, ok := acceptConsoleWebSocket(w, r, h.auth, h.tickets, h.log,
		consoleWebSocketParams{
			kind:             vm.KindTerminal,
			invalidTicketMsg: "invalid or expired serial ticket",
			wsUpgradeMsg:     "serial websocket upgrade failed",
			acceptedMsg:      "serial websocket accepted, relaying to proxmox",
		},
		h.parseSerialPath, h.writeSerialError,
	)
	if !ok {
		return
	}
	defer func() { _ = conn.CloseNow() }()

	// Serial tunnels speak TEXT frames (Proxmox's "type:payload" protocol and
	// the browser's xterm.js client both send text). VNC uses MessageBinary
	// (RFB); the serial path must use MessageText on both legs or the relay
	// fails with "unexpected frame type read (expected MessageBinary):
	// MessageText" the moment Proxmox sends its first text frame.
	peer := websocket.NetConn(context.Background(), conn, websocket.MessageText)
	defer func() { _ = peer.Close() }()

	relay, err := resolveCapability(h.clients, h.relay, ticket.Cluster, "TerminalRelay")
	if err != nil {
		h.log.Error("serial relay resolution failed", "component", "httpapi", "cluster", ticket.Cluster, "error", err)
		return
	}

	proxy := cluster.TermProxyTicket{Ticket: ticket.ProxmoxTicket, Port: ticket.Port, Node: ticket.Node}
	err = relay.RelaySerial(r.Context(), ticket.Cluster, ticket.VMID, proxy, peer)

	if err == nil || isNormalClose(err) {
		h.log.Info("serial relay ended normally", "component", "httpapi", "vmid", ticket.VMID, "error", err)
	} else {
		h.log.Warn("serial relay ended with error", "component", "httpapi", "vmid", ticket.VMID, "error", err)
	}
}

// parseSerialPath extracts cluster and vmid from the path values set by the
// router. Returns false if either is missing or vmid is not a positive int.
func (h *VMSerialConsole) parseSerialPath(r *http.Request) (string, int, bool) {
	clusterName := r.PathValue("cluster")
	if clusterName == "" {
		return "", 0, false
	}

	vmid, err := parseIntPathValue(r, "vmid")
	if err != nil {
		return "", 0, false
	}

	return clusterName, vmid, true
}

func (h *VMSerialConsole) writeSerialError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Error("failed to write serial error", "component", "httpapi", "code", code, "error", err)
	}
}

// terminalProxyFetcher adapts a cluster.TerminalRelay to the vm.ProxyFetcher
// function type, extracting just the ticket and port from the TermProxyTicket.
func terminalProxyFetcher(relay cluster.TerminalRelay) vm.ProxyFetcher {
	return func(ctx context.Context, clusterName string, vmid int, node string) (string, int, error) {
		proxy, err := relay.GetTermProxy(ctx, clusterName, vmid, node)
		if err != nil {
			return "", 0, err
		}

		return proxy.Ticket, proxy.Port, nil
	}
}
