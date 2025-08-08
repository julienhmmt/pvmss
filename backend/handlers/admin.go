package handlers

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"pvmss/proxmox"
	"pvmss/state"

	"pvmss/i18n"
	"pvmss/logger"
)

// AdminHandler gère les routes d'administration
type AdminHandler struct{}

// NewAdminHandler crée une nouvelle instance de AdminHandler
func NewAdminHandler() *AdminHandler {
	return &AdminHandler{}
}

// AdminPageHandler gère la page d'administration
func (h *AdminHandler) AdminPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "AdminHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Msg("Attempting to access admin page")

	// Check permissions (uses session)
	if !IsAuthenticated(r) {
		errMsg := "Access denied: unauthenticated user"
		log.Warn().
			Str("status", "forbidden").
			Str("remote_addr", r.RemoteAddr).
			Msg(errMsg)

		// Rediriger vers la page de connexion avec une URL de retour
		http.Redirect(w, r, "/login?return="+r.URL.Path, http.StatusSeeOther)
		return
	}

	log.Debug().Msg("Preparing data for admin page")

	// Get current application settings
	appSettings := state.GetSettings()
	if appSettings == nil {
		log.Error().Msg("Application settings are not available")
		http.Error(w, "Internal error: Unable to load settings", http.StatusInternalServerError)
		return
	}

	// Get Proxmox client
	client := state.GetGlobalState().GetProxmoxClient()
	var proxmoxClient *proxmox.Client
	var nodeNames []string
	var nodeDetails []*proxmox.NodeDetails

	if client == nil {
		// Offline mode: proceed without Proxmox
		log.Warn().Msg("Proxmox client is not initialized; continuing in offline/read-only mode")
	} else {
		// Type assert client to *proxmox.Client for functions that haven't been updated to use the interface
		pc, ok := client.(*proxmox.Client)
		if !ok {
			log.Error().Msg("Failed to convert client to *proxmox.Client; continuing without Proxmox data")
		} else {
			proxmoxClient = pc
			// Attempt to get node names; on failure, continue gracefully
			n, err := proxmox.GetNodeNames(proxmoxClient)
			if err != nil {
				log.Warn().Err(err).Msg("Unable to retrieve Proxmox nodes; continuing with empty node list")
			} else {
				nodeNames = n
				// Get details for each node
				for _, nodeName := range nodeNames {
					nodeDetail, err := proxmox.GetNodeDetails(proxmoxClient, nodeName)
					if err != nil {
						log.Warn().Err(err).Str("node", nodeName).Msg("Failed to retrieve node details; skipping node")
						continue
					}
					nodeDetails = append(nodeDetails, nodeDetail)
				}
			}
		}
	}

	log.Debug().Msg("Admin page data loaded successfully")

	// Get all VMBRs from all nodes
	allVMBRs := make([]map[string]string, 0)
	if proxmoxClient != nil && len(nodeNames) > 0 {
		for _, node := range nodeNames {
			vmbrs, err := proxmox.GetVMBRs(proxmoxClient, node)
			if err != nil {
				log.Warn().Err(err).Str("node", node).Msg("Failed to get VMBRs for node")
				continue
			}

			for _, vmbr := range vmbrs {
				if vmbr.Type == "bridge" {
					allVMBRs = append(allVMBRs, map[string]string{
						"node":        node,
						"iface":       vmbr.Iface,
						"type":        vmbr.Type,
						"method":      vmbr.Method,
						"address":     vmbr.Address,
						"netmask":     vmbr.Netmask,
						"gateway":     vmbr.Gateway,
						"description": "", // VMBR struct doesn't have a description field
					})
				}
			}
		}
	} else {
		log.Warn().Msg("Skipping VMBR retrieval due to missing Proxmox client or nodes")
	}

	// Get current settings to check which VMBRs are enabled
	enabledVMBRs := make(map[string]bool)
	for _, vmbr := range appSettings.VMBRs {
		enabledVMBRs[vmbr] = true
	}

	// Préparer les données pour le template
	data := map[string]interface{}{
		"Tags":         appSettings.Tags,
		"ISOs":         appSettings.ISOs,
		"VMBRs":        allVMBRs,
		"EnabledVMBRs": enabledVMBRs,
		"Limits":       appSettings.Limits,
		"NodeDetails":  nodeDetails,
	}

	// Ajouter les traductions
	i18n.LocalizePage(w, r, data)
	data["Title"] = data["Admin.Title"]

	log.Debug().Msg("Rendu du template d'administration")
	renderTemplateInternal(w, r, "admin", data)

	log.Info().Msg("Page d'administration affichée avec succès")
}

// RegisterRoutes enregistre les routes d'administration
func (h *AdminHandler) RegisterRoutes(router *httprouter.Router) {
	log := logger.Get().With().
		Str("component", "AdminHandler").
		Str("method", "RegisterRoutes").
		Logger()

	// Définition des routes
	routes := []struct {
		method  string
		path    string
		handler httprouter.Handle
		desc    string
	}{
		{"GET", "/admin", h.AdminPageHandler, "Page d'administration"},
	}

	// Enregistrement des routes
	for _, route := range routes {
		router.Handle(route.method, route.path, route.handler)
		log.Debug().
			Str("method", route.method).
			Str("path", route.path).
			Str("description", route.desc).
			Msg("Route d'administration enregistrée")
	}

	log.Info().
		Int("routes_count", len(routes)).
		Msg("Routes d'administration enregistrées avec succès")
}
