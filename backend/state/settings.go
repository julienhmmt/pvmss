package state

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"pvmss/logger"
)

const settingsFile = "settings.json"

var settingsMutex = &sync.Mutex{}

type AppSettings struct {
	// AdminPassword is the bcrypt hashed password for admin access
	AdminPassword string                 `json:"admin_password"`
	Tags          []string               `json:"tags"`
	ISOs          []string               `json:"isos"`
	VMBRs         []string               `json:"vmbrs"`
	Limits        map[string]interface{} `json:"limits"`
}

// WriteSettings serializes the provided AppSettings struct into a well-formatted JSON string
// and writes it to settings.json, overwriting the previous content.
// It uses a mutex to ensure thread-safe file writing.
func WriteSettings(settings *AppSettings) error {
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
