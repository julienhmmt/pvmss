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

// NodeLimit defines the resource limits for a node.
type NodeLimit struct {
	Sockets MinMax `json:"sockets"`
	Cores   MinMax `json:"cores"`
	RAM     MinMax `json:"ram"`
}

// VMLimit defines the default VM resource limits.
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

// AppSettings defines the structure for the settings file.
type AppSettings struct {
	Tags   []string                  `json:"tags"`
	ISOs   []string                  `json:"isos"`
	VMBRs  []string                  `json:"vmbrs"`
	Limits map[string]interface{}    `json:"limits"` // Map with both NodeLimit and VMLimit
}

// MinMax defines a min/max value pair.
type MinMax struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// readSettings reads the settings from the JSON file.
func readSettings() (*AppSettings, error) {
	settingsMutex.Lock()
	defer settingsMutex.Unlock()

	file, err := os.Open(settingsFile)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	bytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var settings AppSettings
	if err := json.Unmarshal(bytes, &settings); err != nil {
		return nil, err
	}

	if settings.Limits == nil {
		settings.Limits = make(map[string]interface{})
	}
	
	// Ensure VM limits exists
	if _, exists := settings.Limits["vm"]; !exists {
		settings.Limits["vm"] = VMLimit{
			Sockets: MinMax{Min: 1, Max: 1},
			Cores:   MinMax{Min: 1, Max: 2},
			RAM:     MinMax{Min: 1, Max: 4},
			Disk:    MinMax{Min: 1, Max: 10},
		}
	}

	return &settings, nil
}

// writeSettings writes the settings to the JSON file.
func writeSettings(settings *AppSettings) error {
	settingsMutex.Lock()
	defer settingsMutex.Unlock()

	bytes, err := json.MarshalIndent(settings, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(settingsFile, bytes, 0644)
}

// settingsHandler handles GET requests to read the entire settings file.
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

// updateIsoSettingsHandler handles POST requests to update the ISOs list in settings.
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

// updateVmbrSettingsHandler handles POST requests to update the VMBRs list in settings.
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
