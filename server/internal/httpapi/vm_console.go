//nolint:wsl_v5 // console handlers keep upgrade and relay adjacent
package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
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
	auth       *Auth
	relay      cluster.ConsoleRelay
	tickets    *vm.ConsoleTicketStore
	store      *store.Store
	log        *slog.Logger
}

// NewVMConsole creates the handler. The relay is the cluster.ConsoleRelay
// (Fake or Proxmox); the tickets store is the in-memory ConsoleTicketStore
// from main.go; the store is the real audit store.
func NewVMConsole(projection *inventory.Projection, authHandler *Auth, relay cluster.ConsoleRelay, tickets *vm.ConsoleTicketStore, st *store.Store, log *slog.Logger) *VMConsole {
	return &VMConsole{projection: projection, auth: authHandler, relay: relay, tickets: tickets, store: st, log: log}
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

	index := h.projection.Load()
	if index == nil {
		h.writeConsoleError(w, http.StatusServiceUnavailable, "inventory_not_ready", "inventory has not been populated yet")
		return
	}

	ticket, err := vm.GetConsoleTicket(r.Context(), index, identity, clusterName, vmid, h.relay, h.tickets, h.store)
	if err != nil {
		h.writeTicketError(w, err)
		return
	}

	if ticket.Token == "" {
		h.log.Error("console ticket has no token", "component", "httpapi", "cluster", clusterName, "vmid", vmid)
		h.writeConsoleError(w, http.StatusInternalServerError, "internal_error", "failed to issue console ticket")

		return
	}

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

	peer := websocket.NetConn(context.Background(), conn, websocket.MessageBinary)
	defer func() { _ = peer.Close() }()

	proxy := cluster.VNCProxyTicket{Ticket: ticket.ProxmoxTicket, Port: ticket.Port, Node: ticket.Node}
	if err := h.relay.RelayConsole(r.Context(), ticket.Cluster, ticket.VMID, proxy, peer); err != nil {
		// Normal closure or client disconnect — log at debug, not error.
		if !isNormalClose(err) {
			h.log.Warn("console relay ended with error", "component", "httpapi", "vmid", ticket.VMID, "error", err)
		}
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
// Missing Origin (e.g. non-browser tests) is allowed.
func isConsoleOriginAllowed(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}

	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}

	return origin == scheme+"://"+r.Host
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
