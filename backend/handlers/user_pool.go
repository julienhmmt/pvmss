package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/components"
	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// deriveUserFromPool extracts username from pool ID (pvmss_username)
func deriveUserFromPool(poolID string) string {
	username := strings.TrimPrefix(poolID, "pvmss_")
	if username != "" && !strings.Contains(username, "@") {
		return username + "@pve"
	}
	return username
}

// buildUserPoolSuccessMessage creates success message from query parameters
func buildUserPoolSuccessMessage(r *http.Request) string {
	if r.URL.Query().Get("success") == "" {
		return ""
	}

	action := r.URL.Query().Get("action")
	user := r.URL.Query().Get("user")
	pool := r.URL.Query().Get("pool")

	switch action {
	case "create":
		if user != "" && pool != "" {
			return fmt.Sprintf("Created user '%s' and pool '%s' with ACL", user, pool)
		}
		return "User/pool created"
	case "delete":
		if pool != "" {
			return fmt.Sprintf("Deleted pool, user, and VMs for '%s'", pool)
		}
		return "User/pool deleted"
	default:
		return "User/pool updated"
	}
}

// invalidatePoolCaches invalidates pool-related caches
func invalidatePoolCaches(client interface{}, poolID string) {
	if c, ok := client.(*proxmox.Client); ok && c != nil {
		c.InvalidateCache("/pools")
		if poolID != "" {
			c.InvalidateCache("/pools/" + poolID)
		}
	}
}

// UserPoolHandler handles Proxmox user/pool admin flows
type UserPoolHandler struct {
	stateManager state.StateManager
}

