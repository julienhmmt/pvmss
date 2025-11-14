package handlers

import (
	"testing"

	"pvmss/utils"
)

// TestVMActionsHandlerMACValidationTests tests MAC address validation in VM resource updates
func TestVMActionsHandlerMACValidationTests(t *testing.T) {
	// Test that all MAC-related functions work together correctly for VM actions
	t.Run("MAC Validation for VM Updates", func(t *testing.T) {
		testMACs := []struct {
			mac         string
			shouldPass  bool
			description string
		}{
			{"AA:BB:CC:DD:EE:FF", true, "Valid colon format"},
			{"aa:bb:cc:dd:ee:ff", true, "Valid lowercase"},
			{"Aa:Bb:Cc:Dd:Ee:Ff", true, "Valid mixed case"},
			{"AA-BB-CC-DD-EE-FF", false, "Invalid hyphen format"},
			{"AABBCCDDEEFF", false, "Invalid no separators"},
			{"AA:BB:CC:DD:EE", false, "Invalid too short"},
			{"AA:BB:CC:DD:EE:FF:GG", false, "Invalid too long"},
			{"AA:BB:CC:DD:EE:@@", false, "Invalid special characters"},
		}

		for _, tc := range testMACs {
			t.Run(tc.description, func(t *testing.T) {
				// Test validation
				isValid := utils.ValidateMACAddress(tc.mac)
				normalized := utils.NormalizeMACAddress(tc.mac)

				if tc.shouldPass && !isValid {
					t.Errorf("Expected MAC %q to be valid, but validation failed", tc.mac)
				}
				if !tc.shouldPass && isValid {
					t.Errorf("Expected MAC %q to be invalid, but validation passed", tc.mac)
				}

				// Test normalization behavior
				if tc.mac != "" {
					if isValid && !utils.ValidateMACAddress(normalized) {
						t.Errorf("Normalized MAC %q should be valid but validation failed", normalized)
					}
				}
			})
		}
	})

	t.Run("Generated MACs are valid for VM actions", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			generatedMAC := utils.GenerateRandomMACAddress()
			if !utils.ValidateMACAddress(generatedMAC) {
				t.Errorf("Generated MAC %q is not valid for VM actions", generatedMAC)
			}

			// Verify format is correct
			if !utils.ValidateMACAddress(generatedMAC) {
				t.Errorf("Generated MAC %q doesn't have correct format", generatedMAC)
			}
		}
	})
}
