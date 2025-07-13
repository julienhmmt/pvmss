package main

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/rs/zerolog/log"
)

// tagsHandler routes the requests to the appropriate handler.
func tagsHandler(w http.ResponseWriter, r *http.Request) {
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
	settings, err := readSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to read settings")
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
	var reqBody struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	tagName := strings.TrimSpace(reqBody.Name)
	if tagName == "" {
		http.Error(w, "Tag name cannot be empty", http.StatusBadRequest)
		return
	}

	settings, err := readSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to read settings")
		http.Error(w, "Failed to read settings", http.StatusInternalServerError)
		return
	}

	for _, t := range settings.Tags {
		if strings.EqualFold(t, tagName) {
			w.WriteHeader(http.StatusOK) // Tag already exists, do nothing
			return
		}
	}

	settings.Tags = append(settings.Tags, tagName)

	if err := writeSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to write settings")
		http.Error(w, "Failed to write settings", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

// deleteTagHandler handles deleting a tag.
func deleteTagHandler(w http.ResponseWriter, r *http.Request) {
	tagToDelete := strings.TrimPrefix(r.URL.Path, "/api/tags/")
	if strings.EqualFold(tagToDelete, "PVMSS") {
		http.Error(w, "Cannot delete mandatory tag 'PVMSS'", http.StatusBadRequest)
		return
	}

	settings, err := readSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to read settings")
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
		http.Error(w, "Tag not found", http.StatusNotFound)
		return
	}

	settings.Tags = newTags

	if err := writeSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to write settings")
		http.Error(w, "Failed to write settings", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
