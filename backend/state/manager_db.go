package state

import (
	"encoding/json"
	"fmt"

	"pvmss/database"
	"pvmss/logger"
	"pvmss/proxmox"
)

// ── Cache reload ──────────────────────────────────────────────────────────────

// reloadSettingsCache reads all settings from the DB and atomically replaces
// the in-memory settings cache.  It is called after every successful write so
// the cache is always consistent with the database.
// Callers must NOT hold settingsMu when calling this function.
func (s *appState) reloadSettingsCache() error {
	if s.db == nil {
		return nil
	}
	dbSettings, err := s.db.LoadAppSettings()
	if err != nil {
		return fmt.Errorf("reload settings cache: %w", err)
	}
	fresh := mapDBToStateSettings(dbSettings)
	s.settingsMu.Lock()
	s.settings = fresh
	s.settingsMu.Unlock()
	logger.Get().Debug().Msg("Settings cache reloaded from database")
	return nil
}

// mapDBToStateSettings translates a database.AppSettings (assembled from DB
// tables) into the state.AppSettings type used by the rest of the application.
// Fields that have no DB representation (e.g. VM resource-range limits) are
// filled from defaultSettings() so callers always receive a valid struct.
func mapDBToStateSettings(src *database.AppSettings) *AppSettings {
	base := defaultSettings()

	base.EnabledNodes = src.EnabledNodes
	base.EnabledStorages = src.EnabledStorages
	base.ISOs = src.EnabledISOs
	base.VMBRs = src.EnabledVMBRs
	base.Tags = src.Tags
	base.MaxNetworkCards = src.Limits.MaxNetworkCards
	base.MaxDiskPerVM = src.Limits.MaxDiskPerVM
	base.MaxVMPerUser = src.Limits.MaxVMPerUser
	base.AllowCustomYAML = src.Limits.AllowCustomYAML
	base.Limits.MaxSnapshots = src.Limits.MaxSnapshots

	// Map DB node_limits (node → max_vms) into the state-layer Limits.Nodes map.
	// Preserve any existing resource-range defaults from defaultSettings() and
	// overlay the MaxVMs value from the database.
	for nodeName, maxVMs := range src.NodeLimits {
		existing := base.Limits.Nodes[nodeName]
		existing.MaxVMs = maxVMs
		base.Limits.Nodes[nodeName] = existing
	}

	base.CloudInitTemplates = mapCloudInitTemplatesFromDB(src.CloudInitTemplates)
	base.VMProfiles = mapVMProfilesFromDB(src.VMProfiles)
	base.CloudInitSFTP = mapSFTPFromDB(src.SFTPConfig)

	return base
}

// mapCloudInitTemplatesFromDB converts a slice of DB cloud-init template
// records into the state-layer type.
func mapCloudInitTemplatesFromDB(src []database.CloudInitTemplate) []CloudInitTemplate {
	out := make([]CloudInitTemplate, len(src))
	for i, t := range src {
		out[i] = CloudInitTemplate{
			ID:          t.ID,
			Name:        t.Name,
			Description: t.Description,
			Storage:     t.Storage,
			Filename:    t.Filename,
			YAMLContent: t.YAMLContent,
			Enabled:     t.Enabled,
		}
	}
	return out
}

// vmProfileConfigBlob is an alias for the exported database type used to
// unmarshal the JSON config blob stored inside database.VMProfile.Config.
type vmProfileConfigBlob = database.VMProfileConfigBlob

// mapVMProfilesFromDB converts a slice of DB VM profile records into the
// state-layer type, unmarshalling each Config JSON blob.  Records with
// malformed JSON are skipped with a warning log.
func mapVMProfilesFromDB(src []database.VMProfile) []VMProfileConfig {
	out := make([]VMProfileConfig, 0, len(src))
	for _, p := range src {
		var blob vmProfileConfigBlob
		if err := json.Unmarshal([]byte(p.Config), &blob); err != nil {
			logger.Get().Warn().
				Str("profile_id", p.ID).
				Err(err).
				Msg("Skipping VM profile with invalid config JSON")
			continue
		}
		out = append(out, VMProfileConfig{
			ID:          p.ID,
			Name:        p.Name,
			Description: p.Description,
			Sockets:     blob.Sockets,
			Cores:       blob.Cores,
			RAMGB:       blob.RAMGB,
			DiskGB:      blob.DiskGB,
			DiskBus:     blob.DiskBus,
			Node:        blob.Node,
			Storage:     blob.Storage,
			Icon:        blob.Icon,
			Color:       blob.Color,
			Enabled:     p.Enabled,
		})
	}
	return out
}

