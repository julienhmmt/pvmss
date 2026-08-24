// Package cloudinit contains pure cloud-init validation shared by VM and catalog features.
package cloudinit

import (
	"errors"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
)

// MaxSnippetSize is the maximum custom snippet size in bytes.
const MaxSnippetSize = 16 * 1024

var (
	// ErrSnippetTooLarge reports content over MaxSnippetSize.
	ErrSnippetTooLarge = errors.New("cloud-init snippet too large")
	// ErrSnippetPrefix reports a non-empty document without the cloud-init marker.
	ErrSnippetPrefix = errors.New("cloud-init snippet must start with #cloud-config")
	// ErrSnippetInvalidUTF8 reports malformed UTF-8 content.
	ErrSnippetInvalidUTF8 = errors.New("cloud-init snippet is not valid utf-8")
	// ErrSnippetInvalidYAML reports a malformed YAML root or a non-dict root.
	ErrSnippetInvalidYAML = errors.New("cloud-init snippet is not valid yaml")
)

// Validate checks the bounded marker required for a custom cloud-init document.
func Validate(content string) error {
	if len(content) > MaxSnippetSize {
		return ErrSnippetTooLarge
	}

	if !utf8.ValidString(content) {
		return ErrSnippetInvalidUTF8
	}

	if content == "" {
		return nil
	}

	trimmed := strings.TrimLeft(content, " \t\r\n")
	if !strings.HasPrefix(trimmed, "#cloud-config") {
		return ErrSnippetPrefix
	}

	// A valid #cloud-config document must parse as a YAML mapping (dict).
	// Malformed YAML and non-mapping roots are rejected before they reach
	// Proxmox. Empty/comment-only documents are accepted as an empty mapping.
	// Future directive allow-listing can be added here with a policy flag.
	var root any
	if err := yaml.Unmarshal([]byte(trimmed), &root); err != nil {
		return ErrSnippetInvalidYAML
	}

	if root != nil && !isYAMLMap(root) {
		return ErrSnippetInvalidYAML
	}

	return nil
}

func isYAMLMap(v any) bool {
	switch v.(type) {
	case map[any]any, map[string]any:
		return true
	default:
		return false
	}
}
