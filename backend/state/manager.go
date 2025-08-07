package state

import (
	"context"
	"errors"
	"fmt"
	"html/template"
	"sync"
	"time"

	"github.com/alexedwards/scs/v2"

	"pvmss/logger"
	"pvmss/proxmox"
)

// appState is the concrete implementation of StateManager
type appState struct {
	templates      *template.Template
	sessionManager *scs.SessionManager
	proxmoxClient  proxmox.ClientInterface
	settings       *AppSettings
	mu             sync.RWMutex

	// Proxmox connection status
	proxmoxConnected bool
	proxmoxError     string
	proxmoxMu        sync.RWMutex

	// Security-related fields
	csrfTokens map[string]time.Time
	securityMu sync.RWMutex // Mutex for CSRF token operations
}

// NewAppState creates a new instance of the application state manager
func NewAppState() StateManager {
	state := &appState{
		settings:   &AppSettings{},
		csrfTokens: make(map[string]time.Time),
	}

	// Start background cleanup goroutines
	go state.cleanupSecurityData()

	return state
}

// cleanupSecurityData runs periodic cleanup of expired security data
func (s *appState) cleanupSecurityData() {
	ticker := time.NewTicker(30 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.CleanExpiredCSRFTokens()
	}
}

// GetTemplates returns the template cache
func (s *appState) GetTemplates() *template.Template {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.templates
}

// SetTemplates sets the template cache
func (s *appState) SetTemplates(t *template.Template) error {
	if t == nil {
		return errors.New("templates cannot be nil")
	}
	s.mu.Lock()
	s.templates = t
	s.mu.Unlock()
	return nil
}

// GetSessionManager returns the session manager
func (s *appState) GetSessionManager() *scs.SessionManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionManager
}

// SetSessionManager sets the session manager
func (s *appState) SetSessionManager(sm *scs.SessionManager) error {
	if sm == nil {
		return errors.New("session manager cannot be nil")
	}
	s.mu.Lock()
	s.sessionManager = sm
	s.mu.Unlock()
	return nil
}

// GetProxmoxClient returns the Proxmox client
func (s *appState) GetProxmoxClient() proxmox.ClientInterface {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.proxmoxClient
}

// SetProxmoxClient sets the Proxmox client
func (s *appState) SetProxmoxClient(pc proxmox.ClientInterface) error {
	if pc == nil {
		return errors.New("proxmox client cannot be nil")
	}
	s.mu.Lock()
	s.proxmoxClient = pc
	s.mu.Unlock()

	// Check connection when setting a new client
	s.CheckProxmoxConnection()
	return nil
}

// GetProxmoxStatus returns the current Proxmox connection status
func (s *appState) GetProxmoxStatus() (bool, string) {
	s.proxmoxMu.RLock()
	defer s.proxmoxMu.RUnlock()
	return s.proxmoxConnected, s.proxmoxError
}

// CheckProxmoxConnection checks the connection to the Proxmox server and updates the status
func (s *appState) CheckProxmoxConnection() bool {
	s.mu.RLock()
	client := s.proxmoxClient
	s.mu.RUnlock()

	if client == nil {
		s.updateProxmoxStatus(false, "Proxmox client not initialized")
		return false
	}

	// Try to get node names as a simple connection test
	nodes, err := proxmox.GetNodeNamesWithContext(context.Background(), client)
	if err != nil || len(nodes) == 0 {
		errMsg := "Failed to connect to Proxmox"
		if err != nil {
			errMsg = fmt.Sprintf("%s: %v", errMsg, err)
		}
		s.updateProxmoxStatus(false, errMsg)
		return false
	}

	// If we got here, the connection is good
	s.updateProxmoxStatus(true, "")
	return true
}

// updateProxmoxStatus updates the Proxmox connection status in a thread-safe way
func (s *appState) updateProxmoxStatus(connected bool, errorMsg string) {
	s.proxmoxMu.Lock()
	defer s.proxmoxMu.Unlock()

	// Only log if status changed
	if s.proxmoxConnected != connected || s.proxmoxError != errorMsg {
		status := "connected"
		if !connected {
			status = fmt.Sprintf("disconnected: %s", errorMsg)
		}
		logger.Get().Info().
			Bool("connected", connected).
			Str("error", errorMsg).
			Msgf("Proxmox connection status changed: %s", status)

		s.proxmoxConnected = connected
		s.proxmoxError = errorMsg
	}
}

