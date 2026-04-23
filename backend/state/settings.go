package state

import (
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
	// MaxVMs is the per-node VM count override from the database node_limits table.
	// Zero means no per-node override (use the global limit).
	MaxVMs int `json:"max_vms,omitempty"`
	// MaxDiskGB is the total disk capacity cap (GB) for PVMSS VMs on this node.
	// Zero means no cap. Stored in the database but not yet enforced during VM creation.
	MaxDiskGB int `json:"max_disk_gb,omitempty"`
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
	DefaultDiskPerVM      = 4              // Default max disks per VM
	MinVMPerUser          = 0              // Minimum VMs per user (0 = no VMs allowed)
	MaxVMPerUser          = 100            // Maximum VMs per user (reasonable upper limit)
	DefaultVMPerUser      = 5              // Default VMs per user
	MinSnapshotsPerVM     = 0              // Minimum snapshots per VM (0 = no snapshots allowed)
	MaxSnapshotsPerVM     = 32             // Maximum snapshots per VM (reasonable upper limit)
	DefaultSnapshotsPerVM = 8              // Default snapshots per VM
)

// VMProfileConfig defines a VM profile for the simplified creation wizard.
type VMProfileConfig struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Sockets     int    `json:"sockets"`
	Cores       int    `json:"cores"`
	RAMGB       int    `json:"ram_gb"`
	DiskGB      int    `json:"disk_gb"`
	DiskBus     string `json:"disk_bus"`
	// Node and Storage are optional in JSON (omitempty). Empty string means auto-select.
	Node      string `json:"node,omitempty"`
	Storage   string `json:"storage,omitempty"`
	Icon      string `json:"icon"`
	Color     string `json:"color"`
	Enabled   bool   `json:"enabled"`
	EnableEFI bool   `json:"enable_efi,omitempty"`
}

// DefaultVMProfiles returns the built-in profiles used as fallback when none are configured.
func DefaultVMProfiles() []VMProfileConfig {
	return []VMProfileConfig{
		{ID: "web-server", Name: "Web Server", Description: "Host websites, reverse proxies, or static content servers", Sockets: 1, Cores: 1, RAMGB: 2, DiskGB: 24, DiskBus: "virtio", Icon: "Globe", Color: "blue", Enabled: true},
		{ID: "light-api", Name: "Lightweight API", Description: "Run REST APIs, microservices, or lightweight backend services", Sockets: 1, Cores: 2, RAMGB: 2, DiskGB: 24, DiskBus: "virtio", Icon: "Code", Color: "violet", Enabled: true},
		{ID: "light-app-server", Name: "Light App Server", Description: "Deploy application runtimes, job queues, or middleware", Sockets: 1, Cores: 4, RAMGB: 4, DiskGB: 32, DiskBus: "virtio", Icon: "Cube", Color: "emerald", Enabled: true},
		{ID: "medium-app-server", Name: "Medium App Server", Description: "Heavier workloads requiring more RAM — frameworks, daemons, or data pipelines", Sockets: 1, Cores: 4, RAMGB: 6, DiskGB: 32, DiskBus: "virtio", Icon: "Database", Color: "teal", Enabled: true},
		{ID: "test-base", Name: "Test Environment", Description: "Quick sandbox for testing, prototyping, or CI/CD pipelines", Sockets: 1, Cores: 2, RAMGB: 4, DiskGB: 24, DiskBus: "virtio", Icon: "Flask", Color: "amber", Enabled: true},
	}
}

// GetVMProfiles returns all configured profiles, falling back to built-in defaults when none are set.
func (s *AppSettings) GetVMProfiles() []VMProfileConfig {
	if len(s.VMProfiles) == 0 {
		return DefaultVMProfiles()
	}
	return s.VMProfiles
}

// GetEnabledVMProfiles returns only enabled profiles (or all defaults when none are configured).
func (s *AppSettings) GetEnabledVMProfiles() []VMProfileConfig {
	all := s.GetVMProfiles()
	enabled := make([]VMProfileConfig, 0, len(all))
	for _, p := range all {
		if p.Enabled {
			enabled = append(enabled, p)
		}
	}
	return enabled
}

// AddOrUpdateVMProfile upserts a profile by ID.
func (s *AppSettings) AddOrUpdateVMProfile(profile VMProfileConfig) {
	for i, p := range s.VMProfiles {
		if p.ID == profile.ID {
			s.VMProfiles[i] = profile
			return
		}
	}
	s.VMProfiles = append(s.VMProfiles, profile)
}

// RemoveVMProfile deletes a profile by ID and returns true if found.
func (s *AppSettings) RemoveVMProfile(id string) bool {
	for i, p := range s.VMProfiles {
		if p.ID == id {
			s.VMProfiles = append(s.VMProfiles[:i], s.VMProfiles[i+1:]...)
			return true
		}
	}
	return false
}

// CloudInitTemplate represents metadata for a cloud-init template managed by PVMSS.
// Templates are stored in the database; Storage and Filename are kept as
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
		EnabledNodes:    []string{},
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
		MaxDiskPerVM:       DefaultDiskPerVM,
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

type AppSettings struct {
	EnabledNodes       []string                    `json:"enabled_nodes,omitempty"`
	EnabledStorages    []string                    `json:"enabled_storages"`
	ISOs               []string                    `json:"isos"`
	Limits             LimitsConfig                `json:"limits"`
	MaxNetworkCards    int                         `json:"max_network_cards,omitempty"`
	MaxDiskPerVM       int                         `json:"max_disk_per_vm,omitempty"`
	MaxVMPerUser       int                         `json:"max_vm_per_user,omitempty"`
	Tags               []string                    `json:"tags"`
	VMBRs              []string                    `json:"vmbrs"`
	CloudInitTemplates []CloudInitTemplate         `json:"cloudinit_templates,omitempty"`
	VMProfiles         []VMProfileConfig           `json:"vm_profiles,omitempty"`
	AllowCustomYAML    bool                        `json:"allow_custom_yaml,omitempty"` // Allow users to provide custom YAML (default: true)
	CloudInitSFTP      proxmox.CloudInitSFTPConfig `json:"cloudinit_sftp,omitempty"`    // SSH/SFTP configuration for snippet uploads
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
