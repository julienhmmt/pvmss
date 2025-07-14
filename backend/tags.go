package main

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"github.com/rs/zerolog/log"
)

// tagsHandler routes the requests to the appropriate handler.
func tagsHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("handler", "tagsHandler").Str("method", r.Method).Str("path", r.URL.Path).Msg("Tags handler invoked")
	switch r.Method {
	case http.MethodGet:
		getTagsHandler(w, r)
	case http.MethodPost:
		addTagHandler(w, r)
	case http.MethodDelete:
		deleteTagHandler(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// getTagsHandler handles fetching all tags.
func getTagsHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("handler", "getTagsHandler").Str("method", r.Method).Str("path", r.URL.Path).Msg("Fetching tags")
	settings, err := readSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to read settings for tags")
		http.Error(w, "Failed to read settings", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(struct {
		Tags []string `json:"tags"`
	}{Tags: settings.Tags})
}

// addTagHandler handles adding a new tag.
func addTagHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("handler", "addTagHandler").Str("method", r.Method).Str("path", r.URL.Path).Msg("Add tag handler invoked")
	var reqBody struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		log.Error().Err(err).Msg("Failed to decode add tag payload")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	tagName := strings.TrimSpace(reqBody.Name)
	if len(tagName) < 3 || len(tagName) > 24 {
		log.Warn().Str("tag", tagName).Msg("Tag length invalid")
		http.Error(w, "Tag must be between 3 and 24 characters", http.StatusBadRequest)
		return
	}

	// Regex to allow only alphanumeric characters
	isValid, _ := regexp.MatchString(`^[a-zA-Z0-9]+$`, tagName)
	if !isValid {
		log.Warn().Str("tag", tagName).Msg("Tag contains invalid characters")
		http.Error(w, "Tag can only contain alphanumeric characters", http.StatusBadRequest)
		return
	}

	settings, err := readSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to read settings for add tag")
		http.Error(w, "Failed to read settings", http.StatusInternalServerError)
		return
	}

	for _, t := range settings.Tags {
		if strings.EqualFold(t, tagName) {
			log.Warn().Str("tag", tagName).Msg("Tag already exists; not adding duplicate")
			w.WriteHeader(http.StatusOK) // Tag already exists, do nothing
			return
		}
	}

	settings.Tags = append(settings.Tags, tagName)

	if err := writeSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to write settings for add tag")
		http.Error(w, "Failed to write settings", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// deleteTagHandler handles deleting a tag.
func deleteTagHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("handler", "deleteTagHandler").Str("method", r.Method).Str("path", r.URL.Path).Msg("Delete tag handler invoked")
	tagToDelete := strings.TrimPrefix(r.URL.Path, "/api/tags/")
	if strings.EqualFold(tagToDelete, "PVMSS") {
		log.Warn().Str("tag", tagToDelete).Msg("Attempt to delete mandatory tag 'PVMSS' blocked")
		http.Error(w, "Cannot delete mandatory tag 'PVMSS'", http.StatusBadRequest)
		return
	}

	settings, err := readSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to read settings for delete tag")
		http.Error(w, "Failed to read settings", http.StatusInternalServerError)
		return
	}

	var newTags []string
	found := false
	for _, t := range settings.Tags {
		if !strings.EqualFold(t, tagToDelete) {
			newTags = append(newTags, t)
		} else {
			found = true
		}
	}

	if !found {
		log.Warn().Str("tag", tagToDelete).Msg("Tag to delete not found")
		http.Error(w, "Tag not found", http.StatusNotFound)
		return
	}

	settings.Tags = newTags

	if err := writeSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to write settings for delete tag")
		http.Error(w, "Failed to write settings", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
