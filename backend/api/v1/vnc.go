package apiv1

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"

	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// VNCHandler handles VNC console ticket and WebSocket proxy endpoints.
type VNCHandler struct {
	state state.StateManager
}

// MakeVNCHandler creates a new VNCHandler.
func MakeVNCHandler(s state.StateManager) *VNCHandler {
	return &VNCHandler{state: s}
}

// VNCTicketResponse is the JSON response for the VNC ticket endpoint.
type VNCTicketResponse struct {
	Ticket string `json:"ticket"`
	Port   int    `json:"port"`
	Node   string `json:"node"`
}

// GetVNCTicket handles POST /api/v1/vms/:id/vnc-ticket.
// Returns a short-lived VNC ticket and port required to open the console WebSocket.
func (h *VNCHandler) GetVNCTicket(w http.ResponseWriter, r *http.Request) {
	if h.state.IsOfflineMode() {
		errOffline(w)
		return
	}
	ps := httprouter.ParamsFromContext(r.Context())
	vmid, err := strconv.Atoi(ps.ByName("id"))
	if err != nil || vmid <= 0 {
		errBadRequest(w, "invalid vm id")
		return
	}

	client, err := proxmox.MakeRestyClientFromEnv(15 * time.Second)
	if err != nil {
		errInternal(w)
		return
	}

	// Resolve the node for this VM.
	vms, err := proxmox.GetVMsResty(r.Context(), client)
	if err != nil {
		errInternal(w)
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

	writeJSON(w, VNCTicketResponse{
		Ticket: vncProxy.Ticket,
		Port:   port,
		Node:   node,
	})
}

// ConsoleWebSocket handles GET /api/v1/vms/:id/console/websocket.
// Proxies the browser WebSocket to the Proxmox VNC WebSocket endpoint.
// Query params: port=<int>, vncticket=<url-encoded-ticket>, node=<string>
func (h *VNCHandler) ConsoleWebSocket(w http.ResponseWriter, r *http.Request) {
	ps := httprouter.ParamsFromContext(r.Context())
	vmidStr := ps.ByName("id")

	portStr := r.URL.Query().Get("port")
	vncticket := r.URL.Query().Get("vncticket") // already decoded once by Query().Get()
	node := r.URL.Query().Get("node")

	if vmidStr == "" || portStr == "" || vncticket == "" || node == "" {
		http.Error(w, "missing required parameters: vmid, port, vncticket, node", http.StatusBadRequest)
		return
	}

	port, err := strconv.Atoi(portStr)
	if err != nil || port < 5900 || port > 5999 {
		http.Error(w, "invalid port", http.StatusBadRequest)
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
			// Allow localhost connections regardless of port (for development)
			// In production, this should be more restrictive
			host := r.Host
			originURL, err := url.Parse(origin)
			if err != nil {
				return false
			}
			// Allow if same host (ignore port for localhost)
			if originURL.Hostname() == host || originURL.Hostname() == "localhost" {
				return true
			}
			// Allow exact match (same host and port)
			return origin == "http://"+host || origin == "https://"+host
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
