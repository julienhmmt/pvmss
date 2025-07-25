package state

import (
	"sync"
)

// AppSettings represents the application configuration
type AppSettings struct {
	// AdminPassword is the bcrypt hashed password for admin access
	AdminPassword string                 `json:"admin_password"`
	Tags          []string               `json:"tags"`
	ISOs          []string               `json:"isos"`
	VMBRs         []string               `json:"vmbrs"`
	Limits        map[string]interface{} `json:"limits"`
}

var (
	// appSettings stores the application settings
	appSettings *AppSettings
	// settingsMutex protects access to appSettings
	settingsMutex = &sync.RWMutex{}
)

// SetAppSettings sets the global application settings
func SetAppSettings(settings *AppSettings) {
	settingsMutex.Lock()
	defer settingsMutex.Unlock()
	appSettings = settings
}

// GetAppSettings returns the global application settings
func GetAppSettings() *AppSettings {
	settingsMutex.RLock()
	defer settingsMutex.RUnlock()
	return appSettings
}

// GetAdminPassword returns the admin password hash
func GetAdminPassword() string {
	settingsMutex.RLock()
	defer settingsMutex.RUnlock()
	if appSettings == nil {
		return ""
	}
	return appSettings.AdminPassword
}

// GetTags returns the list of available tags
func GetTags() []string {
	settingsMutex.RLock()
	defer settingsMutex.RUnlock()
	if appSettings == nil {
		return []string{}
	}
	return appSettings.Tags
}

// GetISOs returns the list of available ISO files
func GetISOs() []string {
	settingsMutex.RLock()
	defer settingsMutex.RUnlock()
	if appSettings == nil {
		return []string{}
	}
	return appSettings.ISOs
}

// GetVMBRs returns the list of available network bridges
func GetVMBRs() []string {
	settingsMutex.RLock()
	defer settingsMutex.RUnlock()
	if appSettings == nil {
		return []string{}
	}
	return appSettings.VMBRs
}

// GetLimits returns the resource limits
func GetLimits() map[string]interface{} {
	settingsMutex.RLock()
	defer settingsMutex.RUnlock()
	if appSettings == nil {
		return map[string]interface{}{}
	}
	return appSettings.Limits
}
