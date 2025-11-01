package handlers

import (
	"fmt"
	"html"
	"regexp"
	"strings"
)

// InputSanitizer provides methods to sanitize and validate user input
type InputSanitizer struct{}

// NewInputSanitizer creates a new input sanitizer
func NewInputSanitizer() *InputSanitizer {
	return &InputSanitizer{}
}

// SanitizeString removes dangerous characters and enforces length limits
func (s *InputSanitizer) SanitizeString(input string, maxLength int) string {
	// Trim whitespace
	cleaned := strings.TrimSpace(input)

	// HTML escape to prevent XSS
	cleaned = html.EscapeString(cleaned)

	// Enforce max length
	if maxLength > 0 && len(cleaned) > maxLength {
		cleaned = cleaned[:maxLength]
	}

	return cleaned
}

// SanitizeVMID validates VMID is numeric only
func (s *InputSanitizer) SanitizeVMID(vmid string) (string, error) {
	matched, err := regexp.MatchString(`^[0-9]+$`, vmid)
	if err != nil || !matched {
		return "", fmt.Errorf("invalid VMID format (must be numeric)")
	}
	return vmid, nil
}

// SanitizeNodeName validates node name (alphanumeric, dash, underscore only)
func (s *InputSanitizer) SanitizeNodeName(node string) (string, error) {
	matched, err := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, node)
	if err != nil || !matched {
		return "", fmt.Errorf("invalid node name format (alphanumeric, dash, underscore only)")
	}
	if len(node) > 64 {
		return "", fmt.Errorf("node name too long (max 64 characters)")
	}
	return node, nil
}

// SanitizeStorageName validates storage name
func (s *InputSanitizer) SanitizeStorageName(storage string) (string, error) {
	matched, err := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, storage)
	if err != nil || !matched {
		return "", fmt.Errorf("invalid storage name format (alphanumeric, dash, underscore only)")
	}
	if len(storage) > 64 {
		return "", fmt.Errorf("storage name too long (max 64 characters)")
	}
	return storage, nil
}

// SanitizeUsername validates username format
func (s *InputSanitizer) SanitizeUsername(username string) (string, error) {
	// Username can contain letters, numbers, dash, underscore, @, .
	matched, err := regexp.MatchString(`^[a-zA-Z0-9_@.-]+$`, username)
	if err != nil || !matched {
		return "", fmt.Errorf("invalid username format")
	}
	if len(username) > 100 {
		return "", fmt.Errorf("username too long (max 100 characters)")
	}
	return username, nil
}

// RemoveScriptTags removes <script> tags from input (defense in depth)
func (s *InputSanitizer) RemoveScriptTags(input string) string {
	re := regexp.MustCompile(`(?i)<script[^>]*>.*?</script>`)
	return re.ReplaceAllString(input, "")
}

// SanitizeFilename validates filename (no path traversal)
func (s *InputSanitizer) SanitizeFilename(filename string) (string, error) {
	// No path separators, no special characters
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return "", fmt.Errorf("invalid filename (path traversal detected)")
	}

	matched, err := regexp.MatchString(`^[a-zA-Z0-9_.-]+$`, filename)
	if err != nil || !matched {
		return "", fmt.Errorf("invalid filename format")
	}

	if len(filename) > 255 {
		return "", fmt.Errorf("filename too long (max 255 characters)")
	}

	return filename, nil
}