// DeleteUserPool deletes all VMs in the pool (purge), then the derived user, then the pool itself.
// TODO Telmate migration: this handler still uses Telmate-based pool and user helpers (GetJSON/DeleteWithContext/InvalidateCache); migrate it to Resty-based access and pool helpers.
func (h *UserPoolHandler) DeleteUserPool(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("DeleteUserPool", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	poolID := strings.TrimSpace(r.FormValue("pool"))
	if poolID == "" {
		localizer := i18n.GetLocalizerFromRequest(r)
		errMsg := i18n.Localize(localizer, "Admin.UserPool.Error.MissingPool")
		u, _ := url.Parse("/admin/userpool")
		q := u.Query()
		q.Set("error", "1")
		q.Set("error_msg", errMsg)
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}

	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		localizer := i18n.GetLocalizerFromRequest(r)
		errMsg := i18n.Localize(localizer, "Admin.UserPool.Error.ClientUnavailable")
		u, _ := url.Parse("/admin/userpool")
		q := u.Query()
		q.Set("error", "1")
		q.Set("error_msg", errMsg)
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	// Derive user from pool id
	userID := deriveUserFromPool(poolID)

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
		localizer := i18n.GetLocalizerFromRequest(r)
		errMsg := i18n.Localize(localizer, "Admin.UserPool.Error.ResolveMembers")
		u, _ := url.Parse("/admin/userpool")
		q := u.Query()
		q.Set("error", "1")
		q.Set("error_msg", errMsg)
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}
	// First, stop each guest in bulk (concurrently), then wait a short fixed delay
	{
		var wg sync.WaitGroup
		for _, m := range detailResp.Data.Members {
			if m.VMID <= 0 {
				continue
			}
			if m.Node == "" {
				log.Warn().Int("vmid", m.VMID).Msg("Skipping stop due to missing node")
				continue
			}
			m := m // capture loop var
			wg.Add(1)
			go func(member struct {
				Type string `json:"type"`
				VMID int    `json:"vmid"`
				Node string `json:"node"`
			}) {
				defer wg.Done()
				switch strings.ToLower(member.Type) {
				case "qemu":
					if restyClient, err := getDefaultRestyClient(); err == nil {
						if _, err := proxmox.VMActionResty(ctx, restyClient, member.Node, strconv.Itoa(member.VMID), "stop"); err != nil {
							log.Warn().Err(err).Int("vmid", member.VMID).Str("node", member.Node).Msg("Failed to stop QEMU VM (resty); continuing")
						}
					}
				default:
					// ignore other member types
				}
			}(m)
		}
		wg.Wait()
		// Fixed small wait to give Proxmox time to transition state
		time.Sleep(3 * time.Second)
	}

	// Then delete each guest (qemu + lxc)
	for _, m := range detailResp.Data.Members {
		if m.VMID <= 0 {
			continue
		}
		if m.Node == "" {
			log.Warn().Int("vmid", m.VMID).Msg("Skipping deletion due to missing node")
			continue
		}
		switch strings.ToLower(m.Type) {
		case "qemu":
			path := "/nodes/" + url.PathEscape(m.Node) + "/qemu/" + url.PathEscape(strconv.Itoa(m.VMID)) + "?purge=1"
			if _, err := client.DeleteWithContext(ctx, path, nil); err != nil {
				log.Error().Err(err).Str("path", path).Msg("Failed to delete VM")
				localizer := i18n.GetLocalizerFromRequest(r)
				errMsg := i18n.Localize(localizer, "Admin.UserPool.Error.DeleteVM")
				u, _ := url.Parse("/admin/userpool")
				q := u.Query()
				q.Set("error", "1")
				q.Set("error_msg", errMsg)
				u.RawQuery = q.Encode()
				http.Redirect(w, r, u.String(), http.StatusSeeOther)
				return
			}
		default:
			// ignore other member types
		}
	}

	// Verify pool is now empty before attempting to delete it (short polling with cache invalidation)
	{
		emptyDeadline := time.Now().Add(15 * time.Second)
		for {
			if c, ok := client.(*proxmox.Client); ok && c != nil {
				c.InvalidateCache("/pools/" + poolID)
			}
			var check struct {
				Data struct {
					Members []any `json:"members"`
				} `json:"data"`
			}
			if err := client.GetJSON(ctx, "/pools/"+url.PathEscape(poolID), &check); err == nil {
				if len(check.Data.Members) == 0 {
					break
				}
			}
			if time.Now().After(emptyDeadline) {
				log.Warn().Str("pool", poolID).Msg("Pool still not empty after deletions; proceeding to try delete anyway")
				break
			}
			time.Sleep(1 * time.Second)
		}
	}

	// Delete the pool first
	if _, err := client.DeleteWithContext(ctx, "/pools/"+url.PathEscape(poolID), nil); err != nil {
		log.Error().Err(err).Str("pool", poolID).Msg("Failed to delete pool")
		localizer := i18n.GetLocalizerFromRequest(r)
		errMsg := i18n.Localize(localizer, "Admin.UserPool.Error.DeletePool")
		u, _ := url.Parse("/admin/userpool")
		q := u.Query()
		q.Set("error", "1")
		q.Set("error_msg", errMsg)
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}

	// Invalidate caches for fresh state after pool deletion
	invalidatePoolCaches(client, poolID)

	// Then attempt to delete the user (non-fatal)
	if userID != "" {
		if _, err := client.DeleteWithContext(ctx, "/access/users/"+url.PathEscape(userID), nil); err != nil {
			log.Warn().Err(err).Str("user", userID).Msg("Failed to delete user; deletion completed without user removal")
		}
	}

	// Redirect with success (localized)
	localizer := i18n.GetLocalizerFromRequest(r)
	successMsg := i18n.Localize(localizer, "Admin.UserPool.Success.Deleted")
	u, _ := url.Parse("/admin/userpool")
	q := u.Query()
	q.Set("success", "1")
	q.Set("success_msg", successMsg)
	q.Set("action", "delete")
	q.Set("pool", poolID)
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}

func NewUserPoolHandler(sm state.StateManager) *UserPoolHandler {
	return &UserPoolHandler{stateManager: sm}
}

