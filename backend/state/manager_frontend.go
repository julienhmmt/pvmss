package state

// GetFrontendPath returns the frontend path for static file serving.
func (s *appState) GetFrontendPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.frontendPath
}

// SetFrontendPath sets the frontend path for static file serving.
func (s *appState) SetFrontendPath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.frontendPath = path
}
