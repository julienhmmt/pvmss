package handlers

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/state"
)

// TagsUIHandler gère l'interface utilisateur des tags
type TagsUIHandler struct{}

// NewTagsUIHandler crée un nouveau gestionnaire d'interface utilisateur pour les tags
func NewTagsUIHandler() *TagsUIHandler {
	return &TagsUIHandler{}
}

// RegisterRoutes enregistre les routes pour l'interface utilisateur des tags
func (h *TagsUIHandler) RegisterRoutes(router *httprouter.Router) {
	router.HandlerFunc("GET", "/tags", h.handleTagsPage)
}

// handleTagsPage gère l'affichage de la page des tags
func (h *TagsUIHandler) handleTagsPage(w http.ResponseWriter, r *http.Request) {
	log := logger.Get().With().
		Str("handler", "TagsUI").
		Str("method", r.Method).
		Str("path", r.URL.Path).
		Str("remote_addr", r.RemoteAddr).
		Logger()

	log.Debug().Msg("Traitement de la requête de la page des tags")

	// Vérifier l'authentification en utilisant la fonction existante
	if !IsAuthenticated(r) {
		log.Info().Msg("Utilisateur non authentifié, redirection vers la page de connexion")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Récupérer la langue depuis la session ou les en-têtes
	lang := r.URL.Query().Get("lang")
	if lang == "" {
		lang = "en"
	}

	log = log.With().Str("lang", lang).Logger()
	log.Debug().Msg("Récupération des tags depuis l'état global")

	// Récupérer les tags depuis l'état global
	tags := state.GetGlobalState().GetTags()
	log.Debug().Int("tags_count", len(tags)).Msg("Tags récupérés avec succès")

	// Préparer les données pour le template
	data := make(map[string]interface{})

	// Ajouter les traductions aux données du template
	i18n.LocalizePage(w, r, data)
	
	// Ajouter les traductions spécifiques à la page
	translations := i18n.GetI18nData(lang)
	if t, ok := translations["Admin"].(map[string]interface{}); ok {
		if tags, ok := t["Tags"].(map[string]string); ok {
			data["t"] = map[string]interface{}{"Admin": map[string]interface{}{"Tags": tags}}
		}
	}

	// Ajouter les données spécifiques à la page
	data["Tags"] = tags
	data["CSRFToken"] = r.URL.Query().Get("csrf")
	data["CurrentPath"] = r.URL.Path
	data["IsHTTPS"] = r.TLS != nil
	data["Host"] = r.Host

	log.Debug().Msg("Rendu du template des tags")

	// Utiliser la fonction renderTemplateInternal pour le rendu
	renderTemplateInternal(w, r, "tags", data)

	log.Info().Msg("Page des tags affichée avec succès")
}
