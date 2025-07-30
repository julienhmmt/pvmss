// Package state provides centralized management of application state and dependencies.
// It uses dependency injection and interface-based design for better testability and maintainability.
package state

import (
	"pvmss/logger"
	"sync"
)

// globalState is the singleton instance used for backward compatibility
var (
	globalState StateManager
	once        sync.Once
)

// InitGlobalState initializes the global state manager
func InitGlobalState() StateManager {
	once.Do(func() {
		globalState = NewAppState()
		logger.Get().Info().Msg("Global state manager initialized")
	})
	return globalState
}

// GetGlobalState returns the global state manager.
// It assumes InitGlobalState has been called at application startup.
func GetGlobalState() StateManager {
	if globalState == nil {
		// This should not happen if InitGlobalState is called in main().
		// It's a programming error.
		logger.Get().Fatal().Msg("FATAL: Global state accessed before initialization")
	}
	return globalState
}

// GetSettings returns the application settings from the global state
func GetSettings() *AppSettings {
	return GetGlobalState().GetSettings()
}

// SetSettings updates the application settings in the global state
func SetSettings(settings *AppSettings) error {
	return GetGlobalState().SetSettings(settings)
}

// GetAdminPassword returns the admin password hash from the global state
func GetAdminPassword() string {
	return GetGlobalState().GetAdminPassword()
}

// GetTags returns the list of available tags from the global state
func GetTags() []string {
	return GetGlobalState().GetTags()
}

// GetISOs returns the list of available ISO files from the global state
func GetISOs() []string {
	return GetGlobalState().GetISOs()
}

// GetVMBRs returns the list of available network bridges from the global state
func GetVMBRs() []string {
	return GetGlobalState().GetVMBRs()
}

// GetLimits returns the resource limits from the global state
func GetLimits() map[string]interface{} {
	return GetGlobalState().GetLimits()
}
