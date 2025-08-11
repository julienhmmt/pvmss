package handlers

import (
	"net/http"
	"strings"

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
	appSettings := h.stateManager.GetSettings()
	if appSettings == nil {
		log.Error().Msg("Application settings are not available")
		http.Error(w, "Internal error: Unable to load settings", http.StatusInternalServerError)
		return
	}

	// Get Proxmox client
	client := h.stateManager.GetProxmoxClient()
	var proxmoxClient *proxmox.Client
	var nodeNames []string
	var nodeDetails []*proxmox.NodeDetails
	var isosList []ISOInfo

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

				// --- Build ISOs list for server-rendered ISO section ---
				// Enabled map from settings
				enabledISOMap := make(map[string]bool)
				for _, v := range appSettings.ISOs {
					enabledISOMap[v] = true
				}

				storages, err := proxmox.GetStorages(proxmoxClient)
				if err != nil {
					log.Warn().Err(err).Msg("Unable to fetch storages for ISO listing")
				} else {
					for _, nodeName := range nodeNames {
						for _, storage := range storages {
							isNodeInStorage := storage.Nodes == "" || strings.Contains(storage.Nodes, nodeName)
							if !isNodeInStorage || !containsISO(storage.Content) { // reuse helper in this package
								continue
							}
							isoList, err := proxmox.GetISOList(proxmoxClient, nodeName, storage.Storage)
							if err != nil {
								log.Warn().Err(err).Str("node", nodeName).Str("storage", storage.Storage).Msg("Could not get ISO list for storage, skipping")
								continue
							}
							for _, iso := range isoList {
								if !strings.HasSuffix(iso.VolID, ".iso") {
									continue
								}
								_, isEnabled := enabledISOMap[iso.VolID]
								isosList = append(isosList, ISOInfo{
									VolID:   iso.VolID,
									Format:  "iso",
									Size:    iso.Size,
									Node:    nodeName,
									Storage: storage.Storage,
									Enabled: isEnabled,
								})
							}
						}
					}
				}
			}
		}
	}

	log.Debug().Msg("Admin page data loaded successfully")

	// Get all VMBRs from all nodes via common helper
	allVMBRs, err := collectAllVMBRs(h.stateManager)
	if err != nil {
		log.Warn().Err(err).Msg("collectAllVMBRs returned an error; continuing with best-effort data")
	}

	// Get current settings to check which VMBRs are enabled
	enabledVMBRs := make(map[string]bool)
	for _, vmbr := range appSettings.VMBRs {
		enabledVMBRs[vmbr] = true
	}

	// --- Storage section for admin page (shared helper) ---
	var storageMaps []map[string]interface{}
	enabledStoragesMap := make(map[string]bool)
	chosenNode := ""
	if client != nil {
		refresh := r.URL.Query().Get("refresh") == "1"
		stor, enMap, nodeName, err := FetchRenderableStorages(client, "", appSettings.EnabledStorages, refresh)
		if err != nil {
			log.Warn().Err(err).Msg("Unable to fetch storages for admin page")
		} else {
			storageMaps = stor
			enabledStoragesMap = enMap
			chosenNode = nodeName
		}
	}

	// Success banner (no JS) via query params
	success := r.URL.Query().Get("success") != ""
	act := r.URL.Query().Get("action")
	stor := r.URL.Query().Get("storage")
	vmbrName := r.URL.Query().Get("vmbr")
	isoName := r.URL.Query().Get("iso")
	var successMsg string
	if success {
		switch {
		case isoName != "":
			switch act {
			case "enable":
				successMsg = "ISO '" + isoName + "' enabled"
			case "disable":
				successMsg = "ISO '" + isoName + "' disabled"
			default:
				successMsg = "ISO settings updated"
			}
		case vmbrName != "":
			switch act {
			case "enable":
				successMsg = "VMBR '" + vmbrName + "' enabled"
			case "disable":
				successMsg = "VMBR '" + vmbrName + "' disabled"
			default:
				successMsg = "VMBR settings updated"
			}
		case stor != "":
			switch act {
			case "enable":
				successMsg = "Storage '" + stor + "' enabled"
			case "disable":
				successMsg = "Storage '" + stor + "' disabled"
			default:
				successMsg = "Storage settings updated"
			}
		default:
			successMsg = "Settings saved"
		}
	}

	// Préparer les données pour le template (includes storage)
	data := map[string]interface{}{
		"Tags":     appSettings.Tags,
		"ISOs":     appSettings.ISOs,
		"ISOsList": isosList,
		"EnabledISOs": func() map[string]bool {
			m := make(map[string]bool)
			for _, v := range appSettings.ISOs {
				m[v] = true
			}
			return m
		}(),
		"VMBRs":           allVMBRs,
		"EnabledVMBRs":    enabledVMBRs,
		"Limits":          appSettings.Limits,
		"NodeDetails":     nodeDetails,
		"Storages":        storageMaps,
		"EnabledStorages": appSettings.EnabledStorages,
		"EnabledMap":      enabledStoragesMap,
		"Node":            chosenNode,
		"Success":         success,
		"SuccessMessage":  successMsg,
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
