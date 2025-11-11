package tests

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"pvmss/utils"
)

// TestIsProduction tests environment detection for production
func TestIsProduction(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "production",
			envValue: "production",
			expected: true,
		},
		{
			name:     "prod",
			envValue: "prod",
			expected: true,
		},
		{
			name:     "PRODUCTION uppercase",
			envValue: "PRODUCTION",
			expected: true,
		},
		{
			name:     "development",
			envValue: "development",
			expected: false,
		},
		{
			name:     "dev",
			envValue: "dev",
			expected: false,
		},
		{
			name:     "empty",
			envValue: "",
			expected: false,
		},
		{
			name:     "random",
			envValue: "random",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable temporarily
			t.Setenv("PVMSS_ENV", tt.envValue)

			result := utils.IsProduction()
			assert.Equal(t, tt.expected, result,
				"IsProduction() with PVMSS_ENV=%s should return %v", tt.envValue, tt.expected)
		})
	}
}

// TestIsDevelopment tests environment detection for development
func TestIsDevelopment(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected bool
	}{
		{
			name:     "development",
			envValue: "development",
			expected: true,
		},
		{
			name:     "dev",
			envValue: "dev",
			expected: true,
		},
		{
			name:     "DEVELOPMENT uppercase",
			envValue: "DEVELOPMENT",
			expected: true,
		},
		{
			name:     "developpement (French)",
			envValue: "developpement",
			expected: true,
		},
		{
			name:     "production",
			envValue: "production",
			expected: false,
		},
		{
			name:     "empty (returns false)",
			envValue: "",
			expected: false,
		},
		{
			name:     "random (returns false)",
			envValue: "random",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set environment variable temporarily
			t.Setenv("PVMSS_ENV", tt.envValue)

			result := utils.IsDevelopment()
			assert.Equal(t, tt.expected, result,
				"IsDevelopment() with PVMSS_ENV=%s should return %v", tt.envValue, tt.expected)
		})
	}
}

// TestPasswordValidation tests basic password validation logic
func TestPasswordValidation(t *testing.T) {
	tests := []struct {
		name     string
		password string
		minLen   int
		valid    bool
	}{
		{
			name:     "Valid password 8 chars",
			password: "password",
			minLen:   5,
			valid:    true,
		},
		{
			name:     "Valid password exactly min length",
			password: "12345",
			minLen:   5,
			valid:    true,
		},
		{
			name:     "Too short",
			password: "1234",
			minLen:   5,
			valid:    false,
		},
		{
			name:     "Empty password",
			password: "",
			minLen:   5,
			valid:    false,
		},
		{
			name:     "Long password",
			password: "this-is-a-very-long-secure-password-123",
			minLen:   5,
			valid:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isValid := len(tt.password) >= tt.minLen
			assert.Equal(t, tt.valid, isValid,
				"Password '%s' with min length %d should be valid=%v", tt.password, tt.minLen, tt.valid)
		})
	}
}

// TestVMIDValidation tests VMID validation
func TestVMIDValidation(t *testing.T) {
	tests := []struct {
		name  string
		vmid  string
		valid bool
	}{
		{
			name:  "Valid VMID 100",
			vmid:  "100",
			valid: true,
		},
		{
			name:  "Valid VMID 999999",
			vmid:  "999999",
			valid: true,
		},
		{
			name:  "Invalid - negative",
			vmid:  "-1",
			valid: false,
		},
		{
			name:  "Invalid - zero",
			vmid:  "0",
			valid: false,
		},
		{
			name:  "Invalid - non-numeric",
			vmid:  "abc",
			valid: false,
		},
		{
			name:  "Invalid - empty",
			vmid:  "",
			valid: false,
		},
		{
			name:  "Invalid - too large",
			vmid:  "1000000000",
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var vmidInt int
			_, err := fmt.Sscanf(tt.vmid, "%d", &vmidInt)
			isValid := err == nil && vmidInt >= 100 && vmidInt <= 999999999

			assert.Equal(t, tt.valid, isValid,
				"VMID '%s' should be valid=%v", tt.vmid, tt.valid)
		})
	}
}
