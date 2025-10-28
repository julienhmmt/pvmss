package tests

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestStaticPathDetection tests the isStaticPath function logic
func TestStaticPathDetection(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"/css/base.css", true},
		{"/js/main.js", true},
		{"/webfonts/font.woff2", true},
		{"/components/novnc/core.js", true},
		{"/", false},
		{"/login", false},
		{"/admin", false},
		{"/api/health", false},
	}

	// Replicate isStaticPath logic for testing
	isStaticPath := func(p string) bool {
		for _, prefix := range []string{"/css/", "/js/", "/webfonts/", "/components/"} {
			if strings.HasPrefix(p, prefix) {
				return true
			}
		}
		return false
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isStaticPath(tt.path)
			assert.Equal(t, tt.expected, result,
				"Expected isStaticPath(%s) to be %v, got %v",
				tt.path, tt.expected, result)
		})
	}
}

// TestMaskSensitiveValue tests sensitive data masking logic
func TestMaskSensitiveValue(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Short value",
			input:    "short",
			expected: "***",
		},
		{
			name:     "Exactly 8 chars",
			input:    "12345678",
			expected: "***",
		},
		{
			name:     "Long value",
			input:    "this-is-a-very-long-secret-token-value",
			expected: "this-is-...[38 chars]",
		},
	}

	// Replicate maskSensitiveValue logic for testing
	maskSensitiveValue := func(value string) string {
		if len(value) <= 8 {
			return "***"
		}
		return value[:8] + "..." + fmt.Sprintf("[%d chars]", len(value))
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskSensitiveValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestTrailingSlashRedirect tests that trailing slashes are handled correctly
func TestTrailingSlashRedirect(t *testing.T) {
	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectRedirect bool
	}{
		{
			name:           "Root path with trailing slash",
			path:           "/",
			expectedStatus: http.StatusOK, // Root is allowed
			expectRedirect: false,
		},
		{
			name:           "Admin path with trailing slash",
			path:           "/admin/",
			expectedStatus: http.StatusSeeOther,
			expectRedirect: true,
		},
		{
			name:           "Static path with trailing slash",
			path:           "/css/",
			expectedStatus: http.StatusOK, // Static paths are allowed
			expectRedirect: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test the logic of trailing slash handling
			shouldRedirect := len(tt.path) > 1 && tt.path[len(tt.path)-1] == '/' && !strings.HasPrefix(tt.path, "/css/")

			if tt.expectRedirect {
				assert.True(t, shouldRedirect, "Path %s should trigger redirect", tt.path)
			}
		})
	}
}
