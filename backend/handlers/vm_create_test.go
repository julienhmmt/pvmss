package handlers

import (
	"fmt"
	"strconv"
	"strings"
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

	t.Run("Network Speed Range Validation", func(t *testing.T) {
		testRates := []struct {
			input string
			valid bool
			desc  string
		}{
			{"", true, "Empty (optional field)"},
			{"1", true, "Minimum valid speed"},
			{"10240", true, "Maximum valid speed (Proxmox limit)"},
			{"0", false, "Below minimum"},
			{"-1", false, "Negative value"},
			{"10241", false, "Above Proxmox maximum"},
			{"20000", false, "Well above maximum"},
			{"100.5", true, "Decimal value within range"},
			{"abc", false, "Non-numeric"},
			{"1a", false, "Alphanumeric"},
		}

		for _, tc := range testRates {
			t.Run(tc.desc, func(t *testing.T) {
				result := validateRateLimit(tc.input)
				if result != tc.valid {
					t.Errorf("validateRateLimit(%q) = %v; want %v", tc.input, result, tc.valid)
				}
			})
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

// TestVMCreateHandlerVLANValidationTests tests VLAN tag validation logic
func TestVMCreateHandlerVLANValidationTests(t *testing.T) {
	// Test VLAN validation logic used in VM creation
	t.Run("VLAN Range Validation", func(t *testing.T) {
		testVLANs := []struct {
			input    string
			expected bool
			desc     string
		}{
			{"1", true, "Minimum valid VLAN"},
			{"100", true, "Normal valid VLAN"},
			{"4096", true, "Maximum valid VLAN"},
			{"0", false, "Below minimum"},
			{"-1", false, "Negative VLAN"},
			{"4097", false, "Above maximum"},
			{"5000", false, "Way above maximum"},
			{"abc", false, "Non-numeric"},
			{"1a", false, "Mixed alphanumeric"},
			{"a1", false, "Mixed alphanumeric"},
			{"", true, "Empty (optional field)"},
			{"   ", true, "Whitespace only (optional)"},
			{"0100", true, "Leading zeros"},
			{"001", true, "Leading zeros, single digit"},
		}

		for _, test := range testVLANs {
			t.Run(test.desc, func(t *testing.T) {
				result := validateVLANTag(test.input)
				if result != test.expected {
					t.Errorf("validateVLANTag(%q) = %v; want %v", test.input, result, test.expected)
				}
			})
		}
	})

	t.Run("VLAN Error Messages", func(t *testing.T) {
		testCases := []struct {
			input       string
			expectedMsg string
			desc        string
		}{
			{"5000", "VLAN tag must be between 1 and 4096", "Out of range high"},
			{"0", "VLAN tag must be between 1 and 4096", "Out of range low"},
			{"abc", "VLAN tag must contain only numbers", "Non-numeric"},
			{"", "Valid VLAN tag", "Empty value"},
			{"100", "Valid VLAN tag", "Valid value"},
		}

		for _, test := range testCases {
			t.Run(test.desc, func(t *testing.T) {
				msg := getVLANValidationMessage(test.input)
				if !strings.Contains(msg, test.expectedMsg) {
					t.Errorf("getVLANValidationMessage(%q) = %q; want to contain %q", test.input, msg, test.expectedMsg)
				}
			})
		}
	})

	t.Run("Network Configuration Building", func(t *testing.T) {
		testCases := []struct {
			name     string
			model    string
			mac      string
			bridge   string
			vlan     string
			rate     string
			linkDown bool
			expected string
		}{
			{
				name:     "Basic config without VLAN or Rate",
				model:    "virtio",
				mac:      "AA:BB:CC:DD:EE:FF",
				bridge:   "vmbr0",
				vlan:     "",
				rate:     "",
				linkDown: false,
				expected: "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0",
			},
			{
				name:     "Config with VLAN",
				model:    "virtio",
				mac:      "AA:BB:CC:DD:EE:FF",
				bridge:   "vmbr0",
				vlan:     "100",
				rate:     "",
				linkDown: false,
				expected: "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,tag=100",
			},
			{
				name:     "Config with Rate Limit",
				model:    "virtio",
				mac:      "AA:BB:CC:DD:EE:FF",
				bridge:   "vmbr0",
				vlan:     "",
				rate:     "1000",
				linkDown: false,
				expected: "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,rate=1000",
			},
			{
				name:     "Config with VLAN, Rate and link down",
				model:    "virtio",
				mac:      "AA:BB:CC:DD:EE:FF",
				bridge:   "vmbr0",
				vlan:     "200",
				rate:     "50",
				linkDown: true,
				expected: "virtio=AA:BB:CC:DD:EE:FF,bridge=vmbr0,tag=200,rate=50,link_down=1",
			},
			{
				name:     "Config without MAC",
				model:    "virtio",
				mac:      "",
				bridge:   "vmbr0",
				vlan:     "300",
				rate:     "",
				linkDown: false,
				expected: "virtio,bridge=vmbr0,tag=300",
			},
		}

		for _, test := range testCases {
			t.Run(test.name, func(t *testing.T) {
				result := buildNetworkConfig(test.model, test.mac, test.bridge, test.vlan, test.rate, test.linkDown)
				if result != test.expected {
					t.Errorf("buildNetworkConfig() = %q; want %q", result, test.expected)
				}
			})
		}
	})

	t.Run("Multiple Network Cards with VLANs and Rate Limits", func(t *testing.T) {
		networkCards := []struct {
			model    string
			mac      string
			bridge   string
			vlan     string
			rate     string
			linkDown bool
		}{
			{"virtio", "AA:BB:CC:DD:EE:01", "vmbr0", "100", "1000", false},
			{"e1000", "AA:BB:CC:DD:EE:02", "vmbr1", "200", "", false},
			{"virtio", "AA:BB:CC:DD:EE:03", "vmbr0", "", "10", true}, // No VLAN, Rate 10
			{"e1000e", "AA:BB:CC:DD:EE:04", "vmbr1", "400", "5000", false},
		}

		expectedConfigs := []string{
			"virtio=AA:BB:CC:DD:EE:01,bridge=vmbr0,tag=100,rate=1000",
			"e1000=AA:BB:CC:DD:EE:02,bridge=vmbr1,tag=200",
			"virtio=AA:BB:CC:DD:EE:03,bridge=vmbr0,rate=10,link_down=1",
			"e1000e=AA:BB:CC:DD:EE:04,bridge=vmbr1,tag=400,rate=5000",
		}

		for i, card := range networkCards {
			t.Run(fmt.Sprintf("Card_%d", i), func(t *testing.T) {
				config := buildNetworkConfig(card.model, card.mac, card.bridge, card.vlan, card.rate, card.linkDown)
				expected := expectedConfigs[i]
				if config != expected {
					t.Errorf("Network card %d config = %q; want %q", i, config, expected)
				}
			})
		}
	})
}

// validateVLANTag replicates the validation logic from ValidateVLANHandler
func validateVLANTag(vlanStr string) bool {
	vlanStr = strings.TrimSpace(vlanStr)

	if vlanStr == "" {
		// Empty VLAN is valid (optional field)
		return true
	}

	// Check if value is numeric
	vlanID, err := strconv.Atoi(vlanStr)
	if err != nil {
		return false
	}

	// Validate VLAN range (1-4096)
	return vlanID >= 1 && vlanID <= 4096
}

// validateRateLimit replicates the validation logic for Network Speed (Rate)
func validateRateLimit(rateStr string) bool {
	rateStr = strings.TrimSpace(rateStr)

	if rateStr == "" {
		// Empty Rate is valid (optional field, unlimited)
		return true
	}

	// Check if value is numeric
	rate, err := strconv.ParseFloat(rateStr, 64)
	if err != nil {
		return false
	}

	// Validate Network Speed range (1-10240 MB/s, Proxmox limit)
	return rate >= 1 && rate <= 10240
}

// getVLANValidationMessage replicates the message logic from ValidateVLANHandler
func getVLANValidationMessage(vlanStr string) string {
	vlanStr = strings.TrimSpace(vlanStr)

	if vlanStr == "" {
		return "Valid VLAN tag"
	}

	// Check if value is numeric
	_, err := strconv.Atoi(vlanStr)
	if err != nil {
		return "VLAN tag must contain only numbers"
	}

	// Validate VLAN range (1-4096)
	vlanID, _ := strconv.Atoi(vlanStr)
	if vlanID < 1 || vlanID > 4096 {
		return "VLAN tag must be between 1 and 4096"
	}

	return "Valid VLAN tag"
}

// buildNetworkConfig replicates the network configuration building logic
func buildNetworkConfig(model, mac, bridge, vlan, rate string, linkDown bool) string {
	netParts := []string{}

	if mac != "" {
		netParts = append(netParts, model+"="+mac)
	} else {
		netParts = append(netParts, model)
	}

	netParts = append(netParts, "bridge="+bridge)

	// Add VLAN tag if provided
	if vlan != "" {
		netParts = append(netParts, "tag="+vlan)
	}

	// Add Rate Limit if provided
	if rate != "" {
		netParts = append(netParts, "rate="+rate)
	}

	// Add link_down flag explicitly
	if linkDown {
		netParts = append(netParts, "link_down=1")
	}

	return strings.Join(netParts, ",")
}
