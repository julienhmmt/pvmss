package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/julienschmidt/httprouter"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// SettingsHandler gère les routes liées aux paramètres
type SettingsHandler struct {
	stateManager state.StateManager
}

// NewSettingsHandler crée une nouvelle instance de SettingsHandler
func NewSettingsHandler(sm state.StateManager) *SettingsHandler {
	return &SettingsHandler{stateManager: sm}
}

// GetSettingsHandler renvoie les paramètres actuels de l'application
func (h *SettingsHandler) GetSettingsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	settings := h.stateManager.GetSettings()
	if settings == nil {
		logger.Get().Error().Msg("Settings not available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Settings not available",
		})
		return
	}

	// Ne pas renvoyer le mot de passe admin
	settingsResponse := map[string]interface{}{
		"tags":   settings.Tags,
		"isos":   settings.ISOs,
		"vmbrs":  settings.VMBRs,
		"limits": settings.Limits,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settingsResponse)
}

// GetAllISOsHandler récupère toutes les images ISO disponibles
func (h *SettingsHandler) GetAllISOsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		logger.Get().Error().Msg("Proxmox client is not initialized")
		http.Error(w, "Proxmox client not available", http.StatusInternalServerError)
		return
	}

	proxmoxClient, ok := client.(*proxmox.Client)
	if !ok {
		logger.Get().Error().Msg("Failed to convert client to *proxmox.Client")
		http.Error(w, "Internal error: Invalid client type", http.StatusInternalServerError)
		return
	}

	appSettings := h.stateManager.GetSettings()
	enabledISOsMap := make(map[string]bool)
	for _, enabledISO := range appSettings.ISOs { // Correction: itérer sur ISOs, pas EnabledISOs
		enabledISOsMap[enabledISO] = true
	}

	// Get all nodes
	nodes, err := proxmox.GetNodeNames(proxmoxClient)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get nodes from Proxmox")
		http.Error(w, "Failed to get nodes", http.StatusInternalServerError)
		return
	}

	// Get all storages
	storages, err := proxmox.GetStorages(proxmoxClient)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get storages from Proxmox")
		http.Error(w, "Failed to get storages", http.StatusInternalServerError)
		return
	}

	var allISOs []ISOInfo
	logger.Get().Debug().Int("storage_count", len(storages)).Msg("Fetching ISOs from storages")

	for _, nodeName := range nodes {
		for _, storage := range storages {
			isNodeInStorage := storage.Nodes == "" || strings.Contains(storage.Nodes, nodeName)
			if !isNodeInStorage || !containsISO(storage.Content) {
				continue
			}

			logger.Get().Debug().Str("node", nodeName).Str("storage", storage.Storage).Msg("Fetching ISO list for storage")
			// Get ISO list for this storage
			isoList, err := proxmox.GetISOList(proxmoxClient, nodeName, storage.Storage)
			if err != nil {
				logger.Get().Warn().Err(err).Str("node", nodeName).Str("storage", storage.Storage).Msg("Could not get ISO list for storage, skipping")
				continue
			}

			for _, iso := range isoList {
				// On ne traite que les fichiers .iso, en ignorant les autres formats comme .img
				if !strings.HasSuffix(iso.VolID, ".iso") {
					logger.Get().Debug().Str("volid", iso.VolID).Msg("Skipping non-ISO file")
					continue
				}

				_, isEnabled := enabledISOsMap[iso.VolID]
				isoInfo := ISOInfo{
					VolID:   iso.VolID,
					Format:  "iso", // On force le format à "iso" car on a déjà filtré
					Size:    iso.Size,
					Node:    nodeName,
					Storage: storage.Storage,
					Enabled: isEnabled,
				}
				allISOs = append(allISOs, isoInfo)
				logger.Get().Debug().Str("volid", iso.VolID).Bool("enabled", isEnabled).Msg("Found ISO")
			}
		}
	}

	logger.Get().Info().Int("total_isos_found", len(allISOs)).Msg("Finished fetching all ISOs")
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string][]ISOInfo{"isos": allISOs}); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to encode ISOs to JSON")
		http.Error(w, "Failed to encode ISOs", http.StatusInternalServerError)
	}
}

