package handlers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/gorilla/websocket"
	"github.com/julienschmidt/httprouter"

	"pvmss/proxmox"
)

// VMConsoleWebSocketProxy handles WebSocket connections for VM console access
func (h *VMHandler) VMConsoleWebSocketProxy(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := CreateHandlerLogger("VMConsoleWebSocketProxy", r)

	log.Info().Msg("=== WEBSOCKET PROXY HANDLER CALLED ===")
	log.Info().Str("full_url", r.URL.String()).Msg("Full request URL")
	log.Info().Str("raw_query", r.URL.RawQuery).Msg("Raw query string")

	// Extract VM ID and node from URL parameters
	vmIDStr := ps.ByName("vmid")
	node := r.URL.Query().Get("node")
	port := r.URL.Query().Get("port")
	vncticket := r.URL.Query().Get("vncticket")
	_ = r.URL.Query().Get("vncpassword") // Not used in WebSocket proxy

	log.Info().
		Str("vmid", vmIDStr).
		Str("node", node).
		Str("port", port).
		Str("vncticket", vncticket).
		Msg("Extracted parameters")

	if vmIDStr == "" || node == "" || port == "" || vncticket == "" {
		log.Error().Msg("Missing required parameters")
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	// Get Proxmox URL from client
	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not available")
		http.Error(w, "Proxmox client not available", http.StatusInternalServerError)
		return
	}

	// Build the WebSocket URL to Proxmox
	baseURL := strings.TrimSpace(client.GetApiUrl())
	if baseURL == "" {
		log.Error().Msg("Proxmox URL not configured")
		http.Error(w, "Proxmox URL not configured", http.StatusInternalServerError)
		return
	}

	// Parse base URL to get host and scheme info
	parsedURL, err := url.Parse(baseURL)
	if err != nil {
		log.Error().Err(err).Str("url", baseURL).Msg("Failed to parse Proxmox URL")
		http.Error(w, "Invalid Proxmox URL", http.StatusInternalServerError)
		return
	}

	// Build WebSocket URL - Proxmox uses /api2/json/nodes/{node}/qemu/{vmid}/vncwebsocket
	wsScheme := "ws"
	if parsedURL.Scheme == "https" {
		wsScheme = "wss"
	}

	proxmoxWSURL := fmt.Sprintf("%s://%s/api2/json/nodes/%s/qemu/%s/vncwebsocket?port=%s&vncticket=%s",
		wsScheme, parsedURL.Host, node, vmIDStr, port, url.QueryEscape(vncticket))

	log.Info().Str("proxmox_ws_url", proxmoxWSURL).Msg("Connecting to Proxmox WebSocket")

	// Set up WebSocket upgrader
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow all origins for now
		},
	}

	// Upgrade client connection to WebSocket
	clientConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to upgrade client connection to WebSocket")
		return
	}
	defer clientConn.Close()

	log.Info().Msg("Client WebSocket connection established")

	// Set up TLS config for Proxmox connection
	insecureSkip := strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL")) == "false"
	dialer := &websocket.Dialer{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: insecureSkip,
		},
		HandshakeTimeout: 45 * time.Second,
	}

	// Set up headers for Proxmox WebSocket connection
	headers := http.Header{}
	// Add PVEAuthCookie if available
	if cookies := r.Header.Get("Cookie"); cookies != "" {
		headers.Set("Cookie", cookies)
	}

	// Connect to Proxmox WebSocket
	proxmoxConn, _, err := dialer.Dial(proxmoxWSURL, headers)
	if err != nil {
		log.Error().Err(err).Str("url", proxmoxWSURL).Msg("Failed to connect to Proxmox WebSocket")
		clientConn.WriteMessage(websocket.CloseMessage, []byte("Failed to connect to Proxmox"))
		return
	}
	defer proxmoxConn.Close()

	log.Info().Msg("Proxmox WebSocket connection established")

	// Set up bidirectional message forwarding
	done := make(chan bool, 2)

	// Forward messages from client to Proxmox
	go func() {
		defer func() { done <- true }()
		for {
			messageType, message, err := clientConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Error().Err(err).Msg("Client WebSocket read error")
				} else {
					log.Debug().Err(err).Msg("Client WebSocket connection closed")
				}
				return
			}
			log.Debug().Int("type", messageType).Int("size", len(message)).Msg("Forwarding message to Proxmox")
			if err := proxmoxConn.WriteMessage(messageType, message); err != nil {
				log.Error().Err(err).Msg("Failed to forward message to Proxmox")
				return
			}
		}
	}()

	// Forward messages from Proxmox to client
	go func() {
		defer func() { done <- true }()
		for {
			messageType, message, err := proxmoxConn.ReadMessage()
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					log.Error().Err(err).Msg("Proxmox WebSocket read error")
				} else {
					log.Debug().Err(err).Msg("Proxmox WebSocket connection closed")
				}
				return
			}
			log.Debug().Int("type", messageType).Int("size", len(message)).Msg("Forwarding message to client")
			if err := clientConn.WriteMessage(messageType, message); err != nil {
				log.Error().Err(err).Msg("Failed to forward message to client")
				return
			}
		}
	}()

	// Wait for either connection to close
	<-done
	log.Info().Msg("WebSocket proxy connection terminated")

	// Close the other connection
	closeMsg := websocket.FormatCloseMessage(websocket.CloseNormalClosure, "")
	if err := clientConn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
		log.Debug().Err(err).Msg("Error sending close message to client")
	}
	if err := proxmoxConn.WriteMessage(websocket.CloseMessage, closeMsg); err != nil {
		log.Debug().Err(err).Msg("Error sending close message to Proxmox")
	}
	log.Debug().Msg("WebSocket proxy connection closed")
}

