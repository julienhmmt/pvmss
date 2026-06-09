package apiv1

import (
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"

	"pvmss/constants"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

const (
	vncConsoleTokenBytes = 32
	// vncConsoleTokenMaxEntries caps the in-memory token store to bound memory
	// usage if a client spams the ticket endpoint. When the cap is reached,
	// the oldest-expiring entry is evicted before inserting a new one.
	vncConsoleTokenMaxEntries = 1024
	// vncConsoleTokenMinTTL is the minimum TTL applied if the configured
	// VNCTicketValidityDuration - VNCTicketSafetyMargin would be <= 0.
	vncConsoleTokenMinTTL = time.Minute
)

// VNCHandler handles VNC console ticket and WebSocket proxy endpoints.
type VNCHandler struct {
	state      state.StateManager
	vncTickets *vncTicketStore
}

type vncConsoleTicket struct {
	ticket    string
	port      int
	node      string
	vmid      string
	expiresAt time.Time
}

type vncTicketStore struct {
	mu         sync.Mutex
	tickets    map[string]vncConsoleTicket
	ttl        time.Duration
	maxEntries int
}

// MakeVNCHandler creates a new VNCHandler.
func MakeVNCHandler(s state.StateManager) *VNCHandler {
	ttl := constants.VNCTicketValidityDuration - constants.VNCTicketSafetyMargin
	if ttl <= 0 {
		logger.Get().Warn().
			Dur("validity", constants.VNCTicketValidityDuration).
			Dur("margin", constants.VNCTicketSafetyMargin).
			Dur("fallback_ttl", vncConsoleTokenMinTTL).
			Msg("api/v1: VNCTicketSafetyMargin >= VNCTicketValidityDuration; using fallback TTL")
		ttl = vncConsoleTokenMinTTL
	}
	return &VNCHandler{
		state:      s,
		vncTickets: makeVNCTicketStore(ttl),
	}
}

// VNCTicketResponse is the JSON response for the VNC ticket endpoint.
type VNCTicketResponse struct {
	Ticket       string `json:"ticket"`
	Port         int    `json:"port"`
	Node         string `json:"node"`
	ConsoleToken string `json:"consoleToken,omitempty"`
}

// GetVNCTicket handles POST /api/v1/vms/:id/vnc-ticket.
// Returns a short-lived VNC ticket and port required to open the console WebSocket.
func (h *VNCHandler) GetVNCTicket(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		errOffline(w)
		return
	}
	vmid, ok := requireVMID(w, r)
	if !ok {
		return
	}

	cfg := h.state.GetEnvConfig()
	client, err := proxmox.MakeRestyClientFromEnvConfig(cfg, 15*time.Second)
	if err != nil {
		writeAppError(w, err)
		return
	}

	// Resolve the node for this VM.
	vms, err := proxmox.GetVMsResty(r.Context(), client)
	if err != nil {
		writeAppError(w, err)
		return
	}
	var node string
	for _, vm := range vms {
		if vm.VMID == vmid {
			node = vm.Node
			break
		}
	}
	if node == "" {
		writeError(w, http.StatusNotFound, "not_found", "VM not found")
		return
	}

	vncProxy, err := proxmox.GetVNCProxyResty(r.Context(), client, node, vmid, nil)
	if err != nil {
		logger.Get().Error().Err(err).Int("vmid", vmid).Str("node", node).Msg("api/v1: GetVNCProxyResty failed")
		writeError(w, http.StatusInternalServerError, "vnc_proxy_failed", "failed to create VNC proxy ticket")
		return
	}

	port, err := strconv.Atoi(vncProxy.Port)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "vnc_port_invalid", "invalid VNC port in Proxmox response")
		return
	}

	consoleToken, err := h.vncTickets.create(strconv.Itoa(vmid), port, node, vncProxy.Ticket)
	if err != nil {
		writeAppError(w, err)
		return
	}

	writeJSON(w, VNCTicketResponse{
		Ticket:       vncProxy.Ticket,
		Port:         port,
		Node:         node,
		ConsoleToken: consoleToken,
	})
}

