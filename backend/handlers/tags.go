package handlers

import (
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"

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

// CreateTagHandler handles the creation of a new tag via an HTML form.
func (h *TagsHandler) CreateTagHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("CreateTagHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
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
	http.Redirect(w, r, "/admin/tags?success=1&action=create&tag="+url.QueryEscape(tagName), http.StatusSeeOther)
}

// DeleteTagHandler handles tag deletion.
func (h *TagsHandler) DeleteTagHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	log := CreateHandlerLogger("DeleteTagHandler", r)

	if !ValidateMethodAndParseForm(w, r, http.MethodPost) {
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
		log.Warn().Str("tag", tagName).Msg("Attempted to delete a non-existent tag")
		http.Redirect(w, r, "/admin/tags?error=notfound", http.StatusSeeOther)
		return
	}

	settings.Tags = newTags
	if err := gs.SetSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to save settings after deletion")
		http.Error(w, "Internal server error.", http.StatusInternalServerError)
		return
	}

	log.Info().Str("tag", tagName).Msg("Tag deleted successfully")
	http.Redirect(w, r, "/admin/tags?success=1&action=delete&tag="+url.QueryEscape(tagName), http.StatusSeeOther)
}

// TagsPageHandler handles the rendering of the admin tags page.
func (h *TagsHandler) TagsPageHandler(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
	gs := h.stateManager
	settings := gs.GetSettings()

	sortOrder := r.URL.Query().Get("sort") // "asc" or "desc"
	if sortOrder != "desc" {
		sortOrder = "asc" // default to ascending
	}

	// Success banner via query params
	success := r.URL.Query().Get("success") != ""
	act := r.URL.Query().Get("action")
	tag := r.URL.Query().Get("tag")
	var successMsg string
	if success {
		switch act {
		case "create":
			successMsg = "Tag '" + tag + "' created"
		case "delete":
			successMsg = "Tag '" + tag + "' deleted"
		default:
			successMsg = "Tags updated"
		}
	}

	// Proxmox status for consistent UI (even if tags don't need Proxmox)
	proxmoxConnected, proxmoxMsg := gs.GetProxmoxStatus()

	// Build usage counts per tag by inspecting VMs' tags when Proxmox is available
	// Proxmox typically separates tags with ';' but some environments may contain
	// comma-separated lists inside a single part (e.g. "pvmss,test"). We split on
	// both ';' and ',' to ensure each individual tag is counted.
	tagCounts := make(map[string]int)
	if client := gs.GetProxmoxClient(); client != nil {
		if vms, err := proxmox.GetVMsWithContext(r.Context(), client); err == nil {
			for i := range vms {
				if cfg, err := proxmox.GetVMConfigWithContext(r.Context(), client, vms[i].Node, vms[i].VMID); err == nil {
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
	for _, tag := range settings.Tags {
		tags = append(tags, tag)
	}

	// Sort based on requested order
	if sortOrder == "desc" {
		sort.Sort(sort.Reverse(sort.StringSlice(tags)))
	} else {
		sort.Strings(tags)
	}

	data := AdminPageDataWithMessage("Tag Management", "tags", successMsg, "")
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
		// Do nothing if settings are not yet loaded
		return nil
	}

	defaultTag := "pvmss"
	for _, tag := range settings.Tags {
		if strings.EqualFold(tag, defaultTag) {
			return nil // The tag already exists
		}
	}

	// Add the default tag and save
	settings.Tags = append(settings.Tags, defaultTag)
	log := logger.Get()
	log.Info().Msg("Default tag 'pvmss' added to settings.")
	return gs.SetSettings(settings)
}
