package templates

import (
	"fmt"
	"reflect"
	"strings"
)

// toUpper converts a string to its uppercase equivalent.
func toUpper(s string) string {
	return strings.ToUpper(s)
}

// toLower converts a string to its lowercase equivalent.
func toLower(s string) string {
	return strings.ToLower(s)
}

// truncateString truncates a string to a specified length.
// If the string is longer than the given length, it is shortened and an ellipsis ("...") is appended.
// The function is UTF-8 safe.
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

// join concatenates the elements of a slice into a single string, separated by a given separator.
// It accepts a slice of any type, converting each element to its string representation.
func join(parts interface{}, sep string) string {
	v := reflect.ValueOf(parts)
	if v.Kind() != reflect.Slice {
		return "" // Return empty string if not a slice, for template safety.
	}

	var strParts []string
	for i := 0; i < v.Len(); i++ {
		strParts = append(strParts, fmt.Sprintf("%v", v.Index(i).Interface()))
	}

	return strings.Join(strParts, sep)
}
