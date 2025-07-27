package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/julienschmidt/httprouter"
	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// StorageHandler gère les routes liées au stockage
type StorageHandler struct{}

// NewStorageHandler crée une nouvelle instance de StorageHandler
func NewStorageHandler() *StorageHandler {
	return &StorageHandler{}
}

// StoragePageHandler gère la page de gestion du stockage
func (h *StorageHandler) StoragePageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "StorageHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Msg("Début du traitement de la requête StoragePageHandler")

	// Préparer les données pour le template
	data := map[string]interface{}{}

	// Récupérer le client Proxmox depuis l'état global
	client := state.GetGlobalState().GetProxmoxClient()
	if client == nil {
		errMsg := "Proxmox client not available"
		log.Error().Msg(errMsg)
		data["Error"] = errMsg
		i18n.LocalizePage(w, r, data)
		data["Title"] = data["Storage.Title"]
		renderTemplateInternal(w, r, "storage", data)
		return
	}

	// Créer un contexte avec timeout pour la requête API
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	log.Debug().
		Str("timeout", timeout.String()).
		Msg("Récupération des stockages depuis Proxmox")

	// Récupérer les stockages depuis Proxmox
	result, err := proxmox.GetStoragesWithContext(ctx, client)
	if err != nil {
		errMsg := fmt.Sprintf("Échec de la récupération des stockages: %v", err)
		log.Error().Err(err).Msg(errMsg)
		data["Error"] = errMsg
		i18n.LocalizePage(w, r, data)
		data["Title"] = data["Storage.Title"]
		renderTemplateInternal(w, r, "storage", data)
		return
	}

	// Extraire les données de stockage de la réponse
	responseData, ok := result.(map[string]interface{})
	if !ok {
		errMsg := "Format de réponse inattendu de l'API Proxmox"
		log.Error().
			Interface("response_type", fmt.Sprintf("%T", result)).
			Msg(errMsg)
		data["Error"] = errMsg
		i18n.LocalizePage(w, r, data)
		data["Title"] = data["Storage.Title"]
		renderTemplateInternal(w, r, "storage", data)
		return
	}

	// Extraire la liste des stockages
	storageList, ok := responseData["data"].([]interface{})
	if !ok {
		errMsg := "Impossible d'extraire la liste des stockages de la réponse"
		log.Error().
			Interface("data_type", fmt.Sprintf("%T", responseData["data"])).
			Msg(errMsg)
		data["Error"] = errMsg
		i18n.LocalizePage(w, r, data)
		data["Title"] = data["Storage.Title"]
		renderTemplateInternal(w, r, "storage", data)
		return
	}

	log.Debug().
		Int("storage_count", len(storageList)).
		Msg("Liste des stockages récupérée avec succès")

	// Convertir la liste des stockages en format attendu par le template
	storages := make([]map[string]interface{}, 0, len(storageList))
	validItems := 0

	for i, item := range storageList {
		if storageData, ok := item.(map[string]interface{}); ok {
			storages = append(storages, storageData)
			validItems++
		} else {
			log.Warn().
				Int("item_index", i).
				Interface("item_type", fmt.Sprintf("%T", item)).
				Msg("Élément de stockage ignoré (format invalide)")
		}
	}

	log.Debug().
		Int("total_items", len(storageList)).
		Int("valid_items", validItems).
		Msg("Conversion des données de stockage terminée")

	// Ajouter les données à la map pour le template
	data["Storages"] = storages

	// Ajouter les traductions
	i18n.LocalizePage(w, r, data)
	data["Title"] = data["Storage.Title"]

	log.Debug().Msg("Rendu du template de gestion du stockage")
	renderTemplateInternal(w, r, "storage", data)
}

// StorageAPIHandler gère les appels API pour les opérations sur le stockage
func (h *StorageHandler) StorageAPIHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "StorageHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Msg("Début du traitement de la requête StorageAPIHandler")

	// Récupérer le client Proxmox depuis l'état global
	client := state.GetGlobalState().GetProxmoxClient()
	if client == nil {
		errMsg := "Client Proxmox non disponible"
		log.Error().Msg(errMsg)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": errMsg})
		return
	}

	// Créer un contexte avec timeout pour la requête API
	timeout := 10 * time.Second
	ctx, cancel := context.WithTimeout(r.Context(), timeout)
	defer cancel()

	log.Debug().
		Str("timeout", timeout.String()).
		Msg("Exécution de la requête API de stockage")

	// Récupérer les stockages depuis Proxmox
	result, err := proxmox.GetStoragesWithContext(ctx, client)
	if err != nil {
		errMsg := fmt.Sprintf("Échec de la récupération des stockages: %v", err)
		log.Error().Err(err).Msg(errMsg)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": errMsg})
		return
	}

	// Log du succès de la requête
	log.Debug().
		Interface("result_type", fmt.Sprintf("%T", result)).
		Msg("Requête API de stockage exécutée avec succès")

	// Renvoyer la réponse JSON
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(result); err != nil {
		log.Error().Err(err).Msg("Échec de l'encodage de la réponse JSON")
	}
}

// RegisterRoutes enregistre les routes liées au stockage
func (h *StorageHandler) RegisterRoutes(router *httprouter.Router) {
	log := logger.Get().With().
		Str("component", "StorageHandler").
		Str("method", "RegisterRoutes").
		Logger()

	// Définition des routes
	routes := []struct {
		method  string
		path    string
		handler httprouter.Handle
	}{
		{"GET", "/admin/storage", h.StoragePageHandler},
		{"GET", "/api/v1/storage", h.StorageAPIHandler},
		{"GET", "/storage", h.StoragePageHandler},
		{"GET", "/api/storage", h.StorageAPIHandler},
	}

	// Enregistrement des routes
	for _, route := range routes {
		router.Handle(route.method, route.path, route.handler)

		// Journalisation
		log.Debug().
			Str("method", route.method).
			Str("path", route.path).
			Msg("Route de stockage enregistrée")
	}

	// Journalisation du succès
	log.Info().
		Int("routes_count", len(routes)).
		Msg("Routes du gestionnaire de stockage enregistrées avec succès")

	// Note: L'authentification est gérée par le middleware global dans main.go
}