// mapSFTPFromDB converts a database.SFTPConfig into proxmox.CloudInitSFTPConfig.
func mapSFTPFromDB(src database.SFTPConfig) proxmox.CloudInitSFTPConfig {
	return proxmox.CloudInitSFTPConfig{
		Enabled:        src.Enabled,
		Host:           src.Host,
		Port:           src.Port,
		Username:       src.Username,
		PrivateKeyPath: src.PrivateKeyPath,
		SnippetBaseDir: src.RemotePath,
	}
}

// ── DB setter methods ─────────────────────────────────────────────────────────
// Each setter writes to the DB then reloads the in-memory cache.
// changedBy must be the authenticated admin username extracted from the JWT
// claims by the caller; it is recorded in the audit_log.

// SetVMLimits persists updated VM resource limits and refreshes the cache.
func (s *appState) SetVMLimits(limits *database.VMLimits, changedBy string) error {
	if s.db == nil {
		return fmt.Errorf("SetVMLimits: no database configured")
	}
	if err := s.db.SetVMLimits(limits, changedBy); err != nil {
		return fmt.Errorf("SetVMLimits: %w", err)
	}
	return s.reloadSettingsCache()
}

// SetNodeLimit upserts a per-node maximum VM count and refreshes the cache.
func (s *appState) SetNodeLimit(node string, maxVMs int, changedBy string) error {
	if s.db == nil {
		return fmt.Errorf("SetNodeLimit: no database configured")
	}
	if err := s.db.SetNodeLimit(node, maxVMs, changedBy); err != nil {
		return fmt.Errorf("SetNodeLimit: %w", err)
	}
	return s.reloadSettingsCache()
}

// DeleteNodeLimit removes a per-node VM limit and refreshes the cache.
func (s *appState) DeleteNodeLimit(node string, changedBy string) error {
	if s.db == nil {
		return fmt.Errorf("DeleteNodeLimit: no database configured")
	}
	if err := s.db.DeleteNodeLimit(node, changedBy); err != nil {
		return fmt.Errorf("DeleteNodeLimit: %w", err)
	}
	return s.reloadSettingsCache()
}

// SetEnabledNodes replaces the list of enabled Proxmox nodes and refreshes the cache.
func (s *appState) SetEnabledNodes(nodes []string, changedBy string) error {
	if s.db == nil {
		return fmt.Errorf("SetEnabledNodes: no database configured")
	}
	if err := s.db.SetEnabledNodes(nodes, changedBy); err != nil {
		return fmt.Errorf("SetEnabledNodes: %w", err)
	}
	return s.reloadSettingsCache()
}

// SetEnabledStorages replaces the list of enabled storages and refreshes the cache.
func (s *appState) SetEnabledStorages(storages []string, changedBy string) error {
	if s.db == nil {
		return fmt.Errorf("SetEnabledStorages: no database configured")
	}
	if err := s.db.SetEnabledStorages(storages, changedBy); err != nil {
		return fmt.Errorf("SetEnabledStorages: %w", err)
	}
	return s.reloadSettingsCache()
}

// SetEnabledISOs replaces the list of available ISO images and refreshes the cache.
func (s *appState) SetEnabledISOs(isos []string, changedBy string) error {
	if s.db == nil {
		return fmt.Errorf("SetEnabledISOs: no database configured")
	}
	if err := s.db.SetEnabledISOs(isos, changedBy); err != nil {
		return fmt.Errorf("SetEnabledISOs: %w", err)
	}
	return s.reloadSettingsCache()
}

// SetEnabledVMBRs replaces the list of allowed network bridges and refreshes the cache.
func (s *appState) SetEnabledVMBRs(vmbrs []string, changedBy string) error {
	if s.db == nil {
		return fmt.Errorf("SetEnabledVMBRs: no database configured")
	}
	if err := s.db.SetEnabledVMBRs(vmbrs, changedBy); err != nil {
		return fmt.Errorf("SetEnabledVMBRs: %w", err)
	}
	return s.reloadSettingsCache()
}

