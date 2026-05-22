package handlers

import (
	"strings"

	"pvmss/constants"
	"pvmss/logger"
	"pvmss/state"
)

// tagExists checks if a tag exists in the tags list (case-insensitive).
func tagExists(tags []string, tagName string) bool {
	for _, tag := range tags {
		if strings.EqualFold(tag, tagName) {
			return true
		}
	}
	return false
}

// EnsureDefaultTag ensures that the mandatory `pvmss` tag exists in app
// settings on startup. Called once from InitHandlers.
func EnsureDefaultTag(sm state.StateManager) error {
	settings := sm.GetSettings()
	if settings == nil {
		return nil
	}

	if settings.Tags == nil {
		settings.Tags = []string{}
	}

	if tagExists(settings.Tags, constants.RequiredTag) {
		return nil
	}

	settings.Tags = append(settings.Tags, constants.RequiredTag)
	logger.Get().Info().Str("tag", constants.RequiredTag).Msg("default tag added to settings")
	return sm.SetSettings(settings)
}
