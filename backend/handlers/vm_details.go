package handlers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/security"
	"pvmss/state"

	"github.com/gomarkdown/markdown"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"
)

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// VMStateManager defines the minimal state contract needed by VM details.
type VMStateManager interface {
	GetProxmoxClient() proxmox.ClientInterface
	GetSettings() *state.AppSettings
	GetProxmoxStatus() (bool, string)
}

// VMHandler handles VM-related pages and API endpoints
type VMHandler struct {
	stateManager VMStateManager
}

// VMConsoleWebSocketProxy handles WebSocket connections for VM console access
func (h *VMHandler) VMConsoleWebSocketProxy(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "VMConsoleWebSocketProxy").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Info().Msg("=== WEBSOCKET PROXY HANDLER CALLED ===")
	log.Info().Str("full_url", r.URL.String()).Msg("Full request URL")
	log.Info().Str("raw_query", r.URL.RawQuery).Msg("Raw query string")

	// Extract VM ID and node from URL parameters
	vmIDStr := ps.ByName("vmid")
	node := r.URL.Query().Get("node")
	port := r.URL.Query().Get("port")
	vncticket := r.URL.Query().Get("vncticket")

	log.Info().
		Str("vmid", vmIDStr).
		Str("node", node).
		Str("port", port).
		Str("vncticket_present", fmt.Sprintf("%t", vncticket != "")).
		Str("vncticket_length", fmt.Sprintf("%d", len(vncticket))).
		Msg("WebSocket proxy handler received request parameters")

	if vmIDStr == "" || node == "" || port == "" || vncticket == "" {
		log.Error().
			Str("vmid", vmIDStr).
			Str("node", node).
			Str("port", port).
			Str("vncticket", vncticket).
			Msg("Missing required parameters for WebSocket proxy")
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	log.Info().Msg("All required parameters present, proceeding with WebSocket upgrade")
	log.Info().
		Str("vmid", vmIDStr).
		Str("node", node).
		Msg("Setting up WebSocket proxy connection")

	// Build target URL for Proxmox WebSocket
	proxmoxURL := strings.TrimSpace(os.Getenv("PROXMOX_URL"))
	if proxmoxURL == "" {
		log.Error().Msg("PROXMOX_URL not configured")
		http.Error(w, "Proxmox URL not configured", http.StatusInternalServerError)
		return
	}

	var u *url.URL
	var err error
	u, err = url.Parse(proxmoxURL)
	if err != nil || u.Host == "" {
		log.Error().Err(err).Str("proxmoxURL", proxmoxURL).Msg("Invalid Proxmox URL")
		http.Error(w, "Invalid Proxmox URL", http.StatusInternalServerError)
		return
	}

	log.Info().
		Str("vmid", vmIDStr).
		Str("node", node).
		Msg("Setting up WebSocket proxy connection")

	// Create WebSocket upgrader
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for console access
		},
	}

	log.Info().Msg("Attempting WebSocket upgrade")

	// Upgrade the HTTP connection to WebSocket
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade client connection to WebSocket")
		return
	}
	defer clientConn.Close()

	log.Info().Msg("WebSocket upgrade successful")

	// Now that we have the ticket, connect to Proxmox
	log.Info().Msg("Connecting to Proxmox WebSocket with ticket")

	// Build target WebSocket URL with correct scheme
	targetScheme := "ws"
	if u.Scheme == "https" {
		targetScheme = "wss"
	}

	// Log the raw vncticket for debugging (without logging sensitive data)
	log.Debug().
		Str("vmid", vmIDStr).
		Str("node", node).
		Str("port", port).
		Int("ticket_length", len(vncticket)).
		Msg("Creating WebSocket connection to Proxmox")

	targetURL := fmt.Sprintf("%s://%s/api2/json/nodes/%s/qemu/%s/vncwebsocket?port=%s&vncticket=%s",
		targetScheme, u.Host, node, vmIDStr, port, url.QueryEscape(vncticket))

	log.Info().
		Str("target", fmt.Sprintf("%s://%s/api2/json/nodes/%s/qemu/%s/vncwebsocket?port=%s&vncticket=[REDACTED]", targetScheme, u.Host, node, vmIDStr, port)).
		Msg("Proxying WebSocket connection to Proxmox")

	// Create dialer with TLS configuration and sensible timeouts
	insecureSkip := strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL")) == "false"
	serverName := u.Hostname()
	tlsCfg := &tls.Config{InsecureSkipVerify: insecureSkip, ServerName: serverName}
	dialer := websocket.Dialer{
		TLSClientConfig:   tlsCfg,
		HandshakeTimeout:  10 * time.Second,
		Proxy:             http.ProxyFromEnvironment,
		NetDialContext:    (&net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		EnableCompression: true,
		Subprotocols:      []string{"binary"},
	}

	log.Info().
		Bool("insecure_skip_verify", insecureSkip).
		Str("tls_server_name", serverName).
		Msg("Configured WebSocket TLS dialer")

	// Set up headers for Proxmox connection
	headers := http.Header{}

	// Parse and propagate subprotocols via dialer (preferred over manual header)
	if sp := r.Header.Get("Sec-WebSocket-Protocol"); sp != "" {
		parts := strings.Split(sp, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		dialer.Subprotocols = parts
	} else {
		// Set default subprotocol if none provided
		dialer.Subprotocols = []string{"binary"}
	}

	// Isolate and forward only the PVEAuthCookie for authentication.
	// Forwarding all cookies can cause conflicts.
	if pveAuthCookie, err := r.Cookie("PVEAuthCookie"); err == nil {
		headers.Set("Cookie", pveAuthCookie.String())
	} else {
		log.Warn().Err(err).Msg("PVEAuthCookie not found in request to proxy")
	}

	// Set Origin explicitly to the Proxmox origin to avoid upstream Origin checks.
	// This is critical for Proxmox to accept the WebSocket connection.
	proxmoxOrigin := fmt.Sprintf("%s://%s", u.Scheme, u.Host)
	headers.Set("Origin", proxmoxOrigin)

	// Also remove Sec-WebSocket-Key to let the dialer generate a fresh one.
	headers.Del("Sec-WebSocket-Key")

	// Remove other headers that might cause issues
	headers.Del("Sec-WebSocket-Version")
	headers.Del("Sec-WebSocket-Extensions")
	headers.Del("Connection")
	headers.Del("Upgrade")

	log.Debug().Msgf("Forwarding headers to Proxmox: %+v", headers)

	// Connect to Proxmox WebSocket
	proxmoxConn, resp, err := dialer.Dial(targetURL, headers)
	if err != nil {
		status := 0
		statusText := ""
		if resp != nil {
			status = resp.StatusCode
			statusText = resp.Status
			// Log response body for debugging
			if resp.Body != nil {
				body, _ := io.ReadAll(resp.Body)
				log.Debug().Str("response_body", string(body)).Msg("Proxmox WebSocket dial response body")
			}
		}
		log.Error().Err(err).Int("status", status).Str("status_text", statusText).Msg("Failed to dial Proxmox WebSocket")
		http.Error(w, "Failed to connect to Proxmox WebSocket", http.StatusBadGateway)
		return
	}

	defer proxmoxConn.Close()

	// Start bidirectional proxy with proper message type handling
	errChan := make(chan error, 2)

	// Client to Proxmox proxy
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("Panic in client->proxmox copy")
			}
		}()

		for {
			messageType, data, err := clientConn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Debug().Err(err).Msg("Client connection closed normally")
				} else {
					log.Debug().Err(err).Msg("Error reading from client")
				}
				errChan <- err
				return
			}

			// Log message type and size for debugging
			log.Debug().
				Int("message_type", messageType).
				Int("data_length", len(data)).
				Msg("Received message from client")

			// Handle special client messages
			if messageType == websocket.TextMessage {
				// Handle auth_request message from client
				if string(data) == "auth_request" {
					log.Debug().Msg("Received auth_request from client, sending auth_confirm with ticket")
					// Send the ticket back to the client as a JSON message
					response := map[string]interface{}{
						"type":   "auth_confirm",
						"ticket": vncticket,
					}
					responseBytes, err := json.Marshal(response)
					if err != nil {
						log.Error().Err(err).Msg("Failed to marshal auth_confirm response")
						errChan <- err
						return
					}
					if err := clientConn.WriteMessage(websocket.TextMessage, responseBytes); err != nil {
						log.Error().Err(err).Msg("Failed to send auth_confirm to client")
						errChan <- err
						return
					}
					continue
				}

				log.Trace().
					Int("message_type", messageType).
					Str("text", string(data)).
					Msg("Client->Proxmox text message")
			} else if messageType == websocket.BinaryMessage {
				if len(data) == 4 && string(data) == "\x01\x00\x00\x00" {
					log.Info().Msg("VNC handshake: Client init message")
				} else {
					log.Trace().
						Int("message_type", messageType).
						Int("data_length", len(data)).
						Msg("Client->Proxmox binary message")
				}
			}

			// Forward the message to Proxmox
			if err := proxmoxConn.WriteMessage(messageType, data); err != nil {
				log.Error().
					Err(err).
					Int("message_type", messageType).
					Int("data_length", len(data)).
					Msg("Failed to forward message to Proxmox")
				errChan <- err
				return
			}
		}
	}()

	// Proxmox to client proxy
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Error().Interface("panic", r).Msg("Panic in proxmox->client copy")
			}
		}()

		for {
			messageType, data, err := proxmoxConn.ReadMessage()
			if err != nil {
				if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
					log.Debug().Err(err).Msg("Proxmox connection closed normally")
				} else {
					log.Debug().Err(err).Msg("Error reading from Proxmox")
				}
				errChan <- err
				return
			}

			// Debug VNC handshake messages
			if messageType == websocket.BinaryMessage {
				if len(data) >= 12 && string(data[:12]) == "RFB 003.008\n" {
					log.Info().Msg("VNC handshake: Server version received")
				} else if len(data) >= 2 && len(data) <= 4 {
					log.Info().
						Int("security_types_count", int(data[1])).
						Msg("VNC handshake: Server security types")
				} else if len(data) == 4 && data[0] == 0 {
					log.Info().Msg("VNC handshake: Security handshake success")
				} else if len(data) >= 24 {
					log.Info().Msg("VNC handshake: Server init message received")
				} else {
					log.Trace().
						Int("message_type", messageType).
						Int("data_length", len(data)).
						Msg("Proxmox->Client binary message")
				}
			} else if messageType == websocket.TextMessage {
				log.Trace().
					Int("message_type", messageType).
					Str("text", string(data)).
					Msg("Proxmox->Client text message")
			}

			// Forward the message to client
			if err := clientConn.WriteMessage(messageType, data); err != nil {
				log.Error().
					Err(err).
					Int("message_type", messageType).
					Int("data_length", len(data)).
					Msg("Failed to forward message to client")
				errChan <- err
				return
			}
		}
	}()

	// Wait for either connection to close
	select {
	case err := <-errChan:
		log.Debug().Err(err).Msg("WebSocket proxy connection closed with error")
	case <-time.After(5 * time.Minute):
		log.Debug().Msg("WebSocket proxy connection timeout")
	}
	log.Debug().Msg("WebSocket proxy connection closed")
}

