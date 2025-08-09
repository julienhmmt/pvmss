package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"pvmss/logger"
)

// defaultSettings returns the default application settings
func defaultSettings() *AppSettings {
	return &AppSettings{
		Tags:   []string{"pvmss"}, // Tag par défaut
		ISOs:   []string{},
		VMBRs:  []string{},
		Limits: make(map[string]interface{}),
	}
}

var settingsMutex = &sync.Mutex{}

type AppSettings struct {
	Tags            []string               `json:"tags"`
	ISOs            []string               `json:"isos"`
	VMBRs           []string               `json:"vmbrs"`
	Storages 		[]string               `json:"storages"`
	Limits          map[string]interface{} `json:"limits"`
}

// WriteSettings serializes the provided AppSettings struct into a well-formatted JSON string
// and writes it to settings.json, overwriting the previous content.
// It uses a mutex to ensure thread-safe file writing.
// LoadSettings loads the application settings from the settings file.
// If the settings file does not exist, it creates a new one with default values.
func LoadSettings() (*AppSettings, bool, error) {
	settingsMutex.Lock()
	defer settingsMutex.Unlock()

	log := logger.Get()
	modified := false

	exePath, err := os.Executable()
	if err != nil {
		return nil, false, fmt.Errorf("could not get executable path: %w", err)
	}
	settingsFile := filepath.Join(filepath.Dir(exePath), "settings.json")

	// Check if settings file exists
	if _, err := os.Stat(settingsFile); os.IsNotExist(err) {
		log.Info().Msg("Settings file not found, creating with default values")
		return defaultSettings(), true, nil
	}

	// Read settings file
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read settings file: %w", err)
	}

	log.Debug().Str("file_content", string(data)).Msg("Raw content of settings file")

	var settings AppSettings
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, false, fmt.Errorf("failed to parse settings file: %w", err)
	}

	// Ensure default values for required fields
	if settings.Tags == nil {
		modified = true
		settings.Tags = []string{"pvmss"}
	}
	if settings.ISOs == nil {
		modified = true
		settings.ISOs = []string{}
	}
	if settings.VMBRs == nil {
		modified = true
		settings.VMBRs = []string{}
	}
	if settings.Limits == nil {
		modified = true
		settings.Limits = make(map[string]interface{})
	}
	if _, exists := settings.Limits["vm"]; !exists {
		modified = true
		settings.Limits["vm"] = map[string]interface{}{
			"sockets": map[string]int{"min": 1, "max": 1},
			"cores":   map[string]int{"min": 1, "max": 2},
			"ram":     map[string]int{"min": 1, "max": 4},
			"disk":    map[string]int{"min": 1, "max": 10},
		}
	}

	log.Info().
		Bool("modified", modified).
		Msg("Successfully loaded settings")

	return &settings, modified, nil
}

// WriteSettings serializes the provided AppSettings struct into a well-formatted JSON string
// and writes it to the settings file. It uses a mutex to ensure thread-safe file writing.

func WriteSettings(settings *AppSettings) error {
	settingsMutex.Lock()
	defer settingsMutex.Unlock()

	log := logger.Get()

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("could not get executable path: %w", err)
	}
	settingsFile := filepath.Join(filepath.Dir(exePath), "settings.json")

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

	// Write directly to the settings file
	if err := os.WriteFile(settingsFile, data, 0600); err != nil {
		log.Error().
			Err(err).
			Str("settings_file", settingsFile).
			Msg("Failed to write settings file")
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	log.Debug().
		Str("settings_file", settingsFile).
		Msg("Successfully wrote settings to file")

	return nil
}
