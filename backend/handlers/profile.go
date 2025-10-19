package handlers

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
	"pvmss/proxmox"
	"pvmss/state"
)

// ProfileHandler handles user profile page
type ProfileHandler struct {
	stateManager state.StateManager
}

// NewProfileHandler creates a new instance of ProfileHandler
func NewProfileHandler(sm state.StateManager) *ProfileHandler {
	return &ProfileHandler{stateManager: sm}
}

// RegisterRoutes registers profile routes
func (h *ProfileHandler) RegisterRoutes(router *httprouter.Router) {
	router.GET("/profile", RequireAuthHandle(h.ShowProfile))
	router.POST("/profile/update-password", RequireAuthHandle(h.UpdatePassword))
}

// VMInfo represents a VM in the user's pool
type VMInfo struct {
	VMID        int
	Name        string
	Description string
	Node        string
	Status      string
}

// ShowProfile renders the user profile page
func (h *ProfileHandler) ShowProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := NewHandlerContext(w, r, "ProfileHandler.ShowProfile")

	// Require authentication
	if !ctx.RequireAuthentication() {
		return
	}

	// Admin users don't have profiles - redirect to admin dashboard
	if ctx.IsAdmin() {
		ctx.Log.Info().Msg("Admin user accessing profile page, redirecting to admin dashboard")
		http.Redirect(w, r, "/admin", http.StatusSeeOther)
		return
	}

	// Get username from session
	username := ctx.GetUsername()
	if username == "" {
		ctx.Log.Error().Msg("No username in session")
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}

	// Derive pool name from username
	poolName := "pvmss_" + username

	// Get Proxmox client
	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		ctx.Log.Error().Msg("Proxmox client not available")
		// Render page without VMs
		data := map[string]interface{}{
			"Title":           ctx.Translate("Profile.Title"),
			"Username":        username,
			"PoolName":        poolName,
			"VMs":             []VMInfo{},
			"ProxmoxError":    true,
			"Lang":            i18n.GetLanguage(r),
			"IsAuthenticated": true,
			"IsAdmin":         ctx.IsAdmin(),
		}
		ctx.RenderTemplate("profile", data)
		return
	}

	// If 'refresh=1' is present, invalidate pool and node caches for fresh data
	if r.URL.Query().Get("refresh") == "1" {
		ctx.Log.Info().Str("pool", poolName).Msg("Refreshing profile page - invalidating caches")
		// Invalidate pool cache
		client.InvalidateCache("/pools/" + url.PathEscape(poolName))
		// Invalidate all node VM lists
		if nodes, err := h.getNodeNames(r.Context(), client); err == nil {
			for _, node := range nodes {
				client.InvalidateCache("/nodes/" + url.PathEscape(node) + "/qemu")
			}
		}
	}

	// Fetch VMs from the user's pool
	vms := h.fetchUserVMs(r.Context(), client, poolName)

	// Check for password update messages and form visibility
	passwordSuccess := r.URL.Query().Get("password_success") == "1"
	passwordError := r.URL.Query().Get("password_error")
	showPasswordForm := r.URL.Query().Get("show_password_form") == "1" || passwordError != ""

	// Prepare template data
	data := map[string]interface{}{
		"Title":            ctx.Translate("Profile.Title"),
		"Username":         username,
		"PoolName":         poolName,
		"VMs":              vms,
		"Lang":             i18n.GetLanguage(r),
		"IsAuthenticated":  true,
		"IsAdmin":          ctx.IsAdmin(),
		"PasswordSuccess":  passwordSuccess,
		"PasswordError":    passwordError,
		"ShowPasswordForm": showPasswordForm,
	}

	ctx.RenderTemplate("profile", data)
}

// fetchUserVMs retrieves all VMs in the user's pool with their status
func (h *ProfileHandler) fetchUserVMs(ctx context.Context, client proxmox.ClientInterface, poolName string) []VMInfo {
	log := CreateHandlerLogger("fetchUserVMs", nil)

	// Create context with timeout
	fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// First, get pool members to know which VMIDs belong to this pool
	var poolResp struct {
		Data struct {
			Members []struct {
				Type     string `json:"type"`
				VMID     int    `json:"vmid"`
				Template int    `json:"template"`
			} `json:"members"`
		} `json:"data"`
	}

	if err := client.GetJSON(fetchCtx, "/pools/"+url.PathEscape(poolName), &poolResp); err != nil {
		log.Error().Err(err).Str("pool", poolName).Msg("Failed to fetch pool members")
		return []VMInfo{}
	}

	// Build a set of VMIDs in this pool (excluding templates and non-QEMU)
	poolVMIDs := make(map[int]bool)
	for _, member := range poolResp.Data.Members {
		if member.Template == 1 || member.VMID <= 0 {
			continue
		}
		if strings.EqualFold(member.Type, "qemu") {
			poolVMIDs[member.VMID] = true
		}
	}

	if len(poolVMIDs) == 0 {
		log.Info().Str("pool", poolName).Msg("No VMs found in pool")
		return []VMInfo{}
	}

	// Get all VMs with their status (already populated by GetVMsWithContext)
	allVMs, err := proxmox.GetVMsWithContext(fetchCtx, client)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get all VMs")
		return []VMInfo{}
	}

	// Filter VMs to only include those in the user's pool
	vms := make([]VMInfo, 0)
	for _, vm := range allVMs {
		if poolVMIDs[vm.VMID] {
			status := vm.Status
			if status == "" {
				// Fallback: if status is empty and uptime is 0, assume stopped
				if vm.Uptime == 0 {
					status = "stopped"
				} else {
					status = "unknown"
				}
			}

			// Get VM description from config
			var description string
			if vmConfig, err := proxmox.GetVMConfigWithContext(fetchCtx, client, vm.Node, vm.VMID); err == nil {
				if desc, exists := vmConfig["description"]; exists {
					if descStr, ok := desc.(string); ok {
						description = descStr
					}
				}
			}

			vms = append(vms, VMInfo{
				VMID:        vm.VMID,
				Name:        vm.Name,
				Description: description,
				Node:        vm.Node,
				Status:      strings.ToLower(status),
			})
		}
	}

	log.Info().
		Str("pool", poolName).
		Int("vm_count", len(vms)).
		Msg("Successfully fetched user VMs")

	return vms
}

