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
	"github.com/rs/zerolog"
)

// VMConsoleWebSocketHandler handles WebSocket connections for VNC console access.
// GET /vm/console/websocket?vmid={vmid}&node={node}&port={port}&vncticket={vncticket}
//
// This endpoint:
// 1. Validates the user has a valid Proxmox ticket in their session
// 2. Proxies the WebSocket connection to Proxmox VNC WebSocket endpoint
// 3. Handles bidirectional data transfer between browser and Proxmox
func (h *VMHandler) VMConsoleWebSocketHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("VMConsoleWebSocketHandler", r)

	// Validate authentication
	if !IsAuthenticated(r) {
		log.Warn().Msg("Unauthenticated WebSocket console access attempt")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	// Get Proxmox ticket from session
	pveTicket, _, _, ticketOk := GetProxmoxTicketFromSession(r)
	if !ticketOk {
		log.Warn().Msg("No Proxmox ticket found in session for WebSocket console")
		http.Error(w, "Proxmox authentication required", http.StatusUnauthorized)
		return
	}

	// Parse required parameters
	vmid := r.URL.Query().Get("vmid")
	node := r.URL.Query().Get("node")
	portStr := r.URL.Query().Get("port")
	vncticket := r.URL.Query().Get("vncticket")

	if vmid == "" || node == "" || portStr == "" || vncticket == "" {
		log.Warn().
			Str("vmid", vmid).
			Str("node", node).
			Str("port", portStr).
			Bool("has_vncticket", len(vncticket) > 0).
			Msg("Missing required parameters for VNC WebSocket")
		http.Error(w, "Missing required parameters: vmid, node, port, vncticket", http.StatusBadRequest)
		return
	}

	// IMPORTANT: The vncticket comes already URL-encoded from the frontend.
	// r.URL.Query().Get() automatically decodes it once, which is what we want.
	// Do NOT decode it again - just pass it to buildProxmoxWebSocketURL which will re-encode it properly.

	port, err := strconv.Atoi(portStr)
	if err != nil || port < 5900 || port > 5999 {
		log.Warn().Str("port", portStr).Msg("Invalid VNC port number")
		http.Error(w, "Invalid port number", http.StatusBadRequest)
		return
	}

	log.Info().
		Str("vmid", vmid).
		Str("node", node).
		Int("port", port).
		Int("vncticket_len", len(vncticket)).
		Str("vncticket_prefix", vncticket[:min(20, len(vncticket))]).
		Msg("Establishing VNC WebSocket connection")

	// Build Proxmox WebSocket URL
	proxmoxURL := os.Getenv("PROXMOX_URL")
	if proxmoxURL == "" {
		log.Error().Msg("PROXMOX_URL not configured")
		http.Error(w, "Server configuration error", http.StatusInternalServerError)
		return
	}

	// Parse and modify URL for WebSocket
	proxmoxWSURL, err := buildProxmoxWebSocketURL(proxmoxURL, node, vmid, port, vncticket)
	if err != nil {
		log.Error().Err(err).Msg("Failed to build Proxmox WebSocket URL")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Debug().Str("proxmox_ws_url", proxmoxWSURL).Msg("Connecting to Proxmox WebSocket")

	// Proxy the WebSocket connection
	if err := proxyVNCWebSocket(w, r, proxmoxWSURL, pveTicket, &log); err != nil {
		log.Error().Err(err).Msg("WebSocket proxy failed")
		// Error response already handled by proxyVNCWebSocket if upgrade failed
	}
}

// buildProxmoxWebSocketURL constructs the Proxmox VNC WebSocket URL.
// Converts https:// to wss:// and http:// to ws://
func buildProxmoxWebSocketURL(proxmoxURL, node, vmid string, port int, vncticket string) (string, error) {
	// Parse base URL
	baseURL := strings.TrimSpace(proxmoxURL)
	if baseURL == "" {
		return "", fmt.Errorf("proxmox URL is empty")
	}

	// Ensure URL has protocol
	if !strings.HasPrefix(baseURL, "http://") && !strings.HasPrefix(baseURL, "https://") {
		baseURL = "https://" + baseURL
	}

	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid proxmox URL: %w", err)
	}

	// Convert HTTP(S) to WS(S)
	if parsedURL.Scheme == "https" {
		parsedURL.Scheme = "wss"
	} else {
		parsedURL.Scheme = "ws"
	}

	// Build WebSocket path
	// Format: /api2/json/nodes/{node}/qemu/{vmid}/vncwebsocket?port={port}&vncticket={vncticket}
	parsedURL.Path = fmt.Sprintf("/api2/json/nodes/%s/qemu/%s/vncwebsocket",
		url.PathEscape(node),
		url.PathEscape(vmid))

	// Add query parameters
	query := parsedURL.Query()
	query.Set("port", strconv.Itoa(port))
	query.Set("vncticket", vncticket)
	parsedURL.RawQuery = query.Encode()

	return parsedURL.String(), nil
}

