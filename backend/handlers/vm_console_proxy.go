package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/julienschmidt/httprouter"
	"github.com/rs/zerolog"
)

// VMConsoleProxyHandler proxies requests to Proxmox noVNC interface and serves static assets
func (h *VMHandler) VMConsoleProxyHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := CreateHandlerLogger("VMConsoleProxyHandler", r)

	log.Info().Str("path", r.URL.Path).Str("query", r.URL.RawQuery).Msg("Console proxy handler called")

	// Extract additional path from wildcard route
	assetFilepath := ps.ByName("filepath")
	if assetFilepath != "" {
		// This is a static asset request via wildcard route
		serveLocalNoVNCAssetFromPath(w, r, log, assetFilepath)
		return
	}

	// Handle static asset requests (JS, CSS, etc.) from local noVNC
	if isStaticAsset(r.URL.Path) {
		serveLocalNoVNCAsset(w, r, log)
		return
	}

	if !IsAuthenticated(r) {
		log.Warn().Msg("Console proxy request rejected: not authenticated")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get VM details from query params
	vmid := r.URL.Query().Get("vmid")
	node := r.URL.Query().Get("node")
	host := r.URL.Query().Get("host")
	port := r.URL.Query().Get("port")
	ticket := r.URL.Query().Get("ticket")
	scheme := r.URL.Query().Get("scheme")

	// Debug query parameters
	log.Info().
		Str("raw_query", r.URL.RawQuery).
		Interface("all_params", r.URL.Query()).
		Msg("Console proxy query parameter debug")

	log.Info().
		Str("vmid", vmid).
		Str("node", node).
		Str("host", host).
		Str("port", port).
		Bool("has_ticket", ticket != "").
		Str("client_ip", r.RemoteAddr).
		Str("user_agent", r.Header.Get("User-Agent")).
		Msg("Console proxy main request parameters")

	if vmid == "" || node == "" {
		log.Error().Str("vmid", vmid).Str("node", node).Msg("VM ID and node are required")
		http.Error(w, "VM ID and node are required", http.StatusBadRequest)
		return
	}

	if host == "" || port == "" || ticket == "" {
		log.Error().
			Str("host", host).
			Str("port", port).
			Bool("has_ticket", ticket != "").
			Msg("Missing required console parameters - host, port, or ticket")
		http.Error(w, "Missing console parameters", http.StatusBadRequest)
		return
	}

	// If this is the main console request (not a static asset), serve our custom noVNC page
	serveCustomNoVNCPage(w, r, log, vmid, node, host, port, ticket, scheme)
}