// VMConsoleProxyPage serves the noVNC console page from our server instead of redirecting to Proxmox
func (h *VMHandler) VMConsoleProxyPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "VMConsoleProxyPage").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	vmID := ps.ByName("vmid")
	node := strings.TrimSpace(r.URL.Query().Get("node"))
	vmname := strings.TrimSpace(r.URL.Query().Get("vmname"))
	port := strings.TrimSpace(r.URL.Query().Get("port"))
	vncticket := strings.TrimSpace(r.URL.Query().Get("vncticket"))
	vncpassword := strings.TrimSpace(r.URL.Query().Get("vncpassword"))

	if vmID == "" || node == "" || port == "" || vncticket == "" {
		log.Warn().Msg("Missing required parameters for console proxy")
		http.Error(w, "vmid, node, port and vncticket are required", http.StatusBadRequest)
		return
	}

	// Get Proxmox base URL for websocket proxy
	baseURL := ""
	if c := h.stateManager.GetProxmoxClient(); c != nil {
		baseURL = strings.TrimSpace(c.GetApiUrl())
	}
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("PROXMOX_URL"))
	}
	if baseURL == "" {
		log.Error().Msg("Proxmox URL not configured")
		http.Error(w, "Proxmox URL not configured", http.StatusInternalServerError)
		return
	}

	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		log.Error().Err(err).Str("baseURL", baseURL).Msg("Invalid Proxmox URL")
		http.Error(w, "Invalid Proxmox URL", http.StatusInternalServerError)
		return
	}

	// Construct the WebSocket proxy URL for the template.
	// The scheme (ws/wss) and host are handled by the browser's window.location.
	// We only need to provide the path and query parameters.
	// Note: vncticket is passed as a separate parameter to avoid URL encoding issues
	wsProxyURL := fmt.Sprintf("/api/console/qemu/%s/ws?node=%s&port=%s",
		vmID,
		url.QueryEscape(node),
		url.QueryEscape(port))

	log.Debug().
		Str("vmid", vmID).
		Str("node", node).
		Str("ws_proxy_url", wsProxyURL).
		Msg("Serving noVNC console page with proxied websocket")

	// Render the console page with the WebSocket URL
	data := map[string]interface{}{
		"Title":       fmt.Sprintf("Console for %s", vmname),
		"WSProxyURL":  wsProxyURL,
		"VMID":        vmID,
		"VMName":      vmname,
		"Node":        node,
		"Port":        port,
		"VNCTicket":   vncticket,
		"VNCPassword": vncpassword,
	}

	// Add translation data
	i18n.LocalizePage(w, r, data)

	// Render the console page template
	renderTemplateInternal(w, r, "console", data)
}

