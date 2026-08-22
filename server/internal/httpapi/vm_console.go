package httpapi

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/inventory"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"strings"

	"github.com/coder/websocket"
)

// VMConsole serves the two T10 console endpoints, both gated by the same
// vm.Resolve() every other write uses (FR-001):
//   - POST /api/v1/vms/:cluster/:vmid/vnc-ticket — issues an opaque,
//     single-use console ticket (FR-002, FR-003).
//   - GET  /api/v1/vms/:cluster/:vmid/console/websocket?token=<opaque> —
//     upgrades to WebSocket, consumes the ticket (FR-004), and relays RFB
//     frames between the browser and the cluster's VNC server (FR-008).
//
// The legacy query-string console flow (passing the Proxmox ticket, node, and
// port directly in the URL) does not exist (contracts/vm-console.md
// "Behavioural rules": the legacy flow will not exist).
type VMConsole struct {
	projection *inventory.Projection
	resolver   vm.ClusterIndexResolver
	auth       *Auth
	relay      cluster.ConsoleRelay
	clients    cluster.ClientProvider
	tickets    *vm.ConsoleTicketStore
	store      *store.Store
	log        *slog.Logger
}

// NewVMConsole creates the handler. The relay is the cluster.ConsoleRelay
// (Fake or Proxmox); the tickets store is the in-memory ConsoleTicketStore
// from main.go; the store is the real audit store. Bound to a single
// cluster; use NewVMConsoleWithRegistry for multi-cluster deployments.
func NewVMConsole(projection *inventory.Projection, authHandler *Auth, relay cluster.ConsoleRelay, tickets *vm.ConsoleTicketStore, st *store.Store, log *slog.Logger) *VMConsole {
	return &VMConsole{projection: projection, resolver: singleClusterResolver{projection: projection}, auth: authHandler, relay: relay, tickets: tickets, store: st, log: log}
}

// VMConsoleRegistryDeps groups the collaborators NewVMConsoleWithRegistry
// needs for multi-cluster wiring. Bundling them keeps the parameter count
// under go:S107.
type VMConsoleRegistryDeps struct {
	Source     inventory.LookupSource
	Projection *inventory.Projection
	Auth       *Auth
	Relay      cluster.ConsoleRelay
	Clients    cluster.ClientProvider
	Tickets    *vm.ConsoleTicketStore
	Store      *store.Store
	Log        *slog.Logger
}

// NewVMConsoleWithRegistry creates the handler with per-request index and
// cluster.ConsoleRelay resolution, keyed on the request's own :cluster path
// value — without this, a console ticket for a non-default cluster would be
// issued against the default cluster's node/port, and the relay would
// connect to the wrong Proxmox host entirely.
func NewVMConsoleWithRegistry(deps VMConsoleRegistryDeps) *VMConsole {
	handler := NewVMConsole(deps.Projection, deps.Auth, deps.Relay, deps.Tickets, deps.Store, deps.Log)
	if registry, ok := deps.Source.(*inventory.Registry); ok {
		handler.resolver = registryResolver{registry: registry}
	}

	handler.clients = deps.Clients

	return handler
}

// ServeHTTP dispatches between the ticket endpoint and the WebSocket endpoint
// based on the path suffix.
func (h *VMConsole) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if strings.HasSuffix(r.URL.Path, "/vnc-ticket") {
		h.handleVNCTicket(w, r)
		return
	}

	if strings.HasSuffix(r.URL.Path, "/console/websocket") {
		h.handleWebSocket(w, r)
		return
	}

	h.writeConsoleError(w, http.StatusNotFound, "not_found", "unknown console endpoint")
}

// handleVNCTicket serves POST /api/v1/vms/:cluster/:vmid/vnc-ticket. Calls
// vm.GetConsoleTicket (Resolve → GetVNCTicket → Issue → audit), returns only
// the opaque token.
func (h *VMConsole) handleVNCTicket(w http.ResponseWriter, r *http.Request) {
	serveConsoleTicket(w, r, h.auth, h.resolver, h.clients, h.relay, h.tickets, h.store, h.log,
		consoleTicketParams{
			kind:           vm.KindVNC,
			capabilityName: "ConsoleRelay",
			fetcher:        vncProxyFetcher(h.relay),
			invalidMsg:     "console is not available for this VM",
			noTokenMsg:     "console ticket has no token",
			issuedMsg:      "console ticket issued",
		},
		h.parseConsolePath, h.writeConsoleError,
	)
}

