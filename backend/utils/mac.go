package utils

import (
	"crypto/rand"
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ErrRandomMACAddress is returned when the cryptographic random source used
// to generate MAC addresses is unavailable. A failure of crypto/rand is a
// system-level error and must not silently fall back to a predictable source.
var ErrRandomMACAddress = errors.New("crypto/rand unavailable: cannot generate random MAC address")

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
// Returns ErrRandomMACAddress if crypto/rand is unavailable — a failure of the
// cryptographic random source is a system-level error and must not fall back to
// a predictable source (math/rand), which would produce guessable MACs.
func GenerateRandomMACAddress() (string, error) {
	// Use Proxmox-style prefix: BC:24:11 (locally administered, unicast)
	// BC in binary is 10111100 - second bit is 1 (locally administered), first bit is 1 (unicast)
	prefix := "BC:24:11"

	randomBytes := make([]byte, 3)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", fmt.Errorf("%w: %v", ErrRandomMACAddress, err)
	}

	return fmt.Sprintf("%s:%02X:%02X:%02X", prefix, randomBytes[0], randomBytes[1], randomBytes[2]), nil
}
