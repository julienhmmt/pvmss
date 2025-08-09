package state

import "sync"

var (
	templatesPath string
	pathMutex     sync.RWMutex
)

// Deprecated: Prefer injecting parsed templates through StateManager.SetTemplates
// and avoid global template path. This global will be removed in a future release.
func SetTemplatesPath(path string) {
	pathMutex.Lock()
	defer pathMutex.Unlock()
	templatesPath = path
}

// Deprecated: Prefer retrieving templates via StateManager.GetTemplates.
func GetTemplatesPath() string {
	pathMutex.RLock()
	defer pathMutex.RUnlock()
	return templatesPath
}