// DeleteUserPoolConfirmHandler handles the GET request for user pool deletion confirmation page
func (h *UserPoolHandler) DeleteUserPoolConfirmHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("DeleteUserPoolConfirmHandler", r)
	poolID := strings.TrimSpace(r.URL.Query().Get("pool"))
	if poolID == "" {
		http.Redirect(w, r, "/admin/userpool", http.StatusSeeOther)
		return
	}

	// Build Templ data
	deleteData := components.AdminUserPoolDeleteData{
		Username:  getUsernameFromSession(r),
		Lang:      i18n.GetLanguage(r),
		CSRFToken: getCSRFTokenFromContext(r),
		Pool:      poolID,
		User:      strings.TrimPrefix(poolID, "pvmss_"),
	}

	T := getTranslationFunc(r)
	if err := components.AdminUserPoolDeletePage(deleteData, T).Render(r.Context(), w); err != nil {
		log.Error().Err(err).Msg("Failed to render admin userpool delete page")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// RegisterRoutes registers routes for user/pool admin
func (h *UserPoolHandler) RegisterRoutes(router *httprouter.Router) {
	routeHelpers := NewAdminPageRoutes()

	// Register admin user pool routes using helper
	routeHelpers.RegisterCRUDRoutes(router, "/admin/userpool", map[string]func(w http.ResponseWriter, r *http.Request, ps httprouter.Params){
		"page": h.UserPoolPage,
	})

	// Register delete confirmation page (with and without lang prefixes)
	router.GET("/admin/userpool/delete", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.DeleteUserPoolConfirmHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
	router.GET("/en/admin/userpool/delete", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.DeleteUserPoolConfirmHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
	router.GET("/fr/admin/userpool/delete", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.DeleteUserPoolConfirmHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	// Admin user pool creation with CSRF protection (without lang prefix)
	router.POST("/admin/userpool/create", SecureFormHandler("CreateUserPool",
		HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
			h.CreateUserPool(w, r, httprouter.ParamsFromContext(r.Context()))
		})),
	))

	// Admin user pool deletion with CSRF protection (without lang prefix)
	router.POST("/admin/userpool/delete", SecureFormHandler("DeleteUserPool",
		HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
			h.DeleteUserPool(w, r, httprouter.ParamsFromContext(r.Context()))
		})),
	))

	// Also register with lang prefixes for compatibility
	router.POST("/en/admin/userpool/create", SecureFormHandler("CreateUserPool",
		HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
			h.CreateUserPool(w, r, httprouter.ParamsFromContext(r.Context()))
		})),
	))

	router.POST("/fr/admin/userpool/create", SecureFormHandler("CreateUserPool",
		HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
			h.CreateUserPool(w, r, httprouter.ParamsFromContext(r.Context()))
		})),
	))

	// Admin user pool self-creation with CSRF protection (without lang prefix)
	router.POST("/admin/userpool/create-self", SecureFormHandler("CreateUserPoolSelf",
		HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
			h.CreateUserPoolSelf(w, r, httprouter.ParamsFromContext(r.Context()))
		})),
	))

	// Also register with lang prefixes for compatibility
	router.POST("/en/admin/userpool/create-self", SecureFormHandler("CreateUserPoolSelf",
		HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
			h.CreateUserPoolSelf(w, r, httprouter.ParamsFromContext(r.Context()))
		})),
	))

	router.POST("/fr/admin/userpool/create-self", SecureFormHandler("CreateUserPoolSelf",
		HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
			h.CreateUserPoolSelf(w, r, httprouter.ParamsFromContext(r.Context()))
		})),
	))

	router.POST("/en/admin/userpool/delete", SecureFormHandler("DeleteUserPool",
		HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
			h.DeleteUserPool(w, r, httprouter.ParamsFromContext(r.Context()))
		})),
	))

	router.POST("/fr/admin/userpool/delete", SecureFormHandler("DeleteUserPool",
		HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
			h.DeleteUserPool(w, r, httprouter.ParamsFromContext(r.Context()))
		})),
	))
}

