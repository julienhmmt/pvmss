package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
)

const settingsFile = "settings.json"

var settingsMutex = &sync.Mutex{}

// NodeLimits defines the resource limits for a node.
type NodeLimits struct {
	Sockets MinMax `json:"sockets"`
	Cores   MinMax `json:"cores"`
	RAM     MinMax `json:"ram"`
}

// AppSettings defines the structure for the settings file.
type AppSettings struct {
	Tags    []string              `json:"tags"`
	RAM     MinMax                `json:"ram"`
	CPU     MinMax                `json:"cpu"`
	Sockets MinMax                `json:"sockets"`
	ISOs    []string              `json:"isos"`
	VMBRs   []string              `json:"vmbrs"`
	Limits  map[string]NodeLimits `json:"limits"`
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
		settings.Limits = make(map[string]NodeLimits)
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
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		ISOs []string `json:"isos"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	settings, err := readSettings()
	if err != nil {
		http.Error(w, "Failed to read settings for update", http.StatusInternalServerError)
		return
	}

	settings.ISOs = payload.ISOs

	if err := writeSettings(settings); err != nil {
		http.Error(w, "Failed to write updated settings", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "ISO settings updated successfully.")
}

// updateVmbrSettingsHandler handles POST requests to update the VMBRs list in settings.
func updateVmbrSettingsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var payload struct {
		VMBRs []string `json:"vmbrs"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	settings, err := readSettings()
	if err != nil {
		http.Error(w, "Failed to read settings for update", http.StatusInternalServerError)
		return
	}

	settings.VMBRs = payload.VMBRs

	if err := writeSettings(settings); err != nil {
		http.Error(w, "Failed to write updated settings", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "VMBR settings updated successfully.")
}
