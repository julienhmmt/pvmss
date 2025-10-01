package state

// This file contains DEPRECATED global state helper functions.
//
// MIGRATION GUIDE:
// Instead of using global functions like GetGlobalState(), InitGlobalState(),
// pass a StateManager instance via dependency injection to your components.
//
// Example (OLD - deprecated):
//   settings := state.GetSettings()
//
// Example (NEW - recommended):
//   type MyHandler struct {
//       stateManager state.StateManager
//   }
//   settings := h.stateManager.GetSettings()
//
// All functions in this file will be removed in a future release.

import (
	"pvmss/logger"
	"sync"
)

// globalState is the singleton instance used for backward compatibility
var (
	globalState StateManager
	once        sync.Once
)

// InitGlobalState initializes the global state manager.
//
// Deprecated: Prefer creating an instance with `NewAppState()` and passing it
// via dependency injection.
func InitGlobalState() StateManager {
	once.Do(func() {
		globalState = NewAppState()
		logger.Get().Info().Msg("Global state manager initialized")
	})
	return globalState
}

// GetGlobalState returns the global state manager.
// It assumes InitGlobalState has been called at application startup.
//
// Deprecated: Prefer passing a `StateManager` reference explicitly.
func GetGlobalState() StateManager {
	if globalState == nil {
		// This should not happen if InitGlobalState is called in main().
		// It's a programming error.
		logger.Get().Fatal().Msg("FATAL: Global state accessed before initialization")
	}
	return globalState
}

// GetSettings returns the application settings from the global state.
//
// Deprecated: Prefer calling `GetSettings()` on an injected `StateManager`.
func GetSettings() *AppSettings {
	return GetGlobalState().GetSettings()
}

// SetSettings updates the application settings in the global state.
//
// Deprecated: Prefer calling `SetSettings()` on an injected `StateManager`.
func SetSettings(settings *AppSettings) error {
	return GetGlobalState().SetSettings(settings)
}

// GetTags returns the list of available tags from the global state.
//
// Deprecated: Prefer calling `GetTags()` on an injected `StateManager`.
func GetTags() []string {
	return GetGlobalState().GetTags()
}

// GetISOs returns the list of available ISO files from the global state.
//
// Deprecated: Prefer calling `GetISOs()` on an injected `StateManager`.
func GetISOs() []string {
	return GetGlobalState().GetISOs()
}

// GetVMBRs returns the list of available network bridges from the global state.
//
// Deprecated: Prefer calling `GetVMBRs()` on an injected `StateManager`.
func GetVMBRs() []string {
	return GetGlobalState().GetVMBRs()
}

// GetLimits returns the resource limits from the global state.
//
// Deprecated: Prefer calling `GetLimits()` on an injected `StateManager`.
func GetLimits() map[string]interface{} {
	return GetGlobalState().GetLimits()
}

// GetStorages returns the list of available storages from the global state.
//
// Deprecated: Prefer calling `GetStorages()` on an injected `StateManager`.
func GetStorages() []string {
	return GetGlobalState().GetStorages()
}
