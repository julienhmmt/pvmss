package state

import "errors"

// GetSettings returns a deep copy of the application settings.
// Callers may freely mutate the returned pointer without affecting the
// in-memory cache.  All slices and maps are independently allocated.
func (s *appState) GetSettings() *AppSettings {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()
	if s.settings == nil {
		return &AppSettings{}
	}
	return deepCopySettings(s.settings)
}

// deepCopySettings returns an independent deep copy of AppSettings,
// allocating fresh backing arrays for every slice and map field.
func deepCopySettings(src *AppSettings) *AppSettings {
	cp := *src
	cp.EnabledNodes = copyStrings(src.EnabledNodes)
	cp.EnabledStorages = copyStrings(src.EnabledStorages)
	cp.ISOs = copyStrings(src.ISOs)
	cp.Tags = copyStrings(src.Tags)
	cp.VMBRs = copyStrings(src.VMBRs)
	cp.CloudInitTemplates = copyCloudInitTemplates(src.CloudInitTemplates)
	cp.VMProfiles = copyVMProfilesDeep(src.VMProfiles)
	cp.Limits.Nodes = copyNodeLimitsDeep(src.Limits.Nodes)
	return &cp
}

// copyStrings returns an independent copy of a string slice.
func copyStrings(src []string) []string {
	if src == nil {
		return nil
	}
	dst := make([]string, len(src))
	copy(dst, src)
	return dst
}

// copyCloudInitTemplates returns an independent copy of a CloudInitTemplate slice.
func copyCloudInitTemplates(src []CloudInitTemplate) []CloudInitTemplate {
	if src == nil {
		return nil
	}
	dst := make([]CloudInitTemplate, len(src))
	copy(dst, src)
	return dst
}

// copyVMProfilesDeep returns an independent copy of a VMProfileConfig slice.
func copyVMProfilesDeep(src []VMProfileConfig) []VMProfileConfig {
	if src == nil {
		return nil
	}
	dst := make([]VMProfileConfig, len(src))
	copy(dst, src)
	return dst
}

// copyNodeLimitsDeep returns an independent copy of the node limits map.
func copyNodeLimitsDeep(src map[string]NodeResourceLimits) map[string]NodeResourceLimits {
	if src == nil {
		return nil
	}
	dst := make(map[string]NodeResourceLimits, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// SetSettingsWithoutSave updates the settings cache without persisting to any backend.
func (s *appState) SetSettingsWithoutSave(settings *AppSettings) {
	if settings == nil {
		return
	}
	s.settingsMu.Lock()
	s.settings = settings
	s.settingsMu.Unlock()
}

// SetSettings updates the settings cache and persists them.
// DB is always configured after migration; use fine-grained DB setters instead.
func (s *appState) SetSettings(settings *AppSettings) error {
	if settings == nil {
		return errors.New("settings cannot be nil")
	}
	s.settingsMu.Lock()
	s.settings = settings
	s.settingsMu.Unlock()
	return nil // DB-backed: fine-grained setters handle persistence
}

// GetTags returns the list of available tags.
func (s *appState) GetTags() []string {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()
	if s.settings == nil || s.settings.Tags == nil {
		return []string{}
	}
	return s.settings.Tags
}

// GetISOs returns the list of available ISO files.
func (s *appState) GetISOs() []string {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()
	if s.settings == nil || s.settings.ISOs == nil {
		return []string{}
	}
	return s.settings.ISOs
}

// GetVMBRs returns the list of available network bridges.
func (s *appState) GetVMBRs() []string {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()
	if s.settings == nil || s.settings.VMBRs == nil {
		return []string{}
	}
	return s.settings.VMBRs
}

// GetLimits returns the resource limits as a map for backward compatibility.
func (s *appState) GetLimits() map[string]interface{} {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()
	if s.settings == nil {
		return make(map[string]interface{})
	}

	limits := make(map[string]interface{})

	vmLimits := make(map[string]interface{})
	vmLimits["sockets"] = map[string]int{"min": s.settings.Limits.VM.Sockets.Min, "max": s.settings.Limits.VM.Sockets.Max}
	vmLimits["cores"] = map[string]int{"min": s.settings.Limits.VM.Cores.Min, "max": s.settings.Limits.VM.Cores.Max}
	vmLimits["ram"] = map[string]int{"min": s.settings.Limits.VM.RAM.Min, "max": s.settings.Limits.VM.RAM.Max}
	vmLimits["disk"] = map[string]int{"min": s.settings.Limits.VM.Disk.Min, "max": s.settings.Limits.VM.Disk.Max}
	limits["vm"] = vmLimits

	if len(s.settings.Limits.Nodes) > 0 {
		nodesLimits := make(map[string]interface{})
		for nodeName, nodeLimits := range s.settings.Limits.Nodes {
			nodeMap := map[string]interface{}{
				"sockets": map[string]int{"min": nodeLimits.Sockets.Min, "max": nodeLimits.Sockets.Max},
				"cores":   map[string]int{"min": nodeLimits.Cores.Min, "max": nodeLimits.Cores.Max},
				"ram":     map[string]int{"min": nodeLimits.RAM.Min, "max": nodeLimits.RAM.Max},
				"disk":    map[string]int{"min": nodeLimits.Disk.Min, "max": nodeLimits.Disk.Max},
				"max_vms": nodeLimits.MaxVMs,
			}
			nodesLimits[nodeName] = nodeMap
		}
		limits["nodes"] = nodesLimits
	}

	limits["max_snapshots"] = s.settings.Limits.MaxSnapshots

	return limits
}

// GetStorages returns the list of available storages.
func (s *appState) GetStorages() []string {
	s.settingsMu.RLock()
	defer s.settingsMu.RUnlock()

	if s.settings == nil {
		return []string{}
	}
	return s.settings.EnabledStorages
}
