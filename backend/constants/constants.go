// Package constants defines application-wide constants for timeouts, limits, and configuration values.
// This centralizes magic numbers and makes them easier to tune and maintain.
//
// Constants are organized into domain-specific files for better maintainability:
//
//   - http.go: HTTP server configuration, form limits, and transport settings
//   - proxmox.go: Proxmox API configuration, console sessions, and cache settings
//   - security.go: CSRF tokens, rate limiting, and session keys
//   - context.go: Context timeout values
//   - defaults.go: Default values and validation limits
package constants
