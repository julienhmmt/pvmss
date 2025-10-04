package handlers

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"

	"pvmss/i18n"
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
	router.GET("/profile", h.ShowProfile)
}

// VMInfo represents a VM in the user's pool
type VMInfo struct {
	VMID   int
	Name   string
	Node   string
	Status string
}

// ShowProfile renders the user profile page
func (h *ProfileHandler) ShowProfile(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	ctx := NewHandlerContext(w, r, "ProfileHandler.ShowProfile")

	// Require authentication
	if !ctx.RequireAuthentication() {
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

	// Fetch VMs from the user's pool
	vms := h.fetchUserVMs(r.Context(), client, poolName)

	// Prepare template data
	data := map[string]interface{}{
		"Title":           ctx.Translate("Profile.Title"),
		"Username":        username,
		"PoolName":        poolName,
		"VMs":             vms,
		"Lang":            i18n.GetLanguage(r),
		"IsAuthenticated": true,
		"IsAdmin":         ctx.IsAdmin(),
	}

	ctx.RenderTemplate("profile", data)
}

// fetchUserVMs retrieves all VMs in the user's pool with their status
func (h *ProfileHandler) fetchUserVMs(ctx context.Context, client interface {
	GetJSON(ctx context.Context, path string, result interface{}) error
}, poolName string) []VMInfo {
	log := CreateHandlerLogger("fetchUserVMs", nil)

	// Create context with timeout
	fetchCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// Fetch pool details
	var poolResp struct {
		Data struct {
			Members []struct {
				Type     string `json:"type"`
				VMID     int    `json:"vmid"`
				Node     string `json:"node"`
				Name     string `json:"name"`
				Template int    `json:"template"`
			} `json:"members"`
		} `json:"data"`
	}

	if err := client.GetJSON(fetchCtx, "/pools/"+url.PathEscape(poolName), &poolResp); err != nil {
		log.Error().Err(err).Str("pool", poolName).Msg("Failed to fetch pool members")
		return []VMInfo{}
	}

	// Collect VMs (exclude templates and non-QEMU resources)
	vms := make([]VMInfo, 0)
	for _, member := range poolResp.Data.Members {
		// Skip templates and non-VM resources
		if member.Template == 1 || member.VMID <= 0 {
			continue
		}
		if !strings.EqualFold(member.Type, "qemu") {
			continue
		}

		vm := VMInfo{
			VMID: member.VMID,
			Name: member.Name,
			Node: member.Node,
		}

		// Fetch VM status
		if member.Node != "" {
			statusPath := "/nodes/" + url.PathEscape(member.Node) + "/qemu/" + strconv.Itoa(member.VMID) + "/status/current"
			var statusResp struct {
				Data struct {
					Status string `json:"status"`
				} `json:"data"`
			}
			if err := client.GetJSON(fetchCtx, statusPath, &statusResp); err == nil {
				vm.Status = statusResp.Data.Status
			} else {
				log.Warn().Err(err).Int("vmid", member.VMID).Msg("Failed to fetch VM status")
				vm.Status = "unknown"
			}
		} else {
			vm.Status = "unknown"
		}

		vms = append(vms, vm)
	}

	return vms
}
