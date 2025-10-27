package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"pvmss/logger"
)

// Disk bus types and their maximum device counts
const (
	// IDE bus: 4 disks maximum (ide0-ide3)
	DiskBusIDE  = "ide"
	MaxDisksIDE = 4

	// SATA bus: 6 disks maximum (sata0-sata5)
	DiskBusSATA  = "sata"
	MaxDisksSATA = 6

	// VirtIO Block bus: 16 disks maximum (virtio0-virtio15)
	DiskBusVirtIO  = "virtio"
	MaxDisksVirtIO = 16

	// SCSI bus: 14 disks maximum (scsi0-scsi13)
	DiskBusSCSI  = "scsi"
	MaxDisksSCSI = 14
)

// Settings constants
// MaxDiskPerVM is set to the highest limit (VirtIO Block: 16 disks)
// Individual bus limits are enforced per bus type
const (
	MinNetworkCards = 1
	MaxNetworkCards = 32 // Maximum network cards (net0-net31)
	MinDiskPerVM    = 1
	MaxDiskPerVM    = MaxDisksVirtIO // Maximum disks overall (VirtIO Block limit)
)

// defaultSettings returns the default application settings
func defaultSettings() *AppSettings {
	return &AppSettings{
		EnabledStorages: []string{},
		ISOs:            []string{},
		Limits:          make(map[string]interface{}),
		MaxNetworkCards: MinNetworkCards,
		MaxDiskPerVM:    MinDiskPerVM,
		Tags:            []string{"pvmss"},
		VMBRs:           []string{},
	}
}

var settingsMutex = &sync.Mutex{}

type AppSettings struct {
	EnabledStorages []string               `json:"enabled_storages"`
	ISOs            []string               `json:"isos"`
	Limits          map[string]interface{} `json:"limits"`
	MaxNetworkCards int                    `json:"max_network_cards,omitempty"`
	MaxDiskPerVM    int                    `json:"max_disk_per_vm,omitempty"`
	Tags            []string               `json:"tags"`
	VMBRs           []string               `json:"vmbrs"`
}

// getSettingsFilePath returns the absolute path to the settings file.
// It uses PVMSS_SETTINGS_PATH if set; otherwise, it looks for settings.json
// in the backend directory relative to the executable.
func getSettingsFilePath() (string, error) {
	if v := os.Getenv("PVMSS_SETTINGS_PATH"); v != "" {
		return v, nil
	}
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("could not get executable path: %w", err)
	}
	// Look for settings.json in the backend directory
	exeDir := filepath.Dir(exePath)
	// Check if we're running from the project root (development)
	settingsPath := filepath.Join(exeDir, "backend", "settings.json")
	if _, err := os.Stat(settingsPath); err == nil {
		return settingsPath, nil
	}
	// Fallback to next to executable (production)
	return filepath.Join(exeDir, "settings.json"), nil
}

// LoadSettings loads the application settings from the settings file.
// If the settings file does not exist, it returns default values.
// Returns (settings, modified, error) where modified indicates if defaults were applied.
func LoadSettings() (*AppSettings, bool, error) {
	settingsMutex.Lock()
	defer settingsMutex.Unlock()

	log := logger.Get()
	modified := false

	settingsFile, err := getSettingsFilePath()
	if err != nil {
		return nil, false, err
	}

	// Check if settings file exists
	if _, err := os.Stat(settingsFile); os.IsNotExist(err) {
		log.Info().Msg("Settings file not found, returning default values")
		return defaultSettings(), true, nil
	}

	// Read settings file
	data, err := os.ReadFile(settingsFile)
	if err != nil {
		return nil, false, fmt.Errorf("failed to read settings file: %w", err)
	}

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
	// Do not force-initialize Storages; when empty, keep it nil so it is omitted from JSON
	if settings.EnabledStorages == nil {
		modified = true
		settings.EnabledStorages = []string{}
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
	// Ensure MaxNetworkCards has a valid default value
	if settings.MaxNetworkCards < MinNetworkCards || settings.MaxNetworkCards > MaxNetworkCards {
		modified = true
		settings.MaxNetworkCards = MinNetworkCards
	}
	// Ensure MaxDiskPerVM has a valid default value
	if settings.MaxDiskPerVM < MinDiskPerVM || settings.MaxDiskPerVM > MaxDiskPerVM {
		modified = true
		settings.MaxDiskPerVM = MinDiskPerVM
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

	settingsFile, err := getSettingsFilePath()
	if err != nil {
		return err
	}

	// Ensure empty optional fields are omitted
	if settings != nil && len(settings.EnabledStorages) == 0 {
		settings.EnabledStorages = nil
	}

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

// GetMaxDisksForBus returns the maximum number of disks allowed for a specific bus type
func GetMaxDisksForBus(busType string) int {
	switch busType {
	case DiskBusIDE:
		return MaxDisksIDE
	case DiskBusSATA:
		return MaxDisksSATA
	case DiskBusVirtIO:
		return MaxDisksVirtIO
	case DiskBusSCSI:
		return MaxDisksSCSI
	default:
		// Default to VirtIO (most common and highest limit)
		return MaxDisksVirtIO
	}
}