// ProxmoxAssetProxy proxies requests for Proxmox assets (e.g., /pve2/novnc/*) to the upstream Proxmox host
func (h *VMHandler) ProxmoxAssetProxy(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	baseURL := ""
	if c := h.stateManager.GetProxmoxClient(); c != nil {
		baseURL = strings.TrimSpace(c.GetApiUrl())
	}
	if baseURL == "" {
		baseURL = strings.TrimSpace(os.Getenv("PROXMOX_URL"))
	}
	if baseURL == "" {
		http.Error(w, "Proxmox URL not configured", http.StatusInternalServerError)
		return
	}

	u, err := url.Parse(baseURL)
	if err != nil || u.Host == "" {
		http.Error(w, "Invalid PROXMOX_URL", http.StatusInternalServerError)
		return
	}

	targetPath := r.URL.Path
	targetQuery := r.URL.RawQuery

	proxy := httputil.NewSingleHostReverseProxy(&url.URL{Scheme: u.Scheme, Host: u.Host})
	insecureSkip := strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL")) == "false"
	proxy.Transport = &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: insecureSkip}}
	orig := proxy.Director
	proxy.Director = func(req *http.Request) {
		orig(req)
		req.URL.Scheme = u.Scheme
		req.URL.Host = u.Host
		req.URL.Path = targetPath
		req.URL.RawQuery = targetQuery
		req.Host = u.Host
		if c, err := r.Cookie("PVEAuthCookie"); err == nil && c != nil && c.Value != "" {
			req.Header.Set("Cookie", "PVEAuthCookie="+c.Value)
		}
	}
	proxy.ServeHTTP(w, r)
}

