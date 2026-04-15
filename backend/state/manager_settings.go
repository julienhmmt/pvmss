package state

import "errors"

// GetSettings returns the application settings.
func (s *appState) GetSettings() *AppSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

// SetSettingsWithoutSave updates the application settings in memory without persisting them.
func (s *appState) SetSettingsWithoutSave(settings *AppSettings) {
	if settings == nil {
		return
	}
	s.mu.Lock()
	s.settings = settings
	s.mu.Unlock()
}

// SetSettings updates the application settings and saves them to file.
func (s *appState) SetSettings(settings *AppSettings) error {
	if settings == nil {
		return errors.New("settings cannot be nil")
	}

	s.mu.Lock()
	s.settings = settings
	s.mu.Unlock()

	return WriteSettings(settings)
}

// GetTags returns the list of available tags.
func (s *appState) GetTags() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings == nil || s.settings.Tags == nil {
		return []string{}
	}
	return s.settings.Tags
}

// GetISOs returns the list of available ISO files.
func (s *appState) GetISOs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings == nil || s.settings.ISOs == nil {
		return []string{}
	}
	return s.settings.ISOs
}

// GetVMBRs returns the list of available network bridges.
func (s *appState) GetVMBRs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings == nil || s.settings.VMBRs == nil {
		return []string{}
	}
	return s.settings.VMBRs
}

// GetLimits returns the resource limits as a map for backward compatibility.
func (s *appState) GetLimits() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
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
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.settings == nil {
		return []string{}
	}
	return s.settings.EnabledStorages
}
