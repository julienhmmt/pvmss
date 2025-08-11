package handlers

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"pvmss/proxmox"
	"pvmss/state"

	"pvmss/i18n"
	"pvmss/logger"
	"strings"
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

    // --- Storage section for admin page ---
    var storageMaps []map[string]interface{}
    enabledStoragesMap := make(map[string]bool, len(appSettings.EnabledStorages))
    for _, s := range appSettings.EnabledStorages {
        enabledStoragesMap[s] = true
    }

    if client != nil {
        // Detect node: first available
        node := ""
        if n, err := proxmox.GetNodeNames(client); err == nil && len(n) > 0 {
            node = n[0]
        }

        // Fetch global storage config and node storages
        if node != "" {
            globalStorages, err := proxmox.GetStorages(client)
            if err != nil {
                log.Warn().Err(err).Msg("Unable to fetch global storages for admin page")
            } else {
                cfgByName := make(map[string]proxmox.Storage, len(globalStorages))
                for _, s := range globalStorages {
                    cfgByName[s.Storage] = s
                }
                nodeStorages, err := proxmox.GetNodeStorages(client, node)
                if err != nil {
                    log.Warn().Err(err).Str("node", node).Msg("Unable to fetch node storages for admin page")
                } else {
                    for _, st := range nodeStorages {
                        // Enrich from global config
                        if cfg, ok := cfgByName[st.Storage]; ok {
                            if st.Content == "" && cfg.Content != "" {
                                st.Content = cfg.Content
                            }
                            if st.Type == "" && cfg.Type != "" {
                                st.Type = cfg.Type
                            }
                            if st.Description == "" && cfg.Description != "" {
                                st.Description = cfg.Description
                            }
                        }

                        // Filter VM-disk capable: exclude PBS; include images or known types
                        if strings.EqualFold(st.Type, "pbs") {
                            continue
                        }
                        canHold := false
                        if st.Content != "" && strings.Contains(st.Content, "images") {
                            canHold = true
                        } else if st.Content == "" {
                            switch strings.ToLower(st.Type) {
                            case "dir", "lvm", "lvmthin", "zfs", "rbd", "ceph", "cephfs", "nfs", "glusterfs":
                                canHold = true
                            }
                        }
                        if !canHold {
                            continue
                        }

                        used, _ := st.Used.Int64()
                        total, _ := st.Total.Int64()
                        percent := 0
                        if total > 0 {
                            percent = int((used * 100) / total)
                        }
                        sm := map[string]interface{}{
                            "Storage":     st.Storage,
                            "Type":        st.Type,
                            "Used":        used,
                            "Total":       total,
                            "Description": st.Description,
                            "Enabled":     len(appSettings.EnabledStorages) == 0 || enabledStoragesMap[st.Storage],
                            "Content":     st.Content,
                            "UsedPercent": percent,
                        }
                        if st.Avail.String() != "" {
                            if avail, err := st.Avail.Int64(); err == nil {
                                sm["Available"] = avail
                            }
                        }
                        storageMaps = append(storageMaps, sm)
                    }
                }
            }
        }
    }

    // Préparer les données pour le template (includes storage)
    data := map[string]interface{}{
        "Tags":             appSettings.Tags,
        "ISOs":             appSettings.ISOs,
        "VMBRs":            allVMBRs,
        "EnabledVMBRs":     enabledVMBRs,
        "Limits":           appSettings.Limits,
        "NodeDetails":      nodeDetails,
        "Storages":         storageMaps,
        "EnabledStorages":  appSettings.EnabledStorages,
        "EnabledMap":       enabledStoragesMap,
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
