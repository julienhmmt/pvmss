// Package constants defines application-wide constants for timeouts, limits, and configuration values.
package constants

// Default Values
const (
	// DefaultPort is the default HTTP server port
	DefaultPort = "50000"

	// DefaultLogLevel is the default logging level
	DefaultLogLevel = "info"

	// DefaultLanguage is the default application language
	DefaultLanguage = "en"

	// DefaultLoginRealm is the default Proxmox authentication realm
	DefaultLoginRealm = "pve"

	// AppVersion is the current application version
	AppVersion = "0.2.0"
)

// Validation Limits
const (
	// MaxUsernameLength is the maximum allowed username length
	MaxUsernameLength = 100

	// MaxPasswordLength is the maximum allowed password length
	MaxPasswordLength = 200

	// MinPasswordLength is the minimum required password length
	MinPasswordLength = 5

	// MaxVMNameLength is the maximum VM name length
	MaxVMNameLength = 100
)
