package state

import (
	"errors"
	"fmt"
	"html/template"
	"pvmss/logger"
	"pvmss/proxmox"
	"sync"
	"time"

	"github.com/alexedwards/scs/v2"
)

// appState is the concrete implementation of StateManager
type appState struct {
	templates      *template.Template
	sessionManager *scs.SessionManager
	proxmoxClient  *proxmox.Client
	settings       *AppSettings
	mu             sync.RWMutex

	// Security-related fields
	csrfTokens    map[string]time.Time
	loginAttempts map[string][]time.Time
	securityMu    sync.RWMutex // Separate mutex for security operations
}

// NewAppState creates a new instance of the application state manager
func NewAppState() StateManager {
	state := &appState{
		settings:      &AppSettings{},
		csrfTokens:    make(map[string]time.Time),
		loginAttempts: make(map[string][]time.Time),
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
		s.CleanExpiredLoginAttempts()
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
func (s *appState) GetProxmoxClient() *proxmox.Client {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.proxmoxClient
}

// SetProxmoxClient sets the Proxmox client
func (s *appState) SetProxmoxClient(pc *proxmox.Client) error {
	if pc == nil {
		return errors.New("proxmox client cannot be nil")
	}
	s.mu.Lock()
	s.proxmoxClient = pc
	s.mu.Unlock()
	return nil
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

// RecordLoginAttempt records a login attempt for rate limiting
func (s *appState) RecordLoginAttempt(ip string, timestamp time.Time) error {
	if ip == "" {
		return errors.New("IP address cannot be empty")
	}

	s.securityMu.Lock()
	defer s.securityMu.Unlock()

	if s.loginAttempts[ip] == nil {
		s.loginAttempts[ip] = make([]time.Time, 0, 1)
	}
	s.loginAttempts[ip] = append(s.loginAttempts[ip], timestamp)

	return nil
}

// GetLoginAttempts returns all login attempts for an IP address
func (s *appState) GetLoginAttempts(ip string) ([]time.Time, error) {
	if ip == "" {
		return nil, errors.New("IP address cannot be empty")
	}

	s.securityMu.RLock()
	defer s.securityMu.RUnlock()

	// Return a copy of the slice to prevent external modification
	attempts := make([]time.Time, len(s.loginAttempts[ip]))
	copy(attempts, s.loginAttempts[ip])

	return attempts, nil
}

// CleanExpiredLoginAttempts removes login attempts older than the lockout period
func (s *appState) CleanExpiredLoginAttempts() {
	s.securityMu.Lock()
	defer s.securityMu.Unlock()

	now := time.Now()
	lockoutPeriod := 15 * time.Minute // Should match the security package constant

	for ip, attempts := range s.loginAttempts {
		var validAttempts []time.Time
		for _, attempt := range attempts {
			if now.Sub(attempt) < lockoutPeriod {
				validAttempts = append(validAttempts, attempt)
			}
		}

		if len(validAttempts) > 0 {
			s.loginAttempts[ip] = validAttempts
		} else {
			delete(s.loginAttempts, ip)
		}
	}
}