// ConsoleWebSocket handles GET /api/v1/vms/:id/console/websocket.
// Proxies the browser WebSocket to the Proxmox VNC WebSocket endpoint.
// Query params: token=<opaque-console-token>.
// Legacy clients may still send port=<int>, vncticket=<url-encoded-ticket>, node=<string>.
func (h *VNCHandler) ConsoleWebSocket(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	vmidStr := ps.ByName("id")

	if vmidStr == "" {
		http.Error(w, "missing required parameter: vmid", http.StatusBadRequest)
		return
	}
	if _, err := strconv.Atoi(vmidStr); err != nil {
		http.Error(w, "invalid vm id", http.StatusBadRequest)
		return
	}
	port, node, vncticket, ok := h.resolveVNCConsoleParams(w, r, vmidStr)
	if !ok {
		return
	}

	envCfg := h.state.GetEnvConfig()
	proxmoxURL := envCfg.ProxmoxURL
	if proxmoxURL == "" {
		http.Error(w, "server configuration error", http.StatusInternalServerError)
		return
	}

	proxmoxWSURL, err := buildVNCWebSocketURL(proxmoxURL, node, vmidStr, port, vncticket)
	if err != nil {
		logger.Get().Error().Err(err).Msg("api/v1: failed to build proxmox WS URL")
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}

	tokenName := envCfg.ProxmoxAPITokenName
	tokenValue := envCfg.ProxmoxAPITokenValue
	authHeader := fmt.Sprintf("PVEAPIToken=%s=%s", tokenName, tokenValue)

	if err := proxyVNCWebSocketWithToken(w, r, proxmoxWSURL, authHeader, !envCfg.ProxmoxSSLVerify); err != nil {
		logger.Get().Warn().Err(err).Str("vmid", vmidStr).Msg("api/v1: VNC WebSocket proxy closed with error")
	}
}