// SetTags replaces the list of VM tags and refreshes the cache.
func (s *appState) SetTags(tags []string, changedBy string) error {
	if s.db == nil {
		return fmt.Errorf("SetTags: no database configured")
	}
	if err := s.db.SetTags(tags, changedBy); err != nil {
		return fmt.Errorf("SetTags: %w", err)
	}
	return s.reloadSettingsCache()
}

// CreateCloudInitTemplate inserts a new cloud-init template and refreshes the cache.
func (s *appState) CreateCloudInitTemplate(t *database.CloudInitTemplate, changedBy string) error {
	if s.db == nil {
		return fmt.Errorf("CreateCloudInitTemplate: no database configured")
	}
	if err := s.db.CreateCloudInitTemplate(t, changedBy); err != nil {
		return fmt.Errorf("CreateCloudInitTemplate: %w", err)
	}
	return s.reloadSettingsCache()
}

// UpdateCloudInitTemplate updates an existing cloud-init template and refreshes the cache.
func (s *appState) UpdateCloudInitTemplate(t *database.CloudInitTemplate, changedBy string) error {
	if s.db == nil {
		return fmt.Errorf("UpdateCloudInitTemplate: no database configured")
	}
	if err := s.db.UpdateCloudInitTemplate(t, changedBy); err != nil {
		return fmt.Errorf("UpdateCloudInitTemplate: %w", err)
	}
	return s.reloadSettingsCache()
}

// DeleteCloudInitTemplate removes a cloud-init template by ID and refreshes the cache.
func (s *appState) DeleteCloudInitTemplate(id string, changedBy string) error {
	if s.db == nil {
		return fmt.Errorf("DeleteCloudInitTemplate: no database configured")
	}
	if err := s.db.DeleteCloudInitTemplate(id, changedBy); err != nil {
		return fmt.Errorf("DeleteCloudInitTemplate: %w", err)
	}
	return s.reloadSettingsCache()
}

// CreateVMProfile inserts a new VM profile and refreshes the cache.
func (s *appState) CreateVMProfile(p *database.VMProfile, changedBy string) error {
	if s.db == nil {
		return fmt.Errorf("CreateVMProfile: no database configured")
	}
	if err := s.db.CreateVMProfile(p, changedBy); err != nil {
		return fmt.Errorf("CreateVMProfile: %w", err)
	}
	return s.reloadSettingsCache()
}

// UpdateVMProfile updates an existing VM profile and refreshes the cache.
func (s *appState) UpdateVMProfile(p *database.VMProfile, changedBy string) error {
	if s.db == nil {
		return fmt.Errorf("UpdateVMProfile: no database configured")
	}
	if err := s.db.UpdateVMProfile(p, changedBy); err != nil {
		return fmt.Errorf("UpdateVMProfile: %w", err)
	}
	return s.reloadSettingsCache()
}

// DeleteVMProfile removes a VM profile by ID and refreshes the cache.
func (s *appState) DeleteVMProfile(id string, changedBy string) error {
	if s.db == nil {
		return fmt.Errorf("DeleteVMProfile: no database configured")
	}
	if err := s.db.DeleteVMProfile(id, changedBy); err != nil {
		return fmt.Errorf("DeleteVMProfile: %w", err)
	}
	return s.reloadSettingsCache()
}

// SetSFTPConfig persists the SFTP configuration and refreshes the cache.
func (s *appState) SetSFTPConfig(cfg *database.SFTPConfig, changedBy string) error {
	if s.db == nil {
		return fmt.Errorf("SetSFTPConfig: no database configured")
	}
	if err := s.db.SetSFTPConfig(cfg, changedBy); err != nil {
		return fmt.Errorf("SetSFTPConfig: %w", err)
	}
	return s.reloadSettingsCache()
}

// LoadSettingsFromDB loads all settings from the database into the in-memory
// cache.  Called once during startup after the DB is opened and any migration
// has completed.
func (s *appState) LoadSettingsFromDB() error {
	return s.reloadSettingsCache()
}

// HasDB reports whether the state manager is backed by a database.
func (s *appState) HasDB() bool {
	return s.db != nil
}
