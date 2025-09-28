package handlers

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
	"pvmss/security"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			// Allow connections from same origin and local development
			origin := r.Header.Get("Origin")
			if origin == "" {
				return true // Allow connections without origin header
			}
			// Allow localhost, 127.0.0.1, and same-origin connections
			return strings.Contains(origin, "localhost") ||
				strings.Contains(origin, "127.0.0.1") ||
				strings.HasPrefix(origin, "http://"+r.Host) ||
				strings.HasPrefix(origin, "https://"+r.Host)
		},
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
	}
)

// VMConsoleWebSocketProxy handles WebSocket connections between noVNC client and Proxmox VNC server
func (h *VMHandler) VMConsoleWebSocketProxy(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("VMConsoleWebSocketProxy", r)

	fmt.Printf("PVMSS DEBUG: ===== WebSocket proxy handler called =====\n")
	fmt.Printf("PVMSS DEBUG: Request URL: %s\n", r.URL.String())
	fmt.Printf("PVMSS DEBUG: Request Origin: %s\n", r.Header.Get("Origin"))
	fmt.Printf("PVMSS DEBUG: Request User-Agent: %s\n", r.Header.Get("User-Agent"))

	log.Info().Msg("WebSocket proxy handler called")

	if !IsAuthenticated(r) {
		log.Warn().Msg("WebSocket proxy request rejected: not authenticated")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get parameters from query string
	query := r.URL.Query()
	host := query.Get("host")
	node := query.Get("node")
	vmid := query.Get("vmid")
	port := query.Get("port")
	ticket := query.Get("ticket")

	fmt.Printf("PVMSS DEBUG: WebSocket Parameters - Host: %s, Node: %s, VMID: %s, Port: %s, HasTicket: %t\n",
		host, node, vmid, port, ticket != "")

	log.Info().
		Str("host", host).
		Str("node", node).
		Str("vmid", vmid).
		Str("port", port).
		Bool("has_ticket", ticket != "").
		Msg("WebSocket proxy parameters")

	if host == "" || node == "" || vmid == "" || port == "" || ticket == "" {
		log.Error().Msg("Missing required WebSocket parameters")
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	// Get authentication from console session (like Python approach stores auth in session)
	sessionManager := security.GetSession(r)
	var authCookie, csrfToken string

	if sessionManager != nil {
		// First try to get from console session
		vmidInt, _ := strconv.Atoi(vmid)
		sessionKey := fmt.Sprintf("console_session_%d_%s", vmidInt, node)

		if consoleSessionData := sessionManager.Get(r.Context(), sessionKey); consoleSessionData != nil {
			if consolePayload, ok := consoleSessionData.(consoleSessionPayload); ok {
				authCookie = consolePayload.AuthCookie
				csrfToken = consolePayload.CsrfToken
				log.Info().Msg("Retrieved auth from console session")
			}
		}

		// Fallback to user session
		if authCookie == "" {
			authCookie = sessionManager.GetString(r.Context(), "pve_auth_cookie")
			csrfToken = sessionManager.GetString(r.Context(), "csrf_prevention_token")
			log.Info().Msg("Retrieved auth from user session")
		}
	}

	// Also try to get auth cookie from the request itself
	if authCookie == "" {
		if cookie, err := r.Cookie("PVEAuthCookie"); err == nil {
			authCookie = cookie.Value
			log.Info().Msg("Retrieved auth from request cookie")
		}
	}

	log.Info().
		Bool("has_session_auth", authCookie != "").
		Msg("WebSocket authentication status")

	// Upgrade HTTP connection to WebSocket
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade connection to WebSocket")
		return
	}
	defer clientConn.Close()

	log.Info().Msg("WebSocket connection upgraded successfully")

	// Connect to Proxmox VNC WebSocket with authentication
	proxmoxConn, err := connectToProxmoxVNC(host, node, vmid, port, ticket, authCookie, csrfToken)
	if err != nil {
		log.Error().Err(err).Msg("Failed to connect to Proxmox VNC WebSocket")
		clientConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "Failed to connect to Proxmox"))
		return
	}
	defer proxmoxConn.Close()

	log.Info().Msg("Connected to Proxmox VNC WebSocket successfully")

	// Start bidirectional proxy
	proxyWebSocketConnections(clientConn, proxmoxConn)

	log.Info().Msg("WebSocket proxy session ended")
}

