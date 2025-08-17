package handlers

import (
	"net/http"
	"regexp"
	"strings"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/state"

	"github.com/julienschmidt/httprouter"
)

// TagsHandler handles tag-related operations.
type TagsHandler struct {
	stateManager state.StateManager
}

// NewTagsHandler creates a new instance of TagsHandler.
func NewTagsHandler(sm state.StateManager) *TagsHandler {
	return &TagsHandler{stateManager: sm}
}

var tagNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,50}$`)

// CreateTagHandler handles the creation of a new tag via an HTML form.
func (h *TagsHandler) CreateTagHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().Str("handler", "CreateTagHandler").Logger()

	if err := r.ParseForm(); err != nil {
		log.Warn().Err(err).Msg("Error parsing form data")
		http.Error(w, "Invalid form data.", http.StatusBadRequest)
		return
	}

	tagName := strings.TrimSpace(r.FormValue("tag"))

	if !tagNameRegex.MatchString(tagName) {
		log.Warn().Str("tag", tagName).Msg("Invalid tag name")
		http.Error(w, "Invalid tag name. Use only letters, numbers, hyphens, and underscores (1-50 characters).", http.StatusBadRequest)
		return
	}

	gs := h.stateManager
	settings := gs.GetSettings()

	for _, existingTag := range settings.Tags {
		if strings.EqualFold(existingTag, tagName) {
			log.Warn().Str("tag", tagName).Msg("Attempted to add an existing tag")
			http.Redirect(w, r, "/admin/tags?error=exists", http.StatusSeeOther)
			return
		}
	}

	settings.Tags = append(settings.Tags, tagName)
	if err := gs.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save settings")
		http.Error(w, "Internal server error.", http.StatusInternalServerError)
		return
	}

	log.Info().Str("tag", tagName).Msg("Tag added successfully")
	http.Redirect(w, r, "/admin/tags?success=1", http.StatusSeeOther)
}

// DeleteTagHandler handles tag deletion.
func (h *TagsHandler) DeleteTagHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := logger.Get().With().Str("handler", "DeleteTagHandler").Logger()

	if err := r.ParseForm(); err != nil {
		log.Warn().Err(err).Msg("Error parsing delete form")
		http.Error(w, "Invalid request.", http.StatusBadRequest)
		return
	}

	tagName := strings.TrimSpace(r.FormValue("tag"))

	if !tagNameRegex.MatchString(tagName) {
		log.Warn().Str("tag", tagName).Msg("Attempted to delete a tag with invalid format")
		http.Error(w, "Invalid tag name.", http.StatusBadRequest)
		return
	}

	if strings.EqualFold(tagName, "pvmss") {
		log.Warn().Msg("Attempted to delete the default tag")
		http.Error(w, "The default tag 'pvmss' cannot be deleted.", http.StatusForbidden)
		return
	}

	gs := h.stateManager
	settings := gs.GetSettings()

	found := false
	var newTags []string
	for _, tag := range settings.Tags {
		if !strings.EqualFold(tag, tagName) {
			newTags = append(newTags, tag)
		} else {
			found = true
		}
	}

	if !found {
		log.Warn().Str("tag", tagName).Msg("Tentative de suppression d'un tag inexistant")
		http.Redirect(w, r, "/admin/tags?error=notfound", http.StatusSeeOther)
		return
	}

	settings.Tags = newTags
	if err := gs.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Échec de la sauvegarde des paramètres après suppression")
		http.Error(w, "Erreur interne du serveur.", http.StatusInternalServerError)
		return
	}

	log.Info().Str("tag", tagName).Msg("Tag deleted successfully")
	http.Redirect(w, r, "/admin/tags", http.StatusSeeOther)
}

// TagsPageHandler handles the rendering of the admin tags page.
func (h *TagsHandler) TagsPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	gs := h.stateManager
	settings := gs.GetSettings()

	data := map[string]interface{}{
		"Tags": settings.Tags,
	}
	i18n.LocalizePage(w, r, data)
	renderTemplateInternal(w, r, "admin_tags", data)
}

// RegisterRoutes registers the routes for tag management.
func (h *TagsHandler) RegisterRoutes(router *httprouter.Router) {
	router.GET("/admin/tags", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.TagsPageHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
	router.POST("/tags", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.CreateTagHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
	router.POST("/tags/delete", HandlerFuncToHTTPrHandle(RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		h.DeleteTagHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))
}

// EnsureDefaultTag ensures that the default tag "pvmss" exists.
func EnsureDefaultTag(sm state.StateManager) error {
	gs := sm
	settings := gs.GetSettings()
	if settings == nil {
		// Ne rien faire si les paramètres ne sont pas encore chargés
		return nil
	}

	defaultTag := "pvmss"
	for _, tag := range settings.Tags {
		if strings.EqualFold(tag, defaultTag) {
			return nil // Le tag existe déjà
		}
	}

	// Ajouter le tag par défaut et sauvegarder
	settings.Tags = append(settings.Tags, defaultTag)
	log := logger.Get()
	log.Info().Msg("Tag par défaut 'pvmss' ajouté aux paramètres.")
	return gs.SetSettings(settings)
}