// proxyVNCWebSocket handles the WebSocket proxying between client and Proxmox using gorilla/websocket
func proxyVNCWebSocket(w http.ResponseWriter, r *http.Request, proxmoxWSURL, pveTicket string, log *zerolog.Logger) error {
	// Configure WebSocket upgrader for client connection
	clientUpgrader := websocket.Upgrader{
		ReadBufferSize:  4096,
		WriteBufferSize: 4096,
		CheckOrigin: func(r *http.Request) bool {
			// Allow connections from same origin
			return true
		},
	}

	// Upgrade client connection to WebSocket
	clientConn, err := clientUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return fmt.Errorf("failed to upgrade client connection: %w", err)
	}
	defer clientConn.Close()

	log.Info().Msg("Client WebSocket connection established")

	// Configure dialer for Proxmox connection
	insecureSkipVerify := os.Getenv("PROXMOX_VERIFY_SSL") == "false"
	dialer := websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecureSkipVerify,
		},
		HandshakeTimeout: 10 * time.Second,
		ReadBufferSize:   4096,
		WriteBufferSize:  4096,
	}

	// Prepare headers for Proxmox connection
	// Important: While the VNC protocol itself uses vncticket for authentication,
	// the WebSocket HANDSHAKE to Proxmox requires PVEAuthCookie for authorization.
	// This is different from the VNC-level authentication that happens after the WebSocket is established.
	proxmoxHeaders := http.Header{}
	proxmoxHeaders.Set("Cookie", fmt.Sprintf("PVEAuthCookie=%s", pveTicket))

	// Connect to Proxmox WebSocket  
	// Log the URL for debugging (mask sensitive parts)
	maskedURL := proxmoxWSURL
	if len(proxmoxWSURL) > 200 {
		maskedURL = proxmoxWSURL[:200] + "...[TRUNCATED]"
	}
	log.Info().
		Str("url_preview", maskedURL).
		Str("pve_cookie_prefix", pveTicket[:min(20, len(pveTicket))]).
		Msg("Connecting to Proxmox WebSocket with PVEAuthCookie")
	proxmoxConn, resp, err := dialer.Dial(proxmoxWSURL, proxmoxHeaders)
	if err != nil {
		if resp != nil {
			log.Error().
				Int("status", resp.StatusCode).
				Str("status_text", resp.Status).
				Msg("Proxmox WebSocket connection failed")
		}
		return fmt.Errorf("failed to connect to proxmox websocket: %w", err)
	}
	defer proxmoxConn.Close()

	log.Info().Msg("Proxmox WebSocket connection established, starting bidirectional proxy")

	// Set connection timeouts
	clientConn.SetReadDeadline(time.Time{})  // No read deadline
	proxmoxConn.SetReadDeadline(time.Time{}) // No read deadline

	// Create channels for error handling
	errChan := make(chan error, 2)

	// Client -> Proxmox goroutine
	go func() {
		defer log.Debug().Msg("Client->Proxmox goroutine finished")
		for {
			messageType, message, err := clientConn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Debug().Msg("Client closed connection normally")
					errChan <- nil
				} else {
					log.Warn().Err(err).Msg("Error reading from client")
					errChan <- err
				}
				return
			}

			// Forward message to Proxmox
			if err := proxmoxConn.WriteMessage(messageType, message); err != nil {
				log.Warn().Err(err).Msg("Error writing to Proxmox")
				errChan <- err
				return
			}
		}
	}()

	// Proxmox -> Client goroutine
	go func() {
		defer log.Debug().Msg("Proxmox->Client goroutine finished")
		for {
			messageType, message, err := proxmoxConn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Debug().Msg("Proxmox closed connection normally")
					errChan <- nil
				} else {
					log.Warn().Err(err).Msg("Error reading from Proxmox")
					errChan <- err
				}
				return
			}

			// Forward message to client
			if err := clientConn.WriteMessage(messageType, message); err != nil {
				log.Warn().Err(err).Msg("Error writing to client")
				errChan <- err
				return
			}
		}
	}()

	// Wait for either direction to complete
	err = <-errChan

	// Close both connections gracefully
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	clientConn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))
	proxmoxConn.WriteControl(websocket.CloseMessage, closeMsg, time.Now().Add(time.Second))

	if err != nil {
		log.Warn().Err(err).Msg("WebSocket connection closed with error")
		return err
	}

	log.Info().Msg("WebSocket connection closed normally")
	return nil
}
