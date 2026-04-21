package handlers

import (
	"strings"

	"github.com/julienschmidt/httprouter"

	"pvmss/logger"
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

// tagExists checks if a tag exists in the tags list (case-insensitive).
func tagExists(tags []string, tagName string) bool {
	for _, tag := range tags {
		if strings.EqualFold(tag, tagName) {
			return true
		}
	}
	return false
}

// RegisterRoutes registers tag management routes.
// Legacy session-based tag form routes have been removed; tag CRUD is handled by api/v1.
func (h *TagsHandler) RegisterRoutes(_ *httprouter.Router) {}

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