// GetSettings returns the application settings
func (s *appState) GetSettings() *AppSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

// SetSettingsWithoutSave updates the application settings in memory without saving them to file
func (s *appState) SetSettingsWithoutSave(settings *AppSettings) {
	if settings == nil {
		logger.Get().Warn().Msg("Attempted to set nil settings without saving")
		return
	}
	s.mu.Lock()
	s.settings = settings
	s.mu.Unlock()
	logger.Get().Debug().Msg("Application settings updated in memory only")
}

// SetSettings updates the application settings and saves them to the settings file
func (s *appState) SetSettings(settings *AppSettings) error {
	if settings == nil {
		return errors.New("settings cannot be nil")
	}

	s.mu.Lock()
	s.settings = settings
	s.mu.Unlock()

	// Sauvegarder les paramètres dans le fichier
	if err := WriteSettings(settings); err != nil {
		logger.Get().Error().Err(err).Msg("Failed to save settings to file")
		return fmt.Errorf("failed to save settings: %w", err)
	}

	logger.Get().Info().Msg("Application settings updated and saved to file")
	return nil
}

// SaveSettings saves the settings to the settings file
func SaveSettings(settings *AppSettings) error {
	return WriteSettings(settings)
}

// GetAdminPassword returns the admin password hash
func (s *appState) GetAdminPassword() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings == nil {
		return ""
	}
	return s.settings.AdminPassword
}

// GetTags returns the list of available tags
func (s *appState) GetTags() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings == nil || s.settings.Tags == nil {
		return []string{}
	}
	return s.settings.Tags
}

// GetISOs returns the list of available ISO files
func (s *appState) GetISOs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings == nil || s.settings.ISOs == nil {
		return []string{}
	}
	return s.settings.ISOs
}

// GetVMBRs returns the list of available network bridges
func (s *appState) GetVMBRs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings == nil || s.settings.VMBRs == nil {
		return []string{}
	}
	return s.settings.VMBRs
}

// GetLimits returns the resource limits
func (s *appState) GetLimits() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings == nil || s.settings.Limits == nil {
		return make(map[string]interface{})
	}
	return s.settings.Limits
}

// Security Methods
// AddCSRFToken adds a new CSRF token with an expiry time
func (s *appState) AddCSRFToken(token string, expiry time.Time) error {
	s.securityMu.Lock()
	defer s.securityMu.Unlock()

	s.csrfTokens[token] = expiry

	log := logger.Get().With().
		Str("function", "AddCSRFToken").
		Str("token", token).
		Time("expiry", expiry).
		Logger()

	log.Debug().
		Int("total_tokens", len(s.csrfTokens)).
		Msg("Nouveau jeton CSRF ajouté")

	return nil
}

// ValidateAndRemoveCSRFToken validates a CSRF token and removes it if valid
func (s *appState) ValidateAndRemoveCSRFToken(token string) bool {
	log := logger.Get().With().
		Str("function", "ValidateAndRemoveCSRFToken").
		Str("token", token).
		Logger()

	s.securityMu.Lock()
	defer s.securityMu.Unlock()

	log.Debug().
		Int("total_tokens_before", len(s.csrfTokens)).
		Msg("Début de la validation du jeton CSRF")

	expiry, exists := s.csrfTokens[token]
	if !exists {
		log.Warn().
			Msg("Jeton CSRF non trouvé")
		return false
	}

	// Remove the token (one-time use)
	delete(s.csrfTokens, token)

	// Check if token is expired
	now := time.Now()
	if now.After(expiry) {
		log.Warn().
			Time("expiry", expiry).
			Time("now", now).
			Msg("Jeton CSRF expiré")
		return false
	}

	log.Debug().
		Int("total_tokens_after", len(s.csrfTokens)).
		Msg("Jeton CSRF validé et supprimé avec succès")

	return true
}

// CleanExpiredCSRFTokens removes all expired CSRF tokens
func (s *appState) CleanExpiredCSRFTokens() {
	s.securityMu.Lock()
	defer s.securityMu.Unlock()

	now := time.Now()
	for token, expiry := range s.csrfTokens {
		if now.After(expiry) {
			delete(s.csrfTokens, token)
		}
	}
}
