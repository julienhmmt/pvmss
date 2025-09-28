package handlers

import (
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"pvmss/security"
)

// VMConsoleHTTPProxyHandler - HTTP proxy for Proxmox noVNC (eliminates cross-domain issues)
func (h *VMHandler) VMConsoleHTTPProxyHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("VMConsoleHTTPProxyHandler", r)

	if !IsAuthenticated(r) {
		log.Warn().Msg("Console HTTP proxy request rejected: not authenticated")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get parameters
	vmid := r.URL.Query().Get("vmid")
	node := r.URL.Query().Get("node")
	host := r.URL.Query().Get("host")
	port := r.URL.Query().Get("port")
	ticket := r.URL.Query().Get("ticket")
	scheme := r.URL.Query().Get("scheme")

	log.Info().
		Str("vmid", vmid).
		Str("node", node).
		Str("host", host).
		Str("port", port).
		Str("scheme", scheme).
		Bool("has_ticket", ticket != "").
		Msg("HTTP proxy request parameters")

	if vmid == "" || node == "" || host == "" || port == "" || ticket == "" {
		log.Error().Msg("Missing required HTTP proxy parameters")
		http.Error(w, "Missing required parameters", http.StatusBadRequest)
		return
	}

	// Get authentication from session
	sessionManager := security.GetSession(r)
	if sessionManager == nil {
		log.Error().Msg("Session manager not available")
		http.Error(w, "Session not available", http.StatusUnauthorized)
		return
	}

	vmidInt, _ := strconv.Atoi(vmid)
	sessionKey := fmt.Sprintf("console_session_%d_%s", vmidInt, node)

	consoleSessionData := sessionManager.Get(r.Context(), sessionKey)
	if consoleSessionData == nil {
		log.Error().Str("session_key", sessionKey).Msg("Console session not found or expired")
		http.Error(w, "Console session expired. Please try again.", http.StatusUnauthorized)
		return
	}

	consolePayload, ok := consoleSessionData.(consoleSessionPayload)
	if !ok {
		log.Error().Msg("Invalid console session payload type")
		http.Error(w, "Invalid console session", http.StatusInternalServerError)
		return
	}

	// Check if session has expired
	if time.Now().Unix() > consolePayload.ExpiresAt {
		log.Warn().Msg("Console session expired")
		sessionManager.Remove(r.Context(), sessionKey)
		http.Error(w, "Console session expired. Please try again.", http.StatusUnauthorized)
		return
	}

	authCookie := consolePayload.AuthCookie
	csrfToken := consolePayload.CsrfToken

	log.Info().
		Str("node", node).
		Str("vmid", vmid).
		Str("host", host).
		Str("port", port).
		Bool("has_auth", authCookie != "").
		Msg("Proxying Proxmox noVNC page")

	// Build original Proxmox URL
	proxmoxURL := fmt.Sprintf("%s://%s/?console=kvm&novnc=1&node=%s&vmid=%s&resize=1&path=api2/json/nodes/%s/qemu/%s/vncwebsocket/port/%s/vncticket=%s",
		scheme, host, url.QueryEscape(node), vmid, url.QueryEscape(node), vmid, port, url.QueryEscape(ticket))

	// Create HTTP client with proper TLS settings
	skipTLS := strings.EqualFold(strings.TrimSpace(os.Getenv("PROXMOX_VERIFY_SSL")), "false")
	client := &http.Client{
		Timeout: 30 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: skipTLS,
			},
		},
	}

	// Create request to Proxmox
	req, err := http.NewRequestWithContext(r.Context(), "GET", proxmoxURL, nil)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Proxmox request")
		http.Error(w, "Failed to create request", http.StatusInternalServerError)
		return
	}

	// Add authentication headers
	req.Header.Set("Cookie", fmt.Sprintf("PVEAuthCookie=%s", authCookie))
	if csrfToken != "" {
		req.Header.Set("CSRFPreventionToken", csrfToken)
	}
	req.Header.Set("User-Agent", r.Header.Get("User-Agent"))
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
	req.Header.Set("Accept-Language", r.Header.Get("Accept-Language"))

	log.Info().Str("proxmox_url", proxmoxURL).Msg("Requesting Proxmox noVNC page")

	// Make request to Proxmox
	resp, err := client.Do(req)
	if err != nil {
		log.Error().Err(err).Msg("Failed to request Proxmox noVNC page")
		http.Error(w, "Failed to connect to Proxmox", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		log.Error().Int("status_code", resp.StatusCode).Msg("Proxmox returned error")
		http.Error(w, fmt.Sprintf("Proxmox error: %s", resp.Status), resp.StatusCode)
		return
	}

	// Read Proxmox response
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("Failed to read Proxmox response")
		http.Error(w, "Failed to read response", http.StatusInternalServerError)
		return
	}

	// Modify the HTML to fix WebSocket URLs, asset URLs, and authentication
	htmlContent := string(body)

	// Replace WebSocket URLs to use our proxy (comprehensive patterns)
	htmlContent = strings.ReplaceAll(htmlContent,
		fmt.Sprintf("wss://%s/api2/json", host),
		fmt.Sprintf("ws://%s/vm/console-websocket", r.Host))
	htmlContent = strings.ReplaceAll(htmlContent,
		fmt.Sprintf("ws://%s/api2/json", host),
		fmt.Sprintf("ws://%s/vm/console-websocket", r.Host))

	// Fix legacy websockify URLs (often hardcoded in older noVNC)
	htmlContent = strings.ReplaceAll(htmlContent,
		fmt.Sprintf("ws://%s:5900/websockify", strings.Split(host, ":")[0]),
		fmt.Sprintf("ws://%s/vm/console-websocket", r.Host))
	htmlContent = strings.ReplaceAll(htmlContent,
		fmt.Sprintf("wss://%s:5900/websockify", strings.Split(host, ":")[0]),
		fmt.Sprintf("ws://%s/vm/console-websocket", r.Host))

	// Handle any port-specific websockify URLs
	htmlContent = strings.ReplaceAll(htmlContent, "/websockify", "/vm/console-websocket")

	// Fix noVNC asset URLs to point to our local components (comprehensive patterns)
	htmlContent = strings.ReplaceAll(htmlContent, "/novnc/app/", "/components/noVNC-1.6.0/app/")
	htmlContent = strings.ReplaceAll(htmlContent, "/novnc/vendor/", "/components/noVNC-1.6.0/vendor/")
	htmlContent = strings.ReplaceAll(htmlContent, "/novnc/lib/", "/components/noVNC-1.6.0/lib/")
	htmlContent = strings.ReplaceAll(htmlContent, "'/novnc/", "'/components/noVNC-1.6.0/")
	htmlContent = strings.ReplaceAll(htmlContent, "\"/novnc/", "\"/components/noVNC-1.6.0/")

	// Fix relative path issues that cause /vm/app/ patterns (CRITICAL FIX)
	htmlContent = strings.ReplaceAll(htmlContent, "\"app/", "\"/components/noVNC-1.6.0/app/")
	htmlContent = strings.ReplaceAll(htmlContent, "'app/", "'/components/noVNC-1.6.0/app/")
	htmlContent = strings.ReplaceAll(htmlContent, "/vm/app/", "/components/noVNC-1.6.0/app/")
	htmlContent = strings.ReplaceAll(htmlContent, "/vm/core/", "/components/noVNC-1.6.0/core/")
	htmlContent = strings.ReplaceAll(htmlContent, "/vm/vendor/", "/components/noVNC-1.6.0/vendor/")

	// Fix root-level files (package.json, etc.)
	htmlContent = strings.ReplaceAll(htmlContent, "\"/package.json", "\"/components/noVNC-1.6.0/package.json")
	htmlContent = strings.ReplaceAll(htmlContent, "'/package.json", "'/components/noVNC-1.6.0/package.json")
	htmlContent = strings.ReplaceAll(htmlContent, "/vm/package.json", "/components/noVNC-1.6.0/package.json")

	// CRITICAL: Fix base URL issues that cause incorrect relative paths
	// This handles cases where noVNC JavaScript dynamically constructs URLs
	htmlContent = strings.ReplaceAll(htmlContent, "baseURL = './';", "baseURL = '/components/noVNC-1.6.0/';")
	htmlContent = strings.ReplaceAll(htmlContent, "baseURL = \"./\";", "baseURL = \"/components/noVNC-1.6.0/\";")
	htmlContent = strings.ReplaceAll(htmlContent, "const baseURL = './';", "const baseURL = '/components/noVNC-1.6.0/';")
	htmlContent = strings.ReplaceAll(htmlContent, "const baseURL = \"./\";", "const baseURL = \"/components/noVNC-1.6.0/\";")

	// Fix dynamic URL construction patterns
	htmlContent = strings.ReplaceAll(htmlContent, "+ 'app/", "+ '/components/noVNC-1.6.0/app/")
	htmlContent = strings.ReplaceAll(htmlContent, "+ \"app/", "+ \"/components/noVNC-1.6.0/app/")

	// Fix specific file mappings for noVNC 1.6.0 vs Proxmox expectations
	htmlContent = strings.ReplaceAll(htmlContent, "/components/noVNC-1.6.0/app.js", "/components/noVNC-1.6.0/app/ui.js")

	// Inject comprehensive URL rewriting and authentication
	authScript := fmt.Sprintf(`
	<script>
	// Inject authentication for WebSocket connections
	window.PVMSS_AUTH = {
		host: '%s',
		node: '%s', 
		vmid: '%s',
		port: '%s',
		ticket: '%s',
		websocket_url: 'ws://%s/vm/console-websocket?host=%s&node=%s&vmid=%s&port=%s&ticket=%s'
	};
	
	// Override fetch to redirect asset requests  
	const originalFetch = window.fetch;
	window.fetch = function(url, options) {
		console.log('PVMSS DEBUG: ===== FETCH OVERRIDE CALLED =====');
		console.log('PVMSS DEBUG: Original fetch URL:', url);
		console.log('PVMSS DEBUG: URL type:', typeof url);
		
		if (typeof url === 'string') {
			let newUrl = url;
			let redirected = false;
			
			console.log('PVMSS DEBUG: Checking URL patterns for:', url);
			console.log('PVMSS DEBUG: url.startsWith("http"):', url.startsWith('http'));
			console.log('PVMSS DEBUG: url.startsWith("/"):', url.startsWith('/'));
			console.log('PVMSS DEBUG: url.startsWith("data:"):', url.startsWith('data:'));
			
			// Fix relative paths that don't start with / or protocol
			if (!url.startsWith('http') && !url.startsWith('/') && !url.startsWith('data:')) {
				console.log('PVMSS DEBUG: Processing relative URL');
				// This handles "app/locale/fr.json" -> "/components/noVNC-1.6.0/app/locale/fr.json"
				if (url.startsWith('app/')) {
					newUrl = '/components/noVNC-1.6.0/' + url;
					redirected = true;
					console.log('PVMSS DEBUG: REDIRECTED relative app path:', url, '->', newUrl);
				}
				// This handles "./package.json" -> "/components/noVNC-1.6.0/package.json"  
				else if (url === './package.json' || url === 'package.json') {
					newUrl = '/components/noVNC-1.6.0/package.json';
					redirected = true;
					console.log('PVMSS DEBUG: REDIRECTED package.json:', url, '->', newUrl);
				}
			}
			
			// Fix absolute paths that contain /vm/
			if (url.includes('/vm/app/') || url.includes('/vm/core/') || url.includes('/vm/vendor/')) {
				newUrl = url.replace('/vm/app/', '/components/noVNC-1.6.0/app/')
						.replace('/vm/core/', '/components/noVNC-1.6.0/core/')
						.replace('/vm/vendor/', '/components/noVNC-1.6.0/vendor/');
				redirected = true;
				console.log('PVMSS DEBUG: REDIRECTED absolute path:', url, '->', newUrl);
			}
			
			// Fix /vm/package.json
			if (url.includes('/vm/package.json')) {
				newUrl = url.replace('/vm/package.json', '/components/noVNC-1.6.0/package.json');
				redirected = true;
				console.log('PVMSS DEBUG: REDIRECTED absolute package.json:', url, '->', newUrl);
			}
			
			if (!redirected) {
				console.log('PVMSS DEBUG: NO REDIRECT applied for:', url);
			}
			
			console.log('PVMSS DEBUG: Final URL for fetch:', newUrl);
			return originalFetch.call(this, newUrl, options);
		}
		console.log('PVMSS DEBUG: Non-string URL, calling original fetch');
		return originalFetch.call(this, url, options);
	};
	
	// Override noVNC WebSocket URL creation to force our proxy
	if (typeof window.WebSocket !== 'undefined') {
		const originalWebSocket = window.WebSocket;
		window.WebSocket = function(url, protocols) {
			console.log('PVMSS DEBUG: ===== WebSocket connection attempt =====');
			console.log('PVMSS DEBUG: Original URL:', url);
			console.log('PVMSS DEBUG: Protocols:', protocols);
			console.log('PVMSS DEBUG: Current PVMSS_AUTH:', window.PVMSS_AUTH);
			
			let newUrl = url;
			let shouldRedirect = false;
			
			// Comprehensive WebSocket URL redirection patterns
			if (url.includes('/websockify') || 
				url.includes('api2/json') || 
				url.includes('vncwebsocket') ||
				url.includes('192.168.1.1') ||
				url.includes(':5900') ||
				url.startsWith('wss://') ||
				(url.startsWith('ws://') && !url.includes('/vm/console-websocket'))) {
				
				shouldRedirect = true;
				newUrl = window.PVMSS_AUTH.websocket_url;
				console.log('PVMSS DEBUG: REDIRECTING WebSocket');
				console.log('PVMSS DEBUG: From:', url);
				console.log('PVMSS DEBUG: To:', newUrl);
			} else {
				console.log('PVMSS DEBUG: NOT redirecting WebSocket - no matching pattern');
			}
			
			const socket = new originalWebSocket(newUrl, protocols);
			
			// Add event listeners to monitor connection
			socket.addEventListener('open', function(event) {
				console.log('PVMSS DEBUG: WebSocket opened successfully to:', newUrl);
			});
			
			socket.addEventListener('error', function(event) {
				console.log('PVMSS DEBUG: WebSocket error on:', newUrl, event);
			});
			
			socket.addEventListener('close', function(event) {
				console.log('PVMSS DEBUG: WebSocket closed:', event.code, event.reason);
			});
			
			return socket;
		};
		
		// Also override if noVNC uses a different connection method
		setTimeout(function() {
			if (typeof window.RFB !== 'undefined' && window.RFB.prototype.connect) {
				console.log('PVMSS DEBUG: RFB prototype found, overriding connect method');
				const originalConnect = window.RFB.prototype.connect;
				window.RFB.prototype.connect = function(url, options) {
					console.log('PVMSS DEBUG: RFB connect attempt:', url);
					if (url && (url.includes('192.168.1.1') || url.includes('vncwebsocket'))) {
						url = window.PVMSS_AUTH.websocket_url;
						console.log('PVMSS DEBUG: Redirected RFB URL to:', url);
					}
					return originalConnect.call(this, url, options);
				};
			} else {
				console.log('PVMSS DEBUG: RFB prototype not found yet');
			}
		}, 1000);
	}
	
	console.log('PVMSS: Authentication data and URL override injected');
	</script>`, host, node, vmid, port, ticket, r.Host, host, node, vmid, port, url.QueryEscape(ticket))

	// Insert before closing head tag
	htmlContent = strings.Replace(htmlContent, "</head>", authScript+"</head>", 1)

	log.Info().
		Int("original_size", len(body)).
		Int("modified_size", len(htmlContent)).
		Int("websocket_replacements", strings.Count(string(body), "wss://"+host)+strings.Count(string(body), "ws://"+host)+strings.Count(string(body), "/websockify")).
		Int("asset_replacements", strings.Count(string(body), "/novnc/")+strings.Count(string(body), "\"app/")+strings.Count(string(body), "/package.json")).
		Int("websockify_fixes", strings.Count(string(body), "/websockify")).
		Str("sample_websocket_urls", fmt.Sprintf("Found ws://%s patterns", host)).
		Msg("Successfully modified Proxmox noVNC page")

	// Copy headers from Proxmox response
	for key, values := range resp.Header {
		if key != "Content-Length" && key != "Content-Encoding" {
			for _, value := range values {
				w.Header().Add(key, value)
			}
		}
	}

	// Set correct content length for modified content
	w.Header().Set("Content-Length", strconv.Itoa(len(htmlContent)))
	w.Header().Set("Content-Type", "text/html; charset=utf-8")

	// Write the modified HTML
	w.WriteHeader(200)
	w.Write([]byte(htmlContent))

	log.Info().Msg("Successfully served proxied noVNC page")
}
