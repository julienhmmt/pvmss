package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"pvmss/logger"
	"pvmss/proxmox"
)

// ResourceRange defines min/max values for a resource
type ResourceRange struct {
	Min int `json:"min"`
	Max int `json:"max"`
}

// VMResourceLimits defines resource limits for VMs
type VMResourceLimits struct {
	Sockets ResourceRange `json:"sockets"`
	Cores   ResourceRange `json:"cores"`
	RAM     ResourceRange `json:"ram"`
	Disk    ResourceRange `json:"disk"`
}

// NodeResourceLimits defines resource limits for a specific node
type NodeResourceLimits struct {
	Sockets ResourceRange `json:"sockets"`
	Cores   ResourceRange `json:"cores"`
	RAM     ResourceRange `json:"ram"`
	Disk    ResourceRange `json:"disk"`
}

// LimitsConfig defines the structure for all resource limits
type LimitsConfig struct {
	VM    VMResourceLimits              `json:"vm"`
	Nodes map[string]NodeResourceLimits `json:"nodes,omitempty"`
	// MaxSnapshots defines the maximum number of snapshots allowed per VM.
	MaxSnapshots int `json:"max_snapshots,omitempty"`
}

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
	MinNetworkCards       = 1
	MaxNetworkCards       = 32 // Maximum network cards (net0-net31)
	MinDiskPerVM          = 1
	MaxDiskPerVM          = MaxDisksVirtIO // Maximum disks overall (VirtIO Block limit)
	MinVMPerUser          = 0              // Minimum VMs per user (0 = no VMs allowed)
	MaxVMPerUser          = 100            // Maximum VMs per user (reasonable upper limit)
	DefaultVMPerUser      = 5              // Default VMs per user
	MinSnapshotsPerVM     = 0              // Minimum snapshots per VM (0 = no snapshots allowed)
	MaxSnapshotsPerVM     = 32             // Maximum snapshots per VM (reasonable upper limit)
	DefaultSnapshotsPerVM = 8              // Default snapshots per VM
)

// CloudInitTemplate represents metadata for a cloud-init template managed by PVMSS.
// Templates are stored locally in settings.json; Storage and Filename are kept as
// optional metadata about the intended Proxmox storage and filename.
type CloudInitTemplate struct {
	ID          string `json:"id"`           // Unique identifier (filename without prefix)
	Name        string `json:"name"`         // Human-readable name
	Description string `json:"description"`  // Short description shown to users
	Storage     string `json:"storage"`      // Intended Proxmox storage ID (metadata only)
	Filename    string `json:"filename"`     // Intended filename (e.g., pvmss-mytemplate.yml)
	YAMLContent string `json:"yaml_content"` // YAML content stored locally and applied to VMs
	Enabled     bool   `json:"enabled"`      // Whether visible to users
}

// defaultSettings returns the default application settings
func defaultSettings() *AppSettings {
	return &AppSettings{
		EnabledStorages: []string{},
		ISOs:            []string{},
		Limits: LimitsConfig{
			MaxSnapshots: DefaultSnapshotsPerVM,
			VM: VMResourceLimits{
				Sockets: ResourceRange{Min: 1, Max: 1},
				Cores:   ResourceRange{Min: 1, Max: 2},
				RAM:     ResourceRange{Min: 1, Max: 4},
				Disk:    ResourceRange{Min: 1, Max: 10},
			},
			Nodes: make(map[string]NodeResourceLimits),
		},
		MaxNetworkCards:    MinNetworkCards,
		MaxDiskPerVM:       MinDiskPerVM,
		MaxVMPerUser:       DefaultVMPerUser,
		Tags:               []string{"pvmss"},
		VMBRs:              []string{},
		CloudInitTemplates: []CloudInitTemplate{},
		AllowCustomYAML:    true, // Allow custom YAML by default
		CloudInitSFTP: proxmox.CloudInitSFTPConfig{
			Enabled:        false, // Disabled by default
			Host:           "",
			Port:           22,
			Username:       "pvmss-snippets",
			PrivateKeyPath: "/app/pvmss_snippets_ed25519",
			SnippetBaseDir: "/var/lib/vz/snippets",
		},
	}
}

