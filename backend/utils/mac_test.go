package utils

import (
	"regexp"
	"strings"
	"testing"
)

// TestValidateMACAddress tests the MAC address validation function
func TestValidateMACAddress(t *testing.T) {
	tests := []struct {
		name     string
		mac      string
		expected bool
	}{
		// Valid MAC addresses (strict colon format)
		{
			name:     "Valid lowercase MAC",
			mac:      "aa:bb:cc:dd:ee:ff",
			expected: true,
		},
		{
			name:     "Valid uppercase MAC",
			mac:      "AA:BB:CC:DD:EE:FF",
			expected: true,
		},
		{
			name:     "Valid mixed case MAC",
			mac:      "Aa:Bb:Cc:Dd:Ee:Ff",
			expected: true,
		},
		{
			name:     "Valid numbers MAC",
			mac:      "01:23:45:67:89:AB",
			expected: true,
		},
		// Invalid MAC addresses
		{
			name:     "Invalid hyphen format",
			mac:      "AA-BB-CC-DD-EE-FF",
			expected: false,
		},
		{
			name:     "Invalid mixed separators",
			mac:      "AA:BB-CC:DD-EE:FF",
			expected: false,
		},
		{
			name:     "Invalid too short",
			mac:      "AA:BB:CC:DD:EE",
			expected: false,
		},
		{
			name:     "Invalid too long",
			mac:      "AA:BB:CC:DD:EE:FF:GG",
			expected: false,
		},
		{
			name:     "Invalid single character groups",
			mac:      "A:B:C:D:E:F",
			expected: false,
		},
		{
			name:     "Invalid three character groups",
			mac:      "AAA:BBB:CCC:DDD:EEE:FFF",
			expected: false,
		},
		{
			name:     "Invalid missing colons",
			mac:      "AABBCCDDEEFF",
			expected: false,
		},
		{
			name:     "Invalid extra colons",
			mac:      "AA::BB:CC:DD:EE:FF",
			expected: false,
		},
		{
			name:     "Invalid characters",
			mac:      "GG:HH:II:JJ:KK:LL",
			expected: false,
		},
		{
			name:     "Invalid special characters",
			mac:      "AA:BB:CC:DD:EE:@@",
			expected: false,
		},
		// Edge cases
		{
			name:     "Empty string (should be valid - auto-generated)",
			mac:      "",
			expected: true,
		},
		{
			name:     "Only spaces",
			mac:      "   ",
			expected: false,
		},
		{
			name:     "Leading/trailing spaces",
			mac:      " AA:BB:CC:DD:EE:FF ",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ValidateMACAddress(tt.mac)
			if result != tt.expected {
				t.Errorf("ValidateMACAddress(%q) = %v, want %v", tt.mac, result, tt.expected)
			}
		})
	}
}

// TestNormalizeMACAddress tests the MAC address normalization function
func TestNormalizeMACAddress(t *testing.T) {
	tests := []struct {
		name     string
		mac      string
		expected string
	}{
		// Valid formats (should remain unchanged)
		{
			name:     "Valid lowercase MAC",
			mac:      "aa:bb:cc:dd:ee:ff",
			expected: "AA:BB:CC:DD:EE:FF",
		},
		{
			name:     "Valid uppercase MAC",
			mac:      "AA:BB:CC:DD:EE:FF",
			expected: "AA:BB:CC:DD:EE:FF",
		},
		// Hyphen format (should be converted to colon format)
		{
			name:     "Hyphen format lowercase",
			mac:      "aa-bb-cc-dd-ee-ff",
			expected: "AA:BB:CC:DD:EE:FF",
		},
		{
			name:     "Hyphen format uppercase",
			mac:      "AA-BB-CC-DD-EE-FF",
			expected: "AA:BB:CC:DD:EE:FF",
		},
		// Mixed separators
		{
			name:     "Mixed separators",
			mac:      "aa:bb-cc:dd-ee:ff",
			expected: "AA:BB:CC:DD:EE:FF",
		},
		// No separators (should add colons)
		{
			name:     "No separators lowercase",
			mac:      "aabbccddeeff",
			expected: "AA:BB:CC:DD:EE:FF",
		},
		{
			name:     "No separators uppercase",
			mac:      "AABBCCDDEEFF",
			expected: "AA:BB:CC:DD:EE:FF",
		},
		{
			name:     "No separators mixed case",
			mac:      "AaBbCcDdEeFf",
			expected: "AA:BB:CC:DD:EE:FF",
		},
		// Edge cases
		{
			name:     "Empty string",
			mac:      "",
			expected: "",
		},
		{
			name:     "Invalid length (too short)",
			mac:      "AABBCC",
			expected: "AABBCC",
		},
		{
			name:     "Invalid length (too long)",
			mac:      "AABBCCDDEEFFGG",
			expected: "AABBCCDDEEFFGG",
		},
		{
			name:     "Invalid characters",
			mac:      "GGHHIIJJKKLL",
			expected: "GGHHIIJJKKLL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NormalizeMACAddress(tt.mac)
			if result != tt.expected {
				t.Errorf("NormalizeMACAddress(%q) = %v, want %v", tt.mac, result, tt.expected)
			}
		})
	}
}

