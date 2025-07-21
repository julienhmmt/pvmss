package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/rs/zerolog/log"
)

const settingsFile = "settings.json"

var settingsMutex = &sync.Mutex{}

// NodeLimit defines the resource limits (sockets, cores, RAM) for a specific Proxmox node.
// These are typically read-only and reflect the node's hardware capabilities.
type NodeLimit struct {
	Sockets MinMax `json:"sockets"`
	Cores   MinMax `json:"cores"`
	RAM     MinMax `json:"ram"`
}

// VMLimit defines the default resource limits for a new Virtual Machine.
// These values are used to populate the VM creation form.
type VMLimit struct {
	Sockets MinMax `json:"sockets"`
	Cores   MinMax `json:"cores"`
	RAM     MinMax `json:"ram"`
	Disk    MinMax `json:"disk"`
}

// IsDefined checks if the VMLimit has been populated with data.
func (v VMLimit) IsDefined() bool {
	// A simple check to see if the struct is likely to contain real data.
	// If Sockets.Min is set, we assume the rest of the data is intentional.
	return v.Sockets.Min > 0
}

// ResourceLimits defines a generic structure for resource limitations, including sockets, cores, RAM, and disk.
// The Disk field is optional to accommodate both node and VM limit types.
type ResourceLimits struct {
	Sockets MinMax  `json:"sockets"`
	Cores   MinMax  `json:"cores"`
	RAM     MinMax  `json:"ram"`
	Disk    *MinMax `json:"disk,omitempty"` // Only for VM limits
}

// AppSettings defines the main structure for the application's configuration file (settings.json).
// It holds user-configurable lists for tags, ISOs, VMBRs, and resource limits.
type AppSettings struct {
	Tags   []string `json:"tags"`
	ISOs   []string `json:"isos"`
	VMBRs  []string `json:"vmbrs"`
	Limits map[string]interface{} `json:"limits"`
}

// MinMax defines a min/max value pair.
type MinMax struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// readSettings reads the settings from settings.json, decodes the JSON into an AppSettings struct,
// and applies default values for missing sections like VM limits to ensure application stability.
// It uses a mutex to prevent race conditions during file access.
func readSettings() (*AppSettings, error) {
	settingsMutex.Lock()
	defer settingsMutex.Unlock()

	logger := log.With().Logger()

	logger.Debug().
		Str("settings_file", settingsFile).
		Msg("Reading settings from file")

	file, err := os.Open(settingsFile)
	if err != nil {
		logger.Error().
			Err(err).
			Str("settings_file", settingsFile).
			Msg("Failed to open settings file")
		return nil, fmt.Errorf("failed to open settings file: %w", err)
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		logger.Error().
			Err(err).
			Str("settings_file", settingsFile).
			Msg("Failed to read settings file")
		return nil, fmt.Errorf("failed to read settings file: %w", err)
	}

	// Log the raw content read from the file
	logger.Debug().
		Str("settings_file", settingsFile).
		Str("content", string(bytes)).
		Msg("Read settings file content")

	var settings AppSettings
	if err := json.Unmarshal(bytes, &settings); err != nil {
		logger.Error().
			Err(err).
			Str("settings_file", settingsFile).
			Msg("Failed to unmarshal settings")
		return nil, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	if settings.Limits == nil {
		logger.Debug().Msg("Initializing empty limits map")
		settings.Limits = make(map[string]interface{})
	}
	
	// Ensure VM limits exists
	if _, exists := settings.Limits["vm"]; !exists {
		logger.Debug().Msg("Initializing default VM limits")
		settings.Limits["vm"] = map[string]interface{}{
			"sockets": map[string]int{"min": 1, "max": 1},
			"cores":   map[string]int{"min": 1, "max": 2},
			"ram":     map[string]int{"min": 1, "max": 4},
			"disk":    map[string]int{"min": 1, "max": 10},
		}
	}

	logger.Debug().
		Interface("settings", settings).
		Msg("Successfully loaded settings")

	return &settings, nil
}

// writeSettings serializes the provided AppSettings struct into a well-formatted JSON string
// and writes it to settings.json, overwriting the previous content.
// It uses a mutex to ensure thread-safe file writing.
func writeSettings(settings *AppSettings) error {
	settingsMutex.Lock()
	defer settingsMutex.Unlock()

	logger := log.With().Logger()

	// Convert settings to JSON with indentation for readability
	data, err := json.MarshalIndent(settings, "", "    ")
	if err != nil {
		logger.Error().
			Err(err).
			Msg("Failed to marshal settings to JSON")
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	// Write directly to the settings file
	if err := os.WriteFile(settingsFile, data, 0644); err != nil {
		logger.Error().
			Err(err).
			Str("settings_file", settingsFile).
			Msg("Failed to write settings file")
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	logger.Debug().
		Str("settings_file", settingsFile).
		Msg("Settings saved successfully")

	return nil
}

// settingsHandler is an HTTP endpoint that handles GET requests to retrieve the complete current application settings.
// It reads the settings from disk and returns them as a JSON response.
func settingsHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("handler", "settingsHandler").Str("method", r.Method).Str("path", r.URL.Path).Msg("Settings handler invoked")
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	settings, err := readSettings()
	if err != nil {
		http.Error(w, "Failed to read settings", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(settings)
}

// updateIsoSettingsHandler is an HTTP endpoint for updating the list of available ISO images.
// It expects a POST request with a JSON payload containing the new list of ISOs
// and saves the updated configuration to settings.json.
func updateIsoSettingsHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("handler", "updateIsoSettingsHandler").Str("method", r.Method).Str("path", r.URL.Path).Msg("Update ISO settings handler invoked")
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		ISOs []string `json:"isos"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Error().Err(err).Msg("Failed to decode ISO update payload")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	log.Debug().Interface("payload", payload).Msg("Received ISO update payload")

	settings, err := readSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to read settings for ISO update")
		http.Error(w, "Failed to read settings for update", http.StatusInternalServerError)
		return
	}

	settings.ISOs = payload.ISOs

	if err := writeSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to write updated ISO settings")
		http.Error(w, "Failed to write updated settings", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ISO settings updated successfully.")
}

// updateVmbrSettingsHandler is an HTTP endpoint for updating the list of available network bridges (VMBRs).
// It expects a POST request with a JSON payload containing the new list of VMBRs
// and persists the changes to settings.json.
func updateVmbrSettingsHandler(w http.ResponseWriter, r *http.Request) {
	log.Info().Str("handler", "updateVmbrSettingsHandler").Str("method", r.Method).Str("path", r.URL.Path).Msg("Update VMBR settings handler invoked")
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		VMBRs []string `json:"vmbrs"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Error().Err(err).Msg("Failed to decode VMBR update payload")
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	log.Debug().Interface("payload", payload).Msg("Received VMBR update payload")

	settings, err := readSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to read settings for VMBR update")
		http.Error(w, "Failed to read settings for update", http.StatusInternalServerError)
		return
	}

	settings.VMBRs = payload.VMBRs

	if err := writeSettings(settings); err != nil {
		log.Error().Err(err).Msg("Failed to write updated VMBR settings")
		http.Error(w, "Failed to write updated settings", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "VMBR settings updated successfully.")
}
