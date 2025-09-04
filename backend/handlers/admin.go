package handlers

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
	"pvmss/proxmox"
	"pvmss/state"
)

// AdminHandler handles administration routes
type AdminHandler struct {
	stateManager state.StateManager
}

// NodesPageHandler renders the Nodes admin page
func (h *AdminHandler) NodesPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("NodesPageHandler", r)

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
				var wg sync.WaitGroup
				detailsChan := make(chan *proxmox.NodeDetails, len(nodes))

				for _, nodeName := range nodes {
					wg.Add(1)
					go func(name string) {
						defer wg.Done()
						nd, nErr := proxmox.GetNodeDetails(pc, name)
						if nErr != nil {
							log.Warn().Err(nErr).Str("node", name).Msg("Failed to retrieve node details; skipping node")
							return
						}
						detailsChan <- nd
					}(nodeName)
				}

				wg.Wait()
				close(detailsChan)

				for detail := range detailsChan {
					nodeDetails = append(nodeDetails, detail)
				}
			}
		}
	} else {
		log.Warn().Msg("Proxmox client is not initialized; rendering page without live node data")
	}

	data := AdminPageDataWithMessage("Node Management", "nodes", "", errMsg)
	data["ProxmoxConnected"] = proxmoxConnected
	data["NodeDetails"] = nodeDetails
	renderTemplateInternal(w, r, "admin_nodes", data)
}

// NewAdminHandler creates a new instance of AdminHandler
func NewAdminHandler(sm state.StateManager) *AdminHandler {
	return &AdminHandler{stateManager: sm}
}

// AdminPageHandler handles the administration page
func (h *AdminHandler) AdminPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("AdminPageHandler", r)

	// Authentication is enforced by RequireAuth middleware at route level.
	// Legacy combined admin page is deprecated: redirect to the Nodes subpage.
	log.Info().Msg("Redirecting legacy /admin to /admin/nodes")
	http.Redirect(w, r, "/admin/nodes", http.StatusSeeOther)
}

// RegisterRoutes registers administration routes
func (h *AdminHandler) RegisterRoutes(router *httprouter.Router) {
	log := CreateHandlerLogger("AdminHandler.RegisterRoutes", nil)

	// Register main admin dashboard (protected with admin privileges)
	router.GET("/admin", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.AdminPageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
	log.Debug().Str("method", "GET").Str("path", "/admin").Msg("Admin route registered with RequireAdminAuth")

	// Additional admin subpages (protected with admin privileges)
	router.GET("/admin/nodes", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.NodesPageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
	log.Debug().Str("method", "GET").Str("path", "/admin/nodes").Msg("admin/nodes route registered with RequireAdminAuth")
	log.Info().Msg("Admin routes registered successfully with admin protection")
}
