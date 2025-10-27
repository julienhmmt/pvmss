package handlers

import (
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"pvmss/i18n"
	"pvmss/logger"
	"pvmss/proxmox"
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

// buildTagSuccessMessage creates success message from query parameters
func buildTagSuccessMessage(r *http.Request) string {
	if r.URL.Query().Get("success") == "" {
		return ""
	}

	action := r.URL.Query().Get("action")
	tag := r.URL.Query().Get("tag")

	switch action {
	case "create":
		return "Tag '" + tag + "' created"
	case "delete":
		return "Tag '" + tag + "' deleted"
	default:
		return "Tags updated"
	}
}

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
		if !tagExists(settings.Tags, tagName) {
			log.Warn().Str("tag", tagName).Msg("Tag not found")
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
		// Redirect back to tags page with localized error notification
		localizer := i18n.GetLocalizerFromRequest(r)
		errMsg := i18n.Localize(localizer, "Admin.Tags.Error.InvalidFormat")
		// Build URL with error parameters
		u, _ := url.Parse("/admin/tags")
		q := u.Query()
		q.Set("error", "1")
		q.Set("error_msg", errMsg)
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}

	settings := h.stateManager.GetSettings()

	if tagExists(settings.Tags, tagName) {
		log.Warn().Str("tag", tagName).Msg("Attempted to add an existing tag")
		// Redirect with localized "exists" error
		localizer := i18n.GetLocalizerFromRequest(r)
		errMsg := i18n.Localize(localizer, "Admin.Tags.Error.Exists")
		u, _ := url.Parse("/admin/tags")
		q := u.Query()
		q.Set("error", "1")
		q.Set("error_msg", errMsg)
		u.RawQuery = q.Encode()
		http.Redirect(w, r, u.String(), http.StatusSeeOther)
		return
	}

	settings.Tags = append(settings.Tags, tagName)
	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save settings")
		http.Error(w, "Internal server error.", http.StatusInternalServerError)
		return
	}

	log.Info().Str("tag", tagName).Msg("Tag added successfully")
	// Redirect with localized success message
	localizer := i18n.GetLocalizerFromRequest(r)
	// If you have a parameterized translation, keep it simple for now
	successMsg := i18n.Localize(localizer, "Admin.Tags.Success.Created")
	u, _ := url.Parse("/admin/tags")
	q := u.Query()
	q.Set("success", "1")
	q.Set("success_msg", successMsg)
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

	// Remove the tag from settings
	settings.Tags = removeTag(settings.Tags, tagName)

	if err := h.stateManager.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save settings after deletion")
		http.Error(w, "Internal server error.", http.StatusInternalServerError)
		return
	}

	log.Info().Str("tag", tagName).Msg("Tag deleted successfully")
	http.Redirect(w, r, "/admin/tags?success=1&action=delete&tag="+url.QueryEscape(tagName), http.StatusSeeOther)
}

// DeleteTagConfirmHandler handles the GET request for tag deletion confirmation page.
func (h *TagsHandler) DeleteTagConfirmHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	tagName, valid := h.validateTagDeletion(r.URL.Query().Get("tag"), true)
	if !valid {
		http.Redirect(w, r, "/admin/tags", http.StatusSeeOther)
		return
	}

	data := AdminPageDataWithMessage("", "tags_delete", "", "")
	data["TitleKey"] = "Admin.Tags.Title"
	data["Tag"] = tagName

	renderTemplateInternal(w, r, "admin_tags_delete", data)
}