// VMConsoleProxyPage serves the noVNC console page from our server instead of redirecting to Proxmox
func (h *VMHandler) VMConsoleProxyPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := CreateHandlerLogger("VMConsoleProxyPage", r)

	vmID := ps.ByName("vmid")
	node := strings.TrimSpace(r.URL.Query().Get("node"))
	vmname := strings.TrimSpace(r.URL.Query().Get("vmname"))
	port := strings.TrimSpace(r.URL.Query().Get("port"))
	vncticket := strings.TrimSpace(r.URL.Query().Get("vncticket"))
	vncpassword := strings.TrimSpace(r.URL.Query().Get("vncpassword"))

	if vmID == "" || node == "" || port == "" || vncticket == "" {
		log.Warn().Msg("Missing required parameters for console proxy")
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	// Build WebSocket URL for our proxy
	scheme := "ws"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "wss"
	}

	wsURL := fmt.Sprintf("%s://%s/ws/vm/%s?node=%s&port=%s&vncticket=%s",
		scheme, r.Host, vmID, node, port, url.QueryEscape(vncticket))

	if vncpassword != "" {
		wsURL += "&vncpassword=" + url.QueryEscape(vncpassword)
	}

	log.Info().Str("ws_url", wsURL).Msg("Generated WebSocket URL for console")

	// Prepare template data
	data := map[string]interface{}{
		"VMID":         vmID,
		"VMName":       vmname,
		"Node":         node,
		"Port":         port,
		"VNCTicket":    vncticket,
		"VNCPassword":  vncpassword,
		"WebSocketURL": wsURL,
	}

	// Render the console template
	renderTemplateInternal(w, r, "vm_console", data)
}

// ProxmoxAssetProxy proxies requests for Proxmox assets (e.g., /pve2/novnc/*) to the upstream Proxmox host
func (h *VMHandler) ProxmoxAssetProxy(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	baseURL := ""
	if c := h.stateManager.GetProxmoxClient(); c != nil {
		baseURL = strings.TrimSpace(c.GetApiUrl())
	}
	if baseURL == "" {
		http.Error(w, "Proxmox URL not configured", http.StatusInternalServerError)
		return
	}

	// Parse the target URL
	target, err := url.Parse(baseURL)
	if err != nil {
		http.Error(w, "Invalid Proxmox URL", http.StatusInternalServerError)
		return
	}

	// Create reverse proxy
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
		// Keep the original path
	}

	// Add PVE auth cookie if available from session
	for _, c := range r.Cookies() {
		if c.Name == "PVEAuthCookie" {
			req := r.Clone(r.Context())
			req.Header.Set("Cookie", "PVEAuthCookie="+c.Value)
		}
	}
	proxy.ServeHTTP(w, r)
}

// VMConsoleHandler handles requests for a noVNC console ticket.
// It calls the Proxmox API to get a ticket and returns it as JSON.
func (h *VMHandler) VMConsoleHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := CreateHandlerLogger("VMConsoleHandler", r)

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
		http.Error(w, "Invalid vmid parameter", http.StatusBadRequest)
		return
	}

	stateManager := getStateManager(r)
	if stateManager == nil {
		log.Error().Msg("State manager not available")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	sessionManager := stateManager.GetSessionManager()
	if sessionManager == nil {
		log.Error().Msg("Session manager not available")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()

	// Try to get console ticket using stored PVE auth cookie first
	if pveAuthCookie, hasCookie := sessionManager.Get(ctx, "pve_auth_cookie").(string); hasCookie && pveAuthCookie != "" {
		log.Debug().Msg("Attempting console access with stored PVE auth cookie")
		if res, err := h.tryConsoleWithCookie(ctx, pveAuthCookie, sessionManager, node, vmID); err == nil {
			log.Info().Msg("Successfully obtained console ticket using stored cookie")
			h.setConsoleResponse(w, r, res, node, vmID)
			return
		}
		log.Debug().Msg("Console access with stored cookie failed, will try fallback")
	}

	// Fallback: Check if we have separate console credentials for admin users
	proxmoxConsoleUser := strings.TrimSpace(os.Getenv("PROXMOX_CONSOLE_USER"))
	proxmoxConsolePassword := strings.TrimSpace(os.Getenv("PROXMOX_CONSOLE_PASSWORD"))

	if proxmoxConsoleUser == "" || proxmoxConsolePassword == "" {
		log.Warn().Msg("No console credentials available and no valid session cookie")
		http.Error(w, "Console access not available", http.StatusForbidden)
		return
	}

	log.Debug().Str("console_user", proxmoxConsoleUser).Msg("Using fallback console credentials")

	// Use fallback console credentials
	proxmoxURL := strings.TrimSpace(os.Getenv("PROXMOX_URL"))
	insecureSkip := strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL")) == "false"

	consoleClient, err := proxmox.NewClient(proxmoxURL, proxmoxConsoleUser, proxmoxConsolePassword, insecureSkip)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create console client")
		http.Error(w, "Failed to create console client", http.StatusInternalServerError)
		return
	}

	// Login is handled in NewClient for token-based auth
	if err := consoleClient.Login(ctx, proxmoxConsoleUser, proxmoxConsolePassword, "pam"); err != nil {
		log.Error().Err(err).Msg("Failed to authenticate with console credentials")
		http.Error(w, "Authentication failed", http.StatusUnauthorized)
		return
	}

	// Get console ticket
	res, err := proxmox.GetConsoleTicket(ctx, consoleClient, node, vmID)
	if err != nil {
		log.Error().Err(err).Int("vmid", vmID).Str("node", node).Msg("Failed to get console ticket")
		http.Error(w, "Failed to get console ticket", http.StatusInternalServerError)
		return
	}

	log.Info().Int("vmid", vmID).Str("node", node).Msg("Successfully obtained console ticket using fallback credentials")
	h.setConsoleResponse(w, r, res, node, vmID)
}