// connectToProxmoxVNC establishes a WebSocket connection to Proxmox VNC server
func connectToProxmoxVNC(host, node, vmid, port, ticket, authCookie, csrfToken string) (*websocket.Conn, error) {
	// Determine if we should skip TLS verification
	skipTLS := strings.EqualFold(strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL")), "false")

	// Determine WebSocket scheme based on Proxmox API URL (not TLS verification setting)
	scheme := "wss" // Default to secure

	// Check if the host indicates HTTP (like localhost or specific dev setups)
	if strings.Contains(host, ":8006") {
		// Port 8006 is typically HTTPS for Proxmox, but check PROXMOX_API_URL if available
		proxmoxAPIURL := strings.TrimSpace(os.Getenv("PROXMOX_API_URL"))
		if strings.HasPrefix(proxmoxAPIURL, "http://") {
			scheme = "ws"
		}
	}

	// Build the WebSocket URL for Proxmox vncwebsocket endpoint
	proxmoxURL := fmt.Sprintf("%s://%s/api2/json/nodes/%s/qemu/%s/vncwebsocket?port=%s&vncticket=%s",
		scheme, host, node, vmid, port, url.QueryEscape(ticket))

	fmt.Printf("PVMSS DEBUG: Connecting to Proxmox WebSocket URL: %s\n", proxmoxURL)
	fmt.Printf("PVMSS DEBUG: WebSocket scheme: %s, skipTLS: %t\n", scheme, skipTLS)
	fmt.Printf("PVMSS DEBUG: Host: %s, Node: %s, VMID: %s, Port: %s\n", host, node, vmid, port)
	fmt.Printf("PVMSS DEBUG: Ticket: %s...\n", ticket[:min(10, len(ticket))])
	fmt.Printf("PVMSS DEBUG: AuthCookie available: %t\n", authCookie != "")

	// Setup WebSocket dialer with TLS configuration
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: skipTLS,
		},
		HandshakeTimeout: 10 * time.Second,
	}

	// Setup headers for authentication (critical for Proxmox auth)
	headers := http.Header{}
	if authCookie != "" {
		headers.Set("Cookie", fmt.Sprintf("PVEAuthCookie=%s", authCookie))
		fmt.Printf("PVMSS DEBUG: Added PVEAuthCookie header\n")
	}
	if csrfToken != "" {
		headers.Set("CSRFPreventionToken", csrfToken)
		fmt.Printf("PVMSS DEBUG: Added CSRFPreventionToken header\n")
	}

	// Add additional headers that might help with authentication
	headers.Set("User-Agent", "PVMSS-Console/1.0")
	headers.Set("Origin", fmt.Sprintf("https://%s", host))

	fmt.Printf("PVMSS DEBUG: Attempting WebSocket connection with headers: %v\n", headers)

	// Connect to Proxmox WebSocket with detailed logging
	conn, resp, err := dialer.Dial(proxmoxURL, headers)
	if err != nil {
		fmt.Printf("PVMSS DEBUG: WebSocket connection failed - Error: %v\n", err)
		if resp != nil {
			fmt.Printf("PVMSS DEBUG: WebSocket response status: %s (%d)\n", resp.Status, resp.StatusCode)
			fmt.Printf("PVMSS DEBUG: WebSocket response headers: %v\n", resp.Header)
			return nil, fmt.Errorf("proxmox websocket connection failed: %v (status: %s)", err, resp.Status)
		}
		return nil, fmt.Errorf("proxmox websocket connection failed: %v", err)
	}

	fmt.Printf("PVMSS DEBUG: WebSocket connection successful!\n")

	return conn, nil
}

// proxyWebSocketConnections handles bidirectional message forwarding between client and Proxmox
func proxyWebSocketConnections(client, proxmox *websocket.Conn) {
	// Channel to signal when either connection closes
	done := make(chan struct{})

	// Client -> Proxmox
	go func() {
		defer func() {
			select {
			case done <- struct{}{}:
			default:
			}
		}()

		for {
			messageType, data, err := client.ReadMessage()
			if err != nil {
				return
			}

			if err := proxmox.WriteMessage(messageType, data); err != nil {
				return
			}
		}
	}()

	// Proxmox -> Client
	go func() {
		defer func() {
			select {
			case done <- struct{}{}:
			default:
			}
		}()

		for {
			messageType, data, err := proxmox.ReadMessage()
			if err != nil {
				return
			}

			if err := client.WriteMessage(messageType, data); err != nil {
				return
			}
		}
	}()

	// Wait for either connection to close
	<-done
}
