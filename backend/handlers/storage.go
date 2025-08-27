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

	// Redirect back to storage page with context for success banner
	redirectURL := "/admin/storage?success=1&action=" + action + "&storage=" + url.QueryEscape(storageName)
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
		// Offline-friendly: render page with empty storages and existing settings
		log.Warn().Msg("Proxmox client not available; rendering Storage page in offline/read-only mode")

		// Récupérer les paramètres
		settings := h.stateManager.GetSettings()
		if settings.EnabledStorages == nil {
			settings.EnabledStorages = []string{}
		}

		// Build enabled map from settings for toggles
		enabledMap := make(map[string]bool, len(settings.EnabledStorages))
		for _, s := range settings.EnabledStorages {
			enabledMap[s] = true
		}

		// Success banner via query params
		success := r.URL.Query().Get("success") != ""
		act := r.URL.Query().Get("action")
		stor := r.URL.Query().Get("storage")
		var successMsg string
		if success {
			switch act {
			case "enable":
				successMsg = "Storage '" + stor + "' enabled"
			case "disable":
				successMsg = "Storage '" + stor + "' disabled"
			default:
				successMsg = "Storage settings updated"
			}
		}

		// Préparer les données pour le template (Storages vide)
		data := map[string]interface{}{
			"Title":           "Gestion du stockage",
			"Node":            "",
			"Storages":        []map[string]interface{}{},
			"EnabledStorages": settings.EnabledStorages,
			"EnabledMap":      enabledMap,
			"Success":         success,
			"SuccessMessage":  successMsg,
			"AdminActive":     "storage",
		}

		// Ajouter les traductions et rendre
		i18n.LocalizePage(w, r, data)
		renderTemplateInternal(w, r, "admin_storage", data)
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

	// Success banner via query params
	success := r.URL.Query().Get("success") != ""
	act := r.URL.Query().Get("action")
	stor := r.URL.Query().Get("storage")
	var successMsg string
	if success {
		switch act {
		case "enable":
			successMsg = "Storage '" + stor + "' enabled"
		case "disable":
			successMsg = "Storage '" + stor + "' disabled"
		default:
			successMsg = "Storage settings updated"
		}
	}

	// Préparer les données pour le template
	data := map[string]interface{}{
		"Title":           "Gestion du stockage",
		"Node":            chosenNode,
		"Storages":        storages,
		"EnabledStorages": settings.EnabledStorages,
		"EnabledMap":      enabledMap,
		"Success":         success,
		"SuccessMessage":  successMsg,
		"AdminActive":     "storage",
	}

	// Ajouter les traductions et rendre
	i18n.LocalizePage(w, r, data)
	renderTemplateInternal(w, r, "admin_storage", data)
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

	// Rediriger vers la page de stockage avec bannière succès
	http.Redirect(w, r, "/admin/storage?success=1", http.StatusSeeOther)
}

// RegisterRoutes enregistre les routes liées au stockage
func (h *StorageHandler) RegisterRoutes(router *httprouter.Router) {
	router.GET("/admin/storage", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.StoragePageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
	router.GET("/admin/storage/", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		logger.Get().Debug().Str("path", r.URL.Path).Msg("Redirecting /admin/storage/ to /admin/storage")
		http.Redirect(w, r, "/admin/storage", http.StatusSeeOther)
	})))
	router.POST("/admin/storage/update", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.UpdateStorageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
	router.POST("/admin/storage/toggle", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.ToggleStorageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
}
