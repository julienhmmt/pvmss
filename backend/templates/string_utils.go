package templates

import "strings"

// toUpper converts a string to uppercase
func toUpper(s string) string {
	return strings.ToUpper(s)
}

// toLower converts a string to lowercase
func toLower(s string) string {
	return strings.ToLower(s)
}

// truncateString truncates a string to the specified length and adds "..." if truncated
func truncateString(s string, length int) string {
	if length <= 0 {
		return ""
	}
	// UTF-8 safe truncation via runes
	r := []rune(s)
	if len(r) <= length {
		return s
	}
	return string(r[:length]) + "..."
}

// join joins a slice of strings with a separator
func join(parts []string, sep string) string {
	return strings.Join(parts, sep)
}