// UpdateVMDescriptionHandler updates the VM description (Markdown supported on display)
func (h *VMHandler) UpdateVMDescriptionHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().Str("handler", "UpdateVMDescriptionHandler").Logger()
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		log.Warn().Err(err).Msg("parse form failed")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	vmid := strings.TrimSpace(r.FormValue("vmid"))
	node := strings.TrimSpace(r.FormValue("node"))
	desc := r.FormValue("description")
	if vmid == "" || node == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		// Offline/read-only: redirect back to details with warning
		http.Redirect(w, r, "/vm/details/"+vmid+"?error=offline", http.StatusSeeOther)
		return
	}
	// Update description (empty allowed to clear)
	if err := proxmox.UpdateVMConfigWithContext(r.Context(), client, node, vmidInt, map[string]string{"description": desc}); err != nil {
		log.Error().Err(err).Msg("update description failed")
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		return
	}
	http.Redirect(w, r, "/vm/details/"+vmid+"?refresh=1", http.StatusSeeOther)
}

// UpdateVMTagsHandler updates the VM tags from selected checkboxes
func (h *VMHandler) UpdateVMTagsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().Str("handler", "UpdateVMTagsHandler").Logger()
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		log.Warn().Err(err).Msg("parse form failed")
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	vmid := strings.TrimSpace(r.FormValue("vmid"))
	node := strings.TrimSpace(r.FormValue("node"))
	if vmid == "" || node == "" {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
		return
	}
	// Gather tags; ensure stable format for Proxmox: semicolon-separated, unique, trimmed
	sel := r.Form["tags"]
	seen := make(map[string]struct{}, len(sel))
	out := make([]string, 0, len(sel))
	for _, t := range sel {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		if _, ok := seen[t]; ok {
			continue
		}
		seen[t] = struct{}{}
		out = append(out, t)
	}
	// Optionally enforce default tag "pvmss" if available in settings
	if settings := h.stateManager.GetSettings(); settings != nil {
		for _, at := range settings.Tags {
			if strings.EqualFold(at, "pvmss") {
				if _, ok := seen["pvmss"]; !ok {
					out = append(out, "pvmss")
				}
				break
			}
		}
	}
	tagsParam := strings.Join(out, ";")

	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		// Offline/read-only: redirect back to details with warning
		http.Redirect(w, r, "/vm/details/"+vmid+"?error=offline", http.StatusSeeOther)
		return
	}
	if err := proxmox.UpdateVMConfigWithContext(r.Context(), client, node, vmidInt, map[string]string{"tags": tagsParam}); err != nil {
		log.Error().Err(err).Msg("update tags failed")
		http.Error(w, http.StatusText(http.StatusBadGateway), http.StatusBadGateway)
		return
	}
	http.Redirect(w, r, "/vm/details/"+vmid+"?refresh=1", http.StatusSeeOther)
}

