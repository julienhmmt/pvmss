package handlers

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/proxmox"
	"pvmss/state"
)

// UserPoolHandler handles Proxmox user/pool admin flows
type UserPoolHandler struct {
	stateManager state.StateManager
}

// DeleteUserPool deletes all VMs in the pool (purge), then the derived user, then the pool itself.
func (h *UserPoolHandler) DeleteUserPool(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("DeleteUserPool", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	poolID := strings.TrimSpace(r.FormValue("pool"))
	if poolID == "" {
		http.Error(w, "pool is required", http.StatusBadRequest)
		return
	}

	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		http.Error(w, "Proxmox client not available", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Derive user from pool id: pvmss_<username>
	username := strings.TrimPrefix(poolID, "pvmss_")
	userID := username
	if userID != "" && !strings.Contains(userID, "@") {
		userID = userID + "@pve"
	}

	// Always stop and delete all VMs in the pool first (purge)
	var detailResp struct {
		Data struct {
			Members []struct {
				Type string `json:"type"`
				VMID int    `json:"vmid"`
				Node string `json:"node"`
			} `json:"members"`
		} `json:"data"`
	}
	if err := client.GetJSON(ctx, "/pools/"+url.PathEscape(poolID), &detailResp); err != nil {
		log.Error().Err(err).Str("pool", poolID).Msg("Failed to get pool members before deletion")
		http.Error(w, "failed to resolve pool members: "+err.Error(), http.StatusInternalServerError)
		return
	}
	// First, stop each VM (qemu) in bulk (concurrently), then wait a short fixed delay
	{
		var wg sync.WaitGroup
		for _, m := range detailResp.Data.Members {
			if !strings.EqualFold(m.Type, "qemu") || m.VMID <= 0 {
				continue
			}
			if m.Node == "" {
				log.Warn().Int("vmid", m.VMID).Msg("Skipping VM stop due to missing node")
				continue
			}
			m := m // capture loop var
			wg.Add(1)
			go func() {
				defer wg.Done()
				if _, err := proxmox.VMActionWithContext(ctx, client, m.Node, strconv.Itoa(m.VMID), "stop"); err != nil {
					log.Warn().Err(err).Int("vmid", m.VMID).Str("node", m.Node).Msg("Failed to issue VM stop; will continue and attempt deletion")
				}
			}()
		}
		wg.Wait()
		// Fixed small wait to give Proxmox time to transition state
		time.Sleep(3 * time.Second)
	}

	// Then delete each VM with purge=1
	for _, m := range detailResp.Data.Members {
		if !strings.EqualFold(m.Type, "qemu") || m.VMID <= 0 {
			continue
		}
		if m.Node == "" {
			// If node is missing, we cannot form the path; skip with warning
			log.Warn().Int("vmid", m.VMID).Msg("Skipping VM deletion due to missing node")
			continue
		}
		path := "/nodes/" + url.PathEscape(m.Node) + "/qemu/" + url.PathEscape(strconv.Itoa(m.VMID)) + "?purge=1"
		if _, err := client.DeleteWithContext(ctx, path, nil); err != nil {
			log.Error().Err(err).Str("path", path).Msg("Failed to delete VM")
			http.Error(w, "failed to delete VM "+strconv.Itoa(m.VMID)+": "+err.Error(), http.StatusInternalServerError)
			return
		}
	}

	// Delete the user next (non-fatal). Pool deletion will proceed even if user deletion fails or user is missing.
	if userID != "" {
		if _, err := client.DeleteWithContext(ctx, "/access/users/"+url.PathEscape(userID), nil); err != nil {
			log.Warn().Err(err).Str("user", userID).Msg("Failed to delete user; proceeding to delete pool")
		}
	}

	// Always delete the pool as the last step
	if _, err := client.DeleteWithContext(ctx, "/pools/"+url.PathEscape(poolID), nil); err != nil {
		log.Error().Err(err).Str("pool", poolID).Msg("Failed to delete pool")
		http.Error(w, "failed to delete pool "+poolID+": "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Invalidate caches so the next page load reflects fresh state
	if c, ok := client.(*proxmox.Client); ok && c != nil {
		c.InvalidateCache("/pools")
		c.InvalidateCache("/pools/" + poolID)
	}

	// Redirect with success
	msg := "Deleted pool, user, and VMs for '" + poolID + "'"
	redir := "/admin/userpool?success=1&message=" + url.QueryEscape(msg)
	http.Redirect(w, r, redir, http.StatusSeeOther)
}

func NewUserPoolHandler(sm state.StateManager) *UserPoolHandler {
	return &UserPoolHandler{stateManager: sm}
}

// RegisterRoutes registers routes for user/pool admin
func (h *UserPoolHandler) RegisterRoutes(router *httprouter.Router) {
	routeHelpers := NewAdminPageRoutes()

	// Register admin user pool routes using helper
	routeHelpers.RegisterCRUDRoutes(router, "/admin/userpool", map[string]func(w http.ResponseWriter, r *http.Request, ps httprouter.Params){
		"page":   h.UserPoolPage,
		"create": h.CreateUserPool,
		"delete": h.DeleteUserPool,
	})
}

// UserPoolPage renders the admin page for creating users/pools
func (h *UserPoolHandler) UserPoolPage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Success banner via query params
	success := r.URL.Query().Get("success") != ""
	user := r.URL.Query().Get("user")
	pool := r.URL.Query().Get("pool")

	var successMsg string
	if success {
		if user != "" && pool != "" {
			successMsg = "Created/ensured user '" + user + "' and pool '" + pool + "' with ACL"
		} else {
			successMsg = "User/pool ensured"
		}
	}

	// Instruct browser not to cache this page; data must reflect current PVE state
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	// Build base template data
	data := AdminPageDataWithMessage("Proxmox Users & Pools", "userpool", successMsg, "")

	// Fetch pools that match pattern pvmss_*
	client := h.stateManager.GetProxmoxClient()
	if client != nil {
		type poolListItem struct {
			PoolID  string `json:"poolid"`
			Comment string `json:"comment"`
		}
		var listResp struct {
			Data []poolListItem `json:"data"`
		}

		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()

		// Ensure we fetch fresh data for pool listing
		if c, ok := client.(*proxmox.Client); ok && c != nil {
			c.InvalidateCache("/pools")
		}

		// GET /pools to list all pools
		if err := client.GetJSON(ctx, "/pools", &listResp); err == nil {
			// Prepare detailed info per pool
			type poolTableRow struct {
				User    string
				Pool    string
				VMCount int
				Comment string
			}
			rows := make([]poolTableRow, 0)
			var rowsMux sync.Mutex

			// Concurrency limiter
			workerLimit := 6
			sem := make(chan struct{}, workerLimit)
			var wg sync.WaitGroup

			for _, p := range listResp.Data {
				if !strings.HasPrefix(p.PoolID, "pvmss_") {
					continue
				}

				p := p // capture loop var
				wg.Add(1)
				sem <- struct{}{}
				go func() {
					defer wg.Done()
					defer func() { <-sem }()

					row := poolTableRow{
						User:    strings.TrimPrefix(p.PoolID, "pvmss_"),
						Pool:    p.PoolID,
						Comment: p.Comment,
					}

					// Fetch pool members to count VMs: GET /pools/{poolid}
					if c, ok := client.(*proxmox.Client); ok && c != nil {
						c.InvalidateCache("/pools/" + p.PoolID)
					}
					var detailResp struct {
						Data struct {
							Members []struct {
								Type     string `json:"type"`
								VMID     int    `json:"vmid"`
								Template int    `json:"template"`
							} `json:"members"`
						} `json:"data"`
					}
					if err := client.GetJSON(ctx, "/pools/"+url.PathEscape(p.PoolID), &detailResp); err == nil {
						vmCount := 0
						for _, m := range detailResp.Data.Members {
							// Count QEMU or LXC guests (exclude storage and other types). Prefer presence of vmid>0.
							// Skip templates when Template flag is set (1).
							if m.VMID > 0 && m.Template != 1 {
								if strings.EqualFold(m.Type, "qemu") || strings.EqualFold(m.Type, "lxc") || m.Type == "" {
									vmCount++
								}
							}
						}
						row.VMCount = vmCount
					}

					rowsMux.Lock()
					rows = append(rows, row)
					rowsMux.Unlock()
				}()
			}

			wg.Wait()

			if len(rows) > 0 {
				data["UserPools"] = rows
			}
		}
	}

	renderTemplateInternal(w, r, "admin_userpool", data)
}

// CreateUserPool handles POST to create a user in PVE realm, create pool pvmss_<username>, and grant ACL
func (h *UserPoolHandler) CreateUserPool(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("CreateUserPool", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	username := strings.TrimSpace(r.FormValue("username"))
	password := r.FormValue("password")
	email := strings.TrimSpace(r.FormValue("email"))
	comment := strings.TrimSpace(r.FormValue("comment"))
	role := strings.TrimSpace(r.FormValue("role"))
	if role == "" {
		role = "PVMSSUser" // Use our custom role with console permissions
	}
	propagate := r.FormValue("propagate") == "1" || strings.EqualFold(r.FormValue("propagate"), "on")

	if username == "" || password == "" {
		http.Error(w, "username and password are required", http.StatusBadRequest)
		return
	}

	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		http.Error(w, "Proxmox client not available", http.StatusServiceUnavailable)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Ensure user
	if err := proxmox.EnsureUser(ctx, client, username, password, email, comment, "pve", true); err != nil {
		log.Error().Err(err).Str("username", username).Msg("EnsureUser failed")
		http.Error(w, "failed to ensure user: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure custom role with console permissions exists
	consoleRoleID := "PVMSSUser"
	consolePrivileges := []string{
		"VM.Audit",        // View VM status and configuration
		"VM.Console",      // Access VM console (VNC/noVNC)
		"VM.PowerMgmt",    // Start, stop, reset VMs
		"VM.Config.CDROM", // Mount ISO files
		"Datastore.Audit", // View datastore status
		"Pool.Audit",      // View pool contents
	}
	if err := proxmox.EnsureRole(ctx, client, consoleRoleID, consolePrivileges); err != nil {
		log.Error().Err(err).Str("role", consoleRoleID).Msg("EnsureRole failed")
		http.Error(w, "failed to ensure console role: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Ensure pool
	poolID := "pvmss_" + sanitizeID(username)
	if err := proxmox.EnsurePool(ctx, client, poolID, "PVMSS pool for "+username); err != nil {
		log.Error().Err(err).Str("pool", poolID).Msg("EnsurePool failed")
		http.Error(w, "failed to ensure pool: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Grant ACL on pool to user
	userID := username
	if !strings.Contains(userID, "@") {
		userID = userID + "@pve"
	}
	if err := proxmox.EnsurePoolACL(ctx, client, userID, poolID, role, propagate); err != nil {
		log.Error().Err(err).Str("user", userID).Str("pool", poolID).Str("role", role).Msg("EnsurePoolACL failed")
		http.Error(w, "failed to grant pool ACL: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect with success banner
	redir := "/admin/userpool?success=1&user=" + url.QueryEscape(userID) + "&pool=" + url.QueryEscape(poolID)
	http.Redirect(w, r, redir, http.StatusSeeOther)
}

func sanitizeID(s string) string {
	// very basic: lowercase and replace spaces with underscore; Proxmox poolid allows [A-Za-z0-9\-_.]+
	s = strings.TrimSpace(s)
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "_")
	return s
}
