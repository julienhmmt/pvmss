package handlers

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// VMStateManager defines the minimal state contract needed by VM details.
type VMStateManager interface {
	GetProxmoxClient() proxmox.ClientInterface
	GetSettings() *state.AppSettings
}

// VMHandler handles VM-related pages and API endpoints
type VMHandler struct {
	stateManager VMStateManager
}

// VMConsoleWebSocketProxy proxies noVNC websocket connections to Proxmox's vncwebsocket endpoint.
// It forwards required headers and attaches the PVEAuthCookie from the incoming request to the upstream request.
func (h *VMHandler) VMConsoleWebSocketProxy(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	vmIDStr := ps.ByName("vmid")
	node := strings.TrimSpace(r.URL.Query().Get("node"))
	port := strings.TrimSpace(r.URL.Query().Get("port"))
	vncticket := strings.TrimSpace(r.URL.Query().Get("vncticket"))
	if vmIDStr == "" || node == "" || port == "" || vncticket == "" {
		http.Error(w, "node, vmid, port and vncticket parameters are required", http.StatusBadRequest)
		return
	}

	// Determine Proxmox base URL (prefer client when available, fallback to env)
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

	targetScheme := u.Scheme
	if targetScheme == "" {
		targetScheme = "https"
	}

	// Build target vncwebsocket path+query
	targetPath := fmt.Sprintf("/api2/json/nodes/%s/qemu/%s/vncwebsocket", node, vmIDStr)
	targetQuery := url.Values{}
	targetQuery.Set("port", port)
	targetQuery.Set("vncticket", vncticket)

	// Prepare reverse proxy to Proxmox host
	targetHost := &url.URL{Scheme: targetScheme, Host: u.Host}
	proxy := httputil.NewSingleHostReverseProxy(targetHost)

	// Ensure TLS behavior matches env setting
	insecureSkip := strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL")) == "false"
	proxy.Transport = &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		TLSClientConfig:       &tls.Config{InsecureSkipVerify: insecureSkip},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	// Customize director to set exact upstream URL and attach cookie
	origDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		origDirector(req)
		req.URL.Scheme = targetScheme
		req.URL.Host = u.Host
		req.URL.Path = targetPath
		req.URL.RawQuery = targetQuery.Encode()
		req.Host = u.Host
		// Attach PVEAuthCookie from incoming request to upstream
		if c, err := r.Cookie("PVEAuthCookie"); err == nil && c != nil && c.Value != "" {
			req.Header.Set("Cookie", "PVEAuthCookie="+c.Value)
		}
		// Ensure Upgrade headers are preserved for WebSocket
		if strings.EqualFold(r.Header.Get("Connection"), "upgrade") {
			req.Header.Set("Connection", r.Header.Get("Connection"))
			req.Header.Set("Upgrade", r.Header.Get("Upgrade"))
			if v := r.Header.Get("Sec-WebSocket-Version"); v != "" {
				req.Header.Set("Sec-WebSocket-Version", v)
			}
			if v := r.Header.Get("Sec-WebSocket-Key"); v != "" {
				req.Header.Set("Sec-WebSocket-Key", v)
			}
			if v := r.Header.Get("Sec-WebSocket-Protocol"); v != "" {
				req.Header.Set("Sec-WebSocket-Protocol", v)
			}
		}
	}

	// Serve the proxied WebSocket/HTTP stream
	proxy.ServeHTTP(w, r)
}

