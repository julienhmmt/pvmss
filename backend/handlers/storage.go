package handlers

import (
	"net/http"
	"net/url"

	"github.com/julienschmidt/httprouter"
	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
	"pvmss/state"
)

// StateManager is an interface that both real and mock state managers implement
type StateManager interface {
	GetProxmoxClient() proxmox.ClientInterface
	GetSettings() *state.AppSettings
	SetSettings(settings *state.AppSettings) error
}

// ToggleStorageHandler toggles a single storage enabled state (auto-save per click, no JS)
func (h *StorageHandler) ToggleStorageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "ToggleStorageHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Logger()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Erreur lors de l'analyse du formulaire", http.StatusBadRequest)
		return
	}

	storageName := r.FormValue("storage")
	action := r.FormValue("action") // "enable" or "disable"
	if storageName == "" || (action != "enable" && action != "disable") {
		http.Error(w, "Requête invalide", http.StatusBadRequest)
		return
	}

	settings := h.stateManager.GetSettings()
	if settings.EnabledStorages == nil {
		settings.EnabledStorages = []string{}
	}

	enabledMap := make(map[string]bool, len(settings.EnabledStorages))
	for _, s := range settings.EnabledStorages {
		enabledMap[s] = true
	}

	changed := false
	if action == "enable" {
		if !enabledMap[storageName] {
			settings.EnabledStorages = append(settings.EnabledStorages, storageName)
			changed = true
		}
	} else { // disable
		if enabledMap[storageName] {
			// remove
			filtered := make([]string, 0, len(settings.EnabledStorages))
			for _, s := range settings.EnabledStorages {
				if s != storageName {
					filtered = append(filtered, s)
				}
			}
			settings.EnabledStorages = filtered
			changed = true
		}
	}

	if changed {
		if err := h.stateManager.SetSettings(settings); err != nil {
			log.Error().Err(err).Msg("Erreur lors de la sauvegarde des paramètres")
			http.Error(w, "Erreur lors de la sauvegarde des paramètres", http.StatusInternalServerError)
			return
		}
	}

	// Redirect back to admin page with context for success banner
	redirectURL := "/admin?success=1&action=" + action + "&storage=" + url.QueryEscape(storageName)
	http.Redirect(w, r, redirectURL, http.StatusSeeOther)
}

// StorageHandler gère les routes liées au stockage
type StorageHandler struct {
	stateManager StateManager
}

// NewStorageHandler crée une nouvelle instance de StorageHandler
func NewStorageHandler(stateManager state.StateManager) *StorageHandler {
	return &StorageHandler{
		stateManager: stateManager,
	}
}

// StoragePageHandler gère la page de gestion du stockage
func (h *StorageHandler) StoragePageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "StorageHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Logger()

	// Récupérer le client Proxmox
	client := h.stateManager.GetProxmoxClient()
	if client == nil {
		http.Error(w, "Proxmox client not available", http.StatusInternalServerError)
		return
	}

	// Récupérer les paramètres
	node := r.URL.Query().Get("node")
	refresh := r.URL.Query().Get("refresh") == "1"

	// Récupérer les paramètres
	settings := h.stateManager.GetSettings()
	if settings.EnabledStorages == nil {
		settings.EnabledStorages = []string{}
	}

	// Utilise l'utilitaire partagé pour récupérer les stockages rendables
	storages, enabledMap, chosenNode, err := FetchRenderableStorages(client, node, settings.EnabledStorages, refresh)
	if err != nil {
		log.Error().Err(err).Msg("Erreur lors de la récupération des stockages")
		http.Error(w, "Erreur lors de la récupération des stockages: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Préparer les données pour le template
	data := map[string]interface{}{
		"Title":           "Gestion du stockage",
		"Node":            chosenNode,
		"Storages":        storages,
		"EnabledStorages": settings.EnabledStorages,
		"EnabledMap":      enabledMap,
	}

	// Ajouter les traductions et rendre
	i18n.LocalizePage(w, r, data)
	renderTemplateInternal(w, r, "storage", data)
}

// UpdateStorageHandler gère la mise à jour des stockages activés
func (h *StorageHandler) UpdateStorageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().
		Str("handler", "UpdateStorageHandler").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Logger()

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Lire les paramètres du formulaire
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Erreur lors de l'analyse du formulaire", http.StatusBadRequest)
		return
	}

	// Récupérer les stockages cochés depuis le formulaire
	enabledStoragesList := r.Form["enabled_storages"]

	// Mettre à jour les paramètres
	settings := h.stateManager.GetSettings()

	// Mettre à jour la liste des stockages activés
	settings.EnabledStorages = enabledStoragesList

	// Sauvegarder les paramètres
	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Erreur lors de la sauvegarde des paramètres")
		http.Error(w, "Erreur lors de la sauvegarde des paramètres", http.StatusInternalServerError)
		return
	}

	// Rediriger vers la page d'administration principale
	http.Redirect(w, r, "/admin?success=true", http.StatusSeeOther)
}

// RegisterRoutes enregistre les routes liées au stockage
func (h *StorageHandler) RegisterRoutes(router *httprouter.Router) {
	router.GET("/admin/storage", h.StoragePageHandler)
	router.POST("/admin/storage/update", h.UpdateStorageHandler)
	router.POST("/admin/storage/toggle", h.ToggleStorageHandler)
}
