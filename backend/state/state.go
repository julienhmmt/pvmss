package state

import (
	"html/template"
	"pvmss/proxmox"
	"sync"

	"github.com/alexedwards/scs/v2"
)

// Global application state
var (
	// Templates contains all the parsed HTML templates
	templates *template.Template
	// SessionManager handles user sessions
	sessionManager *scs.SessionManager
	// ProxmoxClient is the client for interacting with the Proxmox API
	proxmoxClient *proxmox.Client

	// Mutex for thread-safe access to the state
	mu sync.RWMutex
)

// SetTemplates sets the global templates
func SetTemplates(t *template.Template) {
	mu.Lock()
	defer mu.Unlock()
	templates = t
}

// GetTemplates returns the global templates
func GetTemplates() *template.Template {
	mu.RLock()
	defer mu.RUnlock()
	return templates
}

// SetSessionManager sets the global session manager
func SetSessionManager(sm *scs.SessionManager) {
	mu.Lock()
	defer mu.Unlock()
	sessionManager = sm
}

// GetSessionManager returns the global session manager
func GetSessionManager() *scs.SessionManager {
	mu.RLock()
	defer mu.RUnlock()
	return sessionManager
}

// SetProxmoxClient sets the global Proxmox client
func SetProxmoxClient(pc *proxmox.Client) {
	mu.Lock()
	defer mu.Unlock()
	proxmoxClient = pc
}

// GetProxmoxClient returns the global Proxmox client
func GetProxmoxClient() *proxmox.Client {
	mu.RLock()
	defer mu.RUnlock()
	return proxmoxClient
}
