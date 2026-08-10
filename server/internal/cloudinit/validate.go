// Package cloudinit contains pure cloud-init validation shared by VM and catalog features.
package cloudinit

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// MaxSnippetSize is the maximum custom snippet size in bytes.
const MaxSnippetSize = 16 * 1024

var (
	// ErrSnippetTooLarge reports content over MaxSnippetSize.
	ErrSnippetTooLarge = errors.New("cloud-init snippet too large")
	// ErrSnippetPrefix reports a non-empty document without the cloud-init marker.
	ErrSnippetPrefix = errors.New("cloud-init snippet must start with #cloud-config")
	// ErrSnippetInvalidUTF8 reports malformed UTF-8 content.
	ErrSnippetInvalidUTF8 = errors.New("cloud-init snippet is not valid UTF-8")
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

	if !strings.HasPrefix(strings.TrimLeft(content, " \t\r\n"), "#cloud-config") {
		return ErrSnippetPrefix
	}

	return nil
}