// serveCustomNoVNCPage serves a customized noVNC page configured for Proxmox
func serveCustomNoVNCPage(w http.ResponseWriter, r *http.Request, log zerolog.Logger, vmid, node, host, port, ticket, scheme string) {
	log.Info().
		Str("vmid", vmid).
		Str("node", node).
		Str("host", host).
		Str("port", port).
		Bool("has_ticket", ticket != "").
		Msg("Serving custom noVNC page for console access")

	// Determine WebSocket scheme
	wsScheme := "wss"
	if strings.EqualFold(strings.TrimSpace(scheme), "http") {
		wsScheme = "ws"
	}

	if wsScheme == "ws" {
		log.Info().Msg("Using insecure WebSocket (ws://) per scheme parameter")
	} else {
		log.Info().Msg("Using secure WebSocket (wss://) per scheme parameter")
	}

	// Use local WebSocket proxy instead of direct Proxmox connection to avoid cross-domain auth issues
	websocketURL := fmt.Sprintf("ws://localhost:50000/vm/console-websocket?host=%s&node=%s&vmid=%s&port=%s&ticket=%s",
		url.QueryEscape(host), url.QueryEscape(node), url.QueryEscape(vmid), url.QueryEscape(port), url.QueryEscape(ticket))

	log.Info().
		Str("websocket_url", websocketURL).
		Msg("Generated WebSocket URL for Proxmox VNC connection")

	// Set headers
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	// Create custom noVNC HTML page
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <title>VM Console - %s</title>
    <link rel="icon" type="image/x-icon" href="/components/noVNC-1.6.0/app/images/icons/novnc.ico">
    <link rel="stylesheet" href="/components/noVNC-1.6.0/app/styles/base.css">
    <script type="module" crossorigin="anonymous" src="/components/noVNC-1.6.0/app/ui.js"></script>
</head>
<body>
    <div id="noVNC_container">
        <div id="noVNC_status_bar">
            <div id="noVNC_left_dummy_elem"></div>
            <div id="noVNC_status">Loading</div>
            <div id="noVNC_buttons">
                <input type="button" value="Send CtrlAltDel" id="sendCtrlAltDelButton">
                <input type="button" value="Send Tab" id="sendTabButton"> 
                <input type="button" value="Send Esc" id="sendEscButton">
            </div>
        </div>
        <div id="noVNC_screen">
            <canvas id="noVNC_canvas" width="640" height="20">
                Canvas not supported.
            </canvas>
        </div>
    </div>

    <script type="module">
        import RFB from '/components/noVNC-1.6.0/core/rfb.js';

        let rfb;

        function connectVNC() {
            const target = document.getElementById('noVNC_canvas');
            const url = '%s';
            
            console.log('Connecting to:', url);
            document.getElementById('noVNC_status').textContent = 'Connecting...';
            
            rfb = new RFB(target, url, {
                credentials: {
                    password: ""
                }
            });

            rfb.addEventListener("connect", () => {
                document.getElementById('noVNC_status').textContent = 'Connected';
                console.log('VNC Connected successfully');
            });
            
            rfb.addEventListener("disconnect", (e) => {
                document.getElementById('noVNC_status').textContent = 'Disconnected: ' + (e.detail.reason || 'Unknown reason');
                console.error('VNC Disconnected:', e.detail);
            });
            
            rfb.addEventListener("credentialsrequired", () => {
                document.getElementById('noVNC_status').textContent = 'Authentication Required';
                console.log('VNC credentials required');
            });

            rfb.scaleViewport = true;
            rfb.resizeSession = true;
        }

        // Button handlers
        document.getElementById('sendCtrlAltDelButton').onclick = () => {
            if (rfb) rfb.sendCtrlAltDel();
        };
        document.getElementById('sendTabButton').onclick = () => {
            if (rfb) rfb.sendKey(0xff09, "Tab");
        };
        document.getElementById('sendEscButton').onclick = () => {
            if (rfb) rfb.sendKey(0xff1b, "Escape");
        };

        // Auto-connect on page load
        window.addEventListener('load', connectVNC);
    </script>
</body>
</html>`, vmid, websocketURL)

	w.Write([]byte(html))
}

// isStaticAsset checks if the request path is for a static asset (JS, CSS, images, etc.)
func isStaticAsset(path string) bool {
	staticExtensions := []string{".js", ".css", ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".woff", ".woff2", ".ttf", ".eot"}

	// Also check for common noVNC paths
	staticPaths := []string{"/lib/", "/vendor/", "/core/", "/app/", "/vnc/", "/images/"}

	// Check file extensions
	for _, ext := range staticExtensions {
		if strings.HasSuffix(strings.ToLower(path), ext) {
			return true
		}
	}

	// Check common static paths
	for _, staticPath := range staticPaths {
		if strings.Contains(path, staticPath) {
			return true
		}
	}

	return false
}

// serveLocalNoVNCAsset serves static assets from the local noVNC directory
func serveLocalNoVNCAsset(w http.ResponseWriter, r *http.Request, log zerolog.Logger) {
	localPath, relativePath, err := resolveNoVNCAssetPath(r.URL.Path)
	if err != nil {
		log.Warn().Err(err).Str("requested_path", r.URL.Path).Msg("Failed to resolve noVNC asset path")
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	serveResolvedNoVNCAsset(w, r, log, localPath, relativePath)
}

// serveLocalNoVNCAssetFromPath serves static assets from wildcard route
func serveLocalNoVNCAssetFromPath(w http.ResponseWriter, r *http.Request, log zerolog.Logger, assetPath string) {
	localPath, relativePath, err := resolveNoVNCAssetPath(assetPath)
	if err != nil {
		log.Warn().Err(err).Str("requested_path", assetPath).Msg("Failed to resolve noVNC asset path (wildcard)")
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	serveResolvedNoVNCAsset(w, r, log, localPath, relativePath)
}

// serveResolvedNoVNCAsset performs security checks and serves the resolved asset
func serveResolvedNoVNCAsset(w http.ResponseWriter, r *http.Request, log zerolog.Logger, localPath, relativePath string) {
	log.Info().Str("relative_path", relativePath).Str("local_path", localPath).Msg("Serving local noVNC asset")

	absPath, err := filepath.Abs(localPath)
	if err != nil {
		log.Error().Err(err).Str("path", localPath).Msg("Failed to get absolute path")
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	novncDir, err := filepath.Abs(filepath.Join("frontend", "components", "noVNC-1.6.0"))
	if err != nil {
		log.Error().Err(err).Msg("Failed to get noVNC directory path")
		http.Error(w, "Configuration error", http.StatusInternalServerError)
		return
	}

	if !strings.HasPrefix(absPath, novncDir) {
		log.Warn().Str("requested_path", absPath).Str("allowed_dir", novncDir).Msg("Path traversal attempt blocked")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		log.Warn().Str("path", localPath).Msg("Static asset not found")
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	setContentType(w, localPath)
	http.ServeFile(w, r, localPath)
}

// NoVNCStaticHandler serves static noVNC assets directly (legacy /novnc/ path)
func (h *VMHandler) NoVNCStaticHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := CreateHandlerLogger("NoVNCStaticHandler", r)

	requestedPath := ps.ByName("filepath")
	if requestedPath == "" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	serveLocalNoVNCAssetFromPath(w, r, log, requestedPath)
}

// NoVNCComponentsHandler serves static noVNC assets from /components/noVNC-1.6.0/ path
func (h *VMHandler) NoVNCComponentsHandler(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	log := CreateHandlerLogger("NoVNCComponentsHandler", r)

	requestedPath := ps.ByName("filepath")
	if requestedPath == "" {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Direct mapping: /components/noVNC-1.6.0/path -> frontend/components/noVNC-1.6.0/path
	localPath := filepath.Join("frontend", "components", "noVNC-1.6.0", requestedPath)

	// Security check for path traversal
	absPath, err := filepath.Abs(localPath)
	if err != nil {
		log.Error().Err(err).Str("path", localPath).Msg("Failed to get absolute path")
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	novncDir, err := filepath.Abs(filepath.Join("frontend", "components", "noVNC-1.6.0"))
	if err != nil {
		log.Error().Err(err).Msg("Failed to get noVNC directory path")
		http.Error(w, "Configuration error", http.StatusInternalServerError)
		return
	}

	if !strings.HasPrefix(absPath, novncDir) {
		log.Warn().Str("requested_path", absPath).Str("allowed_dir", novncDir).Msg("Path traversal attempt blocked")
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Check if file exists
	if _, err := os.Stat(localPath); os.IsNotExist(err) {
		log.Warn().Str("path", localPath).Msg("Static asset not found")
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	log.Info().Str("serving_path", localPath).Msg("Serving noVNC component asset")
	setContentType(w, localPath)
	http.ServeFile(w, r, localPath)
}

// resolveNoVNCAssetPath normalizes different request variants to local filesystem paths
func resolveNoVNCAssetPath(rawPath string) (string, string, error) {
	if rawPath == "" {
		return "", "", errors.New("empty path")
	}

	// Remove query parameters and leading slashes
	path := rawPath
	if idx := strings.Index(path, "?"); idx >= 0 {
		path = path[:idx]
	}
	path = strings.TrimSpace(path)
	path = strings.TrimPrefix(path, "/")

	// Remove proxy-specific prefixes
	path = strings.TrimPrefix(path, "vm/console-proxy/")
	path = strings.TrimPrefix(path, "/")

	// Reduce any parent path references for safety
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.TrimSpace(path)

	if path == "" {
		return "", "", errors.New("empty normalized path")
	}

	// Locate the noVNC root within the path when proxmox prefixes are present
	if idx := strings.Index(path, "novnc/"); idx >= 0 {
		path = path[idx+len("novnc/"):]
	}

	path = strings.TrimPrefix(path, "novnc/")
	path = strings.TrimPrefix(path, "/")

	if path == "" {
		return "", "", errors.New("noVNC asset path resolved to empty")
	}

	if strings.Contains(path, "..") {
		return "", "", errors.New("invalid path traversal detected")
	}

	// Use the full noVNC installation path
	localPath := filepath.Join("frontend", "components", "noVNC-1.6.0", path)
	return localPath, path, nil
}

// setContentType sets appropriate content type based on file extension
func setContentType(w http.ResponseWriter, filename string) {
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".js":
		w.Header().Set("Content-Type", "application/javascript")
	case ".css":
		w.Header().Set("Content-Type", "text/css")
	case ".html":
		w.Header().Set("Content-Type", "text/html")
	case ".png":
		w.Header().Set("Content-Type", "image/png")
	case ".jpg", ".jpeg":
		w.Header().Set("Content-Type", "image/jpeg")
	case ".gif":
		w.Header().Set("Content-Type", "image/gif")
	case ".svg":
		w.Header().Set("Content-Type", "image/svg+xml")
	case ".woff":
		w.Header().Set("Content-Type", "font/woff")
	case ".woff2":
		w.Header().Set("Content-Type", "font/woff2")
	}
}
