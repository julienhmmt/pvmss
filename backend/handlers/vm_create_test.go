package handlers

import (
	"testing"

	"pvmss/utils"
)

// TestVMCreateHandlerMACValidationTests tests MAC address validation logic
func TestVMCreateHandlerMACValidationTests(t *testing.T) {
	// Test that all MAC-related functions work together correctly
	t.Run("Validation and Normalization Integration", func(t *testing.T) {
		testMACs := []string{
			"AA:BB:CC:DD:EE:FF", // Valid
			"aa:bb:cc:dd:ee:ff", // Valid lowercase
			"AA-BB-CC-DD-EE-FF", // Invalid hyphen
			"AABBCCDDEEFF",      // Invalid no separators
		}

		for _, mac := range testMACs {
			// Test validation
			isValid := utils.ValidateMACAddress(mac)
			normalized := utils.NormalizeMACAddress(mac)

			t.Logf("MAC: %s, Valid: %v, Normalized: %s", mac, isValid, normalized)

			// If original is valid, normalized should also be valid
			if isValid && !utils.ValidateMACAddress(normalized) {
				t.Errorf("Normalized MAC %q should be valid but validation failed", normalized)
			}

			// If original is invalid, we expect validation to fail
			// (Note: normalization might still work but validation should catch it)
		}
	})

	t.Run("Generated MACs are valid", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			generatedMAC := utils.GenerateRandomMACAddress()
			if !utils.ValidateMACAddress(generatedMAC) {
				t.Errorf("Generated MAC %q is not valid", generatedMAC)
			}
		}
	})
}
