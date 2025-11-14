package utils

import (
	"regexp"
	"strings"
)

// MAC address validation functions
var (
	// MAC address regex pattern - strict format with colons only
	MACAddressRegex = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$`)
)

// ValidateMACAddress checks if a MAC address is in valid format
func ValidateMACAddress(mac string) bool {
	if mac == "" {
		return true // Empty is valid (will be auto-generated)
	}
	return MACAddressRegex.MatchString(mac)
}

// NormalizeMACAddress converts MAC address to Proxmox format (uppercase with colons)
func NormalizeMACAddress(mac string) string {
	if mac == "" {
		return ""
	}
	// Remove any existing separators and convert to uppercase
	clean := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(mac, ":", ""), "-", ""))
	// Add colons every 2 characters
	if len(clean) == 12 {
		return clean[0:2] + ":" + clean[2:4] + ":" + clean[4:6] + ":" + clean[6:8] + ":" + clean[8:10] + ":" + clean[10:12]
	}
	return mac // Return original if something went wrong
}
