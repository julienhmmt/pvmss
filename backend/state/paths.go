package state

import "sync"

var (
	templatesPath string
	pathMutex     sync.RWMutex
)

// SetTemplatesPath définit le chemin global vers le répertoire des templates.
func SetTemplatesPath(path string) {
	pathMutex.Lock()
	defer pathMutex.Unlock()
	templatesPath = path
}

// GetTemplatesPath récupère le chemin global vers le répertoire des templates.
func GetTemplatesPath() string {
	pathMutex.RLock()
	defer pathMutex.RUnlock()
	return templatesPath
}
