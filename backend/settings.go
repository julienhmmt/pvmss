package main

import (
	"encoding/json"
	"io"
	"os"
	"sync"
)

const settingsFile = "settings.json"

var settingsMutex = &sync.Mutex{}

// AppSettings defines the structure for the settings file.
type AppSettings struct {
	Tags    []string `json:"tags"`
	RAM     MinMax   `json:"ram"`
	CPU     MinMax   `json:"cpu"`
	Sockets MinMax   `json:"sockets"`
	ISOs    []string `json:"isos"`
	VMBRs   []string `json:"vmbrs"`
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