// VMConsoleHandler handles requests for a noVNC console ticket.
// It calls the Proxmox API to get a ticket and returns it as JSON.
func (h *VMHandler) VMConsoleHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "VMConsoleHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	vmIDStr := ps.ByName("vmid")
	node := r.URL.Query().Get("node")
	if vmIDStr == "" || node == "" {
		log.Warn().Msg("Missing vmid or node parameter")
		http.Error(w, "vmid and node parameters are required", http.StatusBadRequest)
		return
	}

	vmID, err := strconv.Atoi(vmIDStr)
	if err != nil {
		log.Warn().Err(err).Str("vmid", vmIDStr).Msg("Invalid vmid parameter")
		http.Error(w, "Invalid vmid", http.StatusBadRequest)
		return
	}

	log = log.With().Str("vmid", vmIDStr).Str("node", node).Logger()
	log.Debug().Msg("Processing console ticket request")

	// Get session manager
	sessionManager := security.GetSession(r)
	if sessionManager == nil {
		log.Error().Msg("Session manager not available for console access")
		http.Error(w, "Session unavailable", http.StatusUnauthorized)
		return
	}

	// Use session-based auth directly; remove unnecessary fallbacks.
	var res *proxmox.ConsoleAuthResult
	if pveAuthCookie, hasCookie := sessionManager.Get(r.Context(), "pve_auth_cookie").(string); hasCookie && pveAuthCookie != "" {
		res, err = h.tryConsoleWithCookie(r.Context(), pveAuthCookie, sessionManager, node, vmID)
		if err != nil {
			http.Error(w, "Auth failed", http.StatusUnauthorized)
			return
		}
		h.setConsoleResponse(w, r, res, node, vmID)
		return
	}

	// Check if user is even authenticated
	isAuthenticated, _ := sessionManager.Get(r.Context(), "authenticated").(bool)
	if !isAuthenticated {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		errorResp := map[string]interface{}{
			"error":   "not_authenticated",
			"message": "User not authenticated. Please log in first.",
			"details": "You must be logged in to access VM console.",
		}
		json.NewEncoder(w).Encode(errorResp)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	errorResp := map[string]interface{}{
		"error":   "console_auth_failed",
		"message": "Console authentication failed. This may be due to expired session or insufficient permissions.",
		"details": fmt.Sprintf("User could not access console for VM %d on node %s. Please log out and log back in to refresh your console session, or contact your administrator to verify console permissions.", vmID, node),
		"troubleshooting": map[string]interface{}{
			"node":          node,
			"vmid":          vmID,
			"authenticated": isAuthenticated,
			"suggestions": []string{
				"Log out and log back in to refresh console authentication",
				"Verify your user has console permissions in Proxmox",
				"Check if the VM is in your assigned pool",
				"Contact administrator if the issue persists",
			},
		},
	}
	json.NewEncoder(w).Encode(errorResp)
}

// tryConsoleWithCookie attempts to get console ticket using stored PVE auth cookie
func (h *VMHandler) tryConsoleWithCookie(ctx context.Context, pveAuthCookie string, sessionManager *security.SessionManager, node string, vmID int) (*proxmox.ConsoleAuthResult, error) {
	proxmoxURL := strings.TrimSpace(os.Getenv("PROXMOX_URL"))
	insecureSkip := strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL")) == "false"

	if proxmoxURL == "" {
		return nil, fmt.Errorf("PROXMOX_URL not configured")
	}

	consoleClient, err := proxmox.NewClientCookieAuth(proxmoxURL, insecureSkip)
	if err != nil {
		return nil, fmt.Errorf("failed to create console client: %w", err)
	}

	// Set the stored auth credentials
	consoleClient.PVEAuthCookie = pveAuthCookie
	if csrfToken, hasCSRF := sessionManager.Get(ctx, "csrf_prevention_token").(string); hasCSRF {
		consoleClient.CSRFPreventionToken = csrfToken
	}

	return proxmox.GetConsoleTicket(ctx, consoleClient, node, vmID)
}