// UserPoolPage renders the admin page for creating users/pools
func (h *UserPoolHandler) UserPoolPage(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Prefer explicit localized messages from params
	q := r.URL.Query()
	successMsg := q.Get("success_msg")
	if successMsg == "" {
		successMsg = buildUserPoolSuccessMessage(r)
	}
	var errorMsg string
	if q.Get("error") == "1" {
		errorMsg = q.Get("error_msg")
	}

	// Instruct browser not to cache this page; data must reflect current PVE state
	w.Header().Set("Cache-Control", "no-store, no-cache, must-revalidate, max-age=0")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")

	// Build base template data
	opts := []TemplateOption{
		WithAdminActive("userpool"),
		WithAuth(r),
		WithProxmoxStatus(h.stateManager),
		WithMessages(r),
		WithData("TitleKey", "Admin.UserPool.Title"),
	}

	if successMsg != "" {
		opts = append(opts, WithSuccess(successMsg))
	}
	if errorMsg != "" {
		opts = append(opts, WithError(errorMsg))
	}

	data := NewTemplateDataWithOptions("", opts...).ToMap()

	// Prepare structures for direct data passing
	type poolTableRow struct {
		User    string
		Pool    string
		VMCount int
		Comment string
	}
	var rows []poolTableRow

	type currentUserPoolStatusType struct {
		HasPool   bool
		Username  string
		PoolName  string
		CanCreate bool
	}
	var currentUserPoolStatus currentUserPoolStatusType

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
		invalidatePoolCaches(client, "")

		// GET /pools to list all pools
		if err := client.GetJSON(ctx, "/pools", &listResp); err == nil {
			rows = make([]poolTableRow, 0)
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

			// Get current authenticated user and check their pool status
			if sessionManager := h.stateManager.GetSessionManager(); sessionManager != nil {
				if username, ok := sessionManager.Get(r.Context(), "username").(string); ok && username != "" {
					// Sanitize username to match pool naming convention
					sanitizedUsername := sanitizeID(username)
					expectedPoolName := "pvmss_" + sanitizedUsername

					// Migration fallback: also check for old pool naming with @pve suffix
					var fallbackPoolName string
					if strings.Contains(username, "@") {
						// For users with @pve suffix, also check the old naming convention
						fallbackPoolName = "pvmss_" + username
					}

					currentUserPoolStatus.Username = username
					currentUserPoolStatus.PoolName = expectedPoolName
					currentUserPoolStatus.CanCreate = true // Admin users can create their own pool

					// Check if user's pool exists in the fetched pools (with migration fallback)
					for _, row := range rows {
						if row.Pool == expectedPoolName {
							currentUserPoolStatus.HasPool = true
							currentUserPoolStatus.PoolName = row.Pool // Use actual pool name found
							break
						}
						// Fallback: check old naming convention for migration compatibility
						if fallbackPoolName != "" && row.Pool == fallbackPoolName {
							currentUserPoolStatus.HasPool = true
							currentUserPoolStatus.PoolName = row.Pool // Use actual pool name found
							break
						}
					}
				}
			}

			if len(rows) > 0 {
				sort.Slice(rows, func(i, j int) bool {
					left := strings.ToLower(rows[i].User)
					right := strings.ToLower(rows[j].User)
					if left == right {
						return rows[i].Pool < rows[j].Pool
					}
					return left < right
				})
				logger.Get().Info().Int("pool_count", len(rows)).Msg("Found PVMSS pools")
			} else {
				logger.Get().Warn().Msg("No PVMSS pools found")
			}
		}
	}

	// Build Templ data
	userpoolTemplData := components.AdminUserPoolData{
		Username:       getUsernameFromSession(r),
		Lang:           i18n.GetLanguage(r),
		CSRFToken:      getCSRFTokenFromContext(r),
		Error:          getBoolFromMap(data, "Error"),
		ErrorMessage:   getStringFromMap(data, "ErrorMessage"),
		Success:        getBoolFromMap(data, "Success"),
		SuccessMessage: getStringFromMap(data, "SuccessMessage"),
	}

	// Convert user pools - use direct data instead of map conversion
	logger.Get().Info().Int("pools_to_convert", len(rows)).Msg("Converting pools to template data")
	for _, p := range rows {
		userpoolTemplData.UserPools = append(userpoolTemplData.UserPools, components.UserPoolInfo{
			User:    p.User,
			Pool:    p.Pool,
			Comment: p.Comment,
			VMCount: p.VMCount,
		})
	}
	logger.Get().Info().Int("final_count", len(userpoolTemplData.UserPools)).Msg("Pools converted successfully")

	// Convert current user pool status - use direct data instead of map conversion
	if currentUserPoolStatus.Username != "" {
		userpoolTemplData.CurrentUserPoolStatus = &components.CurrentUserPoolStatus{
			Username:  currentUserPoolStatus.Username,
			PoolName:  currentUserPoolStatus.PoolName,
			HasPool:   currentUserPoolStatus.HasPool,
			CanCreate: currentUserPoolStatus.CanCreate,
		}
		logger.Get().Info().Str("username", currentUserPoolStatus.Username).Bool("has_pool", currentUserPoolStatus.HasPool).Msg("Current user pool status converted")
	}

	T := getTranslationFunc(r)
	if err := components.AdminUserPoolPage(userpoolTemplData, T).Render(r.Context(), w); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to render admin userpool page")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// CreateUserPool handles POST to create a user in PVE realm, create pool pvmss_<username>, and grant ACL