// VMConsoleProxyPage proxies the Proxmox noVNC page under our origin.
func (h *VMHandler) VMConsoleProxyPage(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	vmID := ps.ByName("vmid")
	node := strings.TrimSpace(r.URL.Query().Get("node"))
	vmname := strings.TrimSpace(r.URL.Query().Get("vmname"))
	port := strings.TrimSpace(r.URL.Query().Get("port"))
	vncticket := strings.TrimSpace(r.URL.Query().Get("vncticket"))
	if vmID == "" || node == "" || port == "" || vncticket == "" {
		http.Error(w, "vmid, node, port and vncticket are required", http.StatusBadRequest)
		return
	}

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

	// Build direct Proxmox noVNC page URL and redirect the client there.
	// The "path" parameter must point to Proxmox's own vncwebsocket endpoint since we are leaving our origin.
	wsPathProxmox := "/api2/json/nodes/" + node + "/qemu/" + vmID + "/vncwebsocket?port=" + url.QueryEscape(port) + "&vncticket=" + url.QueryEscape(vncticket)
	q := url.Values{}
	q.Set("console", "kvm")
	q.Set("novnc", "1")
	q.Set("vmid", vmID)
	if vmname != "" {
		q.Set("vmname", vmname)
	}
	q.Set("node", node)
	q.Set("resize", "1")
	q.Set("path", wsPathProxmox)

	redirectURL := &url.URL{Scheme: u.Scheme, Host: u.Host, Path: "/", RawQuery: q.Encode()}
	http.Redirect(w, r, redirectURL.String(), http.StatusSeeOther)
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
	vmIDStr := ps.ByName("vmid")
	node := r.URL.Query().Get("node")
	if vmIDStr == "" || node == "" {
		http.Error(w, "vmid and node parameters are required", http.StatusBadRequest)
		return
	}

	vmID, err := strconv.Atoi(vmIDStr)
	if err != nil {
		http.Error(w, "Invalid vmid", http.StatusBadRequest)
		return
	}

	// Always use dedicated console user to obtain cookie + ticket
	consoleUser := strings.TrimSpace(os.Getenv("PROXMOX_CONSOLE_USER"))
	consolePass := strings.TrimSpace(os.Getenv("PROXMOX_CONSOLE_PASSWORD"))
	proxmoxURL := strings.TrimSpace(os.Getenv("PROXMOX_URL"))
	insecureSkip := strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL")) == "false"

	if consoleUser == "" || consolePass == "" || proxmoxURL == "" {
		logger.Get().Error().Msg("Console credentials or PROXMOX_URL missing; set PROXMOX_CONSOLE_USER and PROXMOX_CONSOLE_PASSWORD")
		http.Error(w, "Console credentials not configured", http.StatusInternalServerError)
		return
	}

	consoleClient, cerr := proxmox.NewClientCookieAuth(proxmoxURL, insecureSkip)
	if cerr != nil {
		logger.Get().Error().Err(cerr).Msg("Failed to create console Proxmox client")
		http.Error(w, "Failed to create console client", http.StatusInternalServerError)
		return
	}
	if err := consoleClient.Login(r.Context(), consoleUser, consolePass, ""); err != nil {
		logger.Get().Error().Err(err).Msg("Console Proxmox login failed")
		http.Error(w, "Console authentication failed", http.StatusUnauthorized)
		return
	}

	raw, err := consoleClient.GetVNCProxy(r.Context(), node, vmID)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get VNC ticket")
		http.Error(w, "Failed to get VNC ticket", http.StatusInternalServerError)
		return
	}

	// Extract nested data { data: { port, ticket, upid } }
	var (
		portVal   string
		ticketVal string
	)
	if d, ok := raw["data"].(map[string]interface{}); ok {
		// port may be number or string; normalize to string
		switch pv := d["port"].(type) {
		case string:
			portVal = pv
		case float64:
			portVal = strconv.Itoa(int(pv))
		case int:
			portVal = strconv.Itoa(pv)
		case json.Number:
			portVal = pv.String()
		}
		if t, ok := d["ticket"].(string); ok {
			ticketVal = t
		}
	}
	if portVal == "" || ticketVal == "" {
		logger.Get().Error().Interface("raw", raw).Msg("Unexpected VNC proxy response; missing port or ticket")
		http.Error(w, "Invalid VNC ticket response", http.StatusBadGateway)
		return
	}

	// Compute proxmox_base (scheme://host)
	proxmoxBase := proxmoxURL
	if u, uErr := url.Parse(strings.TrimSpace(consoleClient.GetApiUrl())); uErr == nil {
		u.Path = ""
		proxmoxBase = u.Scheme + "://" + u.Host
	}

	// Set PVEAuthCookie for browser so subsequent proxied requests carry auth
	secureCookie := (r.TLS != nil) || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https")
	if consoleClient.PVEAuthCookie != "" {
		cookie := &http.Cookie{
			Name:     "PVEAuthCookie",
			Value:    consoleClient.PVEAuthCookie,
			Path:     "/",
			Secure:   secureCookie,
			HttpOnly: false,
			// Use SameSite=None only when Secure (HTTPS). Otherwise browsers will reject the cookie.
			// On HTTP dev, prefer Lax so the cookie is accepted and sent to same-origin WS proxy.
			SameSite: func() http.SameSite {
				if secureCookie {
					return http.SameSiteNoneMode
				}
				return http.SameSiteLaxMode
			}(),
			Expires: time.Now().Add(10 * time.Minute),
		}
		// Optional: allow operators to set cookie domain so Proxmox host can receive it on redirect
		if cd := strings.TrimSpace(os.Getenv("PROXMOX_COOKIE_DOMAIN")); cd != "" {
			cookie.Domain = cd
		}
		http.SetCookie(w, cookie)
	}

	// Build normalized, flat response for the frontend
	resp := map[string]interface{}{
		"port":         portVal,
		"vncticket":    ticketVal,
		"ticket":       ticketVal,
		"node":         node,
		"vmid":         vmID,
		"proxmox_base": proxmoxBase,
	}
	if consoleClient.CSRFPreventionToken != "" {
		resp["csrf_prevention_token"] = consoleClient.CSRFPreventionToken
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
	router.GET("/vm/details/:id", h.VMDetailsHandler)
	router.POST("/vm/update/description", h.UpdateVMDescriptionHandler)
	router.POST("/vm/update/tags", h.UpdateVMTagsHandler)
	router.POST("/vm/action", h.VMActionHandler)
	router.GET("/api/console/qemu/:vmid", h.VMConsoleHandler)
	router.GET("/api/console/qemu/:vmid/ws", h.VMConsoleWebSocketProxy)
	router.GET("/console/qemu/:vmid", h.VMConsoleProxyPage)
	router.GET("/pve2/*filepath", h.ProxmoxAssetProxy)
}

// VMDetailsHandlerFunc is a wrapper function for compatibility with existing code
func VMDetailsHandlerFunc(w http.ResponseWriter, r *http.Request) {
	// Backward-compat wrapper: redirect to the canonical route
	vmid := r.URL.Query().Get("vmid")
	if vmid == "" {
		localizer := i18n.GetLocalizer(r)
		http.Error(w, i18n.Localize(localizer, "Error.Generic"), http.StatusBadRequest)
		return
	}
	http.Redirect(w, r, "/vm/details/"+vmid, http.StatusSeeOther)
}
