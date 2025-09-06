package handlers

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/proxmox"
	"pvmss/state"
)

// UserPoolHandler handles Proxmox user/pool admin flows
type UserPoolHandler struct {
	stateManager state.StateManager
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

		// GET /pools to list all pools
		if err := client.GetJSON(ctx, "/pools", &listResp); err == nil {
			// Prepare detailed info per pool
			type poolTableRow struct {
				User     string
				Pool     string
				VMCount  int
				Users    []string
				Comment  string
			}
			rows := make([]poolTableRow, 0)

			for _, p := range listResp.Data {
				if !strings.HasPrefix(p.PoolID, "pvmss_") {
					continue
				}

				row := poolTableRow{
					User:    strings.TrimPrefix(p.PoolID, "pvmss_"),
					Pool:    p.PoolID,
					Comment: p.Comment,
				}

				// Fetch pool members to count VMs: GET /pools/{poolid}
				var detailResp struct {
					Data struct {
						Members []struct {
							Type string `json:"type"`
							VMID int    `json:"vmid"`
						} `json:"members"`
					} `json:"data"`
				}
				if err := client.GetJSON(ctx, "/pools/"+url.PathEscape(p.PoolID), &detailResp); err == nil {
					vmCount := 0
					for _, m := range detailResp.Data.Members {
						// Count QEMU VMs; Proxmox uses type "qemu" for KVM VMs
						if strings.EqualFold(m.Type, "qemu") || m.VMID > 0 {
							vmCount++
						}
					}
					row.VMCount = vmCount
				}

				// Fetch ACL users on this pool: GET /access/acl?path=/pool/{poolid}
				var aclResp struct {
					Data []struct {
						Path  string `json:"path"`
						Type  string `json:"type"`
						Ugid  string `json:"ugid"`
						Role  string `json:"roleid"`
						Prop  int    `json:"propagate"`
					} `json:"data"`
				}
				q := "/access/acl?path=" + url.QueryEscape("/pool/"+p.PoolID)
				if err := client.GetJSON(ctx, q, &aclResp); err == nil {
					users := make([]string, 0)
					for _, a := range aclResp.Data {
						// Only include user bindings (type=="user"); ugid is userid like name@pve
						if strings.EqualFold(a.Type, "user") && a.Ugid != "" {
							users = append(users, a.Ugid)
						}
					}
					row.Users = users
				}

				rows = append(rows, row)
			}

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
