package handlers

import (
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/julienschmidt/httprouter"
	"pvmss/security"
)

// VMConsoleWindowHandler serves a custom noVNC window with proper authentication
func (h *VMHandler) VMConsoleWindowHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("VMConsoleWindowHandler", r)

	log.Info().Msg("Console window handler called")

	if !IsAuthenticated(r) {
		log.Warn().Msg("Console window request rejected: not authenticated")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get VM details from query params
	vmid := r.URL.Query().Get("vmid")
	node := r.URL.Query().Get("node")

	log.Info().Str("vmid", vmid).Str("node", node).Msg("Console window request parameters")

	if vmid == "" || node == "" {
		log.Error().Str("vmid", vmid).Str("node", node).Msg("VM ID and node are required")
		http.Error(w, "VM ID and node are required", http.StatusBadRequest)
		return
	}

	vmidInt, err := strconv.Atoi(vmid)
	if err != nil {
		log.Error().Err(err).Str("vmid", vmid).Msg("Invalid VM ID")
		http.Error(w, "Invalid VM ID", http.StatusBadRequest)
		return
	}

	sessionManager := security.GetSession(r)
	if sessionManager == nil {
		log.Error().Msg("Session manager not available")
		http.Error(w, "Session not available", http.StatusUnauthorized)
		return
	}

	// Retrieve console session
	sessionKey := fmt.Sprintf("console_session_%d_%s", vmidInt, node)
	log.Info().Str("session_key", sessionKey).Msg("Looking for console session")

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

	log.Info().Msg("Console session found successfully")

	// Check if session has expired
	if time.Now().Unix() > consolePayload.ExpiresAt {
		log.Warn().Msg("Console session expired")
		sessionManager.Remove(r.Context(), sessionKey)
		http.Error(w, "Console session expired. Please try again.", http.StatusUnauthorized)
		return
	}

	// Extract session data
	websocketURL := consolePayload.WebsocketURL
	authCookie := consolePayload.AuthCookie
	csrfToken := consolePayload.CsrfToken
	host := consolePayload.Host
	port := consolePayload.Port

	log.Info().
		Str("node", node).
		Int("vmid", vmidInt).
		Str("host", host).
		Int("port", port).
		Msg("Serving console window with authentication")

	// Remove session after use (single use)
	sessionManager.Remove(r.Context(), sessionKey)

	// Serve custom noVNC HTML page with authentication
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	// Set Proxmox authentication cookies for this domain
	if authCookie != "" {
		cookie := &http.Cookie{
			Name:     "PVEAuthCookie",
			Value:    authCookie,
			Path:     "/",
			HttpOnly: false,
			Secure:   false, // HTTP deployment
			SameSite: http.SameSiteNoneMode,
			MaxAge:   300, // 5 minutes
		}
		http.SetCookie(w, cookie)
	}
	if csrfToken != "" {
		cookie := &http.Cookie{
			Name:     "CSRFPreventionToken",
			Value:    csrfToken,
			Path:     "/",
			HttpOnly: false,
			Secure:   false,
			SameSite: http.SameSiteNoneMode,
			MaxAge:   300,
		}
		http.SetCookie(w, cookie)
	}

	maskSecret := func(value string) string {
		if value == "" {
			return ""
		}
		runes := []rune(value)
		length := len(runes)
		if length <= 8 {
			return fmt.Sprintf("[len:%d]", len(value))
		}
		return string(runes[:4]) + "…" + string(runes[length-4:])
	}

	// Prepare escaped values for HTML and JavaScript contexts
	nodeHTML := template.HTMLEscapeString(node)
	hostHTML := template.HTMLEscapeString(host)
	nodeJS := template.JSEscapeString(node)
	hostJS := template.JSEscapeString(host)
	proxmoxURL := template.JSEscapeString(consolePayload.ConsoleURL)
	websocketURLJS := template.JSEscapeString(websocketURL)
	authCookieJS := template.JSEscapeString(maskSecret(authCookie))
	csrfTokenJS := template.JSEscapeString(maskSecret(csrfToken))

	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>Console - %s (VM %d)</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <style>
        body { 
            margin: 0; padding: 0; background: #000; overflow: hidden; 
            font-family: Arial, sans-serif;
        }
        #console-container { 
            width: 100vw; height: 100vh; position: relative;
        }
        #status { 
            position: absolute; top: 10px; left: 10px; 
            color: white; background: rgba(0,0,0,0.8); 
            padding: 8px 12px; border-radius: 4px; 
            font-size: 14px; z-index: 1000;
            border: 1px solid #333;
        }
        .status-connecting { color: #ffa500; }
        .status-connected { color: #00ff00; }
        .status-error { color: #ff4444; }
        .status-warn { color: #ffff00; }
        #console-frame {
            width: 100%%; height: 100%%; border: none;
            background: #000;
        }
        .loading {
            position: absolute; top: 50%%; left: 50%%;
            transform: translate(-50%%, -50%%);
            color: white; font-size: 18px;
            text-align: center;
        }
    </style>
</head>
<body>
    <div id="console-container">
        <div id="status" class="status-connecting">Connecting to console...</div>
        <div class="loading" id="loading">
            <div>Initializing console connection...</div>
            <div style="font-size: 14px; margin-top: 10px; opacity: 0.7;">
                VM: %s (ID: %d)<br>
                Host: %s<br>
                Ticket expires in: <span id="countdown">8</span>s
            </div>
        </div>
        <iframe id="console-frame" style="display: none;"></iframe>
    </div>

    <script>
        let countdown = 8;
        let countdownElement = document.getElementById('countdown');
        let statusElement = document.getElementById('status');
        let loadingElement = document.getElementById('loading');
        let frameElement = document.getElementById('console-frame');
        
        function updateStatus(message, className) {
            statusElement.textContent = message;
            statusElement.className = 'status-' + className;
            console.log('Console Status:', message);
        }
        
        function startCountdown() {
            const timer = setInterval(() => {
                countdown--;
                if (countdownElement) {
                    countdownElement.textContent = countdown;
                }
                if (countdown <= 0) {
                    clearInterval(timer);
                    if (countdownElement) {
                        countdownElement.textContent = 'expired';
                    }
                }
            }, 1000);
        }
        
        function loadConsole() {
            try {
                // Build the Proxmox noVNC URL with authentication
                const proxmoxUrl = '%s';
                
                updateStatus('Loading Proxmox noVNC...', 'connecting');
                
                // Set the iframe source
                frameElement.src = proxmoxUrl;
                
                frameElement.onload = function() {
                    updateStatus('Console loaded successfully', 'connected');
                    loadingElement.style.display = 'none';
                    frameElement.style.display = 'block';
                    console.log('Console iframe loaded successfully');
                };
                
                frameElement.onerror = function() {
                    updateStatus('Failed to load console', 'error');
                    console.error('Console iframe failed to load');
                };
                
                // Fallback timeout
                setTimeout(() => {
                    if (frameElement.style.display === 'none') {
                        updateStatus('Console ready', 'connected');
                        loadingElement.style.display = 'none';
                        frameElement.style.display = 'block';
                    }
                }, 3000);
                
            } catch (err) {
                updateStatus('Connection failed: ' + err.message, 'error');
                console.error('Console connection error:', err);
            }
        }
        
        // Start countdown and load console immediately
        startCountdown();
        
        // Load console after a brief delay to ensure cookies are set
        setTimeout(loadConsole, 100);
        
        // Debug info
        console.log('Console Window Debug Info:', {
            node: '%s',
            vmid: %d,
            host: '%s',
            websocketURL: '%s',
            authCookie: '%s',
            csrfToken: '%s'
        });
    </script>
</body>
</html>`,
		nodeHTML, vmidInt,
		nodeHTML, vmidInt, hostHTML,
		proxmoxURL,
		nodeJS, vmidInt, hostJS, websocketURLJS, authCookieJS, csrfTokenJS,
	)

	w.Write([]byte(html))
}
