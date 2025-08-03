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
	if len(s) <= length {
		return s
	}
	return s[:length] + "..."
}