// getNodeNames retrieves the list of Proxmox node names
func (h *ProfileHandler) getNodeNames(ctx context.Context, client interface {
	GetJSON(ctx context.Context, path string, result interface{}) error
}) ([]string, error) {
	var nodeResp struct {
		Data []struct {
			Node string `json:"node"`
		} `json:"data"`
	}

	if err := client.GetJSON(ctx, "/nodes", &nodeResp); err != nil {
		return nil, err
	}

	nodes := make([]string, 0, len(nodeResp.Data))
	for _, n := range nodeResp.Data {
		if n.Node != "" {
			nodes = append(nodes, n.Node)
		}
	}
	return nodes, nil
}

// UpdatePassword handles user password change requests
func (h *ProfileHandler) UpdatePassword(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("ProfileHandler.UpdatePassword", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	// Get session manager
	sessionManager := h.stateManager.GetSessionManager()
	if sessionManager == nil {
		log.Error().Msg("Session manager not available")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Get username from session
	username := sessionManager.GetString(r.Context(), "username")
	if username == "" {
		log.Error().Msg("No username in session")
		http.Redirect(w, r, "/profile?error=session_expired", http.StatusSeeOther)
		return
	}

	// Get form values
	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	// Validate inputs
	if currentPassword == "" || newPassword == "" || confirmPassword == "" {
		log.Debug().Msg("Missing password fields")
		http.Redirect(w, r, "/profile?show_password_form=1&password_error="+url.QueryEscape("All password fields are required"), http.StatusSeeOther)
		return
	}

	if newPassword != confirmPassword {
		log.Debug().Msg("New passwords do not match")
		http.Redirect(w, r, "/profile?show_password_form=1&password_error="+url.QueryEscape("New passwords do not match"), http.StatusSeeOther)
		return
	}

	if len(newPassword) < 5 {
		log.Debug().Msg("New password too short")
		http.Redirect(w, r, "/profile?show_password_form=1&password_error="+url.QueryEscape("Password must be at least 5 characters"), http.StatusSeeOther)
		return
	}

	// Get Proxmox client
	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not available")
		http.Redirect(w, r, "/profile?show_password_form=1&password_error="+url.QueryEscape("Service unavailable"), http.StatusSeeOther)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Proxmox password update requires cookie-based authentication
	// First, verify current password by attempting to authenticate
	proxmoxURL := client.GetApiUrl()
	insecureSkipVerify := strings.Contains(proxmoxURL, "192.168.") || strings.Contains(proxmoxURL, "localhost") // Simple heuristic

	cookieClient, err := proxmox.NewClientCookieAuth(proxmoxURL, insecureSkipVerify)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create cookie-based client")
		http.Redirect(w, r, "/profile?show_password_form=1&password_error="+url.QueryEscape("Internal error"), http.StatusSeeOther)
		return
	}

	// Authenticate with current password to verify it's correct
	ticketResp, err := proxmox.CreateTicket(ctx, cookieClient, username, currentPassword, &proxmox.CreateTicketOptions{
		Realm: "pve",
	})
	if err != nil {
		log.Info().Err(err).Str("username", username).Msg("Current password verification failed")
		http.Redirect(w, r, "/profile?show_password_form=1&password_error="+url.QueryEscape("Current password is incorrect"), http.StatusSeeOther)
		return
	}

	// Set authentication credentials
	cookieClient.PVEAuthCookie = ticketResp.Ticket
	cookieClient.CSRFPreventionToken = ticketResp.CSRFPreventionToken

	// Update password - Proxmox requires current password as confirmation
	if err := proxmox.UpdateUserPassword(ctx, cookieClient, username, newPassword, currentPassword, "pve"); err != nil {
		log.Error().Err(err).Str("username", username).Msg("Failed to update password")
		http.Redirect(w, r, "/profile?show_password_form=1&password_error="+url.QueryEscape("Failed to update password: "+err.Error()), http.StatusSeeOther)
		return
	}

	log.Info().Str("username", username).Msg("Password updated successfully")

	// Update session with new PVE credentials
	newTicketResp, err := proxmox.CreateTicket(ctx, cookieClient, username, newPassword, &proxmox.CreateTicketOptions{
		Realm: "pve",
	})
	if err == nil {
		sessionManager.Put(r.Context(), "pve_auth_cookie", newTicketResp.Ticket)
		sessionManager.Put(r.Context(), "pve_csrf_token", newTicketResp.CSRFPreventionToken)
		sessionManager.Put(r.Context(), "pve_ticket_created", time.Now().Unix())
	}

	// Redirect with success message
	http.Redirect(w, r, "/profile?password_success=1", http.StatusSeeOther)
}