// tryConsoleWithCookie attempts to get console ticket using stored PVE auth cookie
func (h *VMHandler) tryConsoleWithCookie(ctx context.Context, pveAuthCookie string, sessionManager *scs.SessionManager, node string, vmID int) (*proxmox.ConsoleAuthResult, error) {
	proxmoxURL := strings.TrimSpace(os.Getenv("PROXMOX_URL"))
	insecureSkip := strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL")) == "false"

	consoleClient, err := proxmox.NewClientCookieAuth(proxmoxURL, insecureSkip)
	if err != nil {
		return nil, fmt.Errorf("failed to create console client: %w", err)
	}

	// Set up authentication using stored cookie
	consoleClient.PVEAuthCookie = pveAuthCookie
	if csrfToken, hasCSRF := sessionManager.Get(ctx, "csrf_prevention_token").(string); hasCSRF {
		consoleClient.CSRFPreventionToken = csrfToken
	}

	return proxmox.GetConsoleTicket(ctx, consoleClient, node, vmID)
}

// setConsoleResponse handles setting the console response including cookies and JSON response
func (h *VMHandler) setConsoleResponse(w http.ResponseWriter, r *http.Request, res *proxmox.ConsoleAuthResult, node string, vmID int) {
	log := CreateHandlerLogger("VMHandler.setConsoleResponse", r)

	// Get Proxmox URL to determine if WebSocket will be wss
	proxmoxURL := strings.TrimSpace(os.Getenv("PROXMOX_URL"))
	secureCookie := (r.TLS != nil) || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	if proxmoxURL != "" {
		if u, err := url.Parse(proxmoxURL); err == nil && u.Scheme == "https" {
			secureCookie = true
		}
	}

	// Set cookies for noVNC access
	http.SetCookie(w, &http.Cookie{
		Name:     "PVEAuthCookie",
		Value:    res.Ticket,
		Path:     "/",
		HttpOnly: false, // noVNC needs to access this cookie
		Secure:   secureCookie,
		SameSite: http.SameSiteLaxMode,
	})

	if res.CSRFPreventionToken != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     "CSRFPreventionToken",
			Value:    res.CSRFPreventionToken,
			Path:     "/",
			HttpOnly: false, // noVNC needs to access this cookie
			Secure:   secureCookie,
			SameSite: http.SameSiteLaxMode,
		})
	}

	// Determine WebSocket scheme
	wsScheme := "ws"
	if secureCookie {
		wsScheme = "wss"
	}

	// Determine the host for WebSocket connection
	wsHost := r.Host
	if proxmoxURL != "" {
		if u, err := url.Parse(proxmoxURL); err == nil && u.Host != "" {
			// If we're accessing console through our proxy, use our host
			// The actual connection will be proxied through our WebSocket handler
		}
	}

	// Build WebSocket URL that will go through our proxy
	wsURL := fmt.Sprintf("%s://%s/ws/vm/%d?node=%s&port=%s&vncticket=%s",
		wsScheme, wsHost, vmID, node, res.Port, url.QueryEscape(res.Ticket))

	// Prepare response data
	resp := map[string]interface{}{
		"success":      true,
		"ticket":       res.Ticket,
		"port":         res.Port,
		"vncticket":    res.Ticket,
		"vncpassword":  res.VNCPassword,
		"websocket":    wsURL,
		"vmid":         vmID,
		"node":         node,
		"proxmox_base": res.ProxmoxBase,
	}

	if res.CSRFPreventionToken != "" {
		resp["csrf_token"] = res.CSRFPreventionToken
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		log.Error().Err(err).Msg("Failed to encode console response")
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	log.Debug().Interface("response", resp).Msg("Console response sent successfully")
}