// GetAllVMBRsHandler récupère tous les bridges réseau disponibles
func (h *SettingsHandler) GetAllVMBRsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Use shared helper to collect VMBRs
	vmbrs, err := collectAllVMBRs(h.stateManager)
	if err != nil {
		logger.Get().Warn().Err(err).Msg("collectAllVMBRs returned an error")
	}

	// Format for API response
	formatted := make([]map[string]interface{}, 0, len(vmbrs))
	for _, v := range vmbrs {
		formatted = append(formatted, map[string]interface{}{
			"name":        v["iface"],
			"description": v["description"],
			"node":        v["node"],
			"type":        v["type"],
			"method":      v["method"],
			"address":     v["address"],
			"netmask":     v["netmask"],
			"gateway":     v["gateway"],
		})
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"vmbrs":  formatted,
	}); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to encode JSON response")
		w.WriteHeader(http.StatusInternalServerError)
	}
}

// UpdateISOSettingsHandler met à jour les paramètres des ISOs
func (h *SettingsHandler) UpdateISOSettingsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Décoder le corps de la requête
	var requestData struct {
		ISOs []string `json:"isos"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to decode request body")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Invalid request format",
		})
		return
	}

	// Récupérer les paramètres actuels
	stateManager := h.stateManager
	settings := stateManager.GetSettings()
	if settings == nil {
		logger.Get().Error().Msg("Settings not available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Settings not available",
		})
		return
	}

	// Mettre à jour les ISOs
	settings.ISOs = requestData.ISOs
	// Mettre à jour les paramètres dans le state manager
	if err := stateManager.SetSettings(settings); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to update settings")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Failed to update settings",
		})
		return
	}

	// Persister les paramètres dans le fichier
	if err := h.stateManager.SetSettings(settings); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to write settings to file")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Failed to write settings to file",
		})
		return
	}

	// Renvoyer la réponse
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "ISO settings updated",
	})
}

// UpdateVMBRSettingsHandler met à jour les paramètres des VMBRs
func (h *SettingsHandler) UpdateVMBRSettingsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	// Décoder le corps de la requête
	var requestData struct {
		VMBRs []string `json:"vmbrs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to decode request body")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Invalid request format",
		})
		return
	}

	// Récupérer les paramètres actuels
	stateManager := h.stateManager
	settings := stateManager.GetSettings()
	if settings == nil {
		logger.Get().Error().Msg("Settings not available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Settings not available",
		})
		return
	}

	// Mettre à jour les VMBRs
	settings.VMBRs = requestData.VMBRs
	// Mettre à jour les paramètres dans le state manager
	if err := stateManager.SetSettings(settings); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to update settings")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Failed to update settings",
		})
		return
	}

	// Persister les paramètres dans le fichier
	if err := h.stateManager.SetSettings(settings); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to write settings to file")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Failed to write settings to file",
		})
		return
	}

	// Renvoyer la réponse
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "VMBR settings updated",
	})
}

// RegisterRoutes enregistre les routes liées aux paramètres
func (h *SettingsHandler) RegisterRoutes(router *httprouter.Router) {
	// Routes API protégées par authentification
	router.GET("/api/settings", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.GetSettingsHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	router.GET("/api/settings/iso", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.GetAllISOsHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	router.POST("/api/iso/settings", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.UpdateISOSettingsHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	router.GET("/api/vmbr/all", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.GetAllVMBRsHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	router.POST("/api/vmbr/settings", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.UpdateVMBRSettingsHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
}

// containsISO vérifie si un type de contenu de stockage peut contenir des ISOs
func containsISO(content string) bool {
	// Les types de contenu sont séparés par des virgules
	for _, part := range strings.Split(content, ",") {
		if strings.TrimSpace(part) == "iso" {
			return true
		}
	}
	return false
}