var settingsMutex = &sync.Mutex{}

type AppSettings struct {
	EnabledStorages    []string                    `json:"enabled_storages"`
	ISOs               []string                    `json:"isos"`
	Limits             LimitsConfig                `json:"limits"`
	MaxNetworkCards    int                         `json:"max_network_cards,omitempty"`
	MaxDiskPerVM       int                         `json:"max_disk_per_vm,omitempty"`
	MaxVMPerUser       int                         `json:"max_vm_per_user,omitempty"`
	Tags               []string                    `json:"tags"`
	VMBRs              []string                    `json:"vmbrs"`
	CloudInitTemplates []CloudInitTemplate         `json:"cloudinit_templates,omitempty"`
	AllowCustomYAML    bool                        `json:"allow_custom_yaml,omitempty"` // Allow users to provide custom YAML (default: true)
	CloudInitSFTP      proxmox.CloudInitSFTPConfig `json:"cloudinit_sftp,omitempty"`    // SSH/SFTP configuration for snippet uploads
	// JWTSecret is the signing key for /api/v1/ JWT tokens (minimum 32 bytes).
	// Stored in settings.json; no environment variable needed.
	JWTSecret string `json:"jwt_secret,omitempty"`
}

// getSettingsFilePath returns the absolute path to the settings file.
// It uses PVMSS_SETTINGS_PATH if set; otherwise, it looks for settings.json
// in the backend directory relative to the executable.
func getSettingsFilePath() (string, error) {
	if v := os.Getenv("PVMSS_SETTINGS_PATH"); v != "" {
		// Validate path to prevent directory traversal attacks
		// Ensure the path is absolute and doesn't contain suspicious patterns
		absPath, err := filepath.Abs(v)
		if err != nil {
			return "", fmt.Errorf("invalid settings path: %w", err)
		}
		// Prevent path traversal by checking for ".." in the path
		if strings.Contains(filepath.Clean(absPath), "..") {
			return "", fmt.Errorf("invalid settings path: path traversal detected")
		}
		return absPath, nil
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
		return nil, false, fmt.Errorf("failed to get settings file path: %w", err)
	}

	// Check if settings file exists
	if _, err := os.Stat(settingsFile); os.IsNotExist(err) {
		log.Info().Msg("Settings file not found, returning default values")
		return defaultSettings(), true, nil
	}

	// Read settings file
	data, err := os.ReadFile(settingsFile) // #nosec G304 - settingsFile comes from getSettingsFilePath() which validates env path and otherwise uses fixed project paths
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

	// Ensure VM limits exist with defaults (for new installations or corrupted data)
	if settings.Limits.VM.Sockets.Min == 0 && settings.Limits.VM.Sockets.Max == 0 {
		modified = true
		settings.Limits.VM = VMResourceLimits{
			Sockets: ResourceRange{Min: 1, Max: 1},
			Cores:   ResourceRange{Min: 1, Max: 2},
			RAM:     ResourceRange{Min: 1, Max: 4},
			Disk:    ResourceRange{Min: 1, Max: 10},
		}
		// Initialize empty nodes map if nil
		if settings.Limits.Nodes == nil {
			settings.Limits.Nodes = make(map[string]NodeResourceLimits)
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
	// Ensure MaxVMPerUser has a valid default value
	if settings.MaxVMPerUser < MinVMPerUser || settings.MaxVMPerUser > MaxVMPerUser {
		modified = true
		settings.MaxVMPerUser = DefaultVMPerUser
	}
	// Ensure MaxSnapshots has a valid default value under limits
	if settings.Limits.MaxSnapshots < MinSnapshotsPerVM || settings.Limits.MaxSnapshots > MaxSnapshotsPerVM {
		modified = true
		settings.Limits.MaxSnapshots = DefaultSnapshotsPerVM
	}

	// Warn when jwt_secret is absent or too short (API auth will refuse to issue tokens)
	if settings.JWTSecret == "" {
		log.Warn().Msg("jwt_secret not set in settings.json — /api/v1/ JWT auth will be unavailable")
	} else if len(settings.JWTSecret) < 32 {
		log.Warn().Int("length", len(settings.JWTSecret)).Msg("jwt_secret is shorter than 32 bytes — consider using a longer secret")
	}

	// Validate SFTP configuration if enabled
	if settings.CloudInitSFTP.Enabled {
		log.Info().Msg("Validating SFTP configuration for cloud-init snippet uploads")

		// Check if private key file exists and is readable
		if _, err := os.Stat(settings.CloudInitSFTP.PrivateKeyPath); os.IsNotExist(err) {
			log.Error().
				Str("private_key_path", settings.CloudInitSFTP.PrivateKeyPath).
				Msg("SFTP private key file not found, disabling SFTP upload")
			settings.CloudInitSFTP.Enabled = false
			modified = true
		} else if err != nil {
			log.Error().
				Err(err).
				Str("private_key_path", settings.CloudInitSFTP.PrivateKeyPath).
				Msg("Cannot access SFTP private key file, disabling SFTP upload")
			settings.CloudInitSFTP.Enabled = false
			modified = true
		} else {
			log.Info().
				Str("host", settings.CloudInitSFTP.Host).
				Str("username", settings.CloudInitSFTP.Username).
				Str("private_key_path", settings.CloudInitSFTP.PrivateKeyPath).
				Msg("SFTP configuration validated and enabled")
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

	settingsFile, err := getSettingsFilePath()
	if err != nil {
		return fmt.Errorf("failed to get settings file path for save: %w", err)
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

// CloudInitTemplatePrefix is the required prefix for PVMSS-managed cloud-init templates.
const CloudInitTemplatePrefix = "pvmss-"

// GetCloudInitTemplateByID finds a template by its ID.
func (s *AppSettings) GetCloudInitTemplateByID(id string) *CloudInitTemplate {
	for i := range s.CloudInitTemplates {
		if s.CloudInitTemplates[i].ID == id {
			return &s.CloudInitTemplates[i]
		}
	}
	return nil
}

// GetEnabledCloudInitTemplates returns only enabled templates.
func (s *AppSettings) GetEnabledCloudInitTemplates() []CloudInitTemplate {
	var enabled []CloudInitTemplate
	for _, t := range s.CloudInitTemplates {
		if t.Enabled {
			enabled = append(enabled, t)
		}
	}
	return enabled
}

// AddOrUpdateCloudInitTemplate adds a new template or updates an existing one.
func (s *AppSettings) AddOrUpdateCloudInitTemplate(template CloudInitTemplate) {
	for i, t := range s.CloudInitTemplates {
		if t.ID == template.ID {
			s.CloudInitTemplates[i] = template
			return
		}
	}
	s.CloudInitTemplates = append(s.CloudInitTemplates, template)
}

// RemoveCloudInitTemplate removes a template by ID.
func (s *AppSettings) RemoveCloudInitTemplate(id string) bool {
	for i, t := range s.CloudInitTemplates {
		if t.ID == id {
			s.CloudInitTemplates = append(s.CloudInitTemplates[:i], s.CloudInitTemplates[i+1:]...)
			return true
		}
	}
	return false
}

// SetCloudInitTemplateEnabled sets the enabled state for a template.
func (s *AppSettings) SetCloudInitTemplateEnabled(id string, enabled bool) bool {
	for i := range s.CloudInitTemplates {
		if s.CloudInitTemplates[i].ID == id {
			s.CloudInitTemplates[i].Enabled = enabled
			return true
		}
	}
	return false
}