// TODO Telmate migration: this handler still relies on Telmate-based EnsureUser/EnsurePool/EnsurePoolACL; switch to the Resty-based admin helpers and remove the ClientInterface.
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
		role = "PVMSSUser" // Use our custom role with VM management permissions
	}

	// Auto-detect if creating pool for current admin user
	var isSelfCreation bool
	if sessionManager := h.stateManager.GetSessionManager(); sessionManager != nil {
		if user, ok := sessionManager.Get(r.Context(), "username").(string); ok && user != "" {
			// Check if requested username matches current admin (with or without @pve)
			if strings.EqualFold(username, user) || strings.EqualFold(username, strings.TrimSuffix(user, "@pve")) {
				isSelfCreation = true
				log.Info().Str("username", username).Msg("Auto-detected self-pool creation for admin user")
			}
		}
	}

	// For self-creation, password is not required (user already exists)
	if username == "" || (!isSelfCreation && password == "") {
		localizer := i18n.GetLocalizerFromRequest(r)
		var errMsg string
		if isSelfCreation {
			errMsg = i18n.Localize(localizer, "Admin.UserPool.Error.MissingUsername")
		} else {
			errMsg = i18n.Localize(localizer, "Admin.UserPool.Error.MissingCredentials")
		}
		u, _ := url.Parse("/admin/userpool")
		q := u.Query()
		q.Set("error", "1")
		q.Set("error_msg", errMsg)
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}

	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		localizer := i18n.GetLocalizerFromRequest(r)
		errMsg := i18n.Localize(localizer, "Admin.UserPool.Error.ClientUnavailable")
		u, _ := url.Parse("/admin/userpool")
		q := u.Query()
		q.Set("error", "1")
		q.Set("error_msg", errMsg)
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Ensure user (skip for self-creation since user already exists)
	if !isSelfCreation {
		if err := proxmox.EnsureUser(ctx, client, username, password, email, comment, "pve", true); err != nil {
			log.Error().Err(err).Str("username", username).Msg("EnsureUser failed")
			localizer := i18n.GetLocalizerFromRequest(r)
			errMsg := i18n.Localize(localizer, "Admin.UserPool.Error.EnsureUser")
			u, _ := url.Parse("/admin/userpool")
			q := u.Query()
			q.Set("error", "1")
			q.Set("error_msg", errMsg)
			u.RawQuery = q.Encode()
			http.Redirect(w, r, u.String(), http.StatusSeeOther)
			return
		}
	} else {
		log.Info().Str("username", username).Msg("Skipping EnsureUser for self-creation (admin user already exists)")
	}

	// Ensure custom role with VM management permissions exists (skip for self-creation)
	if !isSelfCreation {
		roleID := "PVMSSUser"
		privileges := []string{
			"VM.Audit",        // View VM status and configuration
			"VM.PowerMgmt",    // Start, stop, reset VMs
			"VM.Console",      // Access VM console (required for noVNC)
			"VM.Config.CDROM", // Mount ISO files
			"Datastore.Audit", // View datastore status
			"Pool.Audit",      // View pool status
		}
		if err := proxmox.EnsureRole(ctx, client, roleID, privileges); err != nil {
			log.Error().Err(err).Str("role", roleID).Msg("EnsureRole failed")
			localizer := i18n.GetLocalizerFromRequest(r)
			errMsg := i18n.Localize(localizer, "Admin.UserPool.Error.EnsureRole")
			u, _ := url.Parse("/admin/userpool")
			q := u.Query()
			q.Set("error", "1")
			q.Set("error_msg", errMsg)
			u.RawQuery = q.Encode()
			http.Redirect(w, r, u.String(), http.StatusSeeOther)
			return
		}
	} else {
		log.Info().Msg("Skipping EnsureRole for self-creation (using existing PVEVMUser role)")
	}

	// Ensure pool
	poolID := "pvmss_" + sanitizeID(username)
	if err := proxmox.EnsurePool(ctx, client, poolID, "PVMSS pool for "+username); err != nil {
		log.Error().Err(err).Str("pool", poolID).Msg("EnsurePool failed")
		localizer := i18n.GetLocalizerFromRequest(r)
		errMsg := i18n.Localize(localizer, "Admin.UserPool.Error.EnsurePool")
		u, _ := url.Parse("/admin/userpool")
		q := u.Query()
		q.Set("error", "1")
		q.Set("error_msg", errMsg)
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}

	// Use existing role for self-creation, custom role for regular creation
	var finalRole string
	if isSelfCreation {
		finalRole = "PVEVMUser" // Use existing Proxmox role
	} else {
		finalRole = role // Use custom role from form (PVMSSUser)
	}

	// Add debug logging before ACL assignment
	userID := username
	if !strings.Contains(username, "@") {
		userID = username + "@pve"
	}
	log.Info().
		Str("user_id", userID).
		Str("pool_id", poolID).
		Str("role", finalRole).
		Bool("is_self_creation", isSelfCreation).
		Msg("About to assign pool ACL permissions")

	// Assign pool permissions to user
	propagate := r.FormValue("propagate") == "true" || r.FormValue("propagate") == "1" || strings.EqualFold(r.FormValue("propagate"), "on")
	if err := proxmox.EnsurePoolACL(ctx, client, userID, poolID, finalRole, propagate); err != nil {
		log.Error().Err(err).
			Str("user_id", userID).
			Str("pool_id", poolID).
			Str("role", finalRole).
			Bool("propagate", propagate).
			Msg("EnsurePoolACL failed")
		localizer := i18n.GetLocalizerFromRequest(r)
		errMsg := i18n.Localize(localizer, "Admin.UserPool.Error.EnsureACL")
		u, _ := url.Parse("/admin/userpool")
		q := u.Query()
		q.Set("error", "1")
		q.Set("error_msg", errMsg)
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}

	// Redirect with success (localized)
	localizer := i18n.GetLocalizerFromRequest(r)
	var successMsg string
	if isSelfCreation {
		successMsg = fmt.Sprintf(i18n.Localize(localizer, "Admin.UserPool.Success.CreatedSelf"), poolID)
	} else {
		successMsg = fmt.Sprintf(i18n.Localize(localizer, "Admin.UserPool.Success.Created"), poolID)
	}
	u, _ := url.Parse("/admin/userpool")
	q := u.Query()
	q.Set("success", "1")
	q.Set("success_msg", successMsg)
	q.Set("action", "create")
	q.Set("user", userID)
	q.Set("pool", poolID)
	if isSelfCreation {
		q.Set("self", "1")
	}
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}

