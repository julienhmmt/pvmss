package tests

import (
	"fmt"
	"html/template"
	"sync"
	"time"

	"pvmss/proxmox"
	"pvmss/state"

	"github.com/alexedwards/scs/v2"
)

// MockStateManager is a mock implementation of the StateManager interface for testing
type MockStateManager struct {
	templates      *template.Template
	sessionManager *scs.SessionManager
	proxmoxClient  proxmox.ClientInterface // Changed to use interface
	settings       *state.AppSettings
	mu             sync.RWMutex

	// Proxmox status
	proxmoxConnected bool
	proxmoxError     string
	proxmoxMu        sync.RWMutex

	// Security-related fields
	csrfTokens    map[string]time.Time
	loginAttempts map[string][]time.Time
	securityMu    sync.RWMutex
}

// Ensure MockStateManager implements StateManager
var _ state.StateManager = (*MockStateManager)(nil)

// MockAppSettings is an alias for state.AppSettings for backward compatibility
type MockAppSettings = state.AppSettings

// NewMockStateManager creates a new mock state manager for testing
func NewMockStateManager() *MockStateManager {
	return &MockStateManager{
		settings: &MockAppSettings{
			Tags:            []string{},
			ISOs:            []string{},
			VMBRs:           []string{},
			Storages:        []string{},
			EnabledStorages: []string{},
			Limits:          make(map[string]interface{}),
		},
		csrfTokens:    make(map[string]time.Time),
		loginAttempts: make(map[string][]time.Time),
	}
}

// WithMockProxmoxClient sets the mock Proxmox client
func (s *MockStateManager) WithMockProxmoxClient(client proxmox.ClientInterface) *MockStateManager {
	s.proxmoxClient = client
	return s
}

// WithMockSettings sets the mock settings
func (s *MockStateManager) WithMockSettings(settings *MockAppSettings) *MockStateManager {
	s.settings = settings
	return s
}

// GetTemplates returns the template cache
func (s *MockStateManager) GetTemplates() *template.Template {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.templates
}

// SetTemplates sets the template cache
func (s *MockStateManager) SetTemplates(t *template.Template) error {
	if t == nil {
		return fmt.Errorf("templates cannot be nil")
	}
	s.mu.Lock()
	s.templates = t
	s.mu.Unlock()
	return nil
}

// GetSessionManager returns the session manager
func (s *MockStateManager) GetSessionManager() *scs.SessionManager {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.sessionManager
}

// SetSessionManager sets the session manager
func (s *MockStateManager) SetSessionManager(sm *scs.SessionManager) error {
	if sm == nil {
		return fmt.Errorf("session manager cannot be nil")
	}
	s.mu.Lock()
	s.sessionManager = sm
	s.mu.Unlock()
	return nil
}

// GetProxmoxClient returns the Proxmox client
func (s *MockStateManager) GetProxmoxClient() proxmox.ClientInterface {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.proxmoxClient == nil {
		// Return a new mock client if none is set
		s.proxmoxClient = NewMockProxmoxClient("http://localhost:8006", "test-token-id", "test-token-secret", true)
	}

	return s.proxmoxClient
}

// SetProxmoxClient sets the Proxmox client
func (s *MockStateManager) SetProxmoxClient(pc proxmox.ClientInterface) error {
	if pc == nil {
		return fmt.Errorf("proxmox client cannot be nil")
	}
	s.mu.Lock()
	s.proxmoxClient = pc
	s.mu.Unlock()
	return nil
}

// GetSettings returns the mock application settings
func (s *MockStateManager) GetSettings() *state.AppSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Convert MockAppSettings to state.AppSettings
	mockSettings := s.settings
	if mockSettings == nil {
		return nil
	}
	return &state.AppSettings{
		Tags:            mockSettings.Tags,
		ISOs:            mockSettings.ISOs,
		VMBRs:           mockSettings.VMBRs,
		Storages:        mockSettings.Storages,
		EnabledStorages: mockSettings.EnabledStorages,
		Limits:          mockSettings.Limits,
	}
}

// SetSettings updates the mock application settings
func (s *MockStateManager) SetSettings(settings *state.AppSettings) error {
	if settings == nil {
		return fmt.Errorf("settings cannot be nil")
	}
	s.mu.Lock()
	// Convert state.AppSettings to MockAppSettings
	s.settings = &MockAppSettings{
		Tags:            settings.Tags,
		ISOs:            settings.ISOs,
		VMBRs:           settings.VMBRs,
		Storages:        settings.Storages,
		EnabledStorages: settings.EnabledStorages,
		Limits:          settings.Limits,
	}
	s.mu.Unlock()
	return nil
}