// TagsPageHandler handles the rendering of the admin tags page.
func (h *TagsHandler) TagsPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	settings := h.stateManager.GetSettings()

	sortOrder := r.URL.Query().Get("sort") // "asc" or "desc"
	if sortOrder != "desc" {
		sortOrder = "asc" // default to ascending
	}

	// Prefer explicit localized messages from params when present
	q := r.URL.Query()
	successMsg := q.Get("success_msg")
	if successMsg == "" {
		successMsg = buildTagSuccessMessage(r)
	}
	// Determine error message (supports both new and legacy styles)
	var errorMsg string
	if q.Get("error") != "" {
		// New style: error=1 with error_msg
		if q.Get("error") == "1" {
			errorMsg = q.Get("error_msg")
		}
		// Legacy style: error=exists
		if errorMsg == "" && q.Get("error") == "exists" {
			localizer := i18n.GetLocalizerFromRequest(r)
			errorMsg = i18n.Localize(localizer, "Admin.Tags.Error.Exists")
		}
	}

	// Proxmox status for consistent UI (even if tags don't need Proxmox)
	proxmoxConnected, proxmoxMsg := h.stateManager.GetProxmoxStatus()

	// Build usage counts per tag by inspecting VMs' tags when Proxmox is available using resty
	// Proxmox typically separates tags with ';' but some environments may contain
	// comma-separated lists inside a single part (e.g. "pvmss,test"). We split on
	// both ';' and ',' to ensure each individual tag is counted.
	tagCounts := make(map[string]int)
	if restyClient, err := getDefaultRestyClient(); err == nil {
		if vms, err := proxmox.GetVMsResty(r.Context(), restyClient); err == nil {
			for i := range vms {
				if cfg, err := proxmox.GetVMConfigResty(r.Context(), restyClient, vms[i].Node, vms[i].VMID); err == nil {
					if v, ok := cfg["tags"].(string); ok && v != "" {
						// First split by ';'
						semiParts := strings.Split(v, ";")
						for _, sp := range semiParts {
							sp = strings.TrimSpace(sp)
							if sp == "" {
								continue
							}
							// Then split each part by ','
							commaParts := strings.Split(sp, ",")
							for _, cp := range commaParts {
								t := strings.TrimSpace(cp)
								if t != "" {
									tagCounts[t]++
								}
							}
						}
					}
				}
			}
		}
	}

	// Debug logging for tag counts
	log := logger.Get()
	log.Info().Interface("tag_counts", tagCounts).Msg("Tag counts calculated")

	// Filter and sort tags by name for display
	tags := make([]string, 0, len(settings.Tags))
	tags = append(tags, settings.Tags...)

	// Sort based on requested order
	if sortOrder == "desc" {
		sort.Sort(sort.Reverse(sort.StringSlice(tags)))
	} else {
		sort.Strings(tags)
	}

	data := AdminPageDataWithMessage("", "tags", successMsg, errorMsg)
	data["TitleKey"] = "Admin.Tags.Title"
	data["Tags"] = tags
	data["SortOrder"] = sortOrder
	data["TotalTags"] = len(settings.Tags)
	data["FilteredTags"] = len(tags)
	// Always expose TagCounts so the template can safely render a value (including zero)
	data["TagCounts"] = tagCounts
	data["ProxmoxConnected"] = proxmoxConnected
	if !proxmoxConnected && proxmoxMsg != "" {
		data["ProxmoxError"] = proxmoxMsg
	}

	renderTemplateInternal(w, r, "admin_tags", data)
}

// RegisterRoutes registers the routes for tag management.
func (h *TagsHandler) RegisterRoutes(router *httprouter.Router) {
	routeHelpers := NewAdminPageRoutes()

	// Register admin tags routes using helper
	routeHelpers.RegisterCRUDRoutes(router, "/admin/tags", map[string]func(w http.ResponseWriter, r *http.Request, ps httprouter.Params){
		"page": h.TagsPageHandler,
	})

	// Register delete confirmation page
	router.GET("/admin/tags/delete", HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
		h.DeleteTagConfirmHandler(w, r, httprouter.ParamsFromContext(r.Context()))
	})))

	// Admin tag creation with CSRF protection
	router.POST("/tags", SecureFormHandler("CreateTag",
		HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
			h.CreateTagHandler(w, r, httprouter.ParamsFromContext(r.Context()))
		})),
	))

	// Admin tag deletion with CSRF protection
	router.POST("/tags/delete", SecureFormHandler("DeleteTag",
		HandlerFuncToHTTPrHandle(RequireAdminAuth(func(w http.ResponseWriter, r *http.Request) {
			h.DeleteTagHandler(w, r, httprouter.ParamsFromContext(r.Context()))
		})),
	))
}

// EnsureDefaultTag ensures that the default tag "pvmss" exists.
func EnsureDefaultTag(sm state.StateManager) error {
	settings := sm.GetSettings()
	if settings == nil {
		return nil // Settings not yet loaded
	}

	defaultTag := "pvmss"
	if tagExists(settings.Tags, defaultTag) {
		return nil // Tag already exists
	}

	// Add the default tag and save
	settings.Tags = append(settings.Tags, defaultTag)
	logger.Get().Info().Msg("Default tag 'pvmss' added to settings.")
	return sm.SetSettings(settings)
}
