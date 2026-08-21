package httpapi

import (
	"context"
	"encoding/json"
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
	"time"

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

// NewVMConsoleWithRegistry creates the handler with per-request index and
// cluster.ConsoleRelay resolution, keyed on the request's own :cluster path
// value — without this, a console ticket for a non-default cluster would be
// issued against the default cluster's node/port, and the relay would
// connect to the wrong Proxmox host entirely.
func NewVMConsoleWithRegistry(source inventory.LookupSource, projection *inventory.Projection, authHandler *Auth, relay cluster.ConsoleRelay, clients cluster.ClientProvider, tickets *vm.ConsoleTicketStore, st *store.Store, log *slog.Logger) *VMConsole {
	handler := NewVMConsole(projection, authHandler, relay, tickets, st, log)
	if registry, ok := source.(*inventory.Registry); ok {
		handler.resolver = registryResolver{registry: registry}
	}

	handler.clients = clients

	return handler
}

// vncTicketResponse is the JSON body for POST /vnc-ticket. Only the opaque
// token and its TTL are sent — no Proxmox ticket, no node, no port (FR-002,
// FR-003). expiresInSeconds tells the client how long the token is valid
// before the WebSocket upgrade must happen (contracts/vm-console.md).
type vncTicketResponse struct {
	Token            string `json:"token"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
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
	identity, err := h.auth.Principal(r)
	if err != nil {
		h.writeConsoleError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}

	clusterName, vmid, ok := h.parseConsolePath(r)
	if !ok {
		h.writeConsoleError(w, http.StatusBadRequest, "invalid_request", "invalid VM path")
		return
	}

	index, ok := loadClusterIndex(h.resolver, clusterName, func(status int, code, message string) { h.writeConsoleError(w, status, code, message) })
	if !ok {
		return
	}

	relay, err := resolveCapability(h.clients, h.relay, clusterName, "ConsoleRelay")
	if err != nil {
		h.writeConsoleError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}

	ticket, err := vm.GetConsoleTicket(r.Context(), vm.ConsoleTicketDeps{Index: index, Actor: identity, ClusterName: clusterName, VMID: vmid, Client: relay, Store: h.tickets, Audit: h.store})
	if err != nil {
		h.writeTicketError(w, err)
		return
	}

	if ticket.Token == "" {
		h.log.Error("console ticket has no token", "component", "httpapi", "cluster", clusterName, "vmid", vmid)
		h.writeConsoleError(w, http.StatusInternalServerError, "internal_error", "failed to issue console ticket")

		return
	}

	h.log.Info("console ticket issued", "component", "httpapi", "cluster", clusterName, "vmid", vmid, "node", ticket.Node)
	h.writeJSON(w, http.StatusOK, vncTicketResponse{Token: ticket.Token, ExpiresInSeconds: int(vm.TicketTTL.Seconds())})
}

// handleWebSocket serves GET /api/v1/vms/:cluster/:vmid/console/websocket?token=<opaque>.
// Validates the ticket (single-use, TTL, bound to the path's cluster+vmid),
// upgrades to WebSocket, and relays RFB frames until either side closes.
func (h *VMConsole) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// The session must be authenticated — the ticket is the VM-level
	// capability, but the user must still have a valid session. The identity
	// itself is not used for Resolve() here (the ticket already passed
	// Resolve() at issuance time; spec edge case).
	if _, err := h.auth.Principal(r); err != nil {
		h.writeConsoleError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}

	clusterName, vmid, ok := h.parseConsolePath(r)
	if !ok {
		h.writeConsoleError(w, http.StatusBadRequest, "invalid_request", "invalid VM path")
		return
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		h.writeConsoleError(w, http.StatusBadRequest, "invalid_request", "missing token parameter")
		return
	}

	ticket, err := h.tickets.Consume(token, clusterName, vmid)
	if err != nil {
		h.writeConsoleError(w, http.StatusBadRequest, "invalid_ticket", "invalid or expired console ticket")
		return
	}

	// The ticket is the capability — Resolve() was already checked at issuance
	// time. The WebSocket handler does NOT re-check the VM's continued existence
	// (spec edge case: "the WebSocket handler re-checks the ticket itself, not
	// the VM's continued existence a second time"). If the ticket is valid and
	// unconsumed, the relay proceeds against the node it was issued for. The
	// session-level auth check above is the only gate at this layer.

	if !isConsoleOriginAllowed(r) {
		h.writeConsoleError(w, http.StatusForbidden, "forbidden", "invalid origin")
		return
	}

	// The server's global WriteTimeout/ReadTimeout (main.go) bound every
	// ordinary request's underlying connection, and that deadline survives a
	// hijack — it silently kills a long-lived WebSocket ~WriteTimeout after
	// the request started, regardless of how much data is still flowing.
	// A VNC console session is exactly the kind of long-lived connection
	// those deadlines were never meant to bound (main.go's own comment only
	// accounts for InventoryRefreshTimeout, not this route). Clear both
	// deadlines for this connection only — every other handler keeps the
	// global timeouts untouched.
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})

	// No OriginPatterns: coder/websocket's default authenticateOrigin checks
	// Origin against r.Host (its CSWSH guard). The custom isConsoleOriginAllowed
	// check above returns an explicit 403 JSON error; the library's check is
	// defense-in-depth. Never pass OriginPatterns:["*"] — the library docs
	// explicitly warn against it (it disables the CSWSH guard).
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		h.log.Error("console websocket upgrade failed", "component", "httpapi", "error", err)
		return
	}
	defer func() { _ = conn.CloseNow() }()

	h.log.Info("console websocket accepted, relaying to proxmox", "component", "httpapi", "cluster", ticket.Cluster, "vmid", ticket.VMID, "node", ticket.Node, "port", ticket.Port)

	peer := websocket.NetConn(context.Background(), conn, websocket.MessageBinary)
	defer func() { _ = peer.Close() }()

	relay, err := resolveCapability(h.clients, h.relay, ticket.Cluster, "ConsoleRelay")
	if err != nil {
		h.log.Error("console relay resolution failed", "component", "httpapi", "cluster", ticket.Cluster, "error", err)
		return
	}

	proxy := cluster.VNCProxyTicket{Ticket: ticket.ProxmoxTicket, Port: ticket.Port, Node: ticket.Node}
	err = relay.RelayConsole(r.Context(), ticket.Cluster, ticket.VMID, proxy, peer)
	// Always log the outcome — a normal closure needs to be as visible as an
	// error one, otherwise "did the relay even start" is undiagnosable from
	// the logs alone (this used to be silent on success).
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

// writeTicketError maps vm.GetConsoleTicket errors to HTTP responses.
// 403/404 are byte-identical with the other VM endpoints (contracts). A
// cluster-client failure (GetVNCTicket returned an error) is 502
// console_unavailable — the Proxmox server is unreachable or the VM is not
// running (contracts/vm-console.md).
func (h *VMConsole) writeTicketError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, vm.ErrForbidden):
		h.writeConsoleError(w, http.StatusForbidden, "forbidden", "not your VM")
	case errors.Is(err, vm.ErrNotFound):
		h.writeConsoleError(w, http.StatusNotFound, "not_found", "VM not found")
	case errors.Is(err, vm.ErrClusterConsoleUnavailable):
		h.writeConsoleError(w, http.StatusBadGateway, "console_unavailable", "console is not available for this VM")
	default:
		h.log.Error("console ticket issuance failed", "component", "httpapi", "error", err)
		h.writeConsoleError(w, http.StatusInternalServerError, "internal_error", "internal server error")
	}
}

func (h *VMConsole) writeConsoleError(w http.ResponseWriter, status int, code, message string) {
	if err := writeClusterError(w, status, code, message); err != nil {
		h.log.Error("failed to write console error", "component", "httpapi", "code", code, "error", err)
	}
}

// writeJSON marshals value and writes it with the given status code.
func (h *VMConsole) writeJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		h.log.Error("failed to marshal response", "component", "httpapi", "error", err)
		h.writeConsoleError(w, http.StatusInternalServerError, "internal_error", "internal server error")

		return
	}

	if err := writeJSON(w, status, body); err != nil {
		h.log.Error("failed to write response", "component", "httpapi", "error", err)
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
