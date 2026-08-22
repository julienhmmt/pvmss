package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strings"
	"time"

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

// serialTicketResponse is the JSON body for POST /serial-ticket. Only the
// opaque token and its TTL are sent — no Proxmox ticket, node, or port. Mirrors
// vncTicketResponse.
type serialTicketResponse struct {
	Token            string `json:"token"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
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
// Calls vm.GetTerminalTicket (Resolve → GetTermProxy → IssueTerminal → audit),
// returns only the opaque token. Mirrors handleVNCTicket.
func (h *VMSerialConsole) handleSerialTicket(w http.ResponseWriter, r *http.Request) {
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeSerialError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}

	clusterName, vmid, ok := h.parseSerialPath(r)
	if !ok {
		h.writeSerialError(w, http.StatusBadRequest, "invalid_request", "invalid VM path")
		return
	}

	index, ok := loadClusterIndex(h.resolver, clusterName, func(status int, code, message string) { h.writeSerialError(w, status, code, message) })
	if !ok {
		return
	}

	relay, err := resolveCapability(h.clients, h.relay, clusterName, "TerminalRelay")
	if err != nil {
		h.writeSerialError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}

	ticket, err := vm.GetTerminalTicket(r.Context(), vm.TerminalTicketDeps{Index: index, Actor: identity, ClusterName: clusterName, VMID: vmid, Client: relay, Store: h.tickets, Audit: h.store})
	if err != nil {
		h.writeTicketError(w, err)
		return
	}

	if ticket.Token == "" {
		h.log.Error("serial ticket has no token", "component", "httpapi", "cluster", clusterName, "vmid", vmid)
		h.writeSerialError(w, http.StatusInternalServerError, "internal_error", "failed to issue serial ticket")

		return
	}

	h.log.Info("serial ticket issued", "component", "httpapi", "cluster", clusterName, "vmid", vmid, "node", ticket.Node)
	h.writeSerialJSON(w, http.StatusOK, serialTicketResponse{Token: ticket.Token, ExpiresInSeconds: int(vm.TicketTTL.Seconds())})
}

// handleSerialWebSocket serves GET
// /api/v1/vms/:cluster/:vmid/serial/websocket?token=<opaque>. Validates the
// ticket (single-use, TTL, bound to the path's cluster+vmid), upgrades to
// WebSocket, and relays raw bytes until either side closes. Mirrors
// handleWebSocket.
func (h *VMSerialConsole) handleSerialWebSocket(w http.ResponseWriter, r *http.Request) {
	if _, err := h.auth.Principal(r); err != nil {
		h.writeSerialError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}

	clusterName, vmid, ok := h.parseSerialPath(r)
	if !ok {
		h.writeSerialError(w, http.StatusBadRequest, "invalid_request", "invalid VM path")
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		h.writeSerialError(w, http.StatusBadRequest, "invalid_request", "missing token parameter")
		return
	}

	ticket, err := h.tickets.ConsumeTerminal(token, clusterName, vmid)
	if err != nil {
		h.writeSerialError(w, http.StatusBadRequest, "invalid_ticket", "invalid or expired serial ticket")
		return
	}

	if !isConsoleOriginAllowed(r) {
		h.writeSerialError(w, http.StatusForbidden, "forbidden", "invalid origin")
		return
	}

	// Same deadline-clearing rationale as handleWebSocket: the server's global
	// WriteTimeout/ReadTimeout bound ordinary requests and survive a hijack,
	// silently killing a long-lived WebSocket. Clear both for this connection
	// only.
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})

	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		h.log.Error("serial websocket upgrade failed", "component", "httpapi", "error", err)
		return
	}
	defer func() { _ = conn.CloseNow() }()

	h.log.Info("serial websocket accepted, relaying to proxmox", "component", "httpapi", "cluster", ticket.Cluster, "vmid", ticket.VMID, "node", ticket.Node, "port", ticket.Port)

	peer := websocket.NetConn(context.Background(), conn, websocket.MessageBinary)
	defer func() { _ = peer.Close() }()

	relay, err := resolveCapability(h.clients, h.relay, ticket.Cluster, "TerminalRelay")
	if err != nil {
		h.log.Error("serial relay resolution failed", "component", "httpapi", "cluster", ticket.Cluster, "error", err)
		return
	}

	proxy := cluster.TermProxyTicket{Ticket: ticket.ProxmoxTicket, Port: ticket.Port, Node: ticket.Node}
	err = relay.RelaySerial(r.Context(), ticket.Cluster, ticket.VMID, proxy, peer)
	// Always log the outcome — a normal closure needs to be as visible as an
	// error one, otherwise "did the relay even start" is undiagnosable from
	// the logs alone (mirrors the VNC handler).
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

// writeTicketError maps vm.GetTerminalTicket errors to HTTP responses. Same
// 403/404/502 semantics as the VNC path (contracts).
func (h *VMSerialConsole) writeTicketError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeSerialError(w, http.StatusForbidden, "forbidden", "not your VM")
	case errors.Is(err, vm.ErrNotFound):
		h.writeSerialError(w, http.StatusNotFound, "not_found", "VM not found")
	case errors.Is(err, vm.ErrClusterConsoleUnavailable):
		h.writeSerialError(w, http.StatusBadGateway, "console_unavailable", "serial terminal is not available for this VM")
	default:
		h.log.Error("serial ticket issuance failed", "component", "httpapi", "error", err)
		h.writeSerialError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func (h *VMSerialConsole) writeSerialError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Error("failed to write serial error", "component", "httpapi", "code", code, "error", err)
	}
}

func (h *VMSerialConsole) writeSerialJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		h.log.Error("failed to marshal response", "component", "httpapi", "error", err)
		h.writeSerialError(w, http.StatusInternalServerError, "internal_error", "internal server error")

		return
	}

	if err := writeJSON(w, status, body); err != nil {
		h.log.Error("failed to write response", "component", "httpapi", "error", err)
	}
}
