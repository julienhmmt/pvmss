package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// SettingsHandler gère les routes liées aux paramètres
type SettingsHandler struct{}

// NewSettingsHandler crée une nouvelle instance de SettingsHandler
func NewSettingsHandler() *SettingsHandler {
	return &SettingsHandler{}
}

// GetSettingsHandler renvoie les paramètres actuels de l'application
func (h *SettingsHandler) GetSettingsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	settings := state.GetGlobalState().GetSettings()
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
	client := state.GetGlobalState().GetProxmoxClient()
	if client == nil {
		logger.Get().Error().Msg("Proxmox client not available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Proxmox client not available",
		})
		return
	}

	// Créer un contexte avec timeout pour la requête API
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Récupérer la liste des nœuds
	nodesResult, err := proxmox.GetNodeNamesWithContext(ctx, client)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get nodes from Proxmox")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to get nodes: %v", err),
		})
		return
	}

	// Récupérer la liste des storages
	storagesResult, err := proxmox.GetStoragesWithContext(ctx, client)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get storages from Proxmox")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to get storages: %v", err),
		})
		return
	}

	// Extraire les données de stockage
	storagesData, ok := storagesResult.(map[string]interface{})
	if !ok {
		logger.Get().Error().Msg("Unexpected response format from Proxmox API")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Unexpected response format from Proxmox API",
		})
		return
	}

	// Extraire la liste des storages
	storagesList, ok := storagesData["data"].([]interface{})
	if !ok {
		logger.Get().Error().Msg("Failed to extract storage list from response")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Failed to extract storage list from response",
		})
		return
	}

	// Collecter toutes les ISOs
	allISOs := make([]map[string]interface{}, 0)

	// Pour chaque nœud et chaque storage, récupérer les ISOs
	for _, node := range nodesResult {
		for _, storageItem := range storagesList {
			storage, ok := storageItem.(map[string]interface{})
			if !ok {
				continue
			}

			// Vérifier si le stockage contient des ISOs
			content, ok := storage["content"].(string)
			if !ok || !containsISO(content) {
				continue
			}

			storageName, ok := storage["storage"].(string)
			if !ok {
				continue
			}

			// Récupérer les ISOs pour ce storage
			isoResult, err := proxmox.GetISOListWithContext(ctx, client, node, storageName)
			if err != nil {
				logger.Get().Warn().Err(err).
					Str("node", node).
					Str("storage", storageName).
					Msg("Failed to get ISOs for storage")
				continue
			}

			isoList, ok := isoResult["data"].([]interface{})
			if !ok {
				continue
			}

			// Ajouter les ISOs à la liste complète
			for _, iso := range isoList {
				isoMap, ok := iso.(map[string]interface{})
				if !ok {
					continue
				}

				// Vérifier si c'est bien une ISO
				contentType, ok := isoMap["content"].(string)
				if !ok || contentType != "iso" {
					continue
				}

				// Ajouter le nœud et le storage aux informations de l'ISO
				isoMap["node"] = node
				isoMap["storage"] = storageName
				allISOs = append(allISOs, isoMap)
			}
		}
	}

	// Renvoyer la réponse
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"isos":   allISOs,
	})
}

// GetAllVMBRsHandler récupère tous les bridges réseau disponibles
func (h *SettingsHandler) GetAllVMBRsHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	client := state.GetGlobalState().GetProxmoxClient()
	if client == nil {
		logger.Get().Error().Msg("Proxmox client not available")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": "Proxmox client not available",
		})
		return
	}

	// Créer un contexte avec timeout pour la requête API
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	// Récupérer la liste des nœuds
	nodes, err := proxmox.GetNodeNamesWithContext(ctx, client)
	if err != nil {
		logger.Get().Error().Err(err).Msg("Failed to get nodes from Proxmox")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to get nodes: %v", err),
		})
		return
	}

	// Collecter tous les VMBRs
	allVMBRs := make([]map[string]interface{}, 0)

	// Pour chaque nœud, récupérer les interfaces réseau
	for _, node := range nodes {
		logger.Get().Debug().
			Str("node", node).
			Msg("Récupération des interfaces réseau pour le nœud")

		vmbrResult, err := proxmox.GetVMBRsWithContext(ctx, client, node)
		if err != nil {
			logger.Get().Warn().Err(err).
				Str("node", node).
				Msg("Échec de la récupération des VMBRs pour le nœud")
			continue
		}

		logger.Get().Debug().
			Str("node", node).
			Interface("result", vmbrResult).
			Msg("Réponse brute de l'API Proxmox pour les interfaces réseau")

		vmbrList, ok := vmbrResult["data"].([]interface{})
		if !ok {
			logger.Get().Warn().
				Str("node", node).
				Interface("result_type", fmt.Sprintf("%T", vmbrResult["data"])).
				Msg("Format de données inattendu pour les interfaces réseau")
			continue
		}

		logger.Get().Debug().
			Str("node", node).
			Int("count", len(vmbrList)).
			Msg("Nombre d'interfaces réseau trouvées")

		// Filtrer pour ne garder que les bridges
		for i, vmbr := range vmbrList {
			vmbrMap, ok := vmbr.(map[string]interface{})
			if !ok {
				logger.Get().Warn().
					Str("node", node).
					Int("index", i).
					Interface("type", fmt.Sprintf("%T", vmbr)).
					Msg("Format d'interface réseau inattendu")
				continue
			}

			// Vérifier si c'est un bridge
			iface, ok := vmbrMap["iface"].(string)
			if !ok {
				logger.Get().Warn().
					Str("node", node).
					Interface("iface_type", fmt.Sprintf("%T", vmbrMap["iface"])).
					Msg("Champ 'iface' manquant ou de type incorrect")
				continue
			}

			if !isVMBR(iface) {
				logger.Get().Debug().
					Str("node", node).
					Str("interface", iface).
					Msg("Interface ignorée (pas un bridge VMBR)")
				continue
			}

			// Ajouter le nœud aux informations du VMBR
			vmbrMap["node"] = node
			vmbrMap["name"] = iface
			if _, ok := vmbrMap["description"]; !ok {
				vmbrMap["description"] = ""
			}
			allVMBRs = append(allVMBRs, vmbrMap)
		}
	}

	// Journaliser le résultat final
	logger.Get().Debug().
		Int("total_vmbrs", len(allVMBRs)).
		Msg("Bridges réseau trouvés")

	// Renvoyer la réponse
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"vmbrs":  allVMBRs,
	})

	if err != nil {
		logger.Get().Error().
			Err(err).
			Msg("Échec de l'encodage de la réponse JSON")
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
	stateManager := state.GetGlobalState()
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
	if err := state.GetGlobalState().SetSettings(settings); err != nil {
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
	stateManager := state.GetGlobalState()
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
	if err := state.GetGlobalState().SetSettings(settings); err != nil {
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

	router.GET("/api/iso/all", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
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

// Fonctions utilitaires

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

// isVMBR vérifie si une interface est un bridge réseau
func isVMBR(iface string) bool {
	return strings.HasPrefix(iface, "vmbr")
}
