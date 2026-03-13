package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/julienschmidt/httprouter"
	"golang.org/x/sync/errgroup"

	"pvmss/components"
	"pvmss/i18n"
	"pvmss/proxmox"
	"pvmss/state"
)

// ProfileHandler handles user profile page
type ProfileHandler struct {
	stateManager state.StateManager
}

// MakeProfileHandler creates a new instance of ProfileHandler
func MakeProfileHandler(sm state.StateManager) *ProfileHandler {
	return &ProfileHandler{stateManager: sm}
}

// RegisterRoutes registers profile routes
func (h *ProfileHandler) RegisterRoutes(router *httprouter.Router) {
	router.GET("/profile", RequireAuthHandle(h.ShowProfile))
	router.POST("/profile/update-password", RequireAuthHandle(h.UpdatePassword))
	router.GET("/api/profile/vms", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.GetProfileVMsAPI(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
}

// VMInfo represents a VM in the user's pool
type VMInfo struct {
	VMID        int    `json:"vmid"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Node        string `json:"node"`
	Status      string `json:"status"`
}

// formatDescriptionMarkdown converts markdown description to HTML
func formatDescriptionMarkdown(description string) string {
	if description == "" {
		return ""
	}
	return string(markdown.ToHTML([]byte(description), nil, nil))
}

// ShowProfile renders the user profile page
func (h *ProfileHandler) ShowProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := HandlerContextWith(w, r, "ProfileHandler.ShowProfile")

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
		RespondWithError(w, r, ErrSessionExpired)
		return
	}

	// Derive pool name from username
	poolName := "pvmss_" + username

	// Get Proxmox client
	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		ctx.Log.Error().Msg("Proxmox client not available")
		RespondWithError(w, r, ErrProxmoxConnection)
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
	total, running, stopped, _, _ := computeVMStats(vms)

	// Check for password update messages and form visibility
	passwordSuccess := r.URL.Query().Get("password_success") == "1"
	passwordError := r.URL.Query().Get("password_error")
	showPasswordForm := r.URL.Query().Get("show_password_form") == "1" || passwordError != ""

	// Get CSRF token
	csrfToken, err := ctx.GetCSRFToken()
	if err != nil {
		ctx.Log.Error().Err(err).Msg("Failed to get CSRF token")
		csrfToken = ""
	}

	// Convert VMs to ProfileVM format
	profileVMs := make([]components.ProfileVM, len(vms))
	for i, vm := range vms {
		profileVMs[i] = components.ProfileVM{
			VMID:            vm.VMID,
			Name:            vm.Name,
			Description:     vm.Description,
			DescriptionHTML: formatDescriptionMarkdown(vm.Description),
			Node:            vm.Node,
			Status:          vm.Status,
		}
	}

	// Prepare Templ data
	profileData := components.ProfileData{
		Username:         username,
		PoolName:         poolName,
		TotalVMs:         total,
		RunningVMs:       running,
		StoppedVMs:       stopped,
		HasVMs:           total > 0,
		VMs:              profileVMs,
		ProxmoxError:     false,
		PasswordSuccess:  passwordSuccess,
		PasswordError:    passwordError,
		ShowPasswordForm: showPasswordForm,
		CSRFToken:        csrfToken,
		Lang:             i18n.GetLanguage(r),
	}

	// Translation function wrapper
	translateFunc := func(key string) string {
		return ctx.Translate(key)
	}

	// Render with Templ
	if err := components.ProfilePage(profileData, translateFunc).Render(r.Context(), w); err != nil {
		ctx.Log.Error().Err(err).Msg("Failed to render profile page")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// GetProfileVMsAPI returns the user's VM list as JSON for asynchronous refreshes
func (h *ProfileHandler) GetProfileVMsAPI(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := HandlerContextWith(w, r, "ProfileHandler.GetProfileVMsAPI")

	if ctx.IsAdmin() {
		writeProfileAPIError(w, http.StatusForbidden, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Profile.Error.AdminNoPersonalVMs"))
		return
	}

	username := ctx.GetUsername()
	if username == "" {
		writeProfileAPIError(w, http.StatusUnauthorized, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.SessionError"))
		return
	}

	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		writeProfileAPIError(w, http.StatusServiceUnavailable, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.ProxmoxConnectionError"))
		return
	}

	poolName := "pvmss_" + username
	vms := h.fetchUserVMs(r.Context(), client, poolName)
	total, running, stopped, paused, unknown := computeVMStats(vms)

	response := map[string]interface{}{
		"status": "success",
		"vms":    vms,
		"summary": map[string]int{
			"total":   total,
			"running": running,
			"stopped": stopped,
			"paused":  paused,
			"unknown": unknown,
		},
	}

	if err := writeProfileAPISuccess(w, response); err != nil {
		ctx.Log.Error().Err(err).Msg("Failed to write profile VMs JSON response")
	}
}

func computeVMStats(vms []VMInfo) (total, running, stopped, paused, unknown int) {
	total = len(vms)
	for _, vm := range vms {
		switch strings.ToLower(vm.Status) {
		case "running":
			running++
		case "stopped":
			stopped++
		case "paused", "suspended":
			paused++
		case "":
			unknown++
		default:
			unknown++
		}
	}
	return
}

func writeProfileAPISuccess(w http.ResponseWriter, payload interface{}) error {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	return json.NewEncoder(w).Encode(payload)
}

func writeProfileAPIError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "error",
		"message": message,
	})
}

// fetchUserVMs retrieves all VMs in the user's pool with their status
// TODO Telmate migration: this helper still uses the Telmate client for pool membership and node access; migrate these paths to the Resty helpers and remove the Telmate dependency.
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

	// Get all VMs with their status using resty
	restyClient, err := getDefaultRestyClient()
	if err != nil {
		log.Error().Err(err).Msg("Failed to create resty client")
		return []VMInfo{}
	}

	allVMs, err := proxmox.GetVMsResty(fetchCtx, restyClient)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get all VMs (resty)")
		return []VMInfo{}
	}

	// Filter VMs to only include those in the user's pool and fetch config concurrently
	vms := make([]VMInfo, 0, len(allVMs))
	var mu sync.Mutex
	sem := make(chan struct{}, 8)
	g, gctx := errgroup.WithContext(fetchCtx)
	for i := range allVMs {
		vm := allVMs[i]
		if !poolVMIDs[vm.VMID] {
			continue
		}
		g.Go(func() error {
			// throttle parallel requests
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-gctx.Done():
				return gctx.Err()
			}

			status := vm.Status
			if status == "" {
				if vm.Uptime == 0 {
					status = "stopped"
				} else {
					status = "unknown"
				}
			}

			// Fetch VM config via resty to get description
			description := ""
			if vmConfig, err := proxmox.GetVMConfigResty(gctx, restyClient, vm.Node, vm.VMID); err == nil {
				if desc, exists := vmConfig["description"]; exists {
					if descStr, ok := desc.(string); ok {
						description = descStr
					}
				}
			}

			info := VMInfo{
				VMID:        vm.VMID,
				Name:        vm.Name,
				Description: description,
				Node:        vm.Node,
				Status:      strings.ToLower(status),
			}
			mu.Lock()
			vms = append(vms, info)
			mu.Unlock()
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		log.Warn().Err(err).Str("pool", poolName).Msg("Concurrent VM fetch encountered errors")
	}

	// Sort by VMID ascending for consistent display order
	sort.Slice(vms, func(i, j int) bool { return vms[i].VMID < vms[j].VMID })

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
		return nil, fmt.Errorf("failed to get node list from Proxmox: %w", err)
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
// TODO Telmate migration: this password change flow still depends on Telmate ticket creation; migrate it to the Resty-based access helpers.
func (h *ProfileHandler) UpdatePassword(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("ProfileHandler.UpdatePassword", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	// Get session manager
	sessionManager := h.stateManager.GetSessionManager()
	if sessionManager == nil {
		log.Error().Msg("Session manager not available")
		http.Error(w, i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.InternalServer"), http.StatusInternalServerError)
		return
	}

	// Get username from session
	username := sessionManager.GetString(r.Context(), "username")
	if username == "" {
		log.Error().Msg("No username in session")
		http.Redirect(w, r, "/profile?error="+url.QueryEscape(i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.SessionError")), http.StatusSeeOther)
		return
	}

	// Get form values
	currentPassword := r.FormValue("current_password")
	newPassword := r.FormValue("new_password")
	confirmPassword := r.FormValue("confirm_password")

	// Validate inputs
	if currentPassword == "" || newPassword == "" || confirmPassword == "" {
		log.Debug().
			Str("component", "profile").
			Str("operation", "change_password").
			Str("reason", "missing_fields").
			Msg("Missing password fields")
		http.Redirect(w, r, "/profile?show_password_form=1&password_error="+url.QueryEscape(i18n.Localize(i18n.GetLocalizerFromRequest(r), "Profile.PasswordError.MissingFields")), http.StatusSeeOther)
		return
	}

	if newPassword != confirmPassword {
		log.Debug().
			Str("component", "profile").
			Str("operation", "change_password").
			Str("reason", "password_mismatch").
			Msg("New passwords do not match")
		http.Redirect(w, r, "/profile?show_password_form=1&password_error="+url.QueryEscape(i18n.Localize(i18n.GetLocalizerFromRequest(r), "Profile.PasswordError.Mismatch")), http.StatusSeeOther)
		return
	}

	if len(newPassword) < 5 {
		log.Debug().
			Str("component", "profile").
			Str("operation", "change_password").
			Str("reason", "password_too_short").
			Msg("New password too short")
		http.Redirect(w, r, "/profile?show_password_form=1&password_error="+url.QueryEscape(i18n.Localize(i18n.GetLocalizerFromRequest(r), "Profile.PasswordError.TooShort")), http.StatusSeeOther)
		return
	}

	// Get Proxmox client
	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		log.Error().Msg("Proxmox client not available")
		http.Redirect(w, r, "/profile?show_password_form=1&password_error="+url.QueryEscape(i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.ProxmoxClientUnavailable")), http.StatusSeeOther)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Proxmox password update requires cookie-based authentication
	// First, verify current password by attempting to authenticate
	proxmoxURL := client.GetApiUrl()
	insecureSkipVerify := os.Getenv("PROXMOX_VERIFY_SSL") == "false"

	cookieClient, err := proxmox.MakeClientCookieAuth(proxmoxURL, insecureSkipVerify)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create cookie-based client")
		http.Redirect(w, r, "/profile?show_password_form=1&password_error="+url.QueryEscape(i18n.Localize(i18n.GetLocalizerFromRequest(r), "Error.InternalServer")), http.StatusSeeOther)
		return
	}

	// Authenticate with current password to verify it's correct
	ticketResp, err := proxmox.CreateTicket(ctx, cookieClient, username, currentPassword, &proxmox.CreateTicketOptions{
		Realm: "pve",
	})
	if err != nil {
		log.Info().Err(err).Str("username", username).Msg("Current password verification failed")
		http.Redirect(w, r, "/profile?show_password_form=1&password_error="+url.QueryEscape(i18n.Localize(i18n.GetLocalizerFromRequest(r), "Profile.PasswordError.IncorrectCurrent")), http.StatusSeeOther)
		return
	}

	// Set authentication credentials
	cookieClient.PVEAuthCookie = ticketResp.Ticket
	cookieClient.CSRFPreventionToken = ticketResp.CSRFPreventionToken

	// Update password - Proxmox requires current password as confirmation
	if err := proxmox.UpdateUserPassword(ctx, cookieClient, username, newPassword, currentPassword, "pve"); err != nil {
		log.Error().Err(err).Str("username", username).Msg("Failed to update password")
		http.Redirect(w, r, "/profile?show_password_form=1&password_error="+url.QueryEscape(i18n.Localize(i18n.GetLocalizerFromRequest(r), "Profile.PasswordError.UpdateFailed")), http.StatusSeeOther)
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