// setConsoleResponse handles setting the console response including cookies and JSON response
func (h *VMHandler) setConsoleResponse(w http.ResponseWriter, r *http.Request, res *proxmox.ConsoleAuthResult, node string, vmID int) {
	log := logger.Get().With().Str("handler", "VMHandler.setConsoleResponse").Logger()
	// Set PVEAuthCookie for browser so subsequent proxied requests carry auth
	secureCookie := (r.TLS != nil) || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	if res.PVEAuthCookie != "" {
		cookie := &http.Cookie{
			Name:     "PVEAuthCookie",
			Value:    res.PVEAuthCookie,
			Path:     "/",
			Secure:   secureCookie,
			HttpOnly: false,
			// For development compatibility, allow cookies for WebSocket requests
			SameSite: func() http.SameSite {
				if secureCookie {
					return http.SameSiteNoneMode
				}
				// For HTTP dev, don't set SameSite to allow WebSocket cookies
				return http.SameSiteDefaultMode
			}(),
			Expires: time.Now().Add(10 * time.Minute),
		}

		// Set cookie domain to allow cross-domain access to Proxmox server
		proxmoxURL := strings.TrimSpace(os.Getenv("PROXMOX_URL"))
		if proxmoxURL != "" {
			if u, err := url.Parse(proxmoxURL); err == nil && u.Host != "" {
				// Extract just the hostname/IP from the Proxmox URL
				host := u.Hostname()

				// Check if there's a custom domain setting
				if cd := strings.TrimSpace(os.Getenv("PROXMOX_COOKIE_DOMAIN")); cd != "" {
					cookie.Domain = cd
					log.Info().
						Str("custom_domain", cd).
						Msg("Using custom PROXMOX_COOKIE_DOMAIN for PVEAuthCookie")
				} else {
					// For cross-domain cookies, we need to be careful with domain settings
					// Don't set Domain for IP addresses, only for proper domain names
					isIPAddress := func(host string) bool {
						// Try to parse as IP address
						if net.ParseIP(host) != nil {
							return true
						}
						return false
					}

					if !isIPAddress(host) {
						// This is a proper domain name
						cookie.Domain = host
						log.Info().
							Str("domain", host).
							Msg("Setting PVEAuthCookie domain for cross-domain console access")
					} else {
						// This is an IP address - don't set domain for better browser compatibility
						log.Info().
							Str("proxmox_host", host).
							Msg("Proxmox host is IP address - not setting cookie domain for better compatibility")
					}
				}
			}
		}

		// Force SameSite=None for cross-domain compatibility when secure
		if secureCookie {
			cookie.SameSite = http.SameSiteNoneMode
		} else {
			// For HTTP (dev), use Lax for better compatibility
			cookie.SameSite = http.SameSiteLaxMode
		}

		sameSiteStr := "Default"
		switch cookie.SameSite {
		case http.SameSiteDefaultMode:
			sameSiteStr = "Default"
		case http.SameSiteLaxMode:
			sameSiteStr = "Lax"
		case http.SameSiteStrictMode:
			sameSiteStr = "Strict"
		case http.SameSiteNoneMode:
			sameSiteStr = "None"
		}

		log.Info().
			Str("cookie_name", cookie.Name).
			Str("cookie_domain", cookie.Domain).
			Str("cookie_path", cookie.Path).
			Bool("secure", cookie.Secure).
			Bool("http_only", cookie.HttpOnly).
			Str("same_site", sameSiteStr).
			Time("expires", cookie.Expires).
			Msg("Setting PVEAuthCookie for console authentication")

		http.SetCookie(w, cookie)
	}

	// Build normalized, flat response for the frontend
	resp := map[string]interface{}{
		"port":         res.Port,
		"vncticket":    res.Ticket,
		"ticket":       res.Ticket,
		"vncpassword":  res.VNCPassword,
		"node":         node,
		"vmid":         vmID,
		"proxmox_base": res.ProxmoxBase,
	}
	if res.CSRFPreventionToken != "" {
		resp["csrf_prevention_token"] = res.CSRFPreventionToken
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to encode VNC ticket response")
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// NewVMHandler creates a new VMHandler instance
func NewVMHandler(stateManager VMStateManager, wsHub interface{}) *VMHandler {
	return &VMHandler{
		stateManager: stateManager,
	}
}

// VMDetailsHandler handles the VM details page
func (h *VMHandler) VMDetailsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "VMHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	vmID := ps.ByName("id")
	if vmID == "" {
		log.Warn().Msg("VM ID not provided in request")
		// Localized generic error for bad request
		localizer := i18n.GetLocalizer(r)
		http.Error(w, i18n.Localize(localizer, "Error.Generic"), http.StatusBadRequest)
		return
	}

	log = log.With().Str("vm_id", vmID).Logger()
	log.Debug().Msg("Fetching VM details")

	// Get Proxmox client and search VM by ID
	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		log.Warn().Msg("Proxmox client not available; rendering VM details in offline/read-only mode")
		// Prepare minimal data for offline render
		data := map[string]interface{}{
			"Title":           "VM Details",
			"VMID":            vmID,
			"VMName":          "",
			"Status":          "unknown",
			"Uptime":          "0s",
			"Sockets":         0,
			"Cores":           0,
			"RAM":             "0 B",
			"DiskCount":       0,
			"DiskTotalSize":   "0 B",
			"NetworkBridges":  "",
			"Description":     "",
			"DescriptionHTML": template.HTML(""),
			"Tags":            "",
			"CurrentTags":     []string{},
			"Node":            "",
			"Offline":         true,
			"Warning":         "Proxmox connection unavailable. Displaying cached/empty data in read-only mode.",
		}
		if settings := h.stateManager.GetSettings(); settings != nil {
			data["AvailableTags"] = settings.Tags
		}
		i18n.LocalizePage(w, r, data)
		renderTemplateInternal(w, r, "vm_details", data)
		return
	}

	// If refresh is requested, clear client cache to force fresh data
	if r.URL.Query().Get("refresh") == "1" {
		log.Debug().Msg("Refresh requested, invalidating Proxmox client cache")
		client.InvalidateCache("")
	}

	// Always auto-refresh this page every 5 seconds, navigating to the same VM with refresh=1
	w.Header().Set("Refresh", fmt.Sprintf("5; url=/vm/details/%s?refresh=1", vmID))

	vms, err := proxmox.GetVMsWithContext(r.Context(), client)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to fetch VMs; rendering offline/read-only VM details page")
		data := map[string]interface{}{
			"Title":           "VM Details",
			"VMID":            vmID,
			"VMName":          "",
			"Status":          "unknown",
			"Uptime":          "0s",
			"Sockets":         0,
			"Cores":           0,
			"RAM":             "0 B",
			"DiskCount":       0,
			"DiskTotalSize":   "0 B",
			"NetworkBridges":  "",
			"Description":     "",
			"DescriptionHTML": template.HTML(""),
			"Tags":            "",
			"CurrentTags":     []string{},
			"Node":            "",
			"Warning":         "Could not fetch VM data from Proxmox. Displaying empty data.",
		}
		if settings := h.stateManager.GetSettings(); settings != nil {
			data["AvailableTags"] = settings.Tags
		}
		i18n.LocalizePage(w, r, data)
		renderTemplateInternal(w, r, "vm_details", data)
		return
	}

	var found *proxmox.VM
	for i := range vms {
		if strconv.Itoa(vms[i].VMID) == vmID {
			found = &vms[i]
			break
		}
	}
	if found == nil {
		log.Warn().Str("vm_id", vmID).Msg("VM not found")
		localizer := i18n.GetLocalizer(r)
		http.Error(w, i18n.Localize(localizer, "Error.NotFound"), http.StatusNotFound)
		return
	}

	// Format a few fields for the template
	formatBytes := func(b int64) string {
		// Approx formatting: display in GB with one decimal
		if b <= 0 {
			return "0 B"
		}
		const gb = 1024 * 1024 * 1024
		val := float64(b) / float64(gb)
		if val < 1 {
			// show in MB
			const mb = 1024 * 1024
			return strconv.FormatInt(b/int64(mb), 10) + " MB"
		}
		// Round to one decimal place
		s := strconv.FormatFloat(val, 'f', 1, 64)
		return s + " GB"
	}
	formatUptime := func(seconds int64) string {
		if seconds <= 0 {
			return "0s"
		}
		d := time.Duration(seconds) * time.Second
		// Show up to hours/minutes/seconds compactly
		h := int(d.Hours())
		m := int(d.Minutes()) % 60
		s := int(d.Seconds()) % 60
		if h > 0 {
			return strconv.Itoa(h) + "h" + strconv.Itoa(m) + "m"
		}
		if m > 0 {
			return strconv.Itoa(m) + "m" + strconv.Itoa(s) + "s"
		}
		return strconv.Itoa(s) + "s"
	}

	// Attempt to fetch VM config to enrich details (description, tags, network bridges)
	var desc string
	var bridgesStr string
	var tagsStr string
	var currentTags []string
	if cfg, err := proxmox.GetVMConfigWithContext(r.Context(), client, found.Node, found.VMID); err != nil {
		log.Debug().Err(err).Msg("VM config fetch failed; proceeding with basic details")
	} else {
		if v, ok := cfg["description"].(string); ok {
			desc = v
		}
		// Proxmox stores tags as semicolon-separated string (e.g., "pvmss;foo;bar").
		if v, ok := cfg["tags"].(string); ok && v != "" {
			// Proxmox tags are typically semicolon-separated, may include spaces or empty segments
			parts := strings.Split(v, ";")
			cleaned := make([]string, 0, len(parts))
			for _, p := range parts {
				p = strings.TrimSpace(p)
				if p != "" {
					cleaned = append(cleaned, p)
				}
			}
			if len(cleaned) > 0 {
				tagsStr = strings.Join(cleaned, ", ")
				currentTags = cleaned
			}
		}
		if brs := proxmox.ExtractNetworkBridges(cfg); len(brs) > 0 {
			bridgesStr = strings.Join(brs, ", ")
		}
	}

	// Render description as HTML from Markdown if present
	var descHTML template.HTML
	if desc != "" {
		descHTML = template.HTML(markdown.ToHTML([]byte(desc), nil, nil))
	}

	data := map[string]interface{}{
		"Title":           found.Name,
		"VMID":            vmID,
		"VMName":          found.Name,
		"Status":          found.Status,
		"Uptime":          formatUptime(found.Uptime),
		"Sockets":         1, // unknown here
		"Cores":           found.CPUs,
		"RAM":             formatBytes(found.MaxMem),
		"DiskCount":       0, // not available from this endpoint
		"DiskTotalSize":   formatBytes(found.MaxDisk),
		"NetworkBridges":  bridgesStr,
		"Description":     desc,
		"DescriptionHTML": descHTML,
		"Tags":            tagsStr,
		"CurrentTags":     currentTags,
		"Node":            found.Node,
	}

	// Optional error banner via query params
	if e := strings.TrimSpace(r.URL.Query().Get("error")); e != "" {
		data["Error"] = e
	}

	// Toggle edit mode via query parameter without JS
	if q := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("edit"))); q != "" {
		if q == "description" || q == "desc" {
			data["ShowDescriptionEditor"] = true
		}
		if q == "tags" || q == "tag" {
			data["ShowTagsEditor"] = true
		}
	}

	// Expose available tags from settings for selection UIs
	var availableTags []string
	if settings := h.stateManager.GetSettings(); settings != nil {
		availableTags = settings.Tags
		data["AvailableTags"] = availableTags
	}
	// Build union of available tags and currently set tags so users can uncheck tags not in available list
	if len(currentTags) > 0 || len(availableTags) > 0 {
		set := make(map[string]struct{}, len(currentTags)+len(availableTags))
		for _, t := range availableTags {
			t = strings.TrimSpace(t)
			if t != "" {
				set[t] = struct{}{}
			}
		}
		for _, t := range currentTags {
			t = strings.TrimSpace(t)
			if t != "" {
				set[t] = struct{}{}
			}
		}
		all := make([]string, 0, len(set))
		for k := range set {
			all = append(all, k)
		}
		sort.Strings(all)
		data["AllTags"] = all
	}

	log.Debug().Interface("vm_details", data).Msg("VM details fetched from Proxmox")

	// Add translation data
	i18n.LocalizePage(w, r, data)

	log.Debug().Msg("Rendering template vm_details")
	renderTemplateInternal(w, r, "vm_details", data)
}

