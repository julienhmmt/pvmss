package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"pvmss/logger"
	"pvmss/state"
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

// readSettings reads the settings from settings.json, decodes the JSON into a state.AppSettings struct,
// and applies default values for missing sections like VM limits to ensure application stability.
// It uses a mutex to prevent race conditions during file access.
func readSettings() (*state.AppSettings, error) {
	settingsMutex.Lock()
	defer settingsMutex.Unlock()

	log := logger.Get()

	log.Debug().
		Str("settings_file", settingsFile).
		Msg("Reading settings from file")

	// Open the settings file
	file, err := os.Open(settingsFile)
	if err != nil {
		if os.IsNotExist(err) {
			log.Warn().
				Str("settings_file", settingsFile).
				Msg("Settings file not found, creating with default values")
			// Initialize with default settings if file doesn't exist
			defaultSettings := &state.AppSettings{
				AdminPassword: "", // Will be set by admin during first run
				Tags:          []string{},
				ISOs:          []string{},
				VMBRs:         []string{},
				Limits:        make(map[string]interface{}),
			}

			// Write default settings to file
			if err := writeSettings(defaultSettings); err != nil {
				return nil, fmt.Errorf("failed to create default settings: %w", err)
			}
			return defaultSettings, nil
		}
		return nil, fmt.Errorf("error opening settings file: %w", err)
	}
	defer file.Close()

	// Read the file content
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("error reading settings file: %w", err)
	}

	// Parse the JSON into AppSettings struct
	var settings state.AppSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("error parsing settings file: %w", err)
	}

	// Initialize Limits map if it's nil
	if settings.Limits == nil {
		settings.Limits = make(map[string]interface{})
	}

	// Initialize default VM limits if not set
	if _, exists := settings.Limits["vm"]; !exists {
		log.Debug().Msg("Initializing default VM limits")
		settings.Limits["vm"] = map[string]interface{}{
			"sockets": map[string]int{"min": 1, "max": 1},
			"cores":   map[string]int{"min": 1, "max": 2},
			"ram":     map[string]int{"min": 1, "max": 4},
			"disk":    map[string]int{"min": 1, "max": 10},
		}
	}

	log.Debug().
		Interface("settings", settings).
		Msg("Successfully loaded settings")

	return &settings, nil
}

// writeSettings serializes the provided state.AppSettings struct into a well-formatted JSON string
// and writes it to settings.json, overwriting the previous content.
// It uses a mutex to ensure thread-safe file writing.
func writeSettings(settings *state.AppSettings) error {
	settingsMutex.Lock()
	defer settingsMutex.Unlock()

	log := logger.Get()

	// Create a pretty-printed JSON with 4-space indentation
	data, err := json.MarshalIndent(settings, "", "    ")
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed to marshal settings to JSON")
		return fmt.Errorf("failed to marshal settings: %w", err)
	}

	// Add a newline at the end for better file readability
	data = append(data, '\n')

	// Write to a temporary file first to ensure atomicity
	tempFile := settingsFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0600); err != nil {
		log.Error().
			Err(err).
			Str("temp_file", tempFile).
			Msg("Failed to write temporary settings file")
		return fmt.Errorf("failed to write temporary settings file: %w", err)
	}

	// Rename the temporary file to the actual settings file (atomic operation on Unix-like systems)
	if err := os.Rename(tempFile, settingsFile); err != nil {
		log.Error().
			Err(err).
			Str("temp_file", tempFile).
			Str("settings_file", settingsFile).
			Msg("Failed to rename temporary settings file")
		return fmt.Errorf("failed to rename temporary settings file: %w", err)
	}

	log.Debug().
		Str("settings_file", settingsFile).
		Msg("Successfully wrote settings to file")

	return nil
}
