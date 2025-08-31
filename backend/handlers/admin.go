package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"pvmss/proxmox"
	"pvmss/state"

	"pvmss/i18n"
	"pvmss/logger"
)

// AdminHandler gère les routes d'administration
type AdminHandler struct {
	stateManager state.StateManager
}

// NodesPageHandler renders the Nodes admin page
func (h *AdminHandler) NodesPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "NodesPageHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Logger()

	// Proxmox connection status from background monitor
	proxmoxConnected, _ := h.stateManager.GetProxmoxStatus()
	client := h.stateManager.GetProxmoxClient()
	var nodeDetails []*proxmox.NodeDetails
	var errMsg string

	if proxmoxConnected && client != nil {
		pc, ok := client.(*proxmox.Client)
		if !ok {
			errMsg = "Invalid Proxmox client type"
		} else {
			// Use a shorter timeout to avoid long blocking even if status recently changed
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()
			nodes, err := proxmox.GetNodeNamesWithContext(ctx, pc)
			if err != nil {
				log.Warn().Err(err).Msg("Unable to retrieve Proxmox nodes")
				errMsg = "Failed to retrieve nodes"
			} else {
				for _, nodeName := range nodes {
					nd, nErr := proxmox.GetNodeDetails(pc, nodeName)
					if nErr != nil {
						log.Warn().Err(nErr).Str("node", nodeName).Msg("Failed to retrieve node details; skipping node")
						continue
					}
					nodeDetails = append(nodeDetails, nd)
				}
			}
		}
	} else {
		log.Warn().Msg("Proxmox client is not initialized; rendering page without live node data")
	}

	data := map[string]interface{}{
		"ProxmoxConnected": proxmoxConnected,
		"NodeDetails":      nodeDetails,
		"AdminActive":      "nodes",
	}
	if errMsg != "" {
		data["Error"] = errMsg
	}
	i18n.LocalizePage(w, r, data)
	renderTemplateInternal(w, r, "admin_nodes", data)
}

// NewAdminHandler crée une nouvelle instance de AdminHandler
func NewAdminHandler(sm state.StateManager) *AdminHandler {
	return &AdminHandler{stateManager: sm}
}

// AdminPageHandler gère la page d'administration
func (h *AdminHandler) AdminPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "AdminHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	// Authentication is enforced by RequireAuth middleware at route level.
	// Legacy combined admin page is deprecated: redirect to the Nodes subpage.
	log.Info().Msg("Redirecting legacy /admin to /admin/nodes")
	http.Redirect(w, r, "/admin/nodes", http.StatusSeeOther)
}

// RegisterRoutes enregistre les routes d'administration
func (h *AdminHandler) RegisterRoutes(router *httprouter.Router) {
	log := logger.Get().With().
		Str("component", "AdminHandler").
		Str("method", "RegisterRoutes").
		Logger()

	// Register main admin dashboard (protected with admin privileges)
	router.GET("/admin", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.AdminPageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
	log.Debug().Str("method", "GET").Str("path", "/admin").Msg("Route d'administration enregistrée avec RequireAdminAuth")

	// Additional admin subpages (protected with admin privileges)
	router.GET("/admin/nodes", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.NodesPageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
	log.Debug().Str("method", "GET").Str("path", "/admin/nodes").Msg("Route admin/nodes enregistrée avec RequireAdminAuth")

	log.Info().Msg("Routes d'administration enregistrées avec succès avec protection admin")
}
