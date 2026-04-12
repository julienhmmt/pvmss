package handlers

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/julienschmidt/httprouter"

	"pvmss/logger"
	"pvmss/security"
	"pvmss/state"
)

// TagsHandler handles tag-related operations.
type TagsHandler struct {
	stateManager state.StateManager
}

// MakeTagsHandler creates a new instance of TagsHandler.
func MakeTagsHandler(sm state.StateManager) *TagsHandler {
	return &TagsHandler{stateManager: sm}
}

var tagNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,50}$`)

// tagExists checks if a tag exists in the tags list (case-insensitive)
func tagExists(tags []string, tagName string) bool {
	for _, tag := range tags {
		if strings.EqualFold(tag, tagName) {
			return true
		}
	}
	return false
}

// removeTag removes a tag from the list (case-insensitive)
func removeTag(tags []string, tagName string) []string {
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		if !strings.EqualFold(tag, tagName) {
			result = append(result, tag)
		}
	}
	return result
}

// validateTagName validates tag name format
func validateTagName(tagName string) bool {
	return tagNameRegex.MatchString(tagName)
}

// validateTagDeletion validates tag deletion parameters and returns the validated tag name
func (h *TagsHandler) validateTagDeletion(tagName string, checkExists bool) (string, bool) {
	log := logger.Get().With().Str("function", "TagsValidation").Logger()

	tagName = strings.TrimSpace(tagName)
	if tagName == "" {
		log.Warn().Msg("No tag specified for deletion")
		return "", false
	}

	if !validateTagName(tagName) {
		log.Warn().Str("tag", tagName).Msg("Invalid tag name format")
		return "", false
	}

	if strings.EqualFold(tagName, "pvmss") {
		log.Warn().Msg("Attempted to delete the default tag")
		return "", false
	}

	if checkExists {
		settings := h.stateManager.GetSettings()
		if settings == nil || settings.Tags == nil || !tagExists(settings.Tags, tagName) {
			log.Warn().Str("tag", tagName).Msg("Tag not found or settings unavailable")
			return "", false
		}
	}

	return tagName, true
}

// CreateTagHandler handles the creation of a new tag via an HTML form.
func (h *TagsHandler) CreateTagHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("CreateTagHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	tagName := strings.TrimSpace(r.FormValue("tag"))

	if !validateTagName(tagName) {
		log.Warn().Str("tag", tagName).Msg("Invalid tag name")
		u, _ := url.Parse("/admin/tags")
		q := u.Query()
		q.Set("error", "1")
		q.Set("error_msg", "INVALID_TAG_FORMAT")
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}

	settings := h.stateManager.GetSettings()
	if settings == nil || settings.Tags == nil {
		log.Error().Msg("Settings or Tags unavailable")
		RespondWithError(w, r, ErrInternalServer)
		return
	}

	if tagExists(settings.Tags, tagName) {
		log.Warn().Str("tag", tagName).Msg("Attempted to add an existing tag")
		u, _ := url.Parse("/admin/tags")
		q := u.Query()
		q.Set("error", "1")
		q.Set("error_msg", "TAG_ALREADY_EXISTS")
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}

	settings.Tags = append(settings.Tags, tagName)
	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save settings")
		RespondWithError(w, r, ErrInternalServer)
		return
	}

	username := ""
	if sessionManager := security.GetSession(r); sessionManager != nil {
		if user, ok := sessionManager.Get(r.Context(), "username").(string); ok {
			username = user
		}
	}
	logger.AdminEvent("tag_create", username).
		Str("tag_name", tagName).
		Str("client_ip", r.RemoteAddr).
		Msg("Tag created")

	u, _ := url.Parse("/admin/tags")
	q := u.Query()
	q.Set("success", "1")
	q.Set("success_msg", "TAG_CREATED")
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusSeeOther)
}

// DeleteTagHandler handles tag deletion.
func (h *TagsHandler) DeleteTagHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("DeleteTagHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
		return
	}

	tagName, valid := h.validateTagDeletion(r.FormValue("tag"), true)
	if !valid {
		http.Redirect(w, r, "/admin/tags", http.StatusSeeOther)
		return
	}

	settings := h.stateManager.GetSettings()
	if settings == nil || settings.Tags == nil {
		log.Error().Msg("Settings or Tags unavailable")
		RespondWithError(w, r, ErrInternalServer)
		return
	}

	settings.Tags = removeTag(settings.Tags, tagName)

	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save settings after deletion")
		RespondWithError(w, r, ErrInternalServer)
		return
	}

	username := ""
	if sessionManager := security.GetSession(r); sessionManager != nil {
		if user, ok := sessionManager.Get(r.Context(), "username").(string); ok {
			username = user
		}
	}
	logger.AdminEvent("tag_delete", username).
		Str("tag_name", tagName).
		Str("client_ip", r.RemoteAddr).
		Msg("Tag deleted")

	http.Redirect(w, r, "/admin/tags?success=1&action=delete&tag="+url.QueryEscape(tagName), http.StatusSeeOther)
}

// RegisterRoutes registers the routes for tag management.
// Tags are managed through the VM API (/api/v1/vms/:id) — these legacy routes are kept for backward compatibility.
func (h *TagsHandler) RegisterRoutes(router *httprouter.Router) {
	// Legacy API routes for backward compatibility (deprecated)
	router.POST("/tags", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		logger.Get().Warn().Msg("Legacy /tags endpoint is deprecated. Use /api/v1/admin/tags instead.")
		SecureFormHandler("CreateTag",
			HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
				h.CreateTagHandler(w, r, httprouter.ParamsFromContext(r.Context()))
			})),
		)(w, r, p)
	})

	router.POST("/tags/delete", func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		logger.Get().Warn().Msg("Legacy /tags/delete endpoint is deprecated. Use /api/v1/admin/tags instead.")
		SecureFormHandler("DeleteTag",
			HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
				h.DeleteTagHandler(w, r, httprouter.ParamsFromContext(r.Context()))
			})),
		)(w, r, p)
	})
}

// EnsureDefaultTag ensures that the default tag "pvmss" exists.
func EnsureDefaultTag(sm state.StateManager) error {
	settings := sm.GetSettings()
	if settings == nil {
		return nil
	}

	if settings.Tags == nil {
		settings.Tags = []string{}
	}

	defaultTag := "pvmss"
	if tagExists(settings.Tags, defaultTag) {
		return nil
	}

	settings.Tags = append(settings.Tags, defaultTag)
	logger.Get().Info().Msg("Default tag 'pvmss' added to settings.")
	return sm.SetSettings(settings)
}