func sanitizeID(s string) string {
	// Basic sanitization for Proxmox pool IDs: lowercase, replace spaces with underscore
	// Also strip realm suffixes (like @pve) for pool naming
	s = strings.TrimSpace(s)

	// Strip realm suffix (everything after @) for pool ID generation
	if atIndex := strings.Index(s, "@"); atIndex != -1 {
		s = s[:atIndex]
	}

	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, " ", "_")
	return s
}

// CreateUserPoolSelf handles POST to create pool and ACL for the currently authenticated admin user
func (h *UserPoolHandler) CreateUserPoolSelf(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("CreateUserPoolSelf", r)
	log.Info().Msg("CreateUserPoolSelf handler called")

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		log.Warn().Msg("Method validation failed or form parsing failed")
		return
	}
	log.Info().Msg("Method validation passed, form parsed")

	// Get current authenticated user from session
	sessionManager := h.stateManager.GetSessionManager()
	if sessionManager == nil {
		localizer := i18n.GetLocalizerFromRequest(r)
		errMsg := i18n.Localize(localizer, "Admin.UserPool.Error.SessionUnavailable")
		u, _ := url.Parse("/admin/userpool")
		q := u.Query()
		q.Set("error", "1")
		q.Set("error_msg", errMsg)
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}

	username, ok := sessionManager.Get(r.Context(), "username").(string)
	if !ok || username == "" {
		log.Warn().Msg("No authenticated user found in session")
		localizer := i18n.GetLocalizerFromRequest(r)
		errMsg := i18n.Localize(localizer, "Admin.UserPool.Error.NoAuthenticatedUser")
		u, _ := url.Parse("/admin/userpool")
		q := u.Query()
		q.Set("error", "1")
		q.Set("error_msg", errMsg)
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}
	log.Info().Str("username", username).Msg("Authenticated user found")

	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		localizer := i18n.GetLocalizerFromRequest(r)
		errMsg := i18n.Localize(localizer, "Admin.UserPool.Error.ClientUnavailable")
		u, _ := url.Parse("/admin/userpool")
		q := u.Query()
		q.Set("error", "1")
		q.Set("error_msg", errMsg)
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()

	// Use standard role and propagate settings for self-created pools
	role := "PVMSSUser"
	propagate := true
	comment := "Auto-created pool for admin user " + username

	// Ensure pool
	poolID := "pvmss_" + sanitizeID(username)
	log.Info().Str("pool", poolID).Msg("Attempting to create pool")
	if err := proxmox.EnsurePool(ctx, client, poolID, comment); err != nil {
		log.Error().Err(err).Str("pool", poolID).Msg("EnsurePool failed for self-created pool")
		localizer := i18n.GetLocalizerFromRequest(r)
		errMsg := i18n.Localize(localizer, "Admin.UserPool.Error.EnsurePool")
		u, _ := url.Parse("/admin/userpool")
		q := u.Query()
		q.Set("error", "1")
		q.Set("error_msg", errMsg)
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}
	log.Info().Str("pool", poolID).Msg("Pool created successfully")

	// Ensure custom role with VM management permissions exists
	roleID := "PVMSSUser"
	privileges := []string{
		"VM.Audit",        // View VM status and configuration
		"VM.PowerMgmt",    // Start, stop, reset VMs
		"VM.Console",      // Access VM console (required for noVNC)
		"VM.Config.CDROM", // Mount ISO files
		"Datastore.Audit", // View datastore status
		"Pool.Audit",      // View pool contents
	}
	if err := proxmox.EnsureRole(ctx, client, roleID, privileges); err != nil {
		log.Error().Err(err).Str("role", roleID).Msg("EnsureRole failed for self-created pool")
		localizer := i18n.GetLocalizerFromRequest(r)
		errMsg := i18n.Localize(localizer, "Admin.UserPool.Error.EnsureRole")
		u, _ := url.Parse("/admin/userpool")
		q := u.Query()
		q.Set("error", "1")
		q.Set("error_msg", errMsg)
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}

	// Grant ACL on pool to user (user already exists in Proxmox as they're authenticated)
	userID := username
	if !strings.Contains(userID, "@") {
		userID = userID + "@pve"
	}
	log.Info().Str("user", userID).Str("pool", poolID).Str("role", role).Msg("Attempting to grant ACL permissions")
	if err := proxmox.EnsurePoolACL(ctx, client, userID, poolID, role, propagate); err != nil {
		log.Error().Err(err).Str("user", userID).Str("pool", poolID).Str("role", role).Msg("EnsurePoolACL failed for self-created pool")
		localizer := i18n.GetLocalizerFromRequest(r)
		errMsg := i18n.Localize(localizer, "Admin.UserPool.Error.EnsureACL")
		u, _ := url.Parse("/admin/userpool")
		q := u.Query()
		q.Set("error", "1")
		q.Set("error_msg", errMsg)
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}
	log.Info().Str("user", userID).Str("pool", poolID).Str("role", role).Msg("ACL permissions granted successfully")

	// Redirect with success (localized)
	localizer := i18n.GetLocalizerFromRequest(r)
	successMsg := fmt.Sprintf(i18n.Localize(localizer, "Admin.UserPool.Success.CreatedSelf"), poolID)
	u, _ := url.Parse("/admin/userpool")
	q := u.Query()
	q.Set("success", "1")
	q.Set("success_msg", successMsg)
	q.Set("action", "create_self")
	q.Set("user", userID)
	q.Set("pool", poolID)
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}