// SetSettingsWithoutSave updates the mock application settings in memory without saving
func (s *MockStateManager) SetSettingsWithoutSave(settings *state.AppSettings) {
	if settings == nil {
		return
	}
	s.mu.Lock()
	// Convert state.AppSettings to MockAppSettings
	s.settings = &MockAppSettings{
		Tags:            settings.Tags,
		ISOs:            settings.ISOs,
		VMBRs:           settings.VMBRs,
		Storages:        settings.Storages,
		EnabledStorages: settings.EnabledStorages,
		Limits:          settings.Limits,
	}
	s.mu.Unlock()
}

// GetTags returns the list of available tags
func (s *MockStateManager) GetTags() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings == nil || s.settings.Tags == nil {
		return []string{}
	}
	return s.settings.Tags
}

// GetISOs returns the list of available ISO files
func (s *MockStateManager) GetISOs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings == nil || s.settings.ISOs == nil {
		return []string{}
	}
	return s.settings.ISOs
}

// GetVMBRs returns the list of available network bridges
func (s *MockStateManager) GetVMBRs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings == nil || s.settings.VMBRs == nil {
		return []string{}
	}
	return s.settings.VMBRs
}

// GetLimits returns the resource limits
func (s *MockStateManager) GetLimits() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings == nil || s.settings.Limits == nil {
		return make(map[string]interface{})
	}
	return s.settings.Limits
}

// GetStorages returns the list of available storages
func (s *MockStateManager) GetStorages() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.settings == nil || s.settings.Storages == nil {
		return []string{}
	}
	return s.settings.Storages
}

// AddCSRFToken adds a new CSRF token with an expiry time
func (s *MockStateManager) AddCSRFToken(token string, expiry time.Time) error {
	s.securityMu.Lock()
	defer s.securityMu.Unlock()
	s.csrfTokens[token] = expiry
	return nil
}

// ValidateAndRemoveCSRFToken validates a CSRF token and removes it if valid
func (s *MockStateManager) ValidateAndRemoveCSRFToken(token string) bool {
	s.securityMu.Lock()
	defer s.securityMu.Unlock()

	if expiry, exists := s.csrfTokens[token]; exists {
		delete(s.csrfTokens, token)
		return time.Now().Before(expiry)
	}
	return false
}

// CleanExpiredCSRFTokens removes all expired CSRF tokens
func (s *MockStateManager) CleanExpiredCSRFTokens() {
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
func (s *MockStateManager) RecordLoginAttempt(ip string, timestamp time.Time) error {
	if ip == "" {
		return fmt.Errorf("IP address cannot be empty")
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
func (s *MockStateManager) GetLoginAttempts(ip string) ([]time.Time, error) {
	if ip == "" {
		return nil, fmt.Errorf("IP address cannot be empty")
	}

	s.securityMu.RLock()
	defer s.securityMu.RUnlock()

	// Return a copy of the slice to prevent external modification
	attempts := make([]time.Time, len(s.loginAttempts[ip]))
	copy(attempts, s.loginAttempts[ip])

	return attempts, nil
}

// CleanExpiredLoginAttempts removes login attempts older than the lockout period
func (s *MockStateManager) CleanExpiredLoginAttempts() {
	s.securityMu.Lock()
	defer s.securityMu.Unlock()

	now := time.Now()
	lockoutPeriod := 15 * time.Minute

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

// GetProxmoxStatus returns the current Proxmox connection status
func (s *MockStateManager) GetProxmoxStatus() (bool, string) {
	s.proxmoxMu.RLock()
	defer s.proxmoxMu.RUnlock()
	return s.proxmoxConnected, s.proxmoxError
}

// CheckProxmoxConnection checks the connection to the Proxmox server and updates the status
func (s *MockStateManager) CheckProxmoxConnection() bool {
	s.mu.RLock()
	client := s.proxmoxClient
	s.mu.RUnlock()

	if client == nil {
		s.updateProxmoxStatus(false, "Proxmox client not initialized")
		return false
	}

	// In the mock, we'll just return the current status
	// Tests can set this using WithProxmoxStatus
	connected, _ := s.GetProxmoxStatus()
	return connected
}

// updateProxmoxStatus updates the Proxmox connection status in a thread-safe way
func (s *MockStateManager) updateProxmoxStatus(connected bool, errorMsg string) {
	s.proxmoxMu.Lock()
	defer s.proxmoxMu.Unlock()

	s.proxmoxConnected = connected
	s.proxmoxError = errorMsg
}

// WithProxmoxStatus sets the Proxmox connection status for testing
func (s *MockStateManager) WithProxmoxStatus(connected bool, errorMsg string) *MockStateManager {
	s.updateProxmoxStatus(connected, errorMsg)
	return s
}
