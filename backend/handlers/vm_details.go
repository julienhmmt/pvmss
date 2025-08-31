package handlers

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/gomarkdown/markdown"
	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/security"
	"pvmss/state"
)

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

// VMConsoleWebSocketProxy proxies noVNC websocket connections to Proxmox's vncwebsocket endpoint.
// This function uses HTTP reverse proxy with proper WebSocket upgrade handling.
func (h *VMHandler) VMConsoleWebSocketProxy(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "VMConsoleWebSocketProxy").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	vmIDStr := ps.ByName("vmid")
	node := strings.TrimSpace(r.URL.Query().Get("node"))
	port := strings.TrimSpace(r.URL.Query().Get("port"))
	vncticket := strings.TrimSpace(r.URL.Query().Get("vncticket"))

	if vmIDStr == "" || node == "" || port == "" || vncticket == "" {
		log.Warn().Msg("Missing required parameters for WebSocket proxy")
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	// Determine Proxmox base URL
	proxmoxURL := strings.TrimSpace(os.Getenv("PROXMOX_URL"))
	if proxmoxURL == "" {
		log.Error().Msg("PROXMOX_URL not configured")
		http.Error(w, "Proxmox URL not configured", http.StatusInternalServerError)
		return
	}

	u, err := url.Parse(proxmoxURL)
	if err != nil || u.Host == "" {
		log.Error().Err(err).Str("proxmoxURL", proxmoxURL).Msg("Invalid Proxmox URL")
		http.Error(w, "Invalid Proxmox URL", http.StatusInternalServerError)
		return
	}

	targetScheme := u.Scheme
	if targetScheme == "" {
		targetScheme = "https"
	}

	// Build target WebSocket URL for Proxmox
	targetPath := fmt.Sprintf("/api2/json/nodes/%s/qemu/%s/vncwebsocket", node, vmIDStr)
	targetQuery := url.Values{}
	targetQuery.Set("port", port)
	targetQuery.Set("vncticket", vncticket)

	// Create reverse proxy for WebSocket
	targetHost := &url.URL{Scheme: targetScheme, Host: u.Host}
	proxy := httputil.NewSingleHostReverseProxy(targetHost)

	// Configure TLS settings
	insecureSkip := strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL")) == "false"
	proxy.Transport = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: insecureSkip},
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// Customize director to handle WebSocket upgrade properly
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = targetScheme
		req.URL.Host = u.Host
		req.URL.Path = targetPath
		req.URL.RawQuery = targetQuery.Encode()
		req.Host = u.Host

		// Forward all original headers
		for key, values := range r.Header {
			req.Header[key] = values
		}

		// Add PVEAuthCookie if available
		if c, err := r.Cookie("PVEAuthCookie"); err == nil && c != nil && c.Value != "" {
			req.Header.Set("Cookie", "PVEAuthCookie="+c.Value)
			log.Debug().Msg("Forwarding PVEAuthCookie to Proxmox WebSocket")
		}

		// Ensure WebSocket upgrade headers are preserved
		if upgrade := r.Header.Get("Upgrade"); upgrade != "" {
			req.Header.Set("Upgrade", upgrade)
		}
		if connection := r.Header.Get("Connection"); connection != "" {
			req.Header.Set("Connection", connection)
		}
		if wsKey := r.Header.Get("Sec-WebSocket-Key"); wsKey != "" {
			req.Header.Set("Sec-WebSocket-Key", wsKey)
		}
		if wsVersion := r.Header.Get("Sec-WebSocket-Version"); wsVersion != "" {
			req.Header.Set("Sec-WebSocket-Version", wsVersion)
		}
		if wsProtocol := r.Header.Get("Sec-WebSocket-Protocol"); wsProtocol != "" {
			req.Header.Set("Sec-WebSocket-Protocol", wsProtocol)
		}
	}

	// Set up error handling
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		log.Error().Err(err).Str("path", r.URL.Path).Msg("WebSocket proxy error")
		http.Error(w, "WebSocket proxy error", http.StatusBadGateway)
	}

	log.Info().
		Str("vmid", vmIDStr).
		Str("node", node).
		Str("target", targetHost.String()+targetPath).
		Msg("Proxying WebSocket connection to Proxmox")

	// Serve the WebSocket proxy
	proxy.ServeHTTP(w, r)
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
	wsProxyURL := fmt.Sprintf("/api/console/qemu/%s/ws?node=%s&port=%s&vncticket=%s",
		vmID,
		url.QueryEscape(node),
		url.QueryEscape(port),
		url.QueryEscape(vncticket))

	log.Debug().
		Str("vmid", vmID).
		Str("node", node).
		Str("ws_proxy_url", wsProxyURL).
		Msg("Serving noVNC console page with proxied websocket")

	// Render the console page with the WebSocket URL
	data := map[string]interface{}{
		"Title":      fmt.Sprintf("Console for %s", vmname),
		"WSProxyURL": wsProxyURL,
		"VMID":       vmID,
		"VMName":     vmname,
		"Node":       node,
		"Port":       port,
		"VNCTicket":  vncticket,
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

	// Try multiple authentication strategies in order of preference

	// Strategy 1: Use existing PVE cookie from session (for authenticated users)
	if pveAuthCookie, hasCookie := sessionManager.Get(r.Context(), "pve_auth_cookie").(string); hasCookie && pveAuthCookie != "" {
		cookiePrefix := pveAuthCookie
		if len(cookiePrefix) > 10 {
			cookiePrefix = cookiePrefix[:10]
		}
		log.Debug().Str("cookie_prefix", cookiePrefix).Msg("Attempting console access with stored PVE auth cookie")
		if res, err := h.tryConsoleWithCookie(r.Context(), pveAuthCookie, sessionManager, node, vmID); err == nil {
			log.Info().Msg("Console ticket generated successfully with stored cookie")
			h.setConsoleResponse(w, r, res, node, vmID)
			return
		} else {
			log.Warn().Err(err).Msg("Console access failed with stored cookie, trying fallback")
			// Clear invalid cookie from session
			sessionManager.Remove(r.Context(), "pve_auth_cookie")
			sessionManager.Remove(r.Context(), "csrf_prevention_token")
		}
	} else {
		log.Debug().Bool("has_cookie", hasCookie).Msg("No valid PVE auth cookie found in session")
	}

	// Strategy 2: Create fresh authentication session for the user
	if username, hasUsername := sessionManager.Get(r.Context(), "username").(string); hasUsername && username != "" {
		log.Debug().Str("username", username).Msg("Attempting to create fresh console session for user")
		if res, err := h.tryConsoleWithUserAuth(r.Context(), username, sessionManager, node, vmID); err == nil {
			log.Info().Str("username", username).Msg("Console ticket generated successfully with fresh user auth")
			h.setConsoleResponse(w, r, res, node, vmID)
			return
		} else {
			log.Warn().Err(err).Str("username", username).Msg("Failed to create fresh console session")
		}
	}

	// All strategies failed - provide detailed debugging information
	username, _ := sessionManager.Get(r.Context(), "username").(string)
	isAuthenticated, _ := sessionManager.Get(r.Context(), "authenticated").(bool)
	isAdmin, _ := sessionManager.Get(r.Context(), "is_admin").(bool)

	log.Error().
		Str("username", username).
		Bool("authenticated", isAuthenticated).
		Bool("is_admin", isAdmin).
		Str("node", node).
		Int("vmid", vmID).
		Msg("All console authentication strategies failed")

	// Check if user is even authenticated
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
		"details": fmt.Sprintf("User '%s' could not access console for VM %d on node %s. Please log out and log back in to refresh your console session, or contact your administrator to verify console permissions.", username, vmID, node),
		"troubleshooting": map[string]interface{}{
			"username":      username,
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
func (h *VMHandler) tryConsoleWithCookie(ctx context.Context, pveAuthCookie string, sessionManager *scs.SessionManager, node string, vmID int) (*proxmox.ConsoleAuthResult, error) {
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

// tryConsoleWithUserAuth attempts to create a fresh console session using the user's stored username
func (h *VMHandler) tryConsoleWithUserAuth(ctx context.Context, username string, sessionManager *scs.SessionManager, node string, vmID int) (*proxmox.ConsoleAuthResult, error) {
	log := logger.Get().With().
		Str("handler", "tryConsoleWithUserAuth").
		Str("username", username).
		Str("node", node).
		Int("vmid", vmID).
		Logger()

	// Get the main Proxmox client to create a fresh console session
	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not available for fresh console authentication")
		return nil, fmt.Errorf("proxmox client not available")
	}

	// Get the Proxmox URL for creating a new client
	proxmoxURL := strings.TrimSpace(os.Getenv("PROXMOX_URL"))
	insecureSkip := strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL")) == "false"

	if proxmoxURL == "" {
		log.Error().Msg("PROXMOX_URL not configured")
		return nil, fmt.Errorf("PROXMOX_URL not configured")
	}

	// Create a fresh client for console authentication
	consoleClient, err := proxmox.NewClientCookieAuth(proxmoxURL, insecureSkip)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create fresh console client")
		return nil, fmt.Errorf("failed to create console client: %w", err)
	}

	// Use the main client's authentication to get fresh credentials for console access
	// This leverages the existing authenticated session to create a console-specific session
	if concreteClient, ok := client.(*proxmox.Client); ok {
		if concreteClient.PVEAuthCookie != "" {
			consoleClient.PVEAuthCookie = concreteClient.PVEAuthCookie
			log.Debug().Msg("Using main client's PVE auth cookie for fresh console session")
		}

		// Also check if we have CSRF token from the main client
		if concreteClient.CSRFPreventionToken != "" {
			consoleClient.CSRFPreventionToken = concreteClient.CSRFPreventionToken
		}
	} else {
		log.Warn().Msg("Could not access main client's authentication cookies - using interface only")
	}

	// Attempt to get console ticket with the fresh session
	result, err := proxmox.GetConsoleTicket(ctx, consoleClient, node, vmID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get console ticket with fresh authentication")
		return nil, fmt.Errorf("failed to get console ticket: %w", err)
	}

	// Store the fresh auth cookie in session for future use
	if result.PVEAuthCookie != "" {
		sessionManager.Put(ctx, "pve_auth_cookie", result.PVEAuthCookie)
		log.Debug().Msg("Stored fresh PVE auth cookie in session")
	}

	if result.CSRFPreventionToken != "" {
		sessionManager.Put(ctx, "csrf_prevention_token", result.CSRFPreventionToken)
		log.Debug().Msg("Stored fresh CSRF prevention token in session")
	}

	log.Info().Msg("Successfully created fresh console session")
	return result, nil
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
	router.GET("/api/console/qemu/:vmid/ws", RequireAuthHandleWS(h.VMConsoleWebSocketProxy))
	router.GET("/console/qemu/:vmid", RequireAuthHandle(h.VMConsoleProxyPage))
	router.GET("/pve2/*filepath", RequireAuthHandle(h.ProxmoxAssetProxy))
}