// VMActionHandler handles VM lifecycle actions via server-side POST forms
func (h *VMHandler) VMActionHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().Str("handler", "VMActionHandler").Str("method", r.Method).Str("path", r.URL.Path).Logger()
	if r.Method != http.MethodPost {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// Parse posted form values
	if err := r.ParseForm(); err != nil {
		log.Error().Err(err).Msg("failed to parse form")
		localizer := i18n.GetLocalizer(r)
		http.Error(w, i18n.Localize(localizer, "Error.Generic"), http.StatusBadRequest)
		return
	}

	vmid := r.FormValue("vmid")
	node := r.FormValue("node")
	action := r.FormValue("action")
	if vmid == "" || node == "" || action == "" {
		log.Warn().Str("vmid", vmid).Str("node", node).Str("action", action).Msg("missing required fields")
		localizer := i18n.GetLocalizer(r)
		http.Error(w, i18n.Localize(localizer, "Error.Generic"), http.StatusBadRequest)
		return
	}

	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		log.Warn().Msg("Proxmox client not available; redirecting back to VM details with offline warning")
		http.Redirect(w, r, "/vm/details/"+vmid+"?error=offline", http.StatusSeeOther)
		return
	}

	log = log.With().Str("vmid", vmid).Str("node", node).Str("action", action).Logger()
	log.Info().Msg("performing VM action")

	// Execute action against Proxmox
	if _, err := proxmox.VMActionWithContext(r.Context(), client, node, vmid, action); err != nil {
		log.Error().Err(err).Msg("VM action failed")
		localizer := i18n.GetLocalizer(r)
		http.Error(w, i18n.Localize(localizer, "Proxmox.ConnectionError"), http.StatusBadGateway)
		return
	}

	// Redirect back to details page for the same VM with refresh=1 to force fresh data
	http.Redirect(w, r, "/vm/details/"+vmid+"?refresh=1", http.StatusSeeOther)
}

// RegisterRoutes registers VM-related routes
func (h *VMHandler) RegisterRoutes(router *httprouter.Router) {
	router.GET("/vm/create", RequireAuthHandle(h.CreateVMPage))
	router.GET("/vm/details/:id", RequireAuthHandle(h.VMDetailsHandler))
	router.POST("/vm/update/description", RequireAuthHandle(h.UpdateVMDescriptionHandler))
	router.POST("/vm/update/tags", RequireAuthHandle(h.UpdateVMTagsHandler))
	router.POST("/vm/action", RequireAuthHandle(h.VMActionHandler))
	router.POST("/api/vm/create", RequireAuthHandle(h.CreateVMHandler))
	router.GET("/api/console/qemu/:vmid", RequireAuthHandle(h.VMConsoleHandler))
	router.GET("/api/console/qemu/:vmid/ws", RequireAuthHandle(h.VMConsoleWebSocketProxy))
	router.GET("/console/qemu/:vmid", RequireAuthHandle(h.VMConsoleProxyPage))
	router.GET("/pve2/*filepath", RequireAuthHandle(h.ProxmoxAssetProxy))
}