func (h *VNCHandler) resolveVNCConsoleParams(w http.ResponseWriter, r *http.Request, vmid string) (int, string, string, bool) {
	q := r.URL.Query()
	if token := q.Get("token"); token != "" {
		consoleTicket, ok := h.vncTickets.consume(token, vmid)
		if !ok {
			logger.Get().Warn().Str("vmid", vmid).Msg("api/v1: invalid or expired console token")
			http.Error(w, "invalid or expired console token", http.StatusUnauthorized)
			return 0, "", "", false
		}
		return consoleTicket.port, consoleTicket.node, consoleTicket.ticket, true
	}
	portStr := q.Get("port")
	vncticket := q.Get("vncticket") // already decoded once by Query().Get()
	node := q.Get("node")
	if portStr == "" || vncticket == "" || node == "" {
		http.Error(w, "missing required parameters: token or port, vncticket, node", http.StatusBadRequest)
		return 0, "", "", false
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 5900 || port > 5999 {
		http.Error(w, "invalid port", http.StatusBadRequest)
		return 0, "", "", false
	}
	return port, node, vncticket, true
}

func makeVNCTicketStore(ttl time.Duration) *vncTicketStore {
	return &vncTicketStore{
		tickets:    make(map[string]vncConsoleTicket),
		ttl:        ttl,
		maxEntries: vncConsoleTokenMaxEntries,
	}
}

func (s *vncTicketStore) create(vmid string, port int, node string, ticket string) (string, error) {
	token, err := generateVNCConsoleToken()
	if err != nil {
		return "", err
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupExpiredLocked(now)
	if s.maxEntries > 0 && len(s.tickets) >= s.maxEntries {
		s.evictOldestLocked()
	}
	s.tickets[token] = vncConsoleTicket{
		ticket:    ticket,
		port:      port,
		node:      node,
		vmid:      vmid,
		expiresAt: now.Add(s.ttl),
	}
	return token, nil
}

func (s *vncTicketStore) consume(token string, vmid string) (vncConsoleTicket, bool) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	consoleTicket, ok := s.tickets[token]
	if !ok {
		return vncConsoleTicket{}, false
	}
	if now.After(consoleTicket.expiresAt) {
		delete(s.tickets, token)
		return vncConsoleTicket{}, false
	}
	if consoleTicket.vmid != vmid {
		return vncConsoleTicket{}, false
	}
	delete(s.tickets, token)
	return consoleTicket, true
}

func (s *vncTicketStore) evictOldestLocked() {
	var oldestToken string
	var oldestExpiresAt time.Time
	first := true
	for token, consoleTicket := range s.tickets {
		if first || consoleTicket.expiresAt.Before(oldestExpiresAt) {
			oldestToken = token
			oldestExpiresAt = consoleTicket.expiresAt
			first = false
		}
	}
	if oldestToken != "" {
		delete(s.tickets, oldestToken)
	}
}

func (s *vncTicketStore) cleanupExpiredLocked(now time.Time) {
	for token, consoleTicket := range s.tickets {
		if now.After(consoleTicket.expiresAt) {
			delete(s.tickets, token)
		}
	}
}

func generateVNCConsoleToken() (string, error) {
	b := make([]byte, vncConsoleTokenBytes)
	if _, err := rand.Read(b); err != nil {
		logger.Get().Error().Err(err).Msg("api/v1: crypto/rand.Read failed while generating VNC console token")
		return "", fmt.Errorf("generate VNC console token: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// buildVNCWebSocketURL converts a Proxmox HTTP(S) base URL to the VNC websocket URL.
func buildVNCWebSocketURL(proxmoxURL, node, vmid string, port int, vncticket string) (string, error) {
	base := strings.TrimSpace(proxmoxURL)
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "https://" + base
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("invalid proxmox URL: %w", err)
	}
	if parsed.Scheme == "https" {
		parsed.Scheme = "wss"
	} else {
		parsed.Scheme = "ws"
	}
	parsed.Path = fmt.Sprintf("/api2/json/nodes/%s/qemu/%s/vncwebsocket",
		url.PathEscape(node), url.PathEscape(vmid))
	q := parsed.Query()
	q.Set("port", strconv.Itoa(port))
	q.Set("vncticket", vncticket)
	parsed.RawQuery = q.Encode()
	return parsed.String(), nil
}

// proxyVNCWebSocketWithToken upgrades the HTTP connection to a WebSocket and proxies
// traffic to Proxmox using API token authentication for the Proxmox handshake.
func proxyVNCWebSocketWithToken(w http.ResponseWriter, r *http.Request, proxmoxWSURL, authHeader string, insecureSkipVerify bool) error {
	upgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true // Allow direct connections (no Origin header)
			}
			originURL, err := url.Parse(origin)
			if err != nil {
				return false
			}
			// Behind a reverse proxy the external hostname arrives via
			// X-Forwarded-Host; fall back to r.Host for direct connections.
			host := r.Header.Get("X-Forwarded-Host")
			if host == "" {
				host = r.Host
			}
			// Strip port from host so bare hostnames compare correctly.
			if h, _, err := net.SplitHostPort(host); err == nil {
				host = h
			}
			return originURL.Hostname() == host
		},
	}
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return fmt.Errorf("failed to upgrade client connection: %w", err)
	}
	defer func() { _ = clientConn.Close() }()

	dialer := websocket.Dialer{
		TLSClientConfig:  &tls.Config{InsecureSkipVerify: insecureSkipVerify}, // #nosec G402 — controlled by PROXMOX_VERIFY_SSL
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
	}
	proxmoxHeaders := http.Header{}
	proxmoxHeaders.Set("Authorization", authHeader)

	proxmoxConn, _, err := dialer.Dial(proxmoxWSURL, proxmoxHeaders)
	if err != nil {
		return fmt.Errorf("failed to connect to Proxmox WebSocket: %w", err)
	}
	defer func() { _ = proxmoxConn.Close() }()

	errChan := make(chan error, 2)
	go forwardVNCMessages(clientConn, proxmoxConn, errChan)
	go forwardVNCMessages(proxmoxConn, clientConn, errChan)

	err = <-errChan
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	_ = clientConn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
	_ = proxmoxConn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
	return err
}

// forwardVNCMessages copies WebSocket messages from src to dst until an error or close.
func forwardVNCMessages(src, dst *websocket.Conn, errChan chan<- error) {
	for {
		msgType, msg, err := src.ReadMessage()
		if err != nil {
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				errChan <- nil
			} else {
				errChan <- err
			}
			return
		}
		if err := dst.WriteMessage(msgType, msg); err != nil {
			errChan <- err
			return
		}
	}
}
