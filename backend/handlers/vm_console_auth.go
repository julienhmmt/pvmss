package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/rs/zerolog/log"
	"pvmss/proxmox"
)

type ConsoleAuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Node     string `json:"node"`
	VMID     string `json:"vmid"`
}

type ConsoleAuthResponse struct {
	Success    bool   `json:"success"`
	Message    string `json:"message,omitempty"`
	ConsoleURL string `json:"console_url,omitempty"`
}

// VMConsoleAuthHandler handles authentication for console access using user's individual credentials
func VMConsoleAuthHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("method", r.Method).Str("url", r.URL.String()).Msg("VMConsoleAuthHandler called")
	
	// Set CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:50000")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
	w.Header().Set("Access-Control-Allow-Credentials", "true")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		log.Error().Str("method", r.Method).Msg("Invalid method for console auth")
		w.Header().Set("Content-Type", "application/json")
		response := ConsoleAuthResponse{
			Success: false,
			Message: "Method not allowed",
		}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get handler context to access session manager
	ctx := NewHandlerContext(w, r, "VMConsoleAuthHandler")

	if !ctx.ValidateSessionManager() {
		response := ConsoleAuthResponse{
			Success: false,
			Message: "Session not available",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Check if user is authenticated
	authenticated, ok := ctx.SessionManager.Get(r.Context(), "authenticated").(bool)
	if !ok || !authenticated {
		log.Error().Msg("User not authenticated")
		response := ConsoleAuthResponse{
			Success: false,
			Message: "User not authenticated",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Get username from session - we'll need this for individual authentication
	username, ok := ctx.SessionManager.Get(r.Context(), "username").(string)
	if !ok || username == "" {
		log.Error().Msg("No username in session")
		response := ConsoleAuthResponse{
			Success: false,
			Message: "No username available in session",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Parse request to get VM details and user credentials
	var authReq struct {
		Host     string `json:"host"`
		Node     string `json:"node"`
		VMID     string `json:"vmid"`
		Username string `json:"username,omitempty"` // Optional - use session username if not provided
		Password string `json:"password,omitempty"` // Optional - will prompt if not provided
	}
	
	if err := json.NewDecoder(r.Body).Decode(&authReq); err != nil {
		log.Error().Err(err).Msg("Failed to parse console auth request")
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Use session username if not provided in request
	if authReq.Username == "" {
		authReq.Username = username
	}

	log.Info().Str("session_user", username).Str("auth_user", authReq.Username).Str("host", authReq.Host).Str("node", authReq.Node).Str("vmid", authReq.VMID).Msg("Console authentication request for individual user")

	// Validate required fields
	if authReq.Host == "" || authReq.Node == "" || authReq.VMID == "" {
		response := ConsoleAuthResponse{
			Success: false,
			Message: "Missing required VM details",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// If no password provided, we need to prompt for it
	if authReq.Password == "" {
		log.Info().Str("username", authReq.Username).Msg("No password provided - need user authentication")
		response := ConsoleAuthResponse{
			Success: false,
			Message: "NEED_USER_AUTH", // Special message to trigger frontend auth prompt
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Info().Str("username", authReq.Username).Msg("Password provided - proceeding with individual user authentication")

	// Convert VMID to int
	vmidInt, err := strconv.Atoi(authReq.VMID)
	if err != nil {
		log.Error().Err(err).Str("vmid", authReq.VMID).Msg("Invalid VMID")
		response := ConsoleAuthResponse{
			Success: false,
			Message: "Invalid VMID format",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Create Proxmox URL
	proxmoxURL := fmt.Sprintf("https://%s", authReq.Host)

	// Create a Proxmox client for individual user authentication
	userClient, err := proxmox.NewClientCookieAuth(proxmoxURL, true) // insecureSkipVerify=true for dev
	if err != nil {
		log.Error().Err(err).Msg("Failed to create Proxmox client for user")
		response := ConsoleAuthResponse{
			Success: false,
			Message: "Failed to create Proxmox client: " + err.Error(),
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	// Authenticate the individual user with Proxmox
	userForAuth := authReq.Username
	realm := "pve"
	if strings.Contains(userForAuth, "@") {
		parts := strings.Split(userForAuth, "@")
		userForAuth = parts[0]
		realm = parts[1]
	}

	log.Info().Str("username", userForAuth).Str("realm", realm).Msg("Authenticating individual user with Proxmox")

	if err := userClient.Login(r.Context(), userForAuth, authReq.Password, realm); err != nil {
		log.Error().Err(err).Str("username", userForAuth).Str("realm", realm).Msg("Individual user Proxmox authentication failed")
		response := ConsoleAuthResponse{
			Success: false,
			Message: "Authentication failed: Invalid credentials or insufficient permissions",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Info().Str("username", userForAuth).Str("realm", realm).Msg("Individual user Proxmox authentication successful")

	// Get VNC ticket using the user's authenticated client
	log.Info().Str("username", userForAuth).Str("realm", realm).Msg("Getting VNC ticket using individual user's authenticated client")
	vncResponse, err := userClient.GetVNCProxy(r.Context(), authReq.Node, vmidInt)
	if err != nil {
		log.Error().Err(err).Str("username", userForAuth).Str("node", authReq.Node).Int("vmid", vmidInt).Msg("Failed to get VNC ticket for individual user")

		// Provide more specific error message based on the error
		errorMsg := "Failed to get console access: " + err.Error()
		if strings.Contains(err.Error(), "403") || strings.Contains(err.Error(), "Forbidden") {
			errorMsg = "Console access denied. Your user account (" + userForAuth + "@" + realm + ") may not have VM.Console permission for this VM. Please contact your administrator."
		} else if strings.Contains(err.Error(), "401") || strings.Contains(err.Error(), "Unauthorized") {
			errorMsg = "Authentication failed for user " + userForAuth + "@" + realm + ". Please check your username and password."
		}

		response := ConsoleAuthResponse{
			Success: false,
			Message: errorMsg,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	log.Info().Str("username", userForAuth).Str("realm", realm).Msg("VNC ticket obtained successfully for individual user")

	// Extract VNC ticket and port
	vncTicket, ok := vncResponse["ticket"].(string)
	if !ok {
		log.Error().Interface("response", vncResponse).Msg("Missing VNC ticket in response")
		response := ConsoleAuthResponse{
			Success: false,
			Message: "Invalid VNC ticket response",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}

	vncPortFloat, ok := vncResponse["port"].(float64)
	if !ok {
		log.Error().Interface("response", vncResponse).Msg("Missing VNC port in response")
		response := ConsoleAuthResponse{
			Success: false,
			Message: "Invalid VNC port response",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}
	vncPort := int(vncPortFloat)

	// Build the authenticated console URL with the VNC ticket
	consoleURL := fmt.Sprintf("https://%s/?console=kvm&novnc=1&node=%s&vmid=%s&resize=1&path=api2/json/nodes/%s/qemu/%s/vncwebsocket/port/%d/vncticket=%s",
		authReq.Host, authReq.Node, authReq.VMID, authReq.Node, authReq.VMID, vncPort, vncTicket)

	// Set authentication cookies for cross-domain access using the user's auth cookie
	cookieDomain := os.Getenv("PROXMOX_COOKIE_DOMAIN")
	if cookieDomain == "" {
		// Extract domain from host
		if colonIndex := strings.Index(authReq.Host, ":"); colonIndex != -1 {
			cookieDomain = authReq.Host[:colonIndex]
		} else {
			cookieDomain = authReq.Host
		}
	}

	// Set the user's PVE auth cookie for cross-domain access
	http.SetCookie(w, &http.Cookie{
		Name:     "PVEAuthCookie",
		Value:    userClient.PVEAuthCookie,
		Domain:   cookieDomain,
		Path:     "/",
		HttpOnly: false,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		MaxAge:   3600, // 1 hour
	})

	log.Info().Str("username", userForAuth).Str("console_url", consoleURL[:50]+"...").Int("port", vncPort).Msg("Console authentication successful for individual user")

	// Also log the full URL for debugging (truncated for security)
	log.Debug().Str("full_console_url", consoleURL).Msg("Full console URL generated for individual user")

	response := ConsoleAuthResponse{
		Success:    true,
		Message:    "Authentication successful for user: " + userForAuth,
		ConsoleURL: consoleURL,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}