// TestMACAddressRegex tests the regex pattern directly
func TestMACAddressRegex(t *testing.T) {
	tests := []struct {
		name     string
		mac      string
		expected bool
	}{
		{
			name:     "Valid MAC with colons",
			mac:      "AA:BB:CC:DD:EE:FF",
			expected: true,
		},
		{
			name:     "Invalid MAC with hyphens",
			mac:      "AA-BB-CC-DD-EE-FF",
			expected: false,
		},
		{
			name:     "Invalid MAC without separators",
			mac:      "AABBCCDDEEFF",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := MACAddressRegex.MatchString(tt.mac)
			if result != tt.expected {
				t.Errorf("MACAddressRegex.MatchString(%q) = %v, want %v", tt.mac, result, tt.expected)
			}
		})
	}
}

// TestGenerateRandomMACAddress tests the random MAC generation
func TestGenerateRandomMACAddress(t *testing.T) {
	// Generate multiple MACs to test consistency
	macs := make(map[string]bool)

	for i := 0; i < 100; i++ {
		mac := GenerateRandomMACAddress()

		// Test that generated MAC is valid
		if !ValidateMACAddress(mac) {
			t.Errorf("Generated MAC %q is not valid", mac)
		}

		// Test that it has the correct prefix
		if !strings.HasPrefix(mac, "BC:24:11:") {
			t.Errorf("Generated MAC %q doesn't have correct prefix BC:24:11:", mac)
		}

		// Test that it's properly formatted
		parts := strings.Split(mac, ":")
		if len(parts) != 6 {
			t.Errorf("Generated MAC %q doesn't have 6 parts", mac)
		}

		// Test that each part is valid hex
		for _, part := range parts {
			if len(part) != 2 {
				t.Errorf("Generated MAC part %q is not 2 characters", part)
			}
			if !regexp.MustCompile(`^[0-9A-F]{2}$`).MatchString(part) {
				t.Errorf("Generated MAC part %q is not valid hex", part)
			}
		}

		// Track uniqueness (though collisions are possible with random generation)
		macs[mac] = true
	}

	// Test that we got some variety (at least 90 unique MACs out of 100)
	if len(macs) < 90 {
		t.Errorf("Generated %d unique MACs out of 100, expected at least 90", len(macs))
	}
}

// BenchmarkValidateMACAddress benchmarks the validation function
func BenchmarkValidateMACAddress(b *testing.B) {
	mac := "AA:BB:CC:DD:EE:FF"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ValidateMACAddress(mac)
	}
}

// BenchmarkNormalizeMACAddress benchmarks the normalization function
func BenchmarkNormalizeMACAddress(b *testing.B) {
	mac := "aa-bb-cc-dd-ee-ff"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NormalizeMACAddress(mac)
	}
}

// BenchmarkGenerateRandomMACAddress benchmarks the generation function
func BenchmarkGenerateRandomMACAddress(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		GenerateRandomMACAddress()
	}
}
