// Package cloudinit provides cloud-init validation and utility functions.
package cloudinit

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Validation constants
const (
	MaxYAMLSize    = 128 * 1024 // 128 KB max size for cloud-init YAML
	MaxLineCount   = 2000       // Maximum lines allowed
	MinYAMLSize    = 1          // Minimum size (at least 1 byte)
	CloudInitMagic = "#cloud-config"
)

// ValidationError represents a YAML validation error with details.
type ValidationError struct {
	Message string
	Line    int
	Column  int
}

func (e *ValidationError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d, column %d: %s", e.Line, e.Column, e.Message)
	}
	return e.Message
}

// ValidateCloudInitYAML validates a cloud-init YAML string.
// It checks for:
// - Valid YAML syntax
// - Size limits (max 128 KB)
// - Line count limits (max 2000 lines)
// - Does NOT validate cloud-init semantics (out of scope)
func ValidateCloudInitYAML(input string) error {
	// Check for empty input
	if strings.TrimSpace(input) == "" {
		return &ValidationError{Message: "YAML content cannot be empty"}
	}

	// Check size limits
	if len(input) > MaxYAMLSize {
		return &ValidationError{
			Message: fmt.Sprintf("YAML content exceeds maximum size of %d KB", MaxYAMLSize/1024),
		}
	}

	// Check line count
	lineCount := strings.Count(input, "\n") + 1
	if lineCount > MaxLineCount {
		return &ValidationError{
			Message: fmt.Sprintf("YAML content exceeds maximum of %d lines", MaxLineCount),
		}
	}

	// Parse YAML to check syntax
	var parsed interface{}
	if err := yaml.Unmarshal([]byte(input), &parsed); err != nil {
		// Try to extract line/column from yaml error
		var typeErr *yaml.TypeError
		if errors.As(err, &typeErr) {
			return &ValidationError{Message: fmt.Sprintf("YAML syntax error: %v", typeErr.Errors)}
		}
		return &ValidationError{Message: fmt.Sprintf("invalid YAML syntax: %v", err)}
	}

	// Check that parsed result is a map (cloud-init configs should be key-value)
	if parsed != nil {
		if _, ok := parsed.(map[string]interface{}); !ok {
			// Allow string or nil for simple configs
			if _, isString := parsed.(string); !isString && parsed != nil {
				return &ValidationError{Message: "cloud-init config should be a YAML mapping (key: value pairs)"}
			}
		}
	}

	return nil
}

// ValidateCloudInitYAMLStrict validates with stricter rules for cloud-init.
// It additionally checks that the YAML starts with #cloud-config comment.
func ValidateCloudInitYAMLStrict(input string) error {
	// First run basic validation
	if err := ValidateCloudInitYAML(input); err != nil {
		return err
	}

	// Check for cloud-init magic header
	trimmed := strings.TrimSpace(input)
	if !strings.HasPrefix(trimmed, CloudInitMagic) {
		return &ValidationError{
			Message: fmt.Sprintf("cloud-init config should start with '%s' comment", CloudInitMagic),
			Line:    1,
		}
	}

	return nil
}

// IsValidYAML is a simple helper that returns true if the input is valid YAML.
func IsValidYAML(input string) bool {
	return ValidateCloudInitYAML(input) == nil
}

// SanitizeYAML removes potentially dangerous content from YAML.
// Currently just trims whitespace; future enhancements could include
// removing sensitive patterns.
func SanitizeYAML(input string) string {
	// Trim leading/trailing whitespace
	return strings.TrimSpace(input)
}

// ParseCloudInitYAML parses YAML and returns a map representation.
// Returns nil if the input is not a valid map.
func ParseCloudInitYAML(input string) (map[string]interface{}, error) {
	if err := ValidateCloudInitYAML(input); err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := yaml.Unmarshal([]byte(input), &result); err != nil {
		return nil, &ValidationError{Message: fmt.Sprintf("failed to parse YAML: %v", err)}
	}

	return result, nil
}
