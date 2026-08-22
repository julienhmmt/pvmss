package httpapi

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"pvmss/server/internal/cluster"
	"pvmss/server/internal/store"
	"pvmss/server/internal/vm"
	"time"

	"github.com/coder/websocket"
)

// consoleErrorFn writes an error response. Both VMConsole.writeConsoleError and
// VMSerialConsole.writeSerialError satisfy this signature.
type consoleErrorFn func(w http.ResponseWriter, status int, code, message string)

// ticketResponse is the shared JSON body for POST /vnc-ticket and POST
// /serial-ticket. Both endpoints return the same shape: only the opaque token
// and its TTL.
type ticketResponse struct {
	Token            string `json:"token"`
	ExpiresInSeconds int    `json:"expiresInSeconds"`
}

// consoleTicketParams captures the kind-specific differences for the
// ticket-issuance flow. Both VNC and serial paths share the same flow;
// this struct parameterizes the parts that differ.
type consoleTicketParams struct {
	kind           vm.ConsoleKind
	capabilityName string
	fetcher        vm.ProxyFetcher
	invalidMsg     string
	noTokenMsg     string
	issuedMsg      string
}

// serveConsoleTicket is the shared ticket-issuance flow for both VNC and
// serial handlers. It authenticates, parses the path, loads the cluster index,
// resolves the relay, calls vm.GetConsoleTicket, and writes the JSON response.
func serveConsoleTicket(
	w http.ResponseWriter,
	r *http.Request,
	auth *Auth,
	resolver vm.ClusterIndexResolver,
	clients cluster.ClientProvider,
	defaultRelay any,
	tickets *vm.ConsoleTicketStore,
	st *store.Store,
	log *slog.Logger,
	params consoleTicketParams,
	parsePath func(r *http.Request) (string, int, bool),
	writeError consoleErrorFn,
) {
	identity, err := auth.Principal(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return
	}

	clusterName, vmid, ok := parsePath(r)
	if !ok {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid VM path")
		return
	}

	index, ok := loadClusterIndex(resolver, clusterName, func(status int, code, message string) { writeError(w, status, code, message) })
	if !ok {
		return
	}

	relay, err := resolveCapability(clients, defaultRelay, clusterName, params.capabilityName)
	if err != nil {
		writeError(w, http.StatusNotFound, "cluster_not_found", msgClusterNotFound)
		return
	}

	_ = relay // relay is already captured in the fetcher closure

	ticket, err := vm.GetConsoleTicket(r.Context(), vm.ConsoleTicketDeps{
		Index:       index,
		Actor:       identity,
		ClusterName: clusterName,
		VMID:        vmid,
		Kind:        params.kind,
		Fetcher:     params.fetcher,
		Store:       tickets,
		Audit:       st,
	})
	if err != nil {
		writeConsoleTicketError(w, log, err, writeError, params.invalidMsg)
		return
	}

	if ticket.Token == "" {
		log.Error(params.noTokenMsg, "component", "httpapi", "cluster", clusterName, "vmid", vmid)
		writeError(w, http.StatusInternalServerError, "internal_error", "failed to issue console ticket")

		return
	}

	log.Info(params.issuedMsg, "component", "httpapi", "cluster", clusterName, "vmid", vmid, "node", ticket.Node)
	writeConsoleJSON(w, log, writeError, http.StatusOK, ticketResponse{Token: ticket.Token, ExpiresInSeconds: int(vm.TicketTTL.Seconds())})
}

// writeConsoleJSON marshals value as JSON and writes it. On marshal/write
// failure, it logs and falls back to writeError.
func writeConsoleJSON(w http.ResponseWriter, log *slog.Logger, writeError consoleErrorFn, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		log.Error("failed to marshal response", "component", "httpapi", "error", err)
		writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")

		return
	}

	if err := writeJSON(w, status, body); err != nil {
		log.Error("failed to write response", "component", "httpapi", "error", err)
	}
}

// consoleWebSocketParams captures the kind-specific differences for the
// websocket-relay flow.
type consoleWebSocketParams struct {
	kind             vm.ConsoleKind
	invalidTicketMsg string
	wsUpgradeMsg     string
	acceptedMsg      string
}

// acceptConsoleWebSocket is the shared websocket-setup flow for both VNC and
// serial handlers. It authenticates, parses the path, consumes the ticket,
// checks origin, clears deadlines, and upgrades to WebSocket. On success it
// returns the consumed ticket and the websocket connection; the caller is
// responsible for creating the peer and relaying bytes. On failure it writes
// the error response and returns ok=false.
func acceptConsoleWebSocket(
	w http.ResponseWriter,
	r *http.Request,
	auth *Auth,
	tickets *vm.ConsoleTicketStore,
	log *slog.Logger,
	params consoleWebSocketParams,
	parsePath func(r *http.Request) (string, int, bool),
	writeError consoleErrorFn,
) (ticket vm.ConsoleTicket, conn *websocket.Conn, ok bool) {
	if _, err := auth.Principal(r); err != nil {
		writeError(w, http.StatusUnauthorized, "unauthenticated", "authentication required")
		return vm.ConsoleTicket{}, nil, false
	}

	clusterName, vmid, pathOK := parsePath(r)
	if !pathOK {
		writeError(w, http.StatusBadRequest, "invalid_request", "invalid VM path")
		return vm.ConsoleTicket{}, nil, false
	}

	token := r.URL.Query().Get("token")
	if token == "" {
		writeError(w, http.StatusBadRequest, "invalid_request", "missing token parameter")
		return vm.ConsoleTicket{}, nil, false
	}

	consumed, err := tickets.Consume(params.kind, token, clusterName, vmid)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_ticket", params.invalidTicketMsg)
		return vm.ConsoleTicket{}, nil, false
	}

	if !isConsoleOriginAllowed(r) {
		writeError(w, http.StatusForbidden, "forbidden", "invalid origin")
		return vm.ConsoleTicket{}, nil, false
	}

	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Time{})
	_ = rc.SetWriteDeadline(time.Time{})

	wsConn, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Error(params.wsUpgradeMsg, "component", "httpapi", "error", err)
		return vm.ConsoleTicket{}, nil, false
	}

	log.Info(params.acceptedMsg, "component", "httpapi", "cluster", consumed.Cluster, "vmid", consumed.VMID, "node", consumed.Node, "port", consumed.Port)

	return consumed, wsConn, true
}