// handleWebSocket serves GET /api/v1/vms/:cluster/:vmid/console/websocket?token=<opaque>.
// Validates the ticket (single-use, TTL, bound to the path's cluster+vmid),
// upgrades to WebSocket, and relays RFB frames until either side closes.
//
//nolint:dupl // VNC and serial websocket handlers are intentionally parallel: different relay interfaces (ConsoleRelay vs TerminalRelay), proxy ticket types (VNCProxyTicket vs TermProxyTicket), and websocket message types (Binary vs Text). The shared setup is extracted into acceptConsoleWebSocket; the relay-specific tail cannot be unified without erasing the type safety that distinguishes the two paths.
func (h *VMConsole) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	ticket, conn, ok := acceptConsoleWebSocket(w, r, h.auth, h.tickets, h.log,
		consoleWebSocketParams{
			kind:             vm.KindVNC,
			invalidTicketMsg: "invalid or expired console ticket",
			wsUpgradeMsg:     "console websocket upgrade failed",
			acceptedMsg:      "console websocket accepted, relaying to proxmox",
		},
		h.parseConsolePath, h.writeConsoleError,
	)
	if !ok {
		return
	}
	defer func() { _ = conn.CloseNow() }()

	// VNC speaks RFB over binary frames.
	peer := websocket.NetConn(context.Background(), conn, websocket.MessageBinary)
	defer func() { _ = peer.Close() }()

	relay, err := resolveCapability(h.clients, h.relay, ticket.Cluster, "ConsoleRelay")
	if err != nil {
		h.log.Error("console relay resolution failed", "component", "httpapi", "cluster", ticket.Cluster, "error", err)
		return
	}

	proxy := cluster.VNCProxyTicket{Ticket: ticket.ProxmoxTicket, Port: ticket.Port, Node: ticket.Node}
	err = relay.RelayConsole(r.Context(), ticket.Cluster, ticket.VMID, proxy, peer)

	if err == nil || isNormalClose(err) {
		h.log.Info("console relay ended normally", "component", "httpapi", "vmid", ticket.VMID, "error", err)
	} else {
		h.log.Warn("console relay ended with error", "component", "httpapi", "vmid", ticket.VMID, "error", err)
	}
}

// parseConsolePath extracts cluster and vmid from the path values set by the
// router. Returns false if either is missing or vmid is not a positive int.
func (h *VMConsole) parseConsolePath(r *http.Request) (string, int, bool) {
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

func (h *VMConsole) writeConsoleError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Error("failed to write console error", "component", "httpapi", "code", code, "error", err)
	}
}

// isConsoleOriginAllowed validates the WebSocket Origin header against the
// request's own host. Browsers always send Origin for WebSocket handshakes,
// and a mismatch is the signal for a cross-site WebSocket hijacking attempt.
// Missing Origin is rejected — browser console connections always carry it,
// and accepting its absence would allow non-browser CSWSH attacks.
func isConsoleOriginAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}

	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}

	// Compare only the host (hostname:port) — the Origin scheme may be http(s)
	// or ws(s) depending on the client; the host comparison is what matters for
	// CSWSH prevention.
	return originURL.Host == r.Host
}

func isNormalClose(err error) bool {
	if err == nil || errors.Is(err, io.EOF) {
		return true
	}

	var ce websocket.CloseError
	if errors.As(err, &ce) {
		switch ce.Code {
		case websocket.StatusNormalClosure, websocket.StatusGoingAway:
			return true
		case websocket.StatusProtocolError, websocket.StatusUnsupportedData,
			websocket.StatusNoStatusRcvd, websocket.StatusAbnormalClosure,
			websocket.StatusInvalidFramePayloadData, websocket.StatusPolicyViolation,
			websocket.StatusMessageTooBig, websocket.StatusMandatoryExtension,
			websocket.StatusInternalError, websocket.StatusServiceRestart,
			websocket.StatusTryAgainLater, websocket.StatusBadGateway,
			websocket.StatusTLSHandshake:
			return false
		}
	}

	return false
}

// writeConsoleTicketError is the shared error-mapping used by both VMConsole
// and VMSerialConsole. The unavailableMsg differs between VNC ("console is
// not available for this VM") and serial ("serial terminal is not available
// for this VM"); every other status/code is byte-identical (contracts).
func writeConsoleTicketError(w http.ResponseWriter, log *slog.Logger, err error, writeError func(http.ResponseWriter, int, string, string), unavailableMsg string) {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		writeError(w, http.StatusForbidden, "forbidden", "not your VM")
	case errors.Is(err, vm.ErrNotFound):
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
	case errors.Is(err, vm.ErrClusterConsoleUnavailable):
		writeError(w, http.StatusBadGateway, "console_unavailable", unavailableMsg)
	default:
		log.Error("console ticket issuance failed", "component", "httpapi", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

// vncProxyFetcher adapts a cluster.ConsoleRelay to the vm.ProxyFetcher
// function type, extracting just the ticket and port from the VNCProxyTicket.
func vncProxyFetcher(relay cluster.ConsoleRelay) vm.ProxyFetcher {
	return func(ctx context.Context, clusterName string, vmid int, node string) (string, int, error) {
		proxy, err := relay.GetVNCTicket(ctx, clusterName, vmid, node)
		if err != nil {
			return "", 0, err
		}

		return proxy.Ticket, proxy.Port, nil
	}
}
