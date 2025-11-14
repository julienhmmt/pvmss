package utils

import (
	"crypto/rand"
	"fmt"
	mathrand "math/rand"
	"regexp"
	"strings"
)

// MAC address validation functions
var (
	// MAC address regex pattern - strict format with colons only
	MACAddressRegex = regexp.MustCompile(`^([0-9A-Fa-f]{2}:){5}([0-9A-Fa-f]{2})$`)
	// Hex validation for normalization
	hexOnlyRegex = regexp.MustCompile(`^[0-9A-Fa-f]{12}$`)
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

	// Only normalize if it's a valid 12-character hex string
	if len(clean) == 12 && hexOnlyRegex.MatchString(clean) {
		return clean[0:2] + ":" + clean[2:4] + ":" + clean[4:6] + ":" + clean[6:8] + ":" + clean[8:10] + ":" + clean[10:12]
	}

	return mac // Return original if not valid hex or wrong length
}

// GenerateRandomMACAddress generates a random locally-administered MAC address
// Uses the standard format with the second bit of the first byte set to 1 (locally administered)
func GenerateRandomMACAddress() string {
	// Use Proxmox-style prefix: BC:24:11 (locally administered, unicast)
	// BC in binary is 10111100 - second bit is 1 (locally administered), first bit is 1 (unicast)
	prefix := "BC:24:11"

	// Generate 3 random bytes using crypto/rand for better randomness
	randomBytes := make([]byte, 3)
	_, err := rand.Read(randomBytes)
	if err != nil {
		// Fallback to time-based if crypto/rand fails (shouldn't happen in normal conditions)
		return fmt.Sprintf("%s:%02X:%02X:%02X", prefix,
			uint8(mathrand.Intn(256)), uint8(mathrand.Intn(256)), uint8(mathrand.Intn(256)))
	}

	return fmt.Sprintf("%s:%02X:%02X:%02X", prefix, randomBytes[0], randomBytes[1], randomBytes[2])
}
